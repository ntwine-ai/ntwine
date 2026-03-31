package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/ntwine-ai/ntwine/internal/api"
	"github.com/ntwine-ai/ntwine/internal/config"
	"github.com/ntwine-ai/ntwine/internal/harness"
)

func main() {
	dev := flag.Bool("dev", false, "dev mode - proxy to frontend dev server instead of embedded files")
	port := flag.String("port", "8080", "port to listen on")
	noBrowser := flag.Bool("no-browser", false, "don't auto-open browser")
	flag.Parse()

	addr := "localhost:" + *port

	cfg, err := config.Load()
	if err != nil {
		log.Printf("warning: could not load config: %v", err)
		cfg = config.Config{}
	}

	registry := harness.NewRegistry()
	var notes string
	var pins []string
	harness.RegisterBuiltins(registry, ".", &notes, &pins, cfg.TavilyKey)

	tracker := harness.NewReadTracker()
	harness.RegisterEditTools(registry, ".", tracker)
	harness.RegisterShellTool(registry, ".", 30*1000000000)
	harness.RegisterThinkTool(registry)

	memStore := harness.NewMemoryStore(".")
	harness.RegisterMemoryTool(registry, memStore)

	harness.RegisterSkillTool(registry, ".ntwine/skills", homeSkillDir())

	mcpHub := harness.NewMCPHub(registry)
	mcpConfigPath := harness.FindMCPConfig(".")
	if mcpConfigPath != "" {
		mcpConfigs, err := harness.LoadMCPConfig(mcpConfigPath)
		if err != nil {
			log.Printf("warning: failed to load MCP config: %v", err)
		} else {
			for _, mcpCfg := range mcpConfigs {
				if err := mcpHub.Connect(context.Background(), mcpCfg); err != nil {
					log.Printf("warning: failed to connect MCP server %s: %v", mcpCfg.Name, err)
				}
			}
		}
	}

	log.Printf("harness ready: %d tools registered", registry.Count())

	var frontend http.Handler
	if !*dev {
		exePath, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("failed to get working directory: %v", err)
		}
		buildDir := filepath.Join(exePath, "frontend", "build")
		frontend = http.FileServer(http.Dir(buildDir))
	}

	router := api.NewRouter(frontend, registry)

	fmt.Printf("ntwine running at http://%s\n", addr)
	if !*noBrowser {
		openBrowser("http://" + addr)
	}

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func homeSkillDir() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return filepath.Join(u.HomeDir, ".ntwine", "skills")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}
