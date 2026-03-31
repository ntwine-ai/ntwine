package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfig_NoConfigFiles_ReturnsEmpty(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	result := LoadProjectConfig(dir)
	if result != "" {
		t.Errorf("expected empty string when no config files, got: %q", result)
	}
}

func TestLoadProjectConfig_NtwinConfigMd(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(dir, ".ntwine"), 0755)
	os.WriteFile(filepath.Join(dir, ".ntwine", "config.md"), []byte("ntwine config content"), 0644)

	result := LoadProjectConfig(dir)
	if !strings.Contains(result, "ntwine config content") {
		t.Errorf("expected config content, got: %q", result)
	}
}

func TestLoadProjectConfig_NtwineRulesMd(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(dir, ".ntwine"), 0755)
	os.WriteFile(filepath.Join(dir, ".ntwine", "rules.md"), []byte("rules here"), 0644)

	result := LoadProjectConfig(dir)
	if !strings.Contains(result, "rules here") {
		t.Errorf("expected rules content, got: %q", result)
	}
}

func TestLoadProjectConfig_AgentsMd(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents config"), 0644)

	result := LoadProjectConfig(dir)
	if !strings.Contains(result, "agents config") {
		t.Errorf("expected AGENTS.md content, got: %q", result)
	}
}

func TestLoadProjectConfig_ClaudeMd(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude instructions"), 0644)

	result := LoadProjectConfig(dir)
	if !strings.Contains(result, "claude instructions") {
		t.Errorf("expected CLAUDE.md content, got: %q", result)
	}
}

func TestLoadProjectConfig_MultipleFiles_Joined(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(dir, ".ntwine"), 0755)
	os.WriteFile(filepath.Join(dir, ".ntwine", "config.md"), []byte("ntwine config"), 0644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents rules"), 0644)

	result := LoadProjectConfig(dir)
	if !strings.Contains(result, "ntwine config") {
		t.Errorf("expected ntwine config, got: %q", result)
	}
	if !strings.Contains(result, "agents rules") {
		t.Errorf("expected agents rules, got: %q", result)
	}
}

func TestLoadProjectConfig_EmptyFile_Ignored(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("   "), 0644)
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("real content"), 0644)

	result := LoadProjectConfig(dir)
	// whitespace-only file should be ignored
	if strings.Contains(result, "   ") {
		t.Errorf("expected empty file to be ignored, got: %q", result)
	}
	if !strings.Contains(result, "real content") {
		t.Errorf("expected real content from CLAUDE.md, got: %q", result)
	}
}

func TestLoadProjectConfig_ContentTrimmed(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("\n\ncontent here\n\n"), 0644)

	result := LoadProjectConfig(dir)
	// result should not start/end with extra newlines in the part from that file
	if strings.HasPrefix(result, "\n") {
		t.Errorf("expected trimmed content, got: %q", result)
	}
}

func TestLoadProjectConfig_NonexistentDir(t *testing.T) {
	result := LoadProjectConfig("/nonexistent/path/does/not/exist")
	if result != "" {
		t.Errorf("expected empty for nonexistent dir, got: %q", result)
	}
}
