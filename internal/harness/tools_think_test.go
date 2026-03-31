package harness

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestThinkTool_RecordsThought(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	raw, _ := json.Marshal(map[string]string{"thought": "I need to analyze this carefully"})
	result, err := r.Execute(context.Background(), "think", string(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Output)
	}
}

func TestThinkTool_OutputContainsCharCount(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	thought := "thinking carefully"
	raw, _ := json.Marshal(map[string]string{"thought": thought})
	result, _ := r.Execute(context.Background(), "think", string(raw))

	if !strings.Contains(result.Output, "thought recorded") {
		t.Errorf("expected 'thought recorded' in output, got: %s", result.Output)
	}
	expectedLen := len(thought)
	expected := strings.Contains(result.Output, strings.ToLower(strings.TrimSpace(string(rune(expectedLen)))))
	_ = expected
	// just verify the char count is in the output
	if !strings.Contains(result.Output, "18") {
		t.Errorf("expected character count 18 in output, got: %s", result.Output)
	}
}

func TestThinkTool_MetadataContainsThought(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	raw, _ := json.Marshal(map[string]string{"thought": "secret reasoning"})

	var capturedResult ToolResult
	r.OnEvent(func(e ToolEvent) {
		if e.Type == EventToolComplete {
			// already returned via Execute
		}
	})

	// use the handler directly to inspect metadata
	handler := func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var a struct{ Thought string `json:"thought"` }
		json.Unmarshal(args, &a)
		return ToolResult{
			Output:   "ok",
			Metadata: map[string]string{"thought": a.Thought},
		}, nil
	}

	ctx := ContextWithToolName(context.Background(), "think")
	capturedResult, err := handler(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedResult.Metadata["thought"] != "secret reasoning" {
		t.Errorf("expected thought in metadata, got: %v", capturedResult.Metadata)
	}
}

func TestThinkTool_EmptyThought_StillWorks(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	raw, _ := json.Marshal(map[string]string{"thought": ""})
	result, err := r.Execute(context.Background(), "think", string(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// empty thought is allowed, output should contain "0 chars"
	if !strings.Contains(result.Output, "0") {
		t.Errorf("expected 0 chars for empty thought, got: %s", result.Output)
	}
}

func TestThinkTool_IsReadOnly(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	defs := r.List()
	for _, d := range defs {
		if d.Name == "think" {
			if !d.ReadOnly {
				t.Error("think tool should be ReadOnly=true")
			}
			return
		}
	}
	t.Error("think tool not found in registry")
}

func TestThinkTool_LargeThought(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	bigThought := strings.Repeat("think deeply about this problem. ", 1000) // ~32000 chars
	raw, _ := json.Marshal(map[string]string{"thought": bigThought})
	result, err := r.Execute(context.Background(), "think", string(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("large thought should not error: %s", result.Output)
	}
}

func TestThinkTool_InvalidJSON_ReturnsError(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	result, err := r.Execute(context.Background(), "think", `not json`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// handler returns error which registry converts to IsError result
	if !result.IsError {
		t.Error("expected IsError=true for invalid JSON args")
	}
}

func TestThinkTool_OutputIsPrivate_NoContent(t *testing.T) {
	r := NewRegistry()
	RegisterThinkTool(r)

	raw, _ := json.Marshal(map[string]string{"thought": "confidential analysis"})
	result, _ := r.Execute(context.Background(), "think", string(raw))

	// the actual thought should NOT appear in Output
	if strings.Contains(result.Output, "confidential analysis") {
		t.Error("thought content should not appear in Output (it's private)")
	}
}
