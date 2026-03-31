package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type WorktreeManager struct {
	mu        sync.Mutex
	repoPath  string
	worktrees map[string]string
}

func NewWorktreeManager(repoPath string) *WorktreeManager {
	return &WorktreeManager{
		repoPath:  repoPath,
		worktrees: make(map[string]string),
	}
}

func (wm *WorktreeManager) Create(agentID string) (string, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if path, exists := wm.worktrees[agentID]; exists {
		return path, nil
	}

	wtDir := filepath.Join(wm.repoPath, ".ntwine", "worktrees", agentID)
	branch := fmt.Sprintf("ntwine/%s", agentID)

	if err := os.MkdirAll(filepath.Dir(wtDir), 0755); err != nil {
		return "", fmt.Errorf("failed to create worktree dir: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtDir, "HEAD")
	cmd.Dir = wm.repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add failed: %s: %w", string(out), err)
	}

	wm.worktrees[agentID] = wtDir
	return wtDir, nil
}

func (wm *WorktreeManager) Remove(agentID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wtDir, exists := wm.worktrees[agentID]
	if !exists {
		return nil
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", wtDir)
	cmd.Dir = wm.repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove failed: %s: %w", string(out), err)
	}

	branch := fmt.Sprintf("ntwine/%s", agentID)
	delCmd := exec.Command("git", "branch", "-D", branch)
	delCmd.Dir = wm.repoPath
	delCmd.CombinedOutput()

	delete(wm.worktrees, agentID)
	return nil
}

func (wm *WorktreeManager) Path(agentID string) (string, bool) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	path, exists := wm.worktrees[agentID]
	return path, exists
}

func (wm *WorktreeManager) List() map[string]string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	result := make(map[string]string, len(wm.worktrees))
	for k, v := range wm.worktrees {
		result[k] = v
	}
	return result
}

func (wm *WorktreeManager) MergeBack(agentID string, message string) error {
	wm.mu.Lock()
	wtDir, exists := wm.worktrees[agentID]
	wm.mu.Unlock()

	if !exists {
		return fmt.Errorf("no worktree for agent %s", agentID)
	}

	branch := fmt.Sprintf("ntwine/%s", agentID)

	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = wtDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s: %w", string(out), err)
	}

	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = wtDir
	statusOut, _ := statusCmd.Output()

	if len(strings.TrimSpace(string(statusOut))) > 0 {
		commitCmd := exec.Command("git", "commit", "-m", message)
		commitCmd.Dir = wtDir
		if out, err := commitCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit failed: %s: %w", string(out), err)
		}
	}

	mergeCmd := exec.Command("git", "merge", "--no-ff", branch, "-m", fmt.Sprintf("merge: %s", message))
	mergeCmd.Dir = wm.repoPath
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git merge failed: %s: %w", string(out), err)
	}

	return wm.Remove(agentID)
}

func (wm *WorktreeManager) Cleanup() {
	wm.mu.Lock()
	agents := make([]string, 0, len(wm.worktrees))
	for id := range wm.worktrees {
		agents = append(agents, id)
	}
	wm.mu.Unlock()

	for _, id := range agents {
		wm.Remove(id)
	}
}
