package harness

import (
	"testing"
)

func TestNewWorktreeManager_Initializes(t *testing.T) {
	wm := NewWorktreeManager("/some/repo")
	if wm == nil {
		t.Fatal("expected non-nil WorktreeManager")
	}
}

func TestWorktreeManager_Path_NotFound(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	_, exists := wm.Path("nonexistent-agent")
	if exists {
		t.Error("expected exists=false for unknown agent")
	}
}

func TestWorktreeManager_List_Empty(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	list := wm.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d entries", len(list))
	}
}

func TestWorktreeManager_Remove_NonexistentAgent_NoError(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	err := wm.Remove("nonexistent")
	if err != nil {
		t.Errorf("removing nonexistent agent should not error, got: %v", err)
	}
}

func TestWorktreeManager_List_ReturnsSnapshot(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	// directly insert into the map to test List without running git
	wm.mu.Lock()
	wm.worktrees["agent-x"] = "/path/to/worktree"
	wm.mu.Unlock()

	list := wm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list["agent-x"] != "/path/to/worktree" {
		t.Errorf("expected '/path/to/worktree', got %q", list["agent-x"])
	}
}

func TestWorktreeManager_List_IsACopy(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	wm.mu.Lock()
	wm.worktrees["a"] = "/wa"
	wm.mu.Unlock()

	list := wm.List()
	list["a"] = "mutated"

	// original should be unchanged
	wm.mu.Lock()
	orig := wm.worktrees["a"]
	wm.mu.Unlock()

	if orig != "/wa" {
		t.Error("List() should return a copy, not the original map")
	}
}

func TestWorktreeManager_Path_Found(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	wm.mu.Lock()
	wm.worktrees["agent-y"] = "/worktrees/agent-y"
	wm.mu.Unlock()

	path, exists := wm.Path("agent-y")
	if !exists {
		t.Error("expected exists=true")
	}
	if path != "/worktrees/agent-y" {
		t.Errorf("expected '/worktrees/agent-y', got %q", path)
	}
}

func TestWorktreeManager_Cleanup_Empty_NoPanic(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	// should not panic on empty
	wm.Cleanup()
}

func TestWorktreeManager_MergeBack_NonexistentAgent(t *testing.T) {
	wm := NewWorktreeManager("/repo")
	err := wm.MergeBack("nonexistent", "some message")
	if err == nil {
		t.Error("expected error for nonexistent agent in MergeBack")
	}
}
