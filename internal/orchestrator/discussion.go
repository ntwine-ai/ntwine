package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/ntwine-ai/ntwine/internal/config"
	"github.com/ntwine-ai/ntwine/internal/harness"
	"github.com/ntwine-ai/ntwine/internal/openrouter"
)

type DiscussionResult struct {
	Messages       []openrouter.ChatMessage
	Notes          string
	ExecutionPrompt string
	PinnedMessages []string
	NameMap        map[string]string
}

var tagPattern = regexp.MustCompile(`\[[\w\-./]+\]:\s*`)
var toolCallPattern = regexp.MustCompile(`(?s)<[｜|]?(?:tool_call|function_calls?|invoke|parameter|DSML|minimax)[｜|]?[\s>].*$`)
var markdownHeaderPattern = regexp.MustCompile(`(?m)^#{1,4}\s+.*$`)
var emojiHeaderPattern = regexp.MustCompile(`(?m)^[🏁🎯✅❌📋🔧💡🚀]\s*\*\*.*?\*\*`)
var progressPattern = regexp.MustCompile(`(?i)progress update|implementation summary|key evidence|quick summary`)

func stripTags(s string) string {
	cleaned := strings.TrimSpace(tagPattern.ReplaceAllString(s, ""))
	cleaned = strings.TrimSpace(toolCallPattern.ReplaceAllString(cleaned, ""))
	cleaned = markdownHeaderPattern.ReplaceAllStringFunc(cleaned, func(h string) string {
		content := strings.TrimLeft(h, "# ")
		return content
	})
	cleaned = emojiHeaderPattern.ReplaceAllStringFunc(cleaned, func(h string) string {
		h = strings.TrimSpace(h)
		if len(h) > 0 {
			rs := []rune(h)
			for i, r := range rs {
				if r < 128 {
					return strings.Trim(string(rs[i:]), "* ")
				}
			}
		}
		return h
	})
	cleaned = progressPattern.ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "I'd be happy to", "")
	cleaned = strings.ReplaceAll(cleaned, "I'd be happy to", "")
	return strings.TrimSpace(cleaned)
}

type Event struct {
	Type        string      `json:"type"`
	ModelID     string      `json:"model_id,omitempty"`
	DisplayName string      `json:"display_name,omitempty"`
	Content     interface{} `json:"content"`
}

type Discussion struct {
	ID           string
	Prompt       string
	CodebasePath string
	Models       []string
	Messages     []openrouter.ChatMessage
	Notes        string
	Status       string
	MaxRounds    int
}

func NewDiscussion(id, prompt, codebasePath string, models []string, maxRounds int) Discussion {
	if maxRounds <= 0 {
		maxRounds = 5
	}
	return Discussion{
		ID:           id,
		Prompt:       prompt,
		CodebasePath: codebasePath,
		Models:       models,
		Messages:     []openrouter.ChatMessage{},
		Notes:        "",
		Status:       "running",
		MaxRounds:    maxRounds,
	}
}

func shortName(modelID string) string {
	parts := strings.Split(modelID, "/")
	name := parts[len(parts)-1]
	name = strings.Split(name, ":")[0]
	for _, strip := range []string{"-next", "-fast", "-free", "-preview", "-latest"} {
		name = strings.TrimSuffix(name, strip)
	}
	dashParts := strings.Split(name, "-")
	if len(dashParts) > 0 {
		known := map[string]string{
			"claude": "Claude", "gemini": "Gemini", "grok": "Grok",
			"qwen3": "Qwen", "qwen": "Qwen", "deepseek": "DeepSeek",
			"minimax": "MiniMax", "nemotron": "Nemotron", "llama": "Llama",
			"mistral": "Mistral", "mixtral": "Mixtral", "phi": "Phi",
			"command": "Command", "nova": "Nova", "codestral": "Codestral",
			"sonar": "Sonar", "jamba": "Jamba", "kimi": "Kimi",
			"cerebras": "Cerebras", "sambanova": "SambaNova",
		}
		if nice, ok := known[strings.ToLower(dashParts[0])]; ok {
			return nice
		}
	}
	if len(name) > 12 {
		name = name[:12]
	}
	return name
}

