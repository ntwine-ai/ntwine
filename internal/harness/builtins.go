package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RegisterBuiltins(r *Registry, codebasePath string, notes *string, pins *[]string, tavilyKey string) {
	r.Register(ToolDef{
		Name:        "read_file",
		Description: "read a file from the codebase",
		Source:      "builtin",
		Parameters: params(
			prop("path", "string", "file path relative to codebase root"),
			required("path"),
		),
	}, readFileHandler(codebasePath))

	r.Register(ToolDef{
		Name:        "list_files",
		Description: "list files matching a glob pattern",
		Source:      "builtin",
		Parameters: params(
			prop("path", "string", "directory path relative to codebase root"),
			prop("pattern", "string", "glob pattern like *.go or **/*.ts"),
			required("path"),
		),
	}, listFilesHandler(codebasePath))

	r.Register(ToolDef{
		Name:        "search_code",
		Description: "search for a pattern in the codebase using grep",
		Source:      "builtin",
		Parameters: params(
			prop("query", "string", "search pattern (regex supported)"),
			prop("path", "string", "directory to search in, relative to root. defaults to root."),
			required("query"),
		),
	}, searchCodeHandler(codebasePath))

	r.Register(ToolDef{
		Name:        "update_notes",
		Description: "update the shared notes doc. actions: append, replace_all, replace_section, remove_section",
		Source:      "builtin",
		Parameters: params(
			prop("action", "string", "append, replace_all, replace_section, or remove_section"),
			prop("heading", "string", "for replace_section/remove_section: the heading to find"),
			prop("content", "string", "the content to write"),
			required("action", "content"),
		),
	}, updateNotesHandler(notes))

	r.Register(ToolDef{
		Name:        "pin_message",
		Description: "pin an important message so it stays visible to everyone",
		Source:      "builtin",
		Parameters: params(
			prop("message", "string", "the message to pin"),
			required("message"),
		),
	}, pinMessageHandler(pins))

	if tavilyKey != "" {
		r.Register(ToolDef{
			Name:        "web_search",
			Description: "search the web for information",
			Source:      "builtin",
			Parameters: params(
				prop("query", "string", "search query"),
				required("query"),
			),
		}, webSearchHandler(tavilyKey))
	}
}

func readFileHandler(codebasePath string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		absPath, err := validatePath(codebasePath, args.Path)
		if err != nil {
			return ToolResult{}, err
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return ToolResult{}, err
		}
		out := string(data)
		if len(data) > 50000 {
			out = out[:50000] + "\n... (truncated)"
		}
		return ToolResult{Output: out}, nil
	}
}

func listFilesHandler(codebasePath string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Path    string `json:"path"`
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		absPath, err := validatePath(codebasePath, args.Path)
		if err != nil {
			return ToolResult{}, err
		}
		pattern := "*"
		if args.Pattern != "" {
			pattern = args.Pattern
		}
		matches, err := filepath.Glob(filepath.Join(absPath, pattern))
		if err != nil {
			return ToolResult{}, err
		}
		lines := make([]string, len(matches))
		for i, m := range matches {
			rel, _ := filepath.Rel(codebasePath, m)
			lines[i] = rel
		}
		return ToolResult{Output: strings.Join(lines, "\n")}, nil
	}
}

func searchCodeHandler(codebasePath string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		if len(args.Query) > 500 {
			return ToolResult{Output: "search query too long (max 500 chars)", IsError: true}, nil
		}

		searchDir := codebasePath
		if args.Path != "" {
			p, err := validatePath(codebasePath, args.Path)
			if err != nil {
				return ToolResult{}, err
			}
			searchDir = p
		}
		cmd := exec.CommandContext(ctx, "grep", "-rn", "--include=*", "-m", "50", "-E", args.Query, searchDir)
		out, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return ToolResult{Output: "no matches found"}, nil
			}
			return ToolResult{}, err
		}
		return ToolResult{Output: string(out)}, nil
	}
}

