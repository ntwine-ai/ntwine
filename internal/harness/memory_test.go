package harness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMemoryStore_SaveAndLoad(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	err := store.Save("my-key", "my value", "agent-1")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	mem, found := store.Load("my-key")
	if !found {
		t.Fatal("expected to find saved memory")
	}
	if mem.Key != "my-key" {
		t.Errorf("expected key 'my-key', got %q", mem.Key)
	}
	if mem.Value != "my value" {
		t.Errorf("expected value 'my value', got %q", mem.Value)
	}
	if mem.CreatedBy != "agent-1" {
		t.Errorf("expected CreatedBy 'agent-1', got %q", mem.CreatedBy)
	}
	if mem.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestMemoryStore_Load_NotFound(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	_, found := store.Load("nonexistent")
	if found {
		t.Error("expected found=false for missing key")
	}
}

func TestMemoryStore_Save_OverwritesExisting(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	store.Save("key", "v1", "a")
	store.Save("key", "v2", "b")

	mem, _ := store.Load("key")
	if mem.Value != "v2" {
		t.Errorf("expected overwritten value 'v2', got %q", mem.Value)
	}
}

func TestMemoryStore_All_Empty(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	memories := store.All()
	if len(memories) != 0 {
		t.Errorf("expected empty, got %d entries", len(memories))
	}
}

func TestMemoryStore_All_Multiple(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	store.Save("alpha", "1", "agent")
	store.Save("beta", "2", "agent")
	store.Save("gamma", "3", "agent")

	memories := store.All()
	if len(memories) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(memories))
	}
}

func TestMemoryStore_KeySanitization_SlashReplaced(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	err := store.Save("path/to/key", "value", "agent")
	if err != nil {
		t.Fatalf("Save with slash in key failed: %v", err)
	}
	_, found := store.Load("path/to/key")
	if !found {
		t.Error("should be able to load key with slashes")
	}
}

func TestMemoryStore_KeySanitization_SpecialChars(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	specialKey := `key:with*special?chars"and<>|more`
	err := store.Save(specialKey, "val", "agent")
	if err != nil {
		t.Fatalf("Save with special chars failed: %v", err)
	}
	_, found := store.Load(specialKey)
	if !found {
		t.Error("should be able to load key with special chars")
	}
}

func TestMemoryStore_Timestamps_Recorded(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	before := time.Now()
	store.Save("ts-key", "val", "agent")
	after := time.Now()

	mem, _ := store.Load("ts-key")
	if mem.CreatedAt.Before(before) || mem.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v is outside expected range [%v, %v]", mem.CreatedAt, before, after)
	}
}

// RegisterMemoryTool via registry

func TestMemoryTool_CreateAndRecall(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	store := NewMemoryStore(dir)
	RegisterMemoryTool(r, store)

	// create
	raw, _ := json.Marshal(map[string]string{"key": "project_lang", "value": "Go"})
	result, err := r.Execute(context.Background(), "create_memory", string(raw))
	if err != nil || result.IsError {
		t.Fatalf("create_memory failed: err=%v output=%s", err, result.Output)
	}
	if !strings.Contains(result.Output, "project_lang") {
		t.Errorf("expected key in output, got: %s", result.Output)
	}

	// recall
	raw2, _ := json.Marshal(map[string]string{"key": "project_lang"})
	result2, err2 := r.Execute(context.Background(), "recall_memory", string(raw2))
	if err2 != nil || result2.IsError {
		t.Fatalf("recall_memory failed: err=%v output=%s", err2, result2.Output)
	}
	if !strings.Contains(result2.Output, "Go") {
		t.Errorf("expected 'Go' in recalled memory, got: %s", result2.Output)
	}
}

func TestMemoryTool_RecallMissing(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	store := NewMemoryStore(dir)
	RegisterMemoryTool(r, store)

	raw, _ := json.Marshal(map[string]string{"key": "does_not_exist"})
	result, _ := r.Execute(context.Background(), "recall_memory", string(raw))
	if !strings.Contains(result.Output, "no memory found") {
		t.Errorf("expected 'no memory found', got: %s", result.Output)
	}
}

func TestMemoryTool_ListMemories_Empty(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	store := NewMemoryStore(dir)
	RegisterMemoryTool(r, store)

	result, _ := r.Execute(context.Background(), "list_memories", "{}")
	if !strings.Contains(result.Output, "no memories") {
		t.Errorf("expected 'no memories' message, got: %s", result.Output)
	}
}

func TestMemoryTool_ListMemories_ShowsAll(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	store := NewMemoryStore(dir)
	RegisterMemoryTool(r, store)

	r.Execute(context.Background(), "create_memory", `{"key":"k1","value":"v1"}`)
	r.Execute(context.Background(), "create_memory", `{"key":"k2","value":"v2"}`)

	result, _ := r.Execute(context.Background(), "list_memories", "{}")
	if !strings.Contains(result.Output, "k1") || !strings.Contains(result.Output, "k2") {
		t.Errorf("expected both keys in list, got: %s", result.Output)
	}
}

func TestMemoryTool_CreateMemory_EmptyKey(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	store := NewMemoryStore(dir)
	RegisterMemoryTool(r, store)

	_, err := r.Execute(context.Background(), "create_memory", `{"key":"","value":"val"}`)
	// should return an error (either via result or err)
	_ = err
	// just checking it doesn't panic
}

func TestMemoryTool_CreateMemory_EmptyValue(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	r := NewRegistry()
	store := NewMemoryStore(dir)
	RegisterMemoryTool(r, store)

	_, err := r.Execute(context.Background(), "create_memory", `{"key":"k","value":""}`)
	_ = err
	// should not panic
}

func TestMemoryStore_All_SkipsCorruptJSON(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	store.Save("valid", "value", "agent")

	// write corrupt JSON directly into the memories dir
	corruptPath := store.dirPath + "/corrupt.json"
	os.WriteFile(corruptPath, []byte("not valid json {{{"), 0644)

	memories := store.All()
	// should have 1 valid memory, corrupt one silently skipped
	if len(memories) != 1 {
		t.Errorf("expected 1 valid memory (corrupt skipped), got %d", len(memories))
	}
}

func TestMemoryStore_Load_InvalidKey_ReturnsFalse(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	// ".." is an invalid key per sanitizeKey
	_, found := store.Load("..")
	if found {
		t.Error("invalid key should return found=false")
	}
}

func TestMemoryStore_Save_InvalidKey_ReturnsError(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	err := store.Save("..", "value", "agent")
	if err == nil {
		t.Error("expected error for invalid key '..'")
	}
}

func TestMemoryStore_Load_CorruptJSON_ReturnsFalse(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	store := NewMemoryStore(dir)
	os.WriteFile(store.dirPath+"/badkey.json", []byte("not json"), 0644)

	_, found := store.Load("badkey")
	if found {
		t.Error("corrupt JSON should result in found=false")
	}
}
