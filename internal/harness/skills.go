package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Content     string
	FilePath    string
}

func LoadSkills(dirs ...string) []Skill {
	var skills []Skill

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
				data, err := os.ReadFile(skillFile)
				if err != nil {
					continue
				}
				name, desc, content := parseSkillMd(string(data))
				if name == "" {
					name = entry.Name()
				}
				skills = append(skills, Skill{
					Name:        name,
					Description: desc,
					Content:     content,
					FilePath:    skillFile,
				})
				continue
			}

			if strings.HasSuffix(entry.Name(), ".md") {
				data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err != nil {
					continue
				}
				name, desc, content := parseSkillMd(string(data))
				if name == "" {
					name = strings.TrimSuffix(entry.Name(), ".md")
				}
				skills = append(skills, Skill{
					Name:        name,
					Description: desc,
					Content:     content,
					FilePath:    filepath.Join(dir, entry.Name()),
				})
			}
		}
	}

	return skills
}

func parseSkillMd(raw string) (name, description, content string) {
	if !strings.HasPrefix(raw, "---") {
		return "", "", raw
	}

	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return "", "", raw
	}

	frontmatter := parts[1]
	content = strings.TrimSpace(parts[2])

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
		if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}

	return name, description, content
}

func RegisterSkillTool(r *Registry, skillDirs ...string) {
	skills := LoadSkills(skillDirs...)
	if len(skills) == 0 {
		return
	}

	skillMap := make(map[string]Skill)
	var names []string
	for _, s := range skills {
		skillMap[s.Name] = s
		names = append(names, s.Name)
	}

	r.Register(ToolDef{
		Name:             "use_skill",
		Description:      fmt.Sprintf("load a skill for specialized guidance. available: %s", strings.Join(names, ", ")),
		ModelDescription: "load a skill into context for specialized instructions",
		Source:           "builtin",
		ReadOnly:         true,
		Parameters: params(
			prop("name", "string", fmt.Sprintf("skill name. available: %s", strings.Join(names, ", "))),
			required("name"),
		),
	}, func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}

		skill, ok := skillMap[args.Name]
		if !ok {
			return ToolResult{
				Output:  fmt.Sprintf("unknown skill: %s. available: %s", args.Name, strings.Join(names, ", ")),
				IsError: true,
			}, nil
		}

		return ToolResult{
			Output: fmt.Sprintf("skill loaded: %s\n\n%s", skill.Name, skill.Content),
		}, nil
	})
}
