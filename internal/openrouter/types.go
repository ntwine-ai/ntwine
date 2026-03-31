package openrouter

import "encoding/json"

type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Stream      bool             `json:"stream"`
	ToolChoice  string           `json:"tool_choice,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// UnmarshalJSON handles providers (e.g. Mistral) that return arguments as a
// parsed JSON object instead of a string.
func (fc *FunctionCall) UnmarshalJSON(data []byte) error {
	type raw struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	fc.Name = r.Name

	if len(r.Arguments) == 0 {
		fc.Arguments = "{}"
		return nil
	}

	// If it's a JSON string, unwrap the quotes; otherwise keep the raw JSON.
	var s string
	if err := json.Unmarshal(r.Arguments, &s); err == nil {
		fc.Arguments = s
	} else {
		fc.Arguments = string(r.Arguments)
	}
	return nil
}

type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type StreamChunk struct {
	ID      string         `json:"id"`
	Choices []StreamChoice `json:"choices"`
}

type StreamChoice struct {
	Delta        StreamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type StreamToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

type StreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []StreamToolCall `json:"tool_calls,omitempty"`
}

type ChatResponse struct {
	ID      string           `json:"id"`
	Choices []ResponseChoice `json:"choices"`
}

type ResponseChoice struct {
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}