func buildNameMap(models []string) map[string]string {
	names := make(map[string]string)
	used := make(map[string]int)
	for _, m := range models {
		short := shortName(m)
		used[short]++
		names[m] = short
	}
	for m, short := range names {
		if used[short] > 1 {
			parts := strings.Split(m, "/")
			names[m] = parts[len(parts)-1]
		}
	}
	return names
}

func systemPrompt(prompt, codebasePath string, thisModel string, allModels []string, nameMap map[string]string) string {
	agent := harness.AgentRole{
		ID:   thisModel,
		Name: nameMap[thisModel],
	}

	var teammates []harness.AgentRole
	for _, m := range allModels {
		teammates = append(teammates, harness.AgentRole{
			ID:   m,
			Name: nameMap[m],
		})
	}

	projectRules := harness.LoadProjectConfig(codebasePath)

	base := harness.BuildSystemPrompt(agent, teammates, codebasePath, "")

	if projectRules != "" {
		base += "\nproject rules:\n" + projectRules + "\n\n"
	}

	return base + "\ntask: " + prompt + "\n\n" +
		"banned phrases (using these = instant cringe):\n" +
		"\"I'd be happy to\", \"Great question\", \"Shall I proceed\", \"Let me break this down\",\n" +
		"\"Absolutely\", \"That's a fantastic point\", \"Based on my exploration\",\n" +
		"\"Here's what I found\", \"Let me provide\", \"Progress Update\",\n" +
		"\"Implementation Summary\", \"Key Evidence\", \"After analyzing\",\n" +
		"\"Let me start by exploring\", \"I can confirm\", \"I want to understand\"\n\n" +
		"banned formatting:\n" +
		"- no markdown headers (# or ##) in chat\n" +
		"- no emoji bullet headers\n" +
		"- no numbered lists with bold headers\n" +
		"- just write plain sentences like a normal person in a chat\n" +
		"- markdown is fine in the shared spec (thats a doc, not chat)\n\n" +
		"shared spec:\n" +
		"use update_notes to record plans and decisions as a team.\n" +
		"write notes as a team doc - no model names. write like one person.\n" +
		"if your context gets compacted, the spec has everything discussed so far.\n"
}

const maxToolCallsPerTurn = 5

func streamModelResponse(
	ctx context.Context,
	client *openrouter.Client,
	modelID string,
	messages []openrouter.ChatMessage,
	tools []openrouter.ToolDefinition,
	broadcast func(Event),
	nameMap map[string]string,
) (openrouter.ChatMessage, error) {
	ch, err := client.StreamChat(ctx, openrouter.ChatRequest{
		Model:    modelID,
		Messages: messages,
		Tools:    tools,
	})
	if err != nil {
		return openrouter.ChatMessage{}, err
	}

	var content strings.Builder
	var toolCalls []openrouter.ToolCall

	for chunk := range ch {
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			content.WriteString(delta.Content)
			broadcast(Event{Type: "token", ModelID: modelID, Content: delta.Content})
		}

		for _, stc := range delta.ToolCalls {
			for stc.Index >= len(toolCalls) {
				toolCalls = append(toolCalls, openrouter.ToolCall{Type: "function"})
			}
			tc := &toolCalls[stc.Index]
			if stc.ID != "" {
				tc.ID = stc.ID
			}
			if stc.Function.Name != "" {
				tc.Function.Name += stc.Function.Name
			}
			if stc.Function.Arguments != "" {
				tc.Function.Arguments += stc.Function.Arguments
			}
		}
	}

	return openrouter.ChatMessage{
		Role:      "assistant",
		Content:   content.String(),
		ToolCalls: toolCalls,
	}, nil
}

type streamResult struct {
	modelID string
	msg     openrouter.ChatMessage
	err     error
}

