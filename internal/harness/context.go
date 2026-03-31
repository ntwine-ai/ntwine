package harness

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"

	"github.com/ntwine-ai/ntwine/internal/openrouter"
)

type ContextManager struct {
	mu              sync.Mutex
	maxMessages     int
	maskedThreshold int
	fileCache       map[string]string
}

func NewContextManager(maxMessages, maskedThreshold int) *ContextManager {
	return &ContextManager{
		maxMessages:     maxMessages,
		maskedThreshold: maskedThreshold,
		fileCache:       make(map[string]string),
	}
}

func (cm *ContextManager) MaskOldObservations(messages []openrouter.ChatMessage) []openrouter.ChatMessage {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(messages) <= cm.maskedThreshold {
		return messages
	}

	result := make([]openrouter.ChatMessage, len(messages))
	copy(result, messages)

	boundary := len(messages) - cm.maskedThreshold

	for i := 0; i < boundary; i++ {
		if result[i].Role == "tool" && len(result[i].Content) > 100 {
			tokens := estimateTokens(result[i].Content)
			result[i].Content = fmt.Sprintf("[output masked, ~%d tokens]", tokens)
		}
	}

	return result
}

func (cm *ContextManager) CompactIfNeeded(messages []openrouter.ChatMessage, notes string) []openrouter.ChatMessage {
	if len(messages) <= cm.maxMessages {
		return messages
	}

	keep := cm.maxMessages
	compacted := messages[len(messages)-keep:]

	catchup := openrouter.ChatMessage{
		Role: "user",
		Content: fmt.Sprintf(
			"[context handoff -- another agent worked on this before you and produced this summary]\n\n"+
				"what was discussed and decided:\n%s\n\n"+
				"you are picking up where they left off. the shared spec above has all decisions made so far.\n"+
				"dont re-introduce yourself or repeat what was already decided. just keep going.",
			notes),
	}

	result := make([]openrouter.ChatMessage, 0, keep+1)
	result = append(result, catchup)
	result = append(result, compacted...)

	return result
}

func (cm *ContextManager) DiffFileResult(toolName, filePath, content string) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cacheKey := toolName + ":" + filePath
	prev, exists := cm.fileCache[cacheKey]
	cm.fileCache[cacheKey] = content

	if !exists {
		return content
	}

	if prev == content {
		return "[unchanged since last read]"
	}

	prevLines := strings.Split(prev, "\n")
	newLines := strings.Split(content, "\n")

	if len(prevLines) == len(newLines) {
		var diffs []string
		changed := 0
		for i := range prevLines {
			if prevLines[i] != newLines[i] {
				changed++
				diffs = append(diffs, fmt.Sprintf("L%d: %s -> %s", i+1, truncate(prevLines[i], 80), truncate(newLines[i], 80)))
			}
		}
		if changed <= 10 {
			return fmt.Sprintf("[%d lines changed since last read]\n%s", changed, strings.Join(diffs, "\n"))
		}
	}

	return content
}

func (cm *ContextManager) FileHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

func estimateTokens(s string) int {
	return len(s) / 4
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
