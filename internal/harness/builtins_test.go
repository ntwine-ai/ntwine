package harness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// read_file via builtin handler

func TestReadFileHandler_ReadsContent(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	writeFile(t, dir, "hello.txt", "hello world")
	handler := readFileHandler(dir)

	raw, _ := json.Marshal(map[string]string{"path": "hello.txt"})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello world" {
		t.Errorf("expected 'hello world', got: %q", result.Output)
	}
}

func TestReadFileHandler_MissingFile_ReturnsError(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	handler := readFileHandler(dir)
	raw, _ := json.Marshal(map[string]string{"path": "missing.txt"})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadFileHandler_LargeFile_Truncated(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	big := strings.Repeat("a", 60000)
	writeFile(t, dir, "big.txt", big)
	handler := readFileHandler(dir)

	raw, _ := json.Marshal(map[string]string{"path": "big.txt"})
	result, _ := handler(context.Background(), raw)
	if len(result.Output) > 51000 {
		t.Errorf("expected truncation at 50000 chars, got %d", len(result.Output))
	}
	if !strings.Contains(result.Output, "truncated") {
		t.Error("expected '(truncated)' marker in output")
	}
}

func TestReadFileHandler_PathTraversal_Blocked(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	handler := readFileHandler(dir)
	raw, _ := json.Marshal(map[string]string{"path": "../../etc/passwd"})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

// list_files

func TestListFilesHandler_ListsMatchingFiles(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	writeFile(t, dir, "a.go", "")
	writeFile(t, dir, "b.go", "")
	writeFile(t, dir, "c.txt", "")

	handler := listFilesHandler(dir)
	raw, _ := json.Marshal(map[string]string{"path": ".", "pattern": "*.go"})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "a.go") {
		t.Errorf("expected a.go in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "b.go") {
		t.Errorf("expected b.go in output, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "c.txt") {
		t.Errorf("c.txt should not match *.go pattern, got: %s", result.Output)
	}
}

func TestListFilesHandler_NoPattern_ListsAll(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	writeFile(t, dir, "file1.txt", "")
	writeFile(t, dir, "file2.md", "")

	handler := listFilesHandler(dir)
	raw, _ := json.Marshal(map[string]string{"path": "."})
	result, _ := handler(context.Background(), raw)
	if !strings.Contains(result.Output, "file1.txt") || !strings.Contains(result.Output, "file2.md") {
		t.Errorf("expected both files with no pattern, got: %s", result.Output)
	}
}

func TestListFilesHandler_EmptyDir_EmptyOutput(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	handler := listFilesHandler(dir)
	raw, _ := json.Marshal(map[string]string{"path": "."})
	result, _ := handler(context.Background(), raw)
	if result.Output != "" {
		t.Errorf("expected empty output for empty dir, got: %q", result.Output)
	}
}

func TestListFilesHandler_PathTraversal_Blocked(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	handler := listFilesHandler(dir)
	raw, _ := json.Marshal(map[string]string{"path": "../../etc"})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

// search_code

func TestSearchCodeHandler_FindsMatch(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n\nfunc hello() string {\n\treturn \"world\"\n}\n")

	handler := searchCodeHandler(dir)
	raw, _ := json.Marshal(map[string]string{"query": "func hello"})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "func hello") {
		t.Errorf("expected 'func hello' in search output, got: %s", result.Output)
	}
}

func TestSearchCodeHandler_NoMatch_ReturnsNoMatches(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	writeFile(t, dir, "empty.go", "package main\n")

	handler := searchCodeHandler(dir)
	raw, _ := json.Marshal(map[string]string{"query": "THIS_WILL_NOT_MATCH_ANYTHING_EVER"})
	result, _ := handler(context.Background(), raw)
	if result.Output != "no matches found" {
		t.Errorf("expected 'no matches found', got: %q", result.Output)
	}
}

func TestSearchCodeHandler_WithSubPath(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	subDir := dir + "/sub"
	os.MkdirAll(subDir, 0755)
	writeFile(t, subDir, "code.go", "needle in a haystack")

	handler := searchCodeHandler(dir)
	raw, _ := json.Marshal(map[string]string{"query": "needle", "path": "sub"})
	result, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "needle") {
		t.Errorf("expected 'needle' in output, got: %s", result.Output)
	}
}

// update_notes

func TestUpdateNotesHandler_Append(t *testing.T) {
	notes := ""
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{"action": "append", "content": "first note"})
	result, err := handler(context.Background(), raw)
	if err != nil || result.IsError {
		t.Fatalf("append failed: err=%v output=%s", err, result.Output)
	}
	if notes != "first note" {
		t.Errorf("expected notes='first note', got %q", notes)
	}
}

func TestUpdateNotesHandler_Append_TwiceJoinsWithBlankLine(t *testing.T) {
	notes := "existing"
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{"action": "append", "content": "second"})
	handler(context.Background(), raw)

	if notes != "existing\n\nsecond" {
		t.Errorf("expected double-newline join, got %q", notes)
	}
}

