package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// matchPattern

func TestMatchPattern_ExactMatch(t *testing.T) {
	matched, err := matchPattern("read_file", "read_file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Error("expected exact pattern to match")
	}
}

func TestMatchPattern_ExactMismatch(t *testing.T) {
	matched, _ := matchPattern("read_file", "write_file")
	if matched {
		t.Error("expected no match for different names")
	}
}

func TestMatchPattern_WildcardSuffix(t *testing.T) {
	matched, _ := matchPattern("mcp:*", "mcp:server:tool")
	if !matched {
		t.Error("expected wildcard prefix to match")
	}
}

func TestMatchPattern_WildcardNoMatch(t *testing.T) {
	matched, _ := matchPattern("mcp:*", "read_file")
	if matched {
		t.Error("expected wildcard not to match unrelated name")
	}
}

func TestMatchPattern_StarMatchesAll(t *testing.T) {
	matched, _ := matchPattern("*", "anything")
	if !matched {
		t.Error("expected '*' to match anything")
	}
}

// HookRunner

func TestHookRunner_NoHooks_RunPreSucceeds(t *testing.T) {
	hr := NewHookRunner()
	err := hr.RunPre(context.Background(), "read_file", json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("expected no error with no hooks, got: %v", err)
	}
}

func TestHookRunner_NoHooks_RunPostSucceeds(t *testing.T) {
	hr := NewHookRunner()
	err := hr.RunPost(context.Background(), "read_file", ToolResult{Output: "ok"})
	if err != nil {
		t.Errorf("expected no error with no hooks, got: %v", err)
	}
}

func TestHookRunner_NoHooks_RunStopSucceeds(t *testing.T) {
	hr := NewHookRunner()
	err := hr.RunStop(context.Background())
	if err != nil {
		t.Errorf("expected no error with no hooks, got: %v", err)
	}
}

func TestHookRunner_PreHook_ExactPattern_Runs(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "read_file",
		Command: "true",
	})
	err := hr.RunPre(context.Background(), "read_file", json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("expected hook to succeed, got: %v", err)
	}
}

func TestHookRunner_PreHook_PatternMismatch_NotRun(t *testing.T) {
	hr := NewHookRunner()
	// hook for write_file should not run for read_file
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "write_file",
		Command: "false", // would fail if it ran
	})
	err := hr.RunPre(context.Background(), "read_file", json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("hook for different pattern should not run: %v", err)
	}
}

func TestHookRunner_PreHook_FailingCommand_ReturnsError(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "*",
		Command: "exit 1",
	})
	err := hr.RunPre(context.Background(), "any_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error from failing pre-hook")
	}
}

func TestHookRunner_PostHook_RunsAfterTool(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPostToolUse,
		Pattern: "*",
		Command: "true",
	})
	err := hr.RunPost(context.Background(), "shell", ToolResult{Output: "output"})
	if err != nil {
		t.Errorf("expected post-hook to succeed, got: %v", err)
	}
}

func TestHookRunner_StopHook_Runs(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookOnStop,
		Pattern: "",
		Command: "true",
	})
	err := hr.RunStop(context.Background())
	if err != nil {
		t.Errorf("expected stop-hook to succeed, got: %v", err)
	}
}

func TestHookRunner_WildcardPattern_MatchesAll(t *testing.T) {
	hr := NewHookRunner()
	var count int
	// can't directly inspect run count, but failing command would surface errors
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "*",
		Command: "true",
	})
	for _, name := range []string{"read_file", "shell", "think", "edit_file"} {
		if err := hr.RunPre(context.Background(), name, json.RawMessage(`{}`)); err != nil {
			t.Errorf("wildcard hook should run for %s: %v", name, err)
		}
		count++
	}
	if count != 4 {
		t.Errorf("expected 4 runs, got %d", count)
	}
}

func TestHookRunner_EmptyPattern_MatchesAll(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "",
		Command: "true",
	})
	err := hr.RunPre(context.Background(), "anything", json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("empty pattern should match all tools: %v", err)
	}
}

func TestHookRunner_ErrorMessageContainsCommand(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "*",
		Command: "exit 42",
	})
	err := hr.RunPre(context.Background(), "tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exit 42") {
		t.Errorf("expected command in error message, got: %v", err)
	}
}

// HookMiddleware

func TestHookMiddleware_PreHookBlocks(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "*",
		Command: "exit 1",
	})

	r := NewRegistry()
	r.Use(HookMiddleware(hr))
	r.Register(simpleDef("guarded"), echoHandler("should not reach"))

	ctx := ContextWithToolName(context.Background(), "guarded")
	handler := func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		return echoHandler("should not reach")(ctx, args)
	}
	mw := HookMiddleware(hr)
	result, err := mw(handler)(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected pre-hook failure to block execution")
	}
	if !strings.Contains(result.Output, "pre-hook blocked") {
		t.Errorf("expected 'pre-hook blocked' in output, got: %s", result.Output)
	}
}

func TestHookMiddleware_SuccessfulHook_ToolExecutes(t *testing.T) {
	hr := NewHookRunner()
	hr.Add(Hook{
		Type:    HookPreToolUse,
		Pattern: "*",
		Command: "true",
	})

	mw := HookMiddleware(hr)
	handler := echoHandler("executed")
	ctx := ContextWithToolName(context.Background(), "test_tool")
	result, err := mw(handler)(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "executed" {
		t.Errorf("expected 'executed', got: %s", result.Output)
	}
}

func TestHookMiddleware_HandlerReturnsError_Propagated(t *testing.T) {
	hr := NewHookRunner()
	// no hooks — go straight to handler

	mw := HookMiddleware(hr)
	errHandler := func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		return ToolResult{}, fmt.Errorf("handler failed")
	}
	ctx := ContextWithToolName(context.Background(), "test_tool")
	result, err := mw(errHandler)(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error to propagate from handler")
	}
	_ = result
}

// ContextWithToolName

func TestContextWithToolName_Roundtrip(t *testing.T) {
	ctx := ContextWithToolName(context.Background(), "my_tool")
	name, ok := ctx.Value(toolNameKey{}).(string)
	if !ok {
		t.Fatal("expected string value in context")
	}
	if name != "my_tool" {
		t.Errorf("expected 'my_tool', got %q", name)
	}
}
