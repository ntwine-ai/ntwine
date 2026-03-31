package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type ReadTracker struct {
	mu   sync.Mutex
	read map[string]bool
}

func NewReadTracker() *ReadTracker {
	return &ReadTracker{read: make(map[string]bool)}
}

func (rt *ReadTracker) MarkRead(path string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.read[path] = true
}

func (rt *ReadTracker) HasRead(path string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.read[path]
}

func RegisterEditTools(r *Registry, codebasePath string, tracker *ReadTracker) {
	r.Register(ToolDef{
		Name:             "edit_file",
		Description:      "edit a file by replacing an exact string with a new string. you MUST read the file first.",
		ModelDescription: "replace old_string with new_string in a file. requires read_file first.",
		Source:           "builtin",
		Parameters: params(
			prop("path", "string", "file path relative to codebase root"),
			prop("old_string", "string", "the exact text to find and replace. must match exactly."),
			prop("new_string", "string", "the replacement text"),
			required("path", "old_string", "new_string"),
		),
	}, editFileHandler(codebasePath, tracker))

	r.Register(ToolDef{
		Name:             "write_file",
		Description:      "create a new file or overwrite an existing one. for existing files, prefer edit_file instead.",
		ModelDescription: "write content to a file. creates directories if needed.",
		Source:           "builtin",
		Parameters: params(
			prop("path", "string", "file path relative to codebase root"),
			prop("content", "string", "the full file content to write"),
			required("path", "content"),
		),
	}, writeFileHandler(codebasePath))

	oldReadHandler := findHandler(r, "read_file")
	if oldReadHandler != nil {
		r.Register(ToolDef{
			Name:             "read_file",
			Description:      "read a file from the codebase",
			ModelDescription: "read a file from the codebase",
			Source:           "builtin",
			Parameters: params(
				prop("path", "string", "file path relative to codebase root"),
				prop("offset", "number", "line number to start reading from (optional)"),
				prop("limit", "number", "number of lines to read (optional)"),
				required("path"),
			),
		}, trackingReadHandler(codebasePath, tracker))
	}
}

func findHandler(r *Registry, name string) ToolHandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.tools[name]; ok {
		return t.handler
	}
	return nil
}

func trackingReadHandler(codebasePath string, tracker *ReadTracker) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		absPath, err := validatePath(codebasePath, args.Path)
		if err != nil {
			return ToolResult{}, err
		}

		tracker.MarkRead(absPath)

		data, err := os.ReadFile(absPath)
		if err != nil {
			return ToolResult{}, err
		}

		lines := strings.Split(string(data), "\n")

		start := 0
		if args.Offset > 0 {
			start = args.Offset - 1
			if start >= len(lines) {
				return ToolResult{Output: "offset beyond end of file"}, nil
			}
		}

		end := len(lines)
		if args.Limit > 0 && start+args.Limit < end {
			end = start + args.Limit
		}

		var sb strings.Builder
		for i := start; i < end; i++ {
			sb.WriteString(fmt.Sprintf("%d\t%s\n", i+1, lines[i]))
		}

		out := sb.String()
		if len(out) > 50000 {
			out = out[:50000] + "\n... (truncated)"
		}
		return ToolResult{Output: out}, nil
	}
}

func editFileHandler(codebasePath string, tracker *ReadTracker) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Path      string `json:"path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}

		absPath, err := validatePath(codebasePath, args.Path)
		if err != nil {
			return ToolResult{}, err
		}

		if !tracker.HasRead(absPath) {
			return ToolResult{
				Output:  "you must read_file before editing. read the file first so you know what you're changing.",
				IsError: true,
			}, nil
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return ToolResult{}, err
		}

		content := string(data)
		count := strings.Count(content, args.OldString)

		if count == 0 {
			return ToolResult{
				Output:  "old_string not found in file. make sure it matches exactly, including whitespace and indentation.",
				IsError: true,
			}, nil
		}

		if count > 1 {
			return ToolResult{
				Output:  fmt.Sprintf("old_string found %d times. provide more context to make it unique.", count),
				IsError: true,
			}, nil
		}

		newContent := strings.Replace(content, args.OldString, args.NewString, 1)
		if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
			return ToolResult{}, err
		}

		return ToolResult{Output: "file edited successfully"}, nil
	}
}

func writeFileHandler(codebasePath string) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}

		absPath, err := validatePath(codebasePath, args.Path)
		if err != nil {
			return ToolResult{}, err
		}

		dir := strings.TrimSuffix(absPath, "/"+args.Path)
		_ = dir

		parentDir := absPath[:strings.LastIndex(absPath, "/")]
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return ToolResult{}, fmt.Errorf("failed to create directories: %w", err)
		}

		if err := os.WriteFile(absPath, []byte(args.Content), 0644); err != nil {
			return ToolResult{}, err
		}

		return ToolResult{Output: "file written: " + args.Path}, nil
	}
}
