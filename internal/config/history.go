package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MessageRecord struct {
	ModelID     string `json:"model_id"`
	DisplayName string `json:"display_name"`
	Content     string `json:"content"`
	Timestamp   int64  `json:"timestamp"`
}

type DiscussionRecord struct {
	ID              string          `json:"id"`
	Prompt          string          `json:"prompt"`
	CodebasePath    string          `json:"codebase_path"`
	Models          []string        `json:"models"`
	Messages        []MessageRecord `json:"messages"`
	SharedNotes     string          `json:"shared_notes"`
	ExecutionPrompt string          `json:"execution_prompt"`
	PinnedMessages  []string        `json:"pinned_messages"`
	CreatedAt       int64           `json:"created_at"`
}

type DiscussionSummary struct {
	ID           string   `json:"id"`
	Prompt       string   `json:"prompt"`
	Models       []string `json:"models"`
	MessageCount int      `json:"message_count"`
	CreatedAt    int64    `json:"created_at"`
}

func HistoryDir() string {
	return filepath.Join(configDir(), "history")
}

func SaveDiscussion(disc DiscussionRecord) error {
	dir := HistoryDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}
	data, err := json.MarshalIndent(disc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal discussion: %w", err)
	}
	return encryptAndWrite(filepath.Join(dir, disc.ID+".json"), data)
}

func LoadDiscussions() ([]DiscussionSummary, error) {
	dir := HistoryDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []DiscussionSummary{}, nil
		}
		return nil, fmt.Errorf("read history dir: %w", err)
	}

	summaries := make([]DiscussionSummary, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, wasEncrypted, err := readAndDecrypt(path)
		if err != nil {
			continue
		}
		var rec DiscussionRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if !wasEncrypted {
			_ = encryptAndWrite(path, data)
		}
		summaries = append(summaries, DiscussionSummary{
			ID:           rec.ID,
			Prompt:       rec.Prompt,
			Models:       rec.Models,
			MessageCount: len(rec.Messages),
			CreatedAt:    rec.CreatedAt,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt > summaries[j].CreatedAt
	})
	return summaries, nil
}

func LoadDiscussion(id string) (DiscussionRecord, error) {
	path := filepath.Join(HistoryDir(), id+".json")
	data, wasEncrypted, err := readAndDecrypt(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DiscussionRecord{}, fmt.Errorf("discussion not found: %s", id)
		}
		return DiscussionRecord{}, fmt.Errorf("read discussion: %w", err)
	}
	var rec DiscussionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return DiscussionRecord{}, fmt.Errorf("parse discussion: %w", err)
	}
	if !wasEncrypted {
		_ = encryptAndWrite(path, data)
	}
	return rec, nil
}

func DeleteDiscussion(id string) error {
	path := filepath.Join(HistoryDir(), id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("discussion not found: %s", id)
		}
		return fmt.Errorf("delete discussion: %w", err)
	}
	return nil
}