func parallelRound(
	ctx context.Context,
	client *openrouter.Client,
	disc Discussion,
	messages []openrouter.ChatMessage,
	notes string,
	toolDefs []openrouter.ToolDefinition,
	registry *harness.Registry,
	nameMap map[string]string,
	broadcast func(Event),
	mutes *MuteSet,
	pins *PinSet,
) ([]openrouter.ChatMessage, string) {
	activeModels := mutes.ActiveModels(disc.Models)
	if len(activeModels) == 0 {
		return messages, notes
	}

	log.Printf("[DISC %s] Firing %d models in parallel", disc.ID, len(activeModels))
	for _, m := range activeModels {
		broadcast(Event{Type: "status", ModelID: m, Content: "thinking..."})
	}

	results := make(chan streamResult, len(activeModels))
	for _, modelID := range activeModels {
		go func(mid string) {
			sysmsg := openrouter.ChatMessage{
				Role:    "system",
				Content: systemPrompt(disc.Prompt, disc.CodebasePath, mid, disc.Models, nameMap),
			}
			msgs := sanitizeMessages(append([]openrouter.ChatMessage{sysmsg}, withNotesContext(messages, notes, nil)...))
			msg, err := streamModelResponse(ctx, client, mid, msgs, toolDefs, broadcast, nameMap)
			if err != nil {
				log.Printf("[DISC %s] [%s] stream failed, falling back: %s", disc.ID, nameMap[mid], err.Error())
				resp, retryErr := chatWithRetry(ctx, client, mid, msgs, toolDefs, nameMap[mid], disc.ID)
				if retryErr != nil {
					results <- streamResult{modelID: mid, err: retryErr}
					return
				}
				if len(resp.Choices) == 0 {
					results <- streamResult{modelID: mid, err: fmt.Errorf("no response")}
					return
				}
				results <- streamResult{modelID: mid, msg: resp.Choices[0].Message}
				return
			}
			results <- streamResult{modelID: mid, msg: msg}
		}(modelID)
	}

	baseMessages := cloneMessages(messages)
	updatedNotes := notes
	var allResults []streamResult
	for i := 0; i < len(activeModels); i++ {
		allResults = append(allResults, <-results)
	}

	updatedMessages := cloneMessages(baseMessages)
	for _, r := range allResults {
		if r.err != nil {
			log.Printf("[DISC %s] [%s] ERROR: %s", disc.ID, nameMap[r.modelID], r.err.Error())
			broadcast(Event{Type: "error", ModelID: r.modelID, Content: r.err.Error()})
			continue
		}
		log.Printf("[DISC %s] [%s] got: %d chars, %d tools", disc.ID, nameMap[r.modelID], len(r.msg.Content), len(r.msg.ToolCalls))
		isolated := cloneMessages(baseMessages)
		isolated, updatedNotes = handleModelResponse(ctx, client, r.modelID, r.msg, isolated, updatedNotes, toolDefs, registry, disc.CodebasePath, broadcast, nameMap, 0, pins)
		for _, m := range isolated[len(baseMessages):] {
			if m.Role == "assistant" {
				updatedMessages = append(updatedMessages, m)
			}
		}
	}
	return updatedMessages, updatedNotes
}

func sequentialRound(
	ctx context.Context,
	client *openrouter.Client,
	disc Discussion,
	messages []openrouter.ChatMessage,
	notes string,
	toolDefs []openrouter.ToolDefinition,
	registry *harness.Registry,
	nameMap map[string]string,
	broadcast func(Event),
	mutes *MuteSet,
	pins *PinSet,
) ([]openrouter.ChatMessage, string) {
	for _, modelID := range disc.Models {
		if ctx.Err() != nil {
			return messages, notes
		}
		if mutes.IsMuted(modelID) {
			continue
		}

		log.Printf("[DISC %s] [%s] requesting (%d msgs)", disc.ID, nameMap[modelID], len(messages))
		broadcast(Event{Type: "status", ModelID: modelID, Content: "thinking..."})

		sysmsg := openrouter.ChatMessage{
			Role:    "system",
			Content: systemPrompt(disc.Prompt, disc.CodebasePath, modelID, disc.Models, nameMap),
		}
		currentMessages := sanitizeMessages(append([]openrouter.ChatMessage{sysmsg}, withNotesContext(messages, notes, nil)...))

		msg, err := streamModelResponse(ctx, client, modelID, currentMessages, toolDefs, broadcast, nameMap)
		if err != nil {
			log.Printf("[DISC %s] [%s] stream failed, falling back: %s", disc.ID, nameMap[modelID], err.Error())
			resp, retryErr := chatWithRetry(ctx, client, modelID, currentMessages, toolDefs, nameMap[modelID], disc.ID)
			if retryErr != nil {
				log.Printf("[DISC %s] [%s] ERROR: %s", disc.ID, nameMap[modelID], retryErr.Error())
				broadcast(Event{Type: "error", ModelID: modelID, Content: retryErr.Error()})
				continue
			}
			if len(resp.Choices) == 0 {
				broadcast(Event{Type: "error", ModelID: modelID, Content: "no response"})
				continue
			}
			msg = resp.Choices[0].Message
		}

		log.Printf("[DISC %s] [%s] got: %d chars, %d tools", disc.ID, nameMap[modelID], len(msg.Content), len(msg.ToolCalls))
		messages, notes = handleModelResponse(ctx, client, modelID, msg, messages, notes, toolDefs, registry, disc.CodebasePath, broadcast, nameMap, 0, pins)
	}
	return messages, notes
}

