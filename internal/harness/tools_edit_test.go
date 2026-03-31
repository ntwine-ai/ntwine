package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupCodebase creates a temp dir with an optional file and returns (codebasePath, cleanup).
func setupCodebase(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ntwine-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

// ReadTracker

func TestReadTracker_MarkAndHasRead(t *testing.T) {
	rt := NewReadTracker()
	if rt.HasRead("/some/path") {
		t.Error("expected false before marking")
	}
	rt.MarkRead("/some/path")
	if !rt.HasRead("/some/path") {
		t.Error("expected true after marking")
	}
}

func TestReadTracker_DifferentPaths(t *testing.T) {
	rt := NewReadTracker()
	rt.MarkRead("/a")
	if rt.HasRead("/b") {
		t.Error("marking /a should not affect /b")
	}
}

// trackingReadHandler

func TestTrackingReadHandler_MarksReadAndReturnsContent(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	writeFile(t, dir, "hello.txt", "line one\nline two\n")
	tracker := NewReadTracker()
	handler := trackingReadHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{"path": "hello.txt"})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Output)
	}

	absPath := filepath.Join(dir, "hello.txt")
	if !tracker.HasRead(absPath) {
		t.Error("handler should have marked file as read")
	}
	if !strings.Contains(result.Output, "line one") {
		t.Errorf("expected output to contain file content, got: %s", result.Output)
	}
}

func TestTrackingReadHandler_LineNumbers(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "nums.txt", "alpha\nbeta\ngamma\n")
	tracker := NewReadTracker()
	handler := trackingReadHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{"path": "nums.txt"})
	result, _ := handler(context.Background(), raw)

	if !strings.Contains(result.Output, "1\t") {
		t.Errorf("expected line numbers in output, got: %s", result.Output)
	}
}

func TestTrackingReadHandler_OffsetAndLimit(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "multi.txt", "A\nB\nC\nD\nE\n")
	tracker := NewReadTracker()
	handler := trackingReadHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{"path": "multi.txt", "offset": 2, "limit": 2})
	result, _ := handler(context.Background(), raw)

	if !strings.Contains(result.Output, "B") {
		t.Errorf("expected line B (offset=2), got: %s", result.Output)
	}
	if strings.Contains(result.Output, "D") {
		t.Errorf("limit should cut off at 2 lines, got D in: %s", result.Output)
	}
}

func TestTrackingReadHandler_OffsetBeyondEOF(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "short.txt", "one\n")
	tracker := NewReadTracker()
	handler := trackingReadHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{"path": "short.txt", "offset": 999})
	result, _ := handler(context.Background(), raw)
	if !strings.Contains(result.Output, "offset beyond end") {
		t.Errorf("expected 'offset beyond end' message, got: %s", result.Output)
	}
}

func TestTrackingReadHandler_MissingFile(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	tracker := NewReadTracker()
	handler := trackingReadHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{"path": "nonexistent.txt"})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// editFileHandler

func TestEditFileHandler_RequiresReadFirst(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "target.txt", "hello world")
	tracker := NewReadTracker()
	handler := editFileHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "target.txt",
		"old_string": "hello",
		"new_string": "hi",
	})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when file has not been read")
	}
}

func TestEditFileHandler_SucceedsAfterRead(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "edit.txt", "foo bar baz")
	tracker := NewReadTracker()

	absPath := filepath.Join(dir, "edit.txt")
	tracker.MarkRead(absPath)

	handler := editFileHandler(dir, tracker)
	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "edit.txt",
		"old_string": "foo bar",
		"new_string": "qux",
	})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Output)
	}

	data, _ := os.ReadFile(absPath)
	if !strings.Contains(string(data), "qux") {
		t.Errorf("expected 'qux' in file, got: %s", string(data))
	}
}

func TestEditFileHandler_OldStringNotFound(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "nope.txt", "apple orange")
	tracker := NewReadTracker()
	tracker.MarkRead(filepath.Join(dir, "nope.txt"))

	handler := editFileHandler(dir, tracker)
	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "nope.txt",
		"old_string": "banana",
		"new_string": "mango",
	})
	result, _ := handler(context.Background(), raw)
	if !result.IsError {
		t.Error("expected IsError=true when old_string not found")
	}
	if !strings.Contains(result.Output, "not found") {
		t.Errorf("expected 'not found' in output, got: %s", result.Output)
	}
}