func TestUpdateNotesHandler_ReplaceAll(t *testing.T) {
	notes := "old content"
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{"action": "replace_all", "content": "brand new"})
	handler(context.Background(), raw)

	if notes != "brand new" {
		t.Errorf("expected 'brand new', got %q", notes)
	}
}

func TestUpdateNotesHandler_ReplaceSection(t *testing.T) {
	notes := "## Goals\nold goal\n\n## Tasks\ntask1\n"
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{
		"action":  "replace_section",
		"heading": "Goals",
		"content": "new goal",
	})
	result, err := handler(context.Background(), raw)
	if err != nil || result.IsError {
		t.Fatalf("replace_section failed: err=%v output=%s", err, result.Output)
	}
	if !strings.Contains(notes, "new goal") {
		t.Errorf("expected 'new goal' in notes, got: %q", notes)
	}
	if strings.Contains(notes, "old goal") {
		t.Errorf("old goal should be replaced, got: %q", notes)
	}
}

func TestUpdateNotesHandler_ReplaceSection_MissingHeading_Appends(t *testing.T) {
	notes := "## Existing\ncontent\n"
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{
		"action":  "replace_section",
		"heading": "NewSection",
		"content": "new section content",
	})
	handler(context.Background(), raw)

	if !strings.Contains(notes, "new section content") {
		t.Errorf("expected new section to be appended, got: %q", notes)
	}
}

func TestUpdateNotesHandler_ReplaceSection_RequiresHeading(t *testing.T) {
	notes := ""
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{
		"action":  "replace_section",
		"content": "content",
	})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error when heading is missing for replace_section")
	}
}

func TestUpdateNotesHandler_RemoveSection(t *testing.T) {
	notes := "## Keep\nkeep this\n\n## Remove\nremove this\n"
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{
		"action":  "remove_section",
		"heading": "Remove",
		"content": "",
	})
	handler(context.Background(), raw)

	if strings.Contains(notes, "remove this") {
		t.Errorf("expected section to be removed, got: %q", notes)
	}
	if !strings.Contains(notes, "keep this") {
		t.Errorf("expected Keep section to remain, got: %q", notes)
	}
}

func TestUpdateNotesHandler_RemoveSection_RequiresHeading(t *testing.T) {
	notes := ""
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{"action": "remove_section", "content": ""})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error when heading is missing for remove_section")
	}
}

func TestUpdateNotesHandler_UnknownAction(t *testing.T) {
	notes := ""
	handler := updateNotesHandler(&notes)

	raw, _ := json.Marshal(map[string]string{"action": "explode", "content": "x"})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

// pin_message

func TestPinMessageHandler_AddsPin(t *testing.T) {
	pins := []string{}
	handler := pinMessageHandler(&pins)

	raw, _ := json.Marshal(map[string]string{"message": "important!"})
	result, err := handler(context.Background(), raw)
	if err != nil || result.IsError {
		t.Fatalf("pin failed: err=%v output=%s", err, result.Output)
	}
	if len(pins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(pins))
	}
	if pins[0] != "important!" {
		t.Errorf("expected 'important!', got %q", pins[0])
	}
}

func TestPinMessageHandler_MultiplePins(t *testing.T) {
	pins := []string{}
	handler := pinMessageHandler(&pins)

	for _, msg := range []string{"a", "b", "c"} {
		raw, _ := json.Marshal(map[string]string{"message": msg})
		handler(context.Background(), raw)
	}
	if len(pins) != 3 {
		t.Errorf("expected 3 pins, got %d", len(pins))
	}
}