func countAgreements(messages []openrouter.ChatMessage, lookback int) int {
	agreePhrases := []string{
		"agreed", "i agree", "sounds good", "that works", "let's go with",
		"spot on", "solid plan", "nailed it", "makes sense", "on board",
		"same page", "consensus", "let's do it", "ship it",
	}
	count := 0
	start := len(messages) - lookback
	if start < 0 {
		start = 0
	}
	for _, m := range messages[start:] {
		if m.Role != "assistant" {
			continue
		}
		lower := strings.ToLower(m.Content)
		for _, phrase := range agreePhrases {
			if strings.Contains(lower, phrase) {
				count++
				break
			}
		}
	}
	return count
}

func chatWithRetry(ctx context.Context, client *openrouter.Client, modelID string, messages []openrouter.ChatMessage, tools []openrouter.ToolDefinition, displayName string, discID string) (openrouter.ChatResponse, error) {
	resp, err := client.Chat(ctx, openrouter.ChatRequest{
		Model:    modelID,
		Messages: sanitizeMessages(messages),
		Tools:    tools,
	})
	if err == nil {
		return resp, nil
	}

	log.Printf("[DISC %s] [%s] error: %s, retrying with trimmed context + no tools", discID, displayName, err.Error())
	if len(messages) <= 2 {
		return resp, err
	}

	sys := messages[0]
	rest := messages[1:]
	keep := 6
	if keep > len(rest) {
		keep = len(rest)
	}
	kept := sanitizeMessages(rest[len(rest)-keep:])
	trimmed := append([]openrouter.ChatMessage{sys}, kept...)
	log.Printf("[DISC %s] [%s] retry: %d msgs (was %d), no tools", discID, displayName, len(trimmed), len(messages))

	resp2, err2 := client.Chat(ctx, openrouter.ChatRequest{
		Model:    modelID,
		Messages: trimmed,
	})
	if err2 != nil {
		return resp, fmt.Errorf("retry also failed: %w", err2)
	}
	return resp2, nil
}

