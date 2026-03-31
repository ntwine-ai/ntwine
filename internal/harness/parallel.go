package harness

import (
	"context"
	"sync"
)

type ParallelResult struct {
	Name   string
	Result ToolResult
	Index  int
}

func (r *Registry) ExecuteParallel(ctx context.Context, calls []ToolCall) []ParallelResult {
	results := make([]ParallelResult, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c ToolCall) {
			defer wg.Done()

			result, _ := r.Execute(ctx, c.Name, c.Args)
			results[idx] = ParallelResult{
				Name:   c.Name,
				Result: result,
				Index:  idx,
			}
		}(i, call)
	}

	wg.Wait()
	return results
}

type ToolCall struct {
	ID   string
	Name string
	Args string
}