func TestEditFileHandler_AmbiguousMatch(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "dup.txt", "foo foo foo")
	tracker := NewReadTracker()
	tracker.MarkRead(filepath.Join(dir, "dup.txt"))

	handler := editFileHandler(dir, tracker)
	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "dup.txt",
		"old_string": "foo",
		"new_string": "bar",
	})
	result, _ := handler(context.Background(), raw)
	if !result.IsError {
		t.Error("expected IsError=true for ambiguous match")
	}
	if !strings.Contains(result.Output, "times") {
		t.Errorf("expected count mention in output, got: %s", result.Output)
	}
}

func TestEditFileHandler_ExactUniqueMatch(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "unique.txt", "func foo() {\n\treturn 1\n}\n\nfunc bar() {\n\treturn 2\n}\n")
	tracker := NewReadTracker()
	tracker.MarkRead(filepath.Join(dir, "unique.txt"))

	handler := editFileHandler(dir, tracker)
	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "unique.txt",
		"old_string": "return 1",
		"new_string": "return 42",
	})
	result, _ := handler(context.Background(), raw)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "unique.txt"))
	if !strings.Contains(string(data), "return 42") {
		t.Error("expected 'return 42' after edit")
	}
	if strings.Contains(string(data), "return 1") {
		t.Error("old string should be replaced")
	}
}

func TestEditFileHandler_PathTraversal(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	tracker := NewReadTracker()
	handler := editFileHandler(dir, tracker)

	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "../../../etc/passwd",
		"old_string": "root",
		"new_string": "hacked",
	})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

// writeFileHandler

func TestWriteFileHandler_CreatesNewFile(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	handler := writeFileHandler(dir)

	raw, _ := json.Marshal(map[string]interface{}{
		"path":    "new_file.txt",
		"content": "brand new content",
	})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Output)
	}

	data, err := os.ReadFile(filepath.Join(dir, "new_file.txt"))
	if err != nil {
		t.Fatalf("file should have been created: %v", err)
	}
	if string(data) != "brand new content" {
		t.Errorf("expected 'brand new content', got: %s", string(data))
	}
}

func TestWriteFileHandler_OverwritesExistingFile(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "existing.txt", "old content")
	handler := writeFileHandler(dir)

	raw, _ := json.Marshal(map[string]interface{}{
		"path":    "existing.txt",
		"content": "new content",
	})
	result, _ := handler(context.Background(), raw)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "existing.txt"))
	if string(data) != "new content" {
		t.Errorf("expected 'new content', got: %s", string(data))
	}
}

func TestWriteFileHandler_CreatesParentDirs(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	handler := writeFileHandler(dir)

	raw, _ := json.Marshal(map[string]interface{}{
		"path":    "subdir/deep/file.txt",
		"content": "nested",
	})
	result, _ := handler(context.Background(), raw)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "subdir/deep/file.txt"))
	if err != nil {
		t.Fatalf("nested file not created: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("expected 'nested', got: %s", string(data))
	}
}

func TestWriteFileHandler_PathTraversal(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	handler := writeFileHandler(dir)

	raw, _ := json.Marshal(map[string]interface{}{
		"path":    "../../evil.txt",
		"content": "bad",
	})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

// RegisterEditTools integration

func TestRegisterEditTools_ReplacesReadFileWithTracker(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "sample.go", "package main\n")

	r := NewRegistry()
	RegisterBuiltins(r, dir, new(string), new([]string), "")
	tracker := NewReadTracker()
	RegisterEditTools(r, dir, tracker)

	// read_file should now mark the file
	result, err := r.Execute(context.Background(), "read_file", `{"path":"sample.go"}`)
	if err != nil || result.IsError {
		t.Fatalf("read_file failed: err=%v output=%s", err, result.Output)
	}

	absPath := filepath.Join(dir, "sample.go")
	if !tracker.HasRead(absPath) {
		t.Error("read_file via registry should mark file in tracker")
	}
}

func TestRegisterEditTools_EditAfterRead_E2E(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()
	writeFile(t, dir, "code.go", "func hello() string {\n\treturn \"world\"\n}\n")

	r := NewRegistry()
	RegisterBuiltins(r, dir, new(string), new([]string), "")
	tracker := NewReadTracker()
	RegisterEditTools(r, dir, tracker)

	// read first
	r.Execute(context.Background(), "read_file", `{"path":"code.go"}`)

	// now edit should succeed
	raw, _ := json.Marshal(map[string]interface{}{
		"path":       "code.go",
		"old_string": `return "world"`,
		"new_string": `return "ntwine"`,
	})
	result, _ := r.Execute(context.Background(), "edit_file", string(raw))
	if result.IsError {
		t.Fatalf("edit_file should succeed after reading: %s", result.Output)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "code.go"))
	if !strings.Contains(string(data), `return "ntwine"`) {
		t.Error("expected edited content in file")
	}
}