func Run(ctx context.Context, disc Discussion, client *openrouter.Client, registry *harness.Registry, rawBroadcast func(Event), mutes *MuteSet, pins *PinSet, injector *Injector) DiscussionResult {
	nameMap := buildNameMap(disc.Models)
	broadcast := func(e Event) {
		if e.ModelID != "" {
			e.DisplayName = nameMap[e.ModelID]
		}
		rawBroadcast(e)
	}

	log.Printf("[DISC %s] Starting: %d models, %d rounds", disc.ID, len(disc.Models), disc.MaxRounds)
	log.Printf("[DISC %s] Prompt: %s", disc.ID, disc.Prompt)
	for _, m := range disc.Models {
		log.Printf("[DISC %s]   %s → %s", disc.ID, m, nameMap[m])
	}
	broadcast(Event{Type: "status", Content: "starting discussion"})

	toolDefs := registry.Definitions()
	notes := disc.Notes
	messages := append([]openrouter.ChatMessage{}, disc.Messages...)

	ctxMgr := harness.NewContextManager(25, 15)
	loopDet := harness.NewLoopDetector()
	registry.SetLoopDetector(loopDet)

	registry.OnEvent(func(evt harness.ToolEvent) {
		switch evt.Type {
		case harness.EventToolStart:
			broadcast(Event{Type: "status", ModelID: "", Content: fmt.Sprintf("running %s", evt.ToolName)})
		case harness.EventToolError:
			log.Printf("[HARNESS] tool error: %s: %s", evt.ToolName, evt.Content)
		case harness.EventToolProgress:
			log.Printf("[HARNESS] %s: %s", evt.ToolName, evt.Content)
		}
	})

	_ = ctxMgr

	agreementCount := 0

	for round := 0; round < disc.MaxRounds; round++ {
		if ctx.Err() != nil {
			broadcast(Event{Type: "status", Content: "stopped"})
			return DiscussionResult{Messages: messages, Notes: notes, PinnedMessages: pins.All(), NameMap: nameMap}
		}

		injected := injector.Drain()
		for _, msg := range injected {
			messages = append(cloneMessages(messages), msg)
			broadcast(Event{Type: "message", ModelID: "god", DisplayName: "God", Content: strings.TrimPrefix(msg.Content, "[God]: ")})
		}

		log.Printf("[DISC %s] === Round %d/%d ===", disc.ID, round+1, disc.MaxRounds)
		broadcast(Event{Type: "status", Content: fmt.Sprintf("round %d/%d", round+1, disc.MaxRounds)})

		if round == 0 {
			messages, notes = parallelRound(ctx, client, disc, messages, notes, toolDefs, registry, nameMap, broadcast, mutes, pins)
		} else {
			messages, notes = sequentialRound(ctx, client, disc, messages, notes, toolDefs, registry, nameMap, broadcast, mutes, pins)
		}

		agreementCount = countAgreements(messages, 6)
		if agreementCount >= 3 && round < disc.MaxRounds-1 {
			log.Printf("[DISC %s] Convergence detected (%d agreements), wrapping up", disc.ID, agreementCount)
			broadcast(Event{Type: "status", Content: "consensus reached, generating plan"})
			break
		}
	}

	log.Printf("[DISC %s] Generating execution prompt", disc.ID)
	execPrompt := generateExecutionPrompt(ctx, client, disc.Models, messages, notes)
	broadcast(Event{Type: "execution_prompt", Content: execPrompt})
	broadcast(Event{Type: "status", Content: "done"})
	log.Printf("[DISC %s] Done", disc.ID)

	return DiscussionResult{
		Messages:        messages,
		Notes:           notes,
		ExecutionPrompt: execPrompt,
		PinnedMessages:  pins.All(),
		NameMap:         nameMap,
	}
}

func BuildRecord(disc Discussion, result DiscussionResult) config.DiscussionRecord {
	records := make([]config.MessageRecord, 0, len(result.Messages))
	for _, m := range result.Messages {
		if m.Role != "assistant" {
			continue
		}
		modelID := ""
		displayName := ""
		for id, name := range result.NameMap {
			prefix := "[" + name + "]: "
			if strings.HasPrefix(m.Content, prefix) {
				modelID = id
				displayName = name
				break
			}
		}
		content := m.Content
		for _, name := range result.NameMap {
			prefix := "[" + name + "]: "
			content = strings.TrimPrefix(content, prefix)
		}
		records = append(records, config.MessageRecord{
			ModelID:     modelID,
			DisplayName: displayName,
			Content:     content,
			Timestamp:   time.Now().Unix(),
		})
	}
	return config.DiscussionRecord{
		ID:              disc.ID,
		Prompt:          disc.Prompt,
		CodebasePath:    disc.CodebasePath,
		Models:          disc.Models,
		Messages:        records,
		SharedNotes:     result.Notes,
		ExecutionPrompt: result.ExecutionPrompt,
		PinnedMessages:  result.PinnedMessages,
		CreatedAt:       time.Now().Unix(),
	}
}

