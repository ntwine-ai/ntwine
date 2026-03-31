package harness

import (
	"strings"
	"testing"

	"github.com/ntwine-ai/ntwine/internal/openrouter"
)

func makeMsg(role, content string) openrouter.ChatMessage {
	return openrouter.ChatMessage{Role: role, Content: content}
}

func toolMsg(content string) openrouter.ChatMessage {
	return openrouter.ChatMessage{Role: "tool", Content: content}
}

// MaskOldObservations

func TestContextManager_MaskOld_BelowThreshold_NoChange(t *testing.T) {
	cm := NewContextManager(100, 10)
	msgs := []openrouter.ChatMessage{
		toolMsg("important result"),
	}
	out := cm.MaskOldObservations(msgs)
	if out[0].Content != "important result" {
		t.Errorf("content should not be masked below threshold, got: %q", out[0].Content)
	}
}

func TestContextManager_MaskOld_AboveThreshold_MasksOldToolMsgs(t *testing.T) {
	cm := NewContextManager(100, 3)
	// 4 messages total, threshold=3 → oldest 1 message should be masked
	msgs := []openrouter.ChatMessage{
		toolMsg(strings.Repeat("x", 200)), // idx 0 — beyond threshold, should be masked
		toolMsg("recent1"),
		toolMsg("recent2"),
		toolMsg("recent3"),
	}
	out := cm.MaskOldObservations(msgs)

	if !strings.Contains(out[0].Content, "masked") {
		t.Errorf("expected oldest message to be masked, got: %q", out[0].Content)
	}
	if out[1].Content != "recent1" {
		t.Errorf("expected recent messages untouched, got: %q", out[1].Content)
	}
}

func TestContextManager_MaskOld_ShortToolMsgNotMasked(t *testing.T) {
	cm := NewContextManager(100, 3)
	msgs := []openrouter.ChatMessage{
		toolMsg("short"),   // only 5 chars — below 100 char threshold for masking
		toolMsg("recent1"),
		toolMsg("recent2"),
		toolMsg("recent3"),
	}
	out := cm.MaskOldObservations(msgs)
	if out[0].Content != "short" {
		t.Errorf("short tool message should not be masked, got: %q", out[0].Content)
	}
}

func TestContextManager_MaskOld_NonToolRoleNotMasked(t *testing.T) {
	cm := NewContextManager(100, 3)
	longContent := strings.Repeat("y", 200)
	msgs := []openrouter.ChatMessage{
		makeMsg("user", longContent),   // not a tool message
		toolMsg("recent1"),
		toolMsg("recent2"),
		toolMsg("recent3"),
	}
	out := cm.MaskOldObservations(msgs)
	if out[0].Content != longContent {
		t.Errorf("non-tool role should not be masked, got: %q", out[0].Content)
	}
}

func TestContextManager_MaskOld_PreservesOriginalSlice(t *testing.T) {
	cm := NewContextManager(100, 3)
	original := strings.Repeat("z", 200)
	msgs := []openrouter.ChatMessage{
		toolMsg(original),
		toolMsg("a"),
		toolMsg("b"),
		toolMsg("c"),
	}
	cm.MaskOldObservations(msgs)
	// original slice should not be mutated
	if msgs[0].Content != original {
		t.Error("MaskOldObservations should not mutate the original slice")
	}
}

func TestContextManager_MaskOld_TokenEstimateInOutput(t *testing.T) {
	cm := NewContextManager(100, 3)
	content := strings.Repeat("a", 400) // 400 chars → ~100 tokens
	msgs := []openrouter.ChatMessage{
		toolMsg(content),
		toolMsg("b"),
		toolMsg("c"),
		toolMsg("d"),
	}
	out := cm.MaskOldObservations(msgs)
	if !strings.Contains(out[0].Content, "100") {
		t.Errorf("expected token estimate in masked content, got: %q", out[0].Content)
	}
}

// CompactIfNeeded

func TestContextManager_Compact_BelowMax_NoChange(t *testing.T) {
	cm := NewContextManager(10, 5)
	msgs := []openrouter.ChatMessage{
		makeMsg("user", "hi"),
		makeMsg("assistant", "hello"),
	}
	out := cm.CompactIfNeeded(msgs, "notes")
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
}

func TestContextManager_Compact_AboveMax_PrependsHandoff(t *testing.T) {
	cm := NewContextManager(3, 5)
	msgs := make([]openrouter.ChatMessage, 5)
	for i := range msgs {
		msgs[i] = makeMsg("user", "msg")
	}
	out := cm.CompactIfNeeded(msgs, "important notes here")
	// should have handoff + last 3 messages
	if len(out) != 4 {
		t.Fatalf("expected 4 messages (handoff + 3 kept), got %d", len(out))
	}
	if out[0].Role != "user" {
		t.Errorf("handoff message should be role=user, got %q", out[0].Role)
	}
	if !strings.Contains(out[0].Content, "important notes here") {
		t.Errorf("handoff message should contain notes, got: %q", out[0].Content)
	}
}

