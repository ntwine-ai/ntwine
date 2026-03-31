package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// helpers

func echoHandler(msg string) ToolHandlerFunc {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		return ToolResult{Output: msg}, nil
	}
}

func errorHandler() ToolHandlerFunc {
	return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		return ToolResult{}, fmt.Errorf("handler error")
	}
}

func simpleDef(name string) ToolDef {
	return ToolDef{Name: name, Description: "test tool"}
}

// Registry basics

func TestRegistry_RegisterAndHas(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("my_tool"), echoHandler("ok"))
	if !r.Has("my_tool") {
		t.Fatal("expected Has to return true after Register")
	}
	if r.Has("nonexistent") {
		t.Fatal("expected Has to return false for unknown tool")
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Fatalf("expected 0, got %d", r.Count())
	}
	r.Register(simpleDef("a"), echoHandler(""))
	r.Register(simpleDef("b"), echoHandler(""))
	if r.Count() != 2 {
		t.Fatalf("expected 2, got %d", r.Count())
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("tool"), echoHandler(""))
	r.Remove("tool")
	if r.Has("tool") {
		t.Fatal("expected tool to be removed")
	}
	// removing non-existent should not panic
	r.Remove("nonexistent")
}

func TestRegistry_VersionBumpsOnRegister(t *testing.T) {
	r := NewRegistry()
	v0 := r.Version()
	r.Register(simpleDef("a"), echoHandler(""))
	v1 := r.Version()
	if v1 <= v0 {
		t.Fatal("version should increase after Register")
	}
	r.Remove("a")
	v2 := r.Version()
	if v2 <= v1 {
		t.Fatal("version should increase after Remove")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("x"), echoHandler(""))
	r.Register(simpleDef("y"), echoHandler(""))
	defs := r.List()
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
}

func TestRegistry_Definitions(t *testing.T) {
	r := NewRegistry()
	r.Register(ToolDef{
		Name:             "foo",
		Description:      "normal desc",
		ModelDescription: "model desc",
	}, echoHandler(""))
	defs := r.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition")
	}
	if defs[0].Function.Description != "model desc" {
		t.Errorf("expected ModelDescription to take priority, got %q", defs[0].Function.Description)
	}
}

func TestRegistry_Definitions_FallsBackToDescription(t *testing.T) {
	r := NewRegistry()
	r.Register(ToolDef{
		Name:        "bar",
		Description: "plain desc",
	}, echoHandler(""))
	defs := r.Definitions()
	if defs[0].Function.Description != "plain desc" {
		t.Errorf("expected plain desc, got %q", defs[0].Function.Description)
	}
}

// Execute

func TestRegistry_Execute_HappyPath(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("greet"), echoHandler("hello"))
	result, err := r.Execute(context.Background(), "greet", "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
	if result.IsError {
		t.Error("expected IsError=false")
	}
}

func TestRegistry_Execute_UnknownTool(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "missing", "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for unknown tool")
	}
}

func TestRegistry_Execute_HandlerError(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("bad"), errorHandler())
	result, err := r.Execute(context.Background(), "bad", "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when handler errors")
	}
	if result.Output != "handler error" {
		t.Errorf("expected handler error message, got %q", result.Output)
	}
}

func TestRegistry_Execute_EmptyArgs(t *testing.T) {
	r := NewRegistry()
	var got json.RawMessage
	r.Register(simpleDef("t"), func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		got = args
		return ToolResult{Output: "ok"}, nil
	})
	r.Execute(context.Background(), "t", "")
	if string(got) != "{}" {
		t.Errorf("expected empty argsJSON to become {}, got %q", string(got))
	}
}

// Permissions

func TestRegistry_Execute_PermDeny(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("secret"), echoHandler(""))
	r.SetPermissions(func(name string, args json.RawMessage) PermissionLevel {
		return PermDeny
	})
	result, err := r.Execute(context.Background(), "secret", "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when permission is denied")
	}
}

func TestRegistry_Execute_PermAllow(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("ok_tool"), echoHandler("allowed"))
	r.SetPermissions(func(name string, args json.RawMessage) PermissionLevel {
		return PermAllow
	})
	result, _ := r.Execute(context.Background(), "ok_tool", "{}")
	if result.Output != "allowed" {
		t.Errorf("expected allowed, got %q", result.Output)
	}
}

func TestRegistry_Execute_PermConfirm_DoesNotBlock(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("confirm_tool"), echoHandler("passed"))
	r.SetPermissions(func(name string, args json.RawMessage) PermissionLevel {
		return PermConfirm
	})
	result, _ := r.Execute(context.Background(), "confirm_tool", "{}")
	// PermConfirm is not PermDeny, so it should proceed
	if result.IsError {
		t.Error("PermConfirm should not block execution")
	}
}

// Middleware

