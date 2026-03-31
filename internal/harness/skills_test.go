package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSkillDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ntwine-skills-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func writeSkillFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeSkillFile: %v", err)
	}
}

// parseSkillMd

func TestParseSkillMd_NoFrontmatter(t *testing.T) {
	raw := "just plain content"
	name, desc, content := parseSkillMd(raw)
	if name != "" {
		t.Errorf("expected empty name, got %q", name)
	}
	if desc != "" {
		t.Errorf("expected empty desc, got %q", desc)
	}
	if content != raw {
		t.Errorf("expected full raw as content, got %q", content)
	}
}

func TestParseSkillMd_WithFrontmatter(t *testing.T) {
	raw := "---\nname: my-skill\ndescription: does cool stuff\n---\n\nThis is the body."
	name, desc, content := parseSkillMd(raw)
	if name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", name)
	}
	if desc != "does cool stuff" {
		t.Errorf("expected 'does cool stuff', got %q", desc)
	}
	if !strings.Contains(content, "This is the body.") {
		t.Errorf("expected body in content, got %q", content)
	}
}

func TestParseSkillMd_NameOnly(t *testing.T) {
	raw := "---\nname: only-name\n---\nbody here"
	name, desc, content := parseSkillMd(raw)
	if name != "only-name" {
		t.Errorf("expected 'only-name', got %q", name)
	}
	if desc != "" {
		t.Errorf("expected empty desc, got %q", desc)
	}
	if !strings.Contains(content, "body here") {
		t.Errorf("expected body in content, got %q", content)
	}
}

func TestParseSkillMd_DescriptionOnly(t *testing.T) {
	raw := "---\ndescription: just a desc\n---\ncontent"
	name, desc, _ := parseSkillMd(raw)
	if name != "" {
		t.Errorf("expected empty name, got %q", name)
	}
	if desc != "just a desc" {
		t.Errorf("expected 'just a desc', got %q", desc)
	}
}

func TestParseSkillMd_IncompleteFrontmatter(t *testing.T) {
	raw := "---\nname: broken"
	name, _, content := parseSkillMd(raw)
	// fewer than 3 parts after split — should return raw
	if name != "" {
		t.Errorf("expected empty name for incomplete frontmatter, got %q", name)
	}
	if content != raw {
		t.Errorf("expected raw content returned, got %q", content)
	}
}

func TestParseSkillMd_WhitespaceInValues(t *testing.T) {
	raw := "---\n  name:   spaced name  \n  description:   spaced desc  \n---\nbody"
	name, desc, _ := parseSkillMd(raw)
	if name != "spaced name" {
		t.Errorf("expected trimmed name, got %q", name)
	}
	if desc != "spaced desc" {
		t.Errorf("expected trimmed desc, got %q", desc)
	}
}

// LoadSkills

func TestLoadSkills_EmptyDir(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	skills := LoadSkills(dir)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkills_NonexistentDir(t *testing.T) {
	skills := LoadSkills("/nonexistent/path/that/does/not/exist")
	if len(skills) != 0 {
		t.Errorf("expected 0 skills from missing dir, got %d", len(skills))
	}
}

func TestLoadSkills_MarkdownFile(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "my-skill.md", "---\nname: my-skill\ndescription: test skill\n---\n\nSkill content here.")

	skills := LoadSkills(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("expected name 'my-skill', got %q", skills[0].Name)
	}
	if skills[0].Description != "test skill" {
		t.Errorf("expected desc 'test skill', got %q", skills[0].Description)
	}
	if !strings.Contains(skills[0].Content, "Skill content here.") {
		t.Errorf("expected skill content, got %q", skills[0].Content)
	}
}

func TestLoadSkills_FallsBackToFilename(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "unnamed-skill.md", "just content no frontmatter")

	skills := LoadSkills(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "unnamed-skill" {
		t.Errorf("expected filename as name, got %q", skills[0].Name)
	}
}

func TestLoadSkills_IgnoresNonMarkdown(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "not-a-skill.txt", "text file content")
	writeSkillFile(t, dir, "skill.md", "---\nname: real-skill\n---\ncontent")

	skills := LoadSkills(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (only .md), got %d", len(skills))
	}
	if skills[0].Name != "real-skill" {
		t.Errorf("expected 'real-skill', got %q", skills[0].Name)
	}
}