func updateNotesHandler(notes *string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Action  string `json:"action"`
			Heading string `json:"heading"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		const maxNotesSize = 50000

		switch args.Action {
		case "append":
			if len(*notes)+len(args.Content) > maxNotesSize {
				return ToolResult{Output: "notes too large (max 50KB). use replace_section to trim.", IsError: true}, nil
			}
			if strings.TrimSpace(*notes) == "" {
				*notes = args.Content
			} else {
				*notes = *notes + "\n\n" + args.Content
			}
		case "replace_all":
			*notes = args.Content
		case "replace_section":
			if args.Heading == "" {
				return ToolResult{}, fmt.Errorf("heading is required for replace_section")
			}
			*notes = replaceSection(*notes, args.Heading, args.Content)
		case "remove_section":
			if args.Heading == "" {
				return ToolResult{}, fmt.Errorf("heading is required for remove_section")
			}
			*notes = removeSection(*notes, args.Heading)
		default:
			return ToolResult{}, fmt.Errorf("unknown action: %s", args.Action)
		}
		return ToolResult{Output: "notes updated"}, nil
	}
}

func pinMessageHandler(pins *[]string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		if args.Message == "" {
			return ToolResult{}, fmt.Errorf("message is required")
		}
		if len(*pins) >= 50 {
			return ToolResult{Output: "max 50 pins reached", IsError: true}, nil
		}
		*pins = append(*pins, args.Message)
		return ToolResult{Output: "message pinned"}, nil
	}
}

func webSearchHandler(apiKey string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		reqBody, err := json.Marshal(map[string]interface{}{
			"query":          args.Query,
			"max_results":    5,
			"include_answer": true,
		})
		if err != nil {
			return ToolResult{}, fmt.Errorf("failed to build request: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(reqBody))
		if err != nil {
			return ToolResult{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return ToolResult{}, fmt.Errorf("tavily request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ToolResult{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return ToolResult{Output: fmt.Sprintf("tavily error (%d): %s", resp.StatusCode, string(body)), IsError: true}, nil
		}

		var tavilyResp struct {
			Answer  string `json:"answer"`
			Results []struct {
				Title   string `json:"title"`
				URL     string `json:"url"`
				Content string `json:"content"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &tavilyResp); err != nil {
			return ToolResult{}, err
		}

		var sb strings.Builder
		if tavilyResp.Answer != "" {
			sb.WriteString("Answer: ")
			sb.WriteString(tavilyResp.Answer)
			sb.WriteString("\n\n---\n\n")
		}
		for _, r := range tavilyResp.Results {
			sb.WriteString("Title: ")
			sb.WriteString(r.Title)
			sb.WriteString("\nURL: ")
			sb.WriteString(r.URL)
			sb.WriteString("\nSnippet: ")
			sb.WriteString(r.Content)
			sb.WriteString("\n\n")
		}
		result := sb.String()
		if len(result) > 5000 {
			result = result[:5000] + "\n... (truncated)"
		}
		return ToolResult{Output: result}, nil
	}
}

func params(parts ...map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{"type": "object"}
	props := map[string]interface{}{}
	for _, p := range parts {
		if req, ok := p["required"]; ok {
			result["required"] = req
			continue
		}
		for k, v := range p {
			props[k] = v
		}
	}
	result["properties"] = props
	return result
}

func prop(name, typ, desc string) map[string]interface{} {
	return map[string]interface{}{
		name: map[string]interface{}{
			"type":        typ,
			"description": desc,
		},
	}
}

func required(names ...string) map[string]interface{} {
	return map[string]interface{}{"required": names}
}

func validatePath(codebasePath, requestedPath string) (string, error) {
	cbAbs, err := filepath.Abs(codebasePath)
	if err != nil {
		return "", fmt.Errorf("invalid codebase path")
	}

	if filepath.IsAbs(requestedPath) {
		if !strings.HasPrefix(requestedPath, cbAbs) {
			return "", fmt.Errorf("absolute paths outside the codebase are not allowed")
		}
		requestedPath = strings.TrimPrefix(requestedPath, cbAbs)
		requestedPath = strings.TrimPrefix(requestedPath, "/")
	}

	if strings.Contains(requestedPath, "..") {
		cleaned := filepath.Clean(requestedPath)
		if strings.HasPrefix(cleaned, "..") {
			return "", fmt.Errorf("path escapes codebase root")
		}
		requestedPath = cleaned
	}

	abs, err := filepath.Abs(filepath.Join(codebasePath, requestedPath))
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(abs, cbAbs) {
		return "", fmt.Errorf("path escapes codebase root")
	}
	return abs, nil
}

func replaceSection(doc, heading, newContent string) string {
	lines := strings.Split(doc, "\n")
	headingLevel := strings.Count(strings.TrimSpace(heading), "#")
	if headingLevel == 0 {
		heading = "## " + heading
		headingLevel = 2
	}
	normalizedHeading := strings.TrimSpace(heading)
	var result []string
	found := false
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == normalizedHeading {
			found = true
			skipping = true
			result = append(result, line)
			result = append(result, newContent)
			continue
		}
		if skipping {
			lineLevel := 0
			for _, c := range trimmed {
				if c == '#' {
					lineLevel++
				} else {
					break
				}
			}
			if lineLevel > 0 && lineLevel <= headingLevel {
				skipping = false
			} else {
				continue
			}
		}
		result = append(result, line)
	}
	if !found {
		if strings.TrimSpace(doc) == "" {
			return normalizedHeading + "\n" + newContent
		}
		return doc + "\n\n" + normalizedHeading + "\n" + newContent
	}
	return strings.Join(result, "\n")
}

func removeSection(doc, heading string) string {
	headingLevel := strings.Count(strings.TrimSpace(heading), "#")
	if headingLevel == 0 {
		heading = "## " + heading
		headingLevel = 2
	}
	normalizedHeading := strings.TrimSpace(heading)
	lines := strings.Split(doc, "\n")
	var result []string
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == normalizedHeading {
			skipping = true
			continue
		}
		if skipping {
			lineLevel := 0
			for _, c := range trimmed {
				if c == '#' {
					lineLevel++
				} else {
					break
				}
			}
			if lineLevel > 0 && lineLevel <= headingLevel {
				skipping = false
			} else {
				continue
			}
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}