func TestRegistry_Middleware_Wraps(t *testing.T) {
	r := NewRegistry()
	var calls []string

	r.Use(func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			calls = append(calls, "before")
			res, err := next(ctx, args)
			calls = append(calls, "after")
			return res, err
		}
	})

	r.Register(simpleDef("mw_tool"), func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		calls = append(calls, "handler")
		return ToolResult{Output: "done"}, nil
	})

	r.Execute(context.Background(), "mw_tool", "{}")

	if len(calls) != 3 || calls[0] != "before" || calls[1] != "handler" || calls[2] != "after" {
		t.Errorf("unexpected middleware call order: %v", calls)
	}
}

func TestRegistry_Middleware_MultipleChained(t *testing.T) {
	r := NewRegistry()
	var order []string

	r.Use(func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			order = append(order, "mw1")
			return next(ctx, args)
		}
	})
	r.Use(func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			order = append(order, "mw2")
			return next(ctx, args)
		}
	})

	r.Register(simpleDef("chained"), echoHandler(""))
	r.Execute(context.Background(), "chained", "{}")

	// middlewares added before Register wrap outer-first
	if len(order) != 2 {
		t.Fatalf("expected 2 middleware calls, got %v", order)
	}
}

// Middleware registered AFTER the tool should NOT wrap already-registered tools
func TestRegistry_Middleware_OnlyWrapsSubsequentRegistrations(t *testing.T) {
	r := NewRegistry()
	var called bool

	r.Register(simpleDef("early"), echoHandler(""))

	r.Use(func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			called = true
			return next(ctx, args)
		}
	})

	r.Execute(context.Background(), "early", "{}")

	if called {
		t.Error("middleware added after Register should not wrap already-registered tool")
	}
}

// Events

func TestRegistry_Events_FiredOnExecute(t *testing.T) {
	r := NewRegistry()
	var events []ToolEvent

	r.OnEvent(func(e ToolEvent) {
		events = append(events, e)
	})

	r.Register(simpleDef("ev_tool"), echoHandler("hi"))
	r.Execute(context.Background(), "ev_tool", "{}")

	if len(events) == 0 {
		t.Fatal("expected events to be emitted")
	}
	// should have start and complete
	types := map[EventType]bool{}
	for _, e := range events {
		types[e.Type] = true
	}
	if !types[EventToolStart] {
		t.Error("missing EventToolStart")
	}
	if !types[EventToolComplete] {
		t.Error("missing EventToolComplete")
	}
}

func TestRegistry_Events_ErrorEvent(t *testing.T) {
	r := NewRegistry()
	var events []ToolEvent
	r.OnEvent(func(e ToolEvent) {
		events = append(events, e)
	})

	r.Register(simpleDef("bad"), errorHandler())
	r.Execute(context.Background(), "bad", "{}")

	hasError := false
	for _, e := range events {
		if e.Type == EventToolError {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected EventToolError to be emitted on handler error")
	}
}

// Loop detector integration

func TestRegistry_LoopDetector_WarnThenAbort(t *testing.T) {
	r := NewRegistry()
	ld := NewLoopDetector()
	r.SetLoopDetector(ld)

	var warnings []ToolEvent
	r.OnEvent(func(e ToolEvent) {
		if e.Type == EventToolProgress {
			warnings = append(warnings, e)
		}
	})

	r.Register(simpleDef("repeat"), echoHandler("ok"))

	args := `{"x":1}`
	// calls 1, 2 — ok
	for i := 0; i < 2; i++ {
		res, _ := r.Execute(context.Background(), "repeat", args)
		if res.IsError {
			t.Fatalf("call %d should not error yet", i+1)
		}
	}

	// call 3 triggers warn
	r.Execute(context.Background(), "repeat", args)
	if len(warnings) == 0 {
		t.Error("expected a warn event at 3 identical calls")
	}

	// calls 4–4 escalate
	r.Execute(context.Background(), "repeat", args)

	// call 5 — ForceText
	res5, _ := r.Execute(context.Background(), "repeat", args)
	if !res5.IsError {
		t.Error("expected ForceText (IsError) at call 5")
	}

	// calls 6, 7
	r.Execute(context.Background(), "repeat", args)
	r.Execute(context.Background(), "repeat", args)

	// call 8 — Abort
	res8, _ := r.Execute(context.Background(), "repeat", args)
	if !res8.IsError {
		t.Error("expected Abort (IsError) at call 8")
	}
}

// Concurrent access

func TestRegistry_ConcurrentRegisterExecute(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("concurrent"), echoHandler("ok"))

	var wg sync.WaitGroup
	var errCount atomic.Int64

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.Execute(context.Background(), "concurrent", "{}")
			if err != nil || res.IsError {
				errCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if errCount.Load() != 0 {
		t.Errorf("expected 0 errors, got %d", errCount.Load())
	}
}

func TestRegistry_ContextWithToolName(t *testing.T) {
	r := NewRegistry()
	var capturedName string
	r.Register(simpleDef("named"), func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		v, _ := ctx.Value(toolNameKey{}).(string)
		capturedName = v
		return ToolResult{Output: "ok"}, nil
	})
	r.Execute(context.Background(), "named", "{}")
	if capturedName != "named" {
		t.Errorf("expected tool name in context, got %q", capturedName)
	}
}
