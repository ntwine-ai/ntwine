package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type HookType int

const (
	HookPreToolUse HookType = iota
	HookPostToolUse
	HookOnStop
)

type Hook struct {
	Type    HookType
	Pattern string
	Command string
}

type HookRunner struct {
	mu    sync.RWMutex
	hooks []Hook
}

func NewHookRunner() *HookRunner {
	return &HookRunner{}
}

func (hr *HookRunner) Add(h Hook) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.hooks = append(hr.hooks, h)
}

func (hr *HookRunner) RunPre(ctx context.Context, toolName string, args json.RawMessage) error {
	return hr.run(ctx, HookPreToolUse, toolName, string(args), "")
}

func (hr *HookRunner) RunPost(ctx context.Context, toolName string, result ToolResult) error {
	return hr.run(ctx, HookPostToolUse, toolName, "", result.Output)
}

func (hr *HookRunner) RunStop(ctx context.Context) error {
	return hr.run(ctx, HookOnStop, "", "", "")
}

func (hr *HookRunner) run(ctx context.Context, hookType HookType, toolName, input, output string) error {
	hr.mu.RLock()
	hooks := make([]Hook, len(hr.hooks))
	copy(hooks, hr.hooks)
	hr.mu.RUnlock()

	for _, h := range hooks {
		if h.Type != hookType {
			continue
		}

		if h.Pattern != "" && h.Pattern != "*" && h.Pattern != toolName {
			matched, _ := matchPattern(h.Pattern, toolName)
			if !matched {
				continue
			}
		}

		hookCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		cmd := exec.CommandContext(hookCtx, "sh", "-c", h.Command)
		cmd.Env = append(cmd.Environ(),
			fmt.Sprintf("NTWINE_TOOL=%s", toolName),
			fmt.Sprintf("NTWINE_HOOK=%d", hookType),
		)

		if input != "" {
			cmd.Stdin = strings.NewReader(input)
		}

		out, err := cmd.CombinedOutput()
		cancel()

		if err != nil {
			return fmt.Errorf("hook failed (%s): %s: %w", h.Command, string(out), err)
		}
	}

	return nil
}

func matchPattern(pattern, name string) (bool, error) {
	if strings.Contains(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix), nil
	}
	return pattern == name, nil
}

func HookMiddleware(runner *HookRunner) MiddlewareFunc {
	return func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			toolName := ctx.Value(toolNameKey{})
			name, _ := toolName.(string)

			if err := runner.RunPre(ctx, name, args); err != nil {
				return ToolResult{
					Output:  fmt.Sprintf("pre-hook blocked: %s", err.Error()),
					IsError: true,
				}, nil
			}

			result, err := next(ctx, args)
			if err != nil {
				return result, err
			}

			_ = runner.RunPost(ctx, name, result)

			return result, nil
		}
	}
}

type toolNameKey struct{}

func ContextWithToolName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, toolNameKey{}, name)
}
