package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Memory struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type MemoryStore struct {
	dirPath string
}

func NewMemoryStore(codebasePath string) *MemoryStore {
	dir := filepath.Join(codebasePath, ".ntwine", "memories")
	os.MkdirAll(dir, 0755)
	return &MemoryStore{dirPath: dir}
}

func (ms *MemoryStore) Save(key, value, createdBy string) error {
	mem := Memory{
		Key:       key,
		Value:     value,
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		return err
	}

	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, key)

	return os.WriteFile(filepath.Join(ms.dirPath, safe+".json"), data, 0644)
}

func (ms *MemoryStore) Load(key string) (Memory, bool) {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, key)

	data, err := os.ReadFile(filepath.Join(ms.dirPath, safe+".json"))
	if err != nil {
		return Memory{}, false
	}

	var mem Memory
	if err := json.Unmarshal(data, &mem); err != nil {
		return Memory{}, false
	}
	return mem, true
}

func (ms *MemoryStore) All() []Memory {
	entries, err := os.ReadDir(ms.dirPath)
	if err != nil {
		return nil
	}

	var memories []Memory
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ms.dirPath, e.Name()))
		if err != nil {
			continue
		}
		var mem Memory
		if err := json.Unmarshal(data, &mem); err != nil {
			continue
		}
		memories = append(memories, mem)
	}
	return memories
}

func RegisterMemoryTool(r *Registry, store *MemoryStore) {
	r.Register(ToolDef{
		Name:             "create_memory",
		Description:      "save important context for future sessions. use this to remember project decisions, user preferences, codebase patterns, or anything that would be useful later.",
		ModelDescription: "save a key-value memory that persists across sessions",
		Source:           "builtin",
		Parameters: params(
			prop("key", "string", "short descriptive key for this memory"),
			prop("value", "string", "the information to remember"),
			required("key", "value"),
		),
	}, func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		if args.Key == "" || args.Value == "" {
			return ToolResult{}, fmt.Errorf("key and value are required")
		}
		if err := store.Save(args.Key, args.Value, "agent"); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Output: "memory saved: " + args.Key}, nil
	})

	r.Register(ToolDef{
		Name:             "recall_memory",
		Description:      "retrieve a previously saved memory by key",
		ModelDescription: "retrieve a saved memory",
		Source:           "builtin",
		Parameters: params(
			prop("key", "string", "the key to look up"),
			required("key"),
		),
	}, func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}
		mem, found := store.Load(args.Key)
		if !found {
			return ToolResult{Output: "no memory found for key: " + args.Key}, nil
		}
		return ToolResult{Output: fmt.Sprintf("[%s] %s (saved by %s at %s)", mem.Key, mem.Value, mem.CreatedBy, mem.CreatedAt.Format("2006-01-02 15:04"))}, nil
	})

	r.Register(ToolDef{
		Name:             "list_memories",
		Description:      "list all saved memories",
		ModelDescription: "list all saved memories",
		Source:           "builtin",
		Parameters:       params(),
	}, func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		memories := store.All()
		if len(memories) == 0 {
			return ToolResult{Output: "no memories saved yet"}, nil
		}
		var sb strings.Builder
		for _, m := range memories {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Key, m.Value))
		}
		return ToolResult{Output: sb.String()}, nil
	})
}
