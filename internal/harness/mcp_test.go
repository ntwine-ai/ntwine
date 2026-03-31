package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// LoadMCPConfig

func TestLoadMCPConfig_ValidJSON(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	config := map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"name":    "context7",
				"command": "npx",
				"args":    []string{"-y", "@upstash/context7-mcp"},
			},
			{
				"name":      "http-server",
				"transport": "http",
				"url":       "http://localhost:9000",
			},
		},
	}
	data, _ := json.Marshal(config)
	configPath := filepath.Join(dir, "mcp.json")
	os.WriteFile(configPath, data, 0644)

	servers, err := LoadMCPConfig(configPath)
	if err != nil {
		t.Fatalf("LoadMCPConfig failed: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
	if servers[0].Name != "context7" {
		t.Errorf("expected name 'context7', got %q", servers[0].Name)
	}
	if servers[0].Command != "npx" {
		t.Errorf("expected command 'npx', got %q", servers[0].Command)
	}
	if len(servers[0].Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(servers[0].Args))
	}
	if servers[1].Transport != "http" {
		t.Errorf("expected transport 'http', got %q", servers[1].Transport)
	}
}

func TestLoadMCPConfig_WithEnv(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	config := map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"name":    "env-server",
				"command": "my-mcp",
				"env": map[string]string{
					"API_KEY": "secret123",
					"DEBUG":   "true",
				},
			},
		},
	}
	data, _ := json.Marshal(config)
	configPath := filepath.Join(dir, "mcp.json")
	os.WriteFile(configPath, data, 0644)

	servers, err := LoadMCPConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if servers[0].Env["API_KEY"] != "secret123" {
		t.Errorf("expected API_KEY='secret123', got %q", servers[0].Env["API_KEY"])
	}
}

func TestLoadMCPConfig_EmptyServers(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	data := []byte(`{"servers":[]}`)
	configPath := filepath.Join(dir, "mcp.json")
	os.WriteFile(configPath, data, 0644)

	servers, err := LoadMCPConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestLoadMCPConfig_InvalidJSON(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	configPath := filepath.Join(dir, "mcp.json")
	os.WriteFile(configPath, []byte("not json at all {{{"), 0644)

	_, err := LoadMCPConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadMCPConfig_MissingFile(t *testing.T) {
	_, err := LoadMCPConfig("/nonexistent/mcp.json")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoadMCPConfig_SSETransport(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	config := map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"name":      "sse-server",
				"transport": "sse",
				"url":       "http://localhost:8080/sse",
			},
		},
	}
	data, _ := json.Marshal(config)
	configPath := filepath.Join(dir, "mcp.json")
	os.WriteFile(configPath, data, 0644)

	servers, err := LoadMCPConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if servers[0].Transport != "sse" {
		t.Errorf("expected transport='sse', got %q", servers[0].Transport)
	}
	if servers[0].URL != "http://localhost:8080/sse" {
		t.Errorf("expected URL set, got %q", servers[0].URL)
	}
}

// FindMCPConfig

func TestFindMCPConfig_NtwineDir(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	ntwinePath := filepath.Join(dir, ".ntwine")
	os.MkdirAll(ntwinePath, 0755)
	expectedPath := filepath.Join(ntwinePath, "mcp.json")
	os.WriteFile(expectedPath, []byte(`{"servers":[]}`), 0644)

	found := FindMCPConfig(dir)
	if found != expectedPath {
		t.Errorf("expected %q, got %q", expectedPath, found)
	}
}

func TestFindMCPConfig_DotMcpJson(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	expectedPath := filepath.Join(dir, ".mcp.json")
	os.WriteFile(expectedPath, []byte(`{"servers":[]}`), 0644)

	found := FindMCPConfig(dir)
	if found != expectedPath {
		t.Errorf("expected %q, got %q", expectedPath, found)
	}
}

func TestFindMCPConfig_NoneFound_ReturnsEmpty(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	found := FindMCPConfig(dir)
	// may find ~/.ntwine/mcp.json if it exists on the test machine, so only
	// check the codebase-local ones are absent by checking it doesn't point inside dir
	if found != "" {
		// only fail if it points into our temp dir (which has no mcp files)
		if found == filepath.Join(dir, ".ntwine", "mcp.json") || found == filepath.Join(dir, ".mcp.json") {
			t.Errorf("found unexpected mcp config in test dir: %q", found)
		}
	}
}

// MCPHub structure

func TestNewMCPHub_InitializesEmpty(t *testing.T) {
	r := NewRegistry()
	hub := NewMCPHub(r)
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	servers := hub.ListServers()
	if len(servers) != 0 {
		t.Errorf("expected empty server list, got %d", len(servers))
	}
}

func TestMCPHub_Disconnect_NonexistentServer(t *testing.T) {
	r := NewRegistry()
	hub := NewMCPHub(r)
	err := hub.Disconnect("nonexistent")
	if err == nil {
		t.Error("expected error when disconnecting nonexistent server")
	}
}

func TestMCPHub_ServerTools_NonexistentServer(t *testing.T) {
	r := NewRegistry()
	hub := NewMCPHub(r)
	tools := hub.ServerTools("nonexistent")
	if tools != nil {
		t.Errorf("expected nil for nonexistent server, got %v", tools)
	}
}

func TestMCPHub_DisconnectAll_EmptyHub(t *testing.T) {
	r := NewRegistry()
	hub := NewMCPHub(r)
	// should not panic
	hub.DisconnectAll()
}

// MCPServerConfig struct

func TestMCPServerConfig_JSON_Roundtrip(t *testing.T) {
	cfg := MCPServerConfig{
		Name:      "test-server",
		Command:   "my-cmd",
		Args:      []string{"--arg1", "--arg2"},
		Env:       map[string]string{"KEY": "val"},
		Transport: "stdio",
		URL:       "",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got MCPServerConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Name != cfg.Name {
		t.Errorf("Name mismatch: %q vs %q", got.Name, cfg.Name)
	}
	if got.Command != cfg.Command {
		t.Errorf("Command mismatch: %q vs %q", got.Command, cfg.Command)
	}
	if len(got.Args) != len(cfg.Args) {
		t.Errorf("Args length mismatch: %d vs %d", len(got.Args), len(cfg.Args))
	}
}

// IsCommandAvailable

func TestIsCommandAvailable_KnownCommand(t *testing.T) {
	// sh should always be available
	if !IsCommandAvailable("sh") {
		t.Error("expected 'sh' to be available")
	}
}

func TestIsCommandAvailable_UnknownCommand(t *testing.T) {
	if IsCommandAvailable("this_command_definitely_does_not_exist_12345") {
		t.Error("expected unavailable command to return false")
	}
}
