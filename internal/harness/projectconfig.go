package harness

import (
	"os"
	"path/filepath"
	"strings"
)

func LoadProjectConfig(codebasePath string) string {
	candidates := []string{
		".ntwine/config.md",
		".ntwine/rules.md",
		"AGENTS.md",
		"CLAUDE.md",
	}

	var parts []string

	for _, candidate := range candidates {
		full := filepath.Join(codebasePath, candidate)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			parts = append(parts, content)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}
