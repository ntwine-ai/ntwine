package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecuteParallel_AllResultsReturned(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("tool_a"), echoHandler("a"))
	r.Register(simpleDef("tool_b"), echoHandler("b"))
	r.Register(simpleDef("tool_c"), echoHandler("c"))

	calls := []ToolCall{
		{ID: "1", Name: "tool_a", Args: "{}"},
		{ID: "2", Name: "tool_b", Args: "{}"},
		{ID: "3", Name: "tool_c", Args: "{}"},
	}

	results := r.ExecuteParallel(context.Background(), calls)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestExecuteParallel_PreservesOrder(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("tool_%d", i)
		output := fmt.Sprintf("result_%d", i)
		r.Register(simpleDef(name), echoHandler(output))
	}

	calls := make([]ToolCall, 5)
	for i := 0; i < 5; i++ {
		calls[i] = ToolCall{
			ID:   fmt.Sprintf("%d", i),
			Name: fmt.Sprintf("tool_%d", i),
			Args: "{}",
		}
	}

	results := r.ExecuteParallel(context.Background(), calls)

	for i, res := range results {
		expected := fmt.Sprintf("result_%d", i)
		if res.Result.Output != expected {
			t.Errorf("index %d: expected %q, got %q", i, expected, res.Result.Output)
		}
		if res.Index != i {
			t.Errorf("index %d: expected Index=%d, got %d", i, i, res.Index)
		}
	}
}

func TestExecuteParallel_IndexSetCorrectly(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("t"), echoHandler("ok"))

	calls := []ToolCall{
		{Name: "t", Args: "{}"},
		{Name: "t", Args: "{}"},
		{Name: "t", Args: "{}"},
	}

	results := r.ExecuteParallel(context.Background(), calls)
	for i, res := range results {
		if res.Index != i {
			t.Errorf("expected Index=%d, got %d", i, res.Index)
		}
	}
}

func TestExecuteParallel_NameSetCorrectly(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("alpha"), echoHandler(""))
	r.Register(simpleDef("beta"), echoHandler(""))

	calls := []ToolCall{
		{Name: "alpha", Args: "{}"},
		{Name: "beta", Args: "{}"},
	}

	results := r.ExecuteParallel(context.Background(), calls)
	if results[0].Name != "alpha" {
		t.Errorf("expected Name='alpha', got %q", results[0].Name)
	}
	if results[1].Name != "beta" {
		t.Errorf("expected Name='beta', got %q", results[1].Name)
	}
}

func TestExecuteParallel_UnknownTool_ErrorResult(t *testing.T) {
	r := NewRegistry()
	calls := []ToolCall{
		{Name: "nonexistent", Args: "{}"},
	}

	results := r.ExecuteParallel(context.Background(), calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Result.IsError {
		t.Error("expected IsError=true for unknown tool")
	}
}

func TestExecuteParallel_EmptyCalls(t *testing.T) {
	r := NewRegistry()
	results := r.ExecuteParallel(context.Background(), nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil calls, got %d", len(results))
	}
}

func TestExecuteParallel_ActuallyRunsConcurrently(t *testing.T) {
	r := NewRegistry()

	// tool that sleeps 50ms — 3 of them should finish in ~50ms total if concurrent
	r.Register(simpleDef("slow"), func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		time.Sleep(50 * time.Millisecond)
		return ToolResult{Output: "done"}, nil
	})

	calls := []ToolCall{
		{Name: "slow", Args: "{}"},
		{Name: "slow", Args: "{}"},
		{Name: "slow", Args: "{}"},
	}

	start := time.Now()
	results := r.ExecuteParallel(context.Background(), calls)
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// if sequential it would take 150ms; parallel should be < 120ms
	if elapsed > 120*time.Millisecond {
		t.Errorf("expected parallel execution in ~50ms, took %v", elapsed)
	}
}

func TestExecuteParallel_CounterConcurrencyNoPanic(t *testing.T) {
	r := NewRegistry()
	var callCount atomic.Int64

	r.Register(simpleDef("counter"), func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		callCount.Add(1)
		return ToolResult{Output: "counted"}, nil
	})

	calls := make([]ToolCall, 20)
	for i := range calls {
		calls[i] = ToolCall{Name: "counter", Args: "{}"}
	}

	results := r.ExecuteParallel(context.Background(), calls)

	if len(results) != 20 {
		t.Errorf("expected 20 results, got %d", len(results))
	}
	if callCount.Load() != 20 {
		t.Errorf("expected 20 calls, got %d", callCount.Load())
	}
}

func TestExecuteParallel_MixedSuccessAndError(t *testing.T) {
	r := NewRegistry()
	r.Register(simpleDef("good"), echoHandler("ok"))

	calls := []ToolCall{
		{Name: "good", Args: "{}"},
		{Name: "bad", Args: "{}"}, // unknown
		{Name: "good", Args: "{}"},
	}

	results := r.ExecuteParallel(context.Background(), calls)

	if results[0].Result.IsError {
		t.Error("results[0] should succeed")
	}
	if !results[1].Result.IsError {
		t.Error("results[1] should error (unknown tool)")
	}
	if results[2].Result.IsError {
		t.Error("results[2] should succeed")
	}
}
