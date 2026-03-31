package harness

import (
	"context"
	"encoding/json"
	"fmt"
)

func RegisterThinkTool(r *Registry) {
	r.Register(ToolDef{
		Name:             "think",
		Description:      "use this to stop and reason through something before acting. call this before: critical architecture decisions, git operations, changing your approach, or when you're about to say you're done. this is private - other agents dont see your thinking.",
		ModelDescription: "think through a problem. output is private. use before critical decisions.",
		Source:           "builtin",
		ReadOnly:         true,
		Parameters: params(
			prop("thought", "string", "your reasoning"),
			required("thought"),
		),
	}, func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Thought string `json:"thought"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{
			Output:   fmt.Sprintf("[thought recorded, %d chars]", len(args.Thought)),
			Metadata: map[string]string{"thought": args.Thought},
		}, nil
	})
}