func handleModelResponse(
	ctx context.Context,
	client *openrouter.Client,
	modelID string,
	msg openrouter.ChatMessage,
	messages []openrouter.ChatMessage,
	notes string,
	toolDefs []openrouter.ToolDefinition,
	registry *harness.Registry,
	codebasePath string,
	broadcast func(Event),
	nameMap map[string]string,
	totalToolCalls int,
	pins *PinSet,
) ([]openrouter.ChatMessage, string) {

	cleanContent := stripTags(msg.Content)
	if cleanContent != "" {
		broadcast(Event{Type: "message", ModelID: modelID, Content: cleanContent})
	}

	taggedContent := fmt.Sprintf("[%s]: %s", nameMap[modelID], cleanContent)
	assistantMsg := openrouter.ChatMessage{
		Role:      "assistant",
		Content:   taggedContent,
		ToolCalls: msg.ToolCalls,
	}
	updatedMessages := append(cloneMessages(messages), assistantMsg)

	if len(msg.ToolCalls) == 0 {
		return updatedMessages, notes
	}

	newTotal := totalToolCalls + len(msg.ToolCalls)
	if newTotal > maxToolCallsPerTurn {
		log.Printf("[DISC] [%s] hit tool cap (%d), forcing text response", nameMap[modelID], newTotal)
		return forceTextResponse(ctx, client, modelID, updatedMessages, notes, broadcast, nameMap)
	}

	updatedNotes := notes

	calls := make([]harness.ToolCall, len(msg.ToolCalls))
	for i, tc := range msg.ToolCalls {
		calls[i] = harness.ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments}
		log.Printf("[TOOL] [%s] %s(%s)", nameMap[modelID], tc.Function.Name, tc.Function.Arguments)
		broadcast(Event{
			Type:    "tool_call",
			ModelID: modelID,
			Content: map[string]string{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
		})
	}

	results := registry.ExecuteParallel(ctx, calls)

	for i, pr := range results {
		tc := msg.ToolCalls[i]
		result := pr.Result.Output

		if pr.Result.IsError {
			log.Printf("[TOOL] [%s] error: %s", nameMap[modelID], result)
		} else {
			log.Printf("[TOOL] [%s] %s -> %d chars", nameMap[modelID], tc.Function.Name, len(result))
		}

		if tc.Function.Name == "update_notes" && !pr.Result.IsError {
			broadcast(Event{Type: "notes_update", ModelID: modelID, Content: updatedNotes})
		}
		if tc.Function.Name == "pin_message" && !pr.Result.IsError {
			var pinArgs struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &pinArgs)
			broadcast(Event{Type: "pin", ModelID: modelID, Content: pinArgs.Message})
		}

		displayResult := result
		if len(displayResult) > 3000 {
			displayResult = displayResult[:3000] + "\n... (truncated)"
		}
		broadcast(Event{
			Type:    "tool_result",
			ModelID: modelID,
			Content: map[string]string{"name": tc.Function.Name, "result": displayResult},
		})

		contextResult := result
		if len(contextResult) > 800 {
			contextResult = contextResult[:800] + "\n... (truncated, " + fmt.Sprintf("%d", len(result)) + " chars total)"
		}
		updatedMessages = append(updatedMessages, openrouter.ChatMessage{
			Role:       "tool",
			Content:    contextResult,
			ToolCallID: tc.ID,
		})
	}

	followup, err := client.Chat(ctx, openrouter.ChatRequest{
		Model:    modelID,
		Messages: sanitizeMessages(updatedMessages),
		Tools:    toolDefs,
	})
	if err != nil {
		broadcast(Event{Type: "error", ModelID: modelID, Content: err.Error()})
		return updatedMessages, updatedNotes
	}

	if len(followup.Choices) > 0 {
		fmsg := followup.Choices[0].Message
		if len(fmsg.ToolCalls) > 0 {
			return handleModelResponse(ctx, client, modelID, fmsg, updatedMessages, updatedNotes, toolDefs, registry, codebasePath, broadcast, nameMap, newTotal, pins)
		}
		fClean := stripTags(fmsg.Content)
		if fClean != "" {
			broadcast(Event{Type: "message", ModelID: modelID, Content: fClean})
			updatedMessages = append(updatedMessages, openrouter.ChatMessage{
				Role:    "assistant",
				Content: fmt.Sprintf("[%s]: %s", nameMap[modelID], fClean),
			})
		}
	}

	return updatedMessages, updatedNotes
}

