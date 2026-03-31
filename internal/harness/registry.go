package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ntwine-ai/ntwine/internal/openrouter"
)

type ToolResult struct {
	Output   string
	IsError  bool
	Metadata map[string]string
}

type ToolHandlerFunc func(ctx context.Context, args json.RawMessage) (ToolResult, error)

type ToolDef struct {
	Name             string
	Description      string
	ModelDescription string
	Parameters       map[string]interface{}
	Source           string
	Owner            string
	ReadOnly         bool
	Destructive      bool
	TokenBudget      int
}

type PermissionLevel int

const (
	PermAllow PermissionLevel = iota
	PermConfirm
	PermDeny
)

type PermissionFunc func(toolName string, args json.RawMessage) PermissionLevel

type registeredTool struct {
	def     ToolDef
	handler ToolHandlerFunc
}

type MiddlewareFunc func(ToolHandlerFunc) ToolHandlerFunc

type EventType int

const (
	EventToolStart EventType = iota
	EventToolProgress
	EventToolComplete
	EventToolError
)

type ToolEvent struct {
	Type     EventType
	ToolName string
	Title    string
	Content  string
	IsError  bool
}

type EventCallback func(ToolEvent)

type Registry struct {
	mu           sync.RWMutex
	tools        map[string]registeredTool
	cacheVersion atomic.Uint64
	middlewares   []MiddlewareFunc
	permissions  PermissionFunc
	denyRules    []DenyRule
	events       EventCallback
	loopDetector *LoopDetector
}

type DenyRule struct {
	Pattern string
	Reason  string
}

var hardcodedDenyRules = []DenyRule{
	{Pattern: `rm\s+(-rf|-fr)\s+/\s*$`, Reason: "recursive delete of root"},
	{Pattern: `rm\s+(-rf|-fr)\s+~/?\s*$`, Reason: "recursive delete of home"},
	{Pattern: `rm\s+(-rf|-fr)\s+\.\s*$`, Reason: "recursive delete of current directory"},
	{Pattern: `mkfs\.`, Reason: "format filesystem"},
	{Pattern: `dd\s+.*of=/dev/`, Reason: "raw device write"},
	{Pattern: `>\s*/dev/sd`, Reason: "overwrite block device"},
	{Pattern: `:(){.*};:`, Reason: "fork bomb"},
}

func NewRegistry() *Registry {
	return &Registry{
		tools:     make(map[string]registeredTool),
		denyRules: hardcodedDenyRules,
	}
}

func (r *Registry) Register(def ToolDef, handler ToolHandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	wrapped := handler
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		wrapped = r.middlewares[i](wrapped)
	}

	r.tools[def.Name] = registeredTool{
		def:     def,
		handler: wrapped,
	}
	r.cacheVersion.Add(1)
}

func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
	r.cacheVersion.Add(1)
}

func (r *Registry) Use(mw MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, mw)
}

func (r *Registry) SetPermissions(fn PermissionFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.permissions = fn
}

func (r *Registry) OnEvent(cb EventCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = cb
}

func (r *Registry) SetLoopDetector(ld *LoopDetector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loopDetector = ld
}

func (r *Registry) emit(evt ToolEvent) {
	r.mu.RLock()
	cb := r.events
	r.mu.RUnlock()
	if cb != nil {
		cb(evt)
	}
}

func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (ToolResult, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	permFn := r.permissions
	loopDet := r.loopDetector
	r.mu.RUnlock()

	if !ok {
		return ToolResult{
			Output:  fmt.Sprintf("unknown tool: %s", name),
			IsError: true,
		}, nil
	}

	if permFn != nil {
		level := permFn(name, json.RawMessage(argsJSON))
		if level == PermDeny {
			return ToolResult{
				Output:  fmt.Sprintf("tool %s is not allowed", name),
				IsError: true,
			}, nil
		}
	}

	if loopDet != nil {
		action := loopDet.Record(name, argsJSON)
		switch action {
		case LoopAbort:
			return ToolResult{
				Output:  "tool loop detected. you've called this with the same args too many times. try a completely different approach.",
				IsError: true,
			}, nil
		case LoopForceText:
			return ToolResult{
				Output:  "you're repeating yourself. stop calling tools and explain what you're trying to do instead.",
				IsError: true,
			}, nil
		case LoopWarn:
			r.emit(ToolEvent{
				Type:     EventToolProgress,
				ToolName: name,
				Content:  "possible loop detected, same tool+args called multiple times",
			})
		}
	}

	var raw json.RawMessage
	if argsJSON == "" || argsJSON == "{}" {
		raw = json.RawMessage("{}")
	} else {
		raw = json.RawMessage(argsJSON)
	}

	toolCtx := ContextWithToolName(ctx, name)

	r.emit(ToolEvent{
		Type:     EventToolStart,
		ToolName: name,
		Title:    tool.def.Description,
	})

	result, err := tool.handler(toolCtx, raw)

	if err != nil {
		r.emit(ToolEvent{
			Type:     EventToolError,
			ToolName: name,
			Content:  err.Error(),
			IsError:  true,
		})
		return ToolResult{
			Output:  err.Error(),
			IsError: true,
		}, nil
	}

	r.emit(ToolEvent{
		Type:     EventToolComplete,
		ToolName: name,
		Content:  result.Output,
		IsError:  result.IsError,
	})

	return result, nil
}

func (r *Registry) Definitions() []openrouter.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]openrouter.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		desc := t.def.Description
		if t.def.ModelDescription != "" {
			desc = t.def.ModelDescription
		}
		defs = append(defs, openrouter.ToolDefinition{
			Type: "function",
			Function: openrouter.FunctionDef{
				Name:        t.def.Name,
				Description: desc,
				Parameters:  t.def.Parameters,
			},
		})
	}
	return defs
}

func (r *Registry) List() []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.def)
	}
	return defs
}

func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

func (r *Registry) Version() uint64 {
	return r.cacheVersion.Load()
}