func TestPinMessageHandler_EmptyMessage_Errors(t *testing.T) {
	pins := []string{}
	handler := pinMessageHandler(&pins)

	raw, _ := json.Marshal(map[string]string{"message": ""})
	_, err := handler(context.Background(), raw)
	if err == nil {
		t.Error("expected error for empty message")
	}
}

// validatePath

func TestValidatePath_ValidRelative(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	abs, err := validatePath(dir, "subdir/file.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(abs, dir) {
		t.Errorf("expected abs path to be within dir, got: %s", abs)
	}
}

func TestValidatePath_TraversalBlocked(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	_, err := validatePath(dir, "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidatePath_AbsoluteOutsideRoot_Blocked(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	_, err := validatePath(dir, "/etc/passwd")
	if err == nil {
		// absolute path outside codebase should be blocked
		t.Error("expected error for absolute path outside codebase")
	}
}

// replaceSection

func TestReplaceSection_ExistingSection(t *testing.T) {
	doc := "## Foo\nold content\n\n## Bar\nkeep this\n"
	result := replaceSection(doc, "Foo", "new content")
	if !strings.Contains(result, "new content") {
		t.Errorf("expected new content, got: %q", result)
	}
	if strings.Contains(result, "old content") {
		t.Error("old content should be replaced")
	}
	if !strings.Contains(result, "keep this") {
		t.Error("Bar section should be kept")
	}
}

func TestReplaceSection_NotFound_Appends(t *testing.T) {
	doc := "## Existing\ncontent\n"
	result := replaceSection(doc, "New", "new stuff")
	if !strings.Contains(result, "new stuff") {
		t.Errorf("expected appended new section, got: %q", result)
	}
	if !strings.Contains(result, "content") {
		t.Error("original content should be preserved")
	}
}

func TestReplaceSection_EmptyDoc(t *testing.T) {
	result := replaceSection("", "MySection", "my content")
	if !strings.Contains(result, "my content") {
		t.Errorf("expected content in empty doc result, got: %q", result)
	}
}

func TestReplaceSection_PreservesOtherSections(t *testing.T) {
	doc := "## A\na content\n\n## B\nb content\n\n## C\nc content\n"
	result := replaceSection(doc, "B", "replaced b")
	if !strings.Contains(result, "a content") || !strings.Contains(result, "c content") {
		t.Errorf("expected A and C preserved, got: %q", result)
	}
}

// removeSection

func TestRemoveSection_RemovesTargetSection(t *testing.T) {
	doc := "## Keep\nkeep content\n\n## Remove\nremove content\n"
	result := removeSection(doc, "Remove")
	if strings.Contains(result, "remove content") {
		t.Errorf("expected section removed, got: %q", result)
	}
	if !strings.Contains(result, "keep content") {
		t.Error("keep section should remain")
	}
}

func TestRemoveSection_NonexistentSection_NoChange(t *testing.T) {
	doc := "## Only\nonly content\n"
	result := removeSection(doc, "Nonexistent")
	if !strings.Contains(result, "only content") {
		t.Errorf("expected original content preserved, got: %q", result)
	}
}

func TestRemoveSection_RemovesHeadingToo(t *testing.T) {
	doc := "## Remove\ncontent\n"
	result := removeSection(doc, "Remove")
	if strings.Contains(result, "## Remove") {
		t.Errorf("heading should also be removed, got: %q", result)
	}
}

// RegisterBuiltins integration

func TestRegisterBuiltins_AllToolsPresent(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	notes := ""
	pins := []string{}
	RegisterBuiltins(r, dir, &notes, &pins, "")

	for _, name := range []string{"read_file", "list_files", "search_code", "update_notes", "pin_message"} {
		if !r.Has(name) {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestRegisterBuiltins_WebSearch_OnlyWithKey(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	RegisterBuiltins(r, dir, new(string), new([]string), "")
	if r.Has("web_search") {
		t.Error("web_search should not be registered without API key")
	}

	r2 := NewRegistry()
	RegisterBuiltins(r2, dir, new(string), new([]string), "fake-key")
	if !r2.Has("web_search") {
		t.Error("web_search should be registered with API key")
	}
}