func TestContextManager_Compact_EmptyNotes_StillWorks(t *testing.T) {
	cm := NewContextManager(2, 5)
	msgs := make([]openrouter.ChatMessage, 4)
	for i := range msgs {
		msgs[i] = makeMsg("assistant", "response")
	}
	out := cm.CompactIfNeeded(msgs, "")
	if len(out) != 3 { // handoff + 2 kept
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
}

// DiffFileResult

func TestContextManager_Diff_FirstRead_ReturnsFullContent(t *testing.T) {
	cm := NewContextManager(100, 10)
	result := cm.DiffFileResult("read_file", "/path/to/file.go", "line1\nline2\n")
	if result != "line1\nline2\n" {
		t.Errorf("first read should return full content, got: %q", result)
	}
}

func TestContextManager_Diff_Unchanged_ReturnsMarker(t *testing.T) {
	cm := NewContextManager(100, 10)
	cm.DiffFileResult("read_file", "/f.go", "same content")
	result := cm.DiffFileResult("read_file", "/f.go", "same content")
	if result != "[unchanged since last read]" {
		t.Errorf("expected unchanged marker, got: %q", result)
	}
}

func TestContextManager_Diff_SmallChange_ReturnsDiff(t *testing.T) {
	cm := NewContextManager(100, 10)
	cm.DiffFileResult("read_file", "/f.go", "line1\nline2\nline3\n")
	result := cm.DiffFileResult("read_file", "/f.go", "line1\nXXXX\nline3\n")
	if !strings.Contains(result, "L2") {
		t.Errorf("expected diff showing L2 change, got: %q", result)
	}
	if !strings.Contains(result, "line2") {
		t.Errorf("expected old value in diff, got: %q", result)
	}
}

func TestContextManager_Diff_LargeChange_ReturnsFullContent(t *testing.T) {
	cm := NewContextManager(100, 10)
	// create a 20-line file
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line")
	}
	original := strings.Join(lines, "\n")

	// change all 20 lines
	var newLines []string
	for i := 0; i < 20; i++ {
		newLines = append(newLines, "changed")
	}
	modified := strings.Join(newLines, "\n")

	cm.DiffFileResult("read_file", "/big.go", original)
	result := cm.DiffFileResult("read_file", "/big.go", modified)

	// more than 10 changes → should return full content
	if strings.Contains(result, "lines changed") {
		t.Errorf("large change should return full content, not diff: %q", result)
	}
	if result != modified {
		t.Errorf("expected full modified content, got something else")
	}
}

func TestContextManager_Diff_DifferentTools_IndependentCache(t *testing.T) {
	cm := NewContextManager(100, 10)
	cm.DiffFileResult("read_file", "/f.go", "v1")
	// same path, different tool name — should be a first-read
	result := cm.DiffFileResult("other_tool", "/f.go", "v1")
	if result != "v1" {
		t.Errorf("different tool should have independent cache, got: %q", result)
	}
}

// FileHash

func TestContextManager_FileHash_Deterministic(t *testing.T) {
	cm := NewContextManager(100, 10)
	h1 := cm.FileHash("hello world")
	h2 := cm.FileHash("hello world")
	if h1 != h2 {
		t.Error("FileHash should be deterministic")
	}
}

func TestContextManager_FileHash_DifferentContent(t *testing.T) {
	cm := NewContextManager(100, 10)
	h1 := cm.FileHash("hello")
	h2 := cm.FileHash("world")
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestContextManager_FileHash_NonEmpty(t *testing.T) {
	cm := NewContextManager(100, 10)
	h := cm.FileHash("content")
	if h == "" {
		t.Error("FileHash should return non-empty string")
	}
}

// estimateTokens helper

func TestEstimateTokens(t *testing.T) {
	// 400 chars → 100 tokens
	n := estimateTokens(strings.Repeat("a", 400))
	if n != 100 {
		t.Errorf("expected 100, got %d", n)
	}
}

// truncate helper

func TestTruncate_ShortString(t *testing.T) {
	out := truncate("hi", 10)
	if out != "hi" {
		t.Errorf("expected 'hi', got %q", out)
	}
}

func TestTruncate_LongString(t *testing.T) {
	out := truncate("hello world", 5)
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected '...' suffix, got %q", out)
	}
	if len(out) > 8 {
		t.Errorf("truncated string too long: %q", out)
	}
}
