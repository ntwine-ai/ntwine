package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPServerConfig struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Transport string            `json:"transport"`
	URL       string            `json:"url"`
}

type MCPConnection struct {
	config    MCPServerConfig
	client    *mcpclient.Client
	transport transport.Interface
	tools     []mcp.Tool
}

type MCPHub struct {
	mu          sync.RWMutex
	connections map[string]*MCPConnection
	registry    *Registry
}

func NewMCPHub(registry *Registry) *MCPHub {
	return &MCPHub{
		connections: make(map[string]*MCPConnection),
		registry:    registry,
	}
}

func (hub *MCPHub) Connect(ctx context.Context, config MCPServerConfig) error {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if _, exists := hub.connections[config.Name]; exists {
		return fmt.Errorf("server %s already connected", config.Name)
	}

	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	var tp transport.Interface
	switch config.Transport {
	case "sse":
		sseTp, err := transport.NewSSE(config.URL)
		if err != nil {
			return fmt.Errorf("failed to create SSE transport for %s: %w", config.Name, err)
		}
		tp = sseTp
	case "http", "streamable-http":
		httpTp, err := transport.NewStreamableHTTP(config.URL)
		if err != nil {
			return fmt.Errorf("failed to create HTTP transport for %s: %w", config.Name, err)
		}
		tp = httpTp
	default:
		stdioTp := transport.NewStdio(config.Command, env, config.Args...)
		if err := stdioTp.Start(ctx); err != nil {
			return fmt.Errorf("failed to start stdio transport for %s: %w", config.Name, err)
		}
		tp = stdioTp
	}

	c := mcpclient.NewClient(tp)

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "ntwine",
		Version: "0.1.0",
	}

	_, err := c.Initialize(initCtx, initReq)
	if err != nil {
		tp.Close()
		return fmt.Errorf("failed to initialize MCP server %s: %w", config.Name, err)
	}

	toolsCtx, toolsCancel := context.WithTimeout(ctx, 15*time.Second)
	defer toolsCancel()

	toolsResult, err := c.ListTools(toolsCtx, mcp.ListToolsRequest{})
	if err != nil {
		tp.Close()
		return fmt.Errorf("failed to list tools from %s: %w", config.Name, err)
	}

	conn := &MCPConnection{
		config:    config,
		client:    c,
		transport: tp,
		tools:     toolsResult.Tools,
	}

	hub.connections[config.Name] = conn

	for _, tool := range toolsResult.Tools {
		hub.registerMCPTool(config.Name, tool, c)
	}

	log.Printf("[MCP] connected to %s: %d tools", config.Name, len(toolsResult.Tools))
	return nil
}

func (hub *MCPHub) registerMCPTool(serverName string, tool mcp.Tool, c *mcpclient.Client) {
	toolName := fmt.Sprintf("mcp:%s:%s", serverName, tool.Name)

	var paramSchema map[string]interface{}
	if tool.InputSchema.Properties != nil {
		raw, err := json.Marshal(tool.InputSchema)
		if err == nil {
			json.Unmarshal(raw, &paramSchema)
		}
	}
	if paramSchema == nil {
		paramSchema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	desc := tool.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool from %s", serverName)
	}

	hub.registry.Register(ToolDef{
		Name:             toolName,
		Description:      desc,
		ModelDescription: fmt.Sprintf("[%s] %s", serverName, desc),
		Parameters:       paramSchema,
		Source:           "mcp",
		Owner:            serverName,
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		var argsMap map[string]interface{}
		if err := json.Unmarshal(args, &argsMap); err != nil {
			argsMap = make(map[string]interface{})
		}

		callReq := mcp.CallToolRequest{}
		callReq.Params.Name = tool.Name
		callReq.Params.Arguments = argsMap

		result, err := c.CallTool(callCtx, callReq)
		if err != nil {
			return ToolResult{
				Output:  fmt.Sprintf("MCP tool error: %s", err.Error()),
				IsError: true,
			}, nil
		}

		var output strings.Builder
		for _, content := range result.Content {
			if textContent, ok := content.(mcp.TextContent); ok {
				output.WriteString(textContent.Text)
				output.WriteString("\n")
			}
		}

		out := strings.TrimSpace(output.String())
		if len(out) > 10000 {
			out = out[:10000] + "\n... (truncated)"
		}

		return ToolResult{
			Output:  out,
			IsError: result.IsError,
		}, nil
	})
}

func (hub *MCPHub) Disconnect(name string) error {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	conn, exists := hub.connections[name]
	if !exists {
		return fmt.Errorf("no connection: %s", name)
	}

	for _, tool := range conn.tools {
		toolName := fmt.Sprintf("mcp:%s:%s", name, tool.Name)
		hub.registry.Remove(toolName)
	}

	conn.transport.Close()
	delete(hub.connections, name)

	log.Printf("[MCP] disconnected from %s", name)
	return nil
}

func (hub *MCPHub) DisconnectAll() {
	hub.mu.Lock()
	names := make([]string, 0, len(hub.connections))
	for name := range hub.connections {
		names = append(names, name)
	}
	hub.mu.Unlock()

	for _, name := range names {
		hub.Disconnect(name)
	}
}

func (hub *MCPHub) ListServers() []MCPServerConfig {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	configs := make([]MCPServerConfig, 0, len(hub.connections))
	for _, conn := range hub.connections {
		configs = append(configs, conn.config)
	}
	return configs
}

func (hub *MCPHub) ServerTools(name string) []string {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	conn, exists := hub.connections[name]
	if !exists {
		return nil
	}

	names := make([]string, len(conn.tools))
	for i, t := range conn.tools {
		names[i] = fmt.Sprintf("mcp:%s:%s", name, t.Name)
	}
	return names
}

func LoadMCPConfig(path string) ([]MCPServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var configs struct {
		Servers []MCPServerConfig `json:"servers"`
	}
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, err
	}

	return configs.Servers, nil
}

func FindMCPConfig(codebasePath string) string {
	candidates := []string{
		codebasePath + "/.ntwine/mcp.json",
		codebasePath + "/.mcp.json",
		homeDir() + "/.ntwine/mcp.json",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func IsCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
