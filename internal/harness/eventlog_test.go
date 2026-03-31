package harness

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEventLog_AppendAndEvents(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, err := NewEventLog(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("NewEventLog failed: %v", err)
	}
	defer el.Close()

	evt := NewAgentEvent("agent-1", "Worker", "tool_call", map[string]string{"tool": "read_file"})
	if err := el.Append(evt); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	events := el.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].AgentID != "agent-1" {
		t.Errorf("expected AgentID 'agent-1', got %q", events[0].AgentID)
	}
	if events[0].Type != "tool_call" {
		t.Errorf("expected type 'tool_call', got %q", events[0].Type)
	}
}

func TestEventLog_Append_SetsTimestamp(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	before := time.Now()
	evt := AgentEvent{AgentID: "a", Type: "test"}
	el.Append(evt)
	after := time.Now()

	events := el.Events()
	ts := events[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v outside range [%v, %v]", ts, before, after)
	}
}

func TestEventLog_Append_PreservesExistingTimestamp(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	fixed := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	evt := AgentEvent{AgentID: "a", Type: "test", Timestamp: fixed}
	el.Append(evt)

	events := el.Events()
	if !events[0].Timestamp.Equal(fixed) {
		t.Errorf("expected timestamp preserved as %v, got %v", fixed, events[0].Timestamp)
	}
}

func TestEventLog_WritesNDJSON(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	logPath := filepath.Join(dir, "events.jsonl")
	el, _ := NewEventLog(logPath)

	el.Append(NewAgentEvent("a1", "Alice", "start", nil))
	el.Append(NewAgentEvent("a2", "Bob", "stop", nil))
	el.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d", len(lines))
	}

	var parsed AgentEvent
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Errorf("line 1 is not valid JSON: %v", err)
	}
	if parsed.AgentID != "a1" {
		t.Errorf("expected AgentID 'a1', got %q", parsed.AgentID)
	}
}

func TestEventLog_EventsByAgent(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	el.Append(NewAgentEvent("alice", "Alice", "read", nil))
	el.Append(NewAgentEvent("bob", "Bob", "write", nil))
	el.Append(NewAgentEvent("alice", "Alice", "think", nil))

	aliceEvents := el.EventsByAgent("alice")
	if len(aliceEvents) != 2 {
		t.Errorf("expected 2 events for alice, got %d", len(aliceEvents))
	}
	bobEvents := el.EventsByAgent("bob")
	if len(bobEvents) != 1 {
		t.Errorf("expected 1 event for bob, got %d", len(bobEvents))
	}
	noneEvents := el.EventsByAgent("charlie")
	if len(noneEvents) != 0 {
		t.Errorf("expected 0 events for charlie, got %d", len(noneEvents))
	}
}

func TestEventLog_EventsByType(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	el.Append(NewAgentEvent("a", "A", "tool_call", nil))
	el.Append(NewAgentEvent("b", "B", "message", nil))
	el.Append(NewAgentEvent("c", "C", "tool_call", nil))

	toolCalls := el.EventsByType("tool_call")
	if len(toolCalls) != 2 {
		t.Errorf("expected 2 tool_call events, got %d", len(toolCalls))
	}
}

func TestEventLog_Subscribe(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	ch := el.Subscribe()

	el.Append(NewAgentEvent("a", "A", "ping", nil))

	select {
	case evt := <-ch:
		if evt.Type != "ping" {
			t.Errorf("expected type 'ping', got %q", evt.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive event on subscription channel")
	}
}

func TestEventLog_MultipleSubscribers(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	ch1 := el.Subscribe()
	ch2 := el.Subscribe()

	el.Append(NewAgentEvent("a", "A", "broadcast", nil))

	for _, ch := range []chan AgentEvent{ch1, ch2} {
		select {
		case evt := <-ch:
			if evt.Type != "broadcast" {
				t.Errorf("expected 'broadcast', got %q", evt.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("expected broadcast on all subscribers")
		}
	}
}

func TestEventLog_Close_ClosesChannels(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	ch := el.Subscribe()
	el.Close()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected closed channel to be readable immediately")
	}
}

func TestEventLog_Events_ReturnsSnapshot(t *testing.T) {
	dir, cleanup := setupCodebase(t)
	defer cleanup()

	el, _ := NewEventLog(filepath.Join(dir, "events.jsonl"))
	defer el.Close()

	el.Append(NewAgentEvent("a", "A", "t1", nil))
	snap := el.Events()

	// append after snapshot
	el.Append(NewAgentEvent("b", "B", "t2", nil))

	if len(snap) != 1 {
		t.Errorf("snapshot should be independent, expected 1, got %d", len(snap))
	}
}

func TestNewEventLog_InvalidPath_ReturnsError(t *testing.T) {
	_, err := NewEventLog("/nonexistent/directory/events.jsonl")
	if err == nil {
		t.Error("expected error for invalid log path")
	}
}

func TestNewAgentEvent_DataSerialized(t *testing.T) {
	evt := NewAgentEvent("id", "name", "type", map[string]string{"key": "val"})
	if string(evt.Data) == "null" || !strings.Contains(string(evt.Data), "val") {
		t.Errorf("expected data serialized, got: %s", string(evt.Data))
	}
}

func TestNewAgentEvent_NilData_SerializesNull(t *testing.T) {
	evt := NewAgentEvent("id", "name", "type", nil)
	if evt.Data == nil {
		t.Error("expected non-nil Data even for nil input")
	}
}