func forceTextResponse(
	ctx context.Context,
	client *openrouter.Client,
	modelID string,
	messages []openrouter.ChatMessage,
	notes string,
	broadcast func(Event),
	nameMap map[string]string,
) ([]openrouter.ChatMessage, string) {
	broadcast(Event{Type: "status", ModelID: modelID, Content: "wrapping up tools..."})
	resp, err := client.Chat(ctx, openrouter.ChatRequest{
		Model:    modelID,
		Messages: sanitizeMessages(messages),
	})
	if err != nil {
		broadcast(Event{Type: "error", ModelID: modelID, Content: err.Error()})
		return messages, notes
	}
	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
		fClean := stripTags(resp.Choices[0].Message.Content)
		if fClean != "" {
			broadcast(Event{Type: "message", ModelID: modelID, Content: fClean})
			messages = append(cloneMessages(messages), openrouter.ChatMessage{
				Role:    "assistant",
				Content: fmt.Sprintf("[%s]: %s", nameMap[modelID], fClean),
			})
		}
	}
	return messages, notes
}

func generateExecutionPrompt(
	ctx context.Context,
	client *openrouter.Client,
	models []string,
	messages []openrouter.ChatMessage,
	notes string,
) string {
	if len(models) == 0 {
		return notes
	}
	if strings.TrimSpace(notes) == "" {
		notes = "(no notes were recorded during the discussion)"
	}

	last10 := messages
	if len(last10) > 10 {
		last10 = last10[len(last10)-10:]
	}

	summaryMessages := []openrouter.ChatMessage{
		{Role: "system", Content: "You are a technical writer. Synthesize discussion notes into a clear execution prompt."},
	}
	summaryMessages = append(summaryMessages, last10...)
	summaryMessages = append(summaryMessages, openrouter.ChatMessage{
		Role: "user",
		Content: "Based on the discussion and shared notes below, generate a clear, actionable execution prompt " +
			"that another AI could use to implement the plan. Include specific file paths, code changes, " +
			"and steps. Output only the prompt, no preamble.\n\nShared notes:\n" + notes,
	})

	for _, modelID := range models {
		resp, err := client.Chat(ctx, openrouter.ChatRequest{
			Model:    modelID,
			Messages: summaryMessages,
		})
		if err != nil {
			log.Printf("[EXEC] %s failed: %s, trying next model", modelID, err.Error())
			continue
		}
		if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
			return resp.Choices[0].Message.Content
		}
	}
	return notes
}

const maxContextMessages = 25

func sanitizeMessages(msgs []openrouter.ChatMessage) []openrouter.ChatMessage {
	var out []openrouter.ChatMessage
	for _, m := range msgs {
		if m.Role == "tool" {
			continue
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			cleaned := openrouter.ChatMessage{
				Role:    "assistant",
				Content: m.Content,
			}
			out = append(out, cleaned)
			continue
		}
		out = append(out, m)
	}

	if len(out) > 0 && out[len(out)-1].Role == "assistant" {
		out = append(out, openrouter.ChatMessage{
			Role:    "user",
			Content: "[Continue the discussion]",
		})
	}

	return out
}

func withNotesContext(messages []openrouter.ChatMessage, notes string, ctxMgr *harness.ContextManager) []openrouter.ChatMessage {
	var masked []openrouter.ChatMessage
	if ctxMgr != nil {
		masked = ctxMgr.MaskOldObservations(messages)
	} else {
		masked = messages
	}

	trimmed := masked
	wasCompacted := false
	if len(trimmed) > maxContextMessages {
		trimmed = trimmed[len(trimmed)-maxContextMessages:]
		wasCompacted = true
	}

	cloned := sanitizeMessages(cloneMessages(trimmed))

	if wasCompacted && strings.TrimSpace(notes) != "" {
		catchup := openrouter.ChatMessage{
			Role: "user",
			Content: "[Context was compacted - earlier messages were removed to save space. " +
				"The shared notes below contain everything discussed so far. " +
				"Read them and continue the discussion from where it left off.]\n\n" +
				"[Shared Notes]\n" + notes,
		}
		cloned = append([]openrouter.ChatMessage{catchup}, cloned...)
	} else if strings.TrimSpace(notes) != "" {
		cloned = append(cloned, openrouter.ChatMessage{
			Role:    "user",
			Content: "[Shared Notes]\n" + notes,
		})
	}
	return cloned
}

func cloneMessages(msgs []openrouter.ChatMessage) []openrouter.ChatMessage {
	out := make([]openrouter.ChatMessage, len(msgs))
	copy(out, msgs)
	return out
}