func TestLoadSkills_SubdirWithSKILLmd(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	subdir := filepath.Join(dir, "tdd-guide")
	os.MkdirAll(subdir, 0755)
	writeSkillFile(t, subdir, "SKILL.md", "---\nname: tdd-guide\ndescription: TDD workflow\n---\nTDD content")

	skills := LoadSkills(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill from subdir, got %d", len(skills))
	}
	if skills[0].Name != "tdd-guide" {
		t.Errorf("expected 'tdd-guide', got %q", skills[0].Name)
	}
}

func TestLoadSkills_SubdirWithoutSKILLmd_Skipped(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	subdir := filepath.Join(dir, "empty-subdir")
	os.MkdirAll(subdir, 0755)
	// no SKILL.md inside

	skills := LoadSkills(dir)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for subdir without SKILL.md, got %d", len(skills))
	}
}

func TestLoadSkills_MultipleDirs(t *testing.T) {
	dir1, cleanup1 := makeSkillDir(t)
	defer cleanup1()
	dir2, cleanup2 := makeSkillDir(t)
	defer cleanup2()

	writeSkillFile(t, dir1, "skill-a.md", "---\nname: skill-a\n---\ncontent a")
	writeSkillFile(t, dir2, "skill-b.md", "---\nname: skill-b\n---\ncontent b")

	skills := LoadSkills(dir1, dir2)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills from 2 dirs, got %d", len(skills))
	}
}

func TestLoadSkills_FilePath_IsSet(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "fp-skill.md", "---\nname: fp-skill\n---\ncontent")
	skills := LoadSkills(dir)

	if skills[0].FilePath == "" {
		t.Error("expected FilePath to be set")
	}
	if !strings.Contains(skills[0].FilePath, "fp-skill.md") {
		t.Errorf("expected FilePath to contain filename, got %q", skills[0].FilePath)
	}
}

// RegisterSkillTool

func TestRegisterSkillTool_UseSkill_Success(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "tdd.md", "---\nname: tdd\ndescription: test driven dev\n---\nWrite tests first.")

	r := NewRegistry()
	RegisterSkillTool(r, dir)

	if !r.Has("use_skill") {
		t.Fatal("expected 'use_skill' tool to be registered")
	}

	raw, _ := json.Marshal(map[string]string{"name": "tdd"})
	result, err := r.Execute(context.Background(), "use_skill", string(raw))
	if err != nil || result.IsError {
		t.Fatalf("use_skill failed: err=%v output=%s", err, result.Output)
	}
	if !strings.Contains(result.Output, "Write tests first.") {
		t.Errorf("expected skill content in output, got: %s", result.Output)
	}
}

func TestRegisterSkillTool_UnknownSkill(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "existing.md", "---\nname: existing\n---\ncontent")

	r := NewRegistry()
	RegisterSkillTool(r, dir)

	raw, _ := json.Marshal(map[string]string{"name": "nonexistent"})
	result, _ := r.Execute(context.Background(), "use_skill", string(raw))
	if !result.IsError {
		t.Error("expected IsError=true for unknown skill")
	}
	if !strings.Contains(result.Output, "unknown skill") {
		t.Errorf("expected 'unknown skill' in output, got: %s", result.Output)
	}
}

func TestRegisterSkillTool_NoSkills_NoToolRegistered(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()
	// empty dir

	r := NewRegistry()
	RegisterSkillTool(r, dir)

	if r.Has("use_skill") {
		t.Error("use_skill should not be registered when there are no skills")
	}
}

func TestRegisterSkillTool_DescriptionContainsNames(t *testing.T) {
	dir, cleanup := makeSkillDir(t)
	defer cleanup()

	writeSkillFile(t, dir, "alpha.md", "---\nname: alpha\n---\ncontent")
	writeSkillFile(t, dir, "beta.md", "---\nname: beta\n---\ncontent")

	r := NewRegistry()
	RegisterSkillTool(r, dir)

	defs := r.Definitions()
	var skillDef *struct{ desc string }
	for _, d := range defs {
		if d.Function.Name == "use_skill" {
			skillDef = &struct{ desc string }{d.Function.Description}
		}
	}
	if skillDef == nil {
		t.Fatal("use_skill not found in definitions")
	}
	if !strings.Contains(skillDef.desc, "alpha") || !strings.Contains(skillDef.desc, "beta") {
		t.Errorf("expected skill names in description, got: %s", skillDef.desc)
	}
}
