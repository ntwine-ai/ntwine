package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type AgentEvent struct {
	Timestamp time.Time       `json:"ts"`
	AgentID   string          `json:"agent"`
	AgentName string          `json:"name"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
}

type EventLog struct {
	mu       sync.Mutex
	events   []AgentEvent
	filePath string
	file     *os.File
	subs     []chan AgentEvent
}

func NewEventLog(filePath string) (*EventLog, error) {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open event log: %w", err)
	}

	return &EventLog{
		filePath: filePath,
		file:     f,
	}, nil
}

func (el *EventLog) Append(evt AgentEvent) error {
	el.mu.Lock()
	defer el.mu.Unlock()

	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	el.events = append(el.events, evt)

	line, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	if _, err := el.file.Write(append(line, '\n')); err != nil {
		return err
	}

	for _, ch := range el.subs {
		select {
		case ch <- evt:
		default:
		}
	}

	return nil
}

func (el *EventLog) Subscribe() chan AgentEvent {
	el.mu.Lock()
	defer el.mu.Unlock()

	ch := make(chan AgentEvent, 64)
	el.subs = append(el.subs, ch)
	return ch
}

func (el *EventLog) Events() []AgentEvent {
	el.mu.Lock()
	defer el.mu.Unlock()

	result := make([]AgentEvent, len(el.events))
	copy(result, el.events)
	return result
}

func (el *EventLog) EventsByAgent(agentID string) []AgentEvent {
	el.mu.Lock()
	defer el.mu.Unlock()

	var result []AgentEvent
	for _, e := range el.events {
		if e.AgentID == agentID {
			result = append(result, e)
		}
	}
	return result
}

func (el *EventLog) EventsByType(eventType string) []AgentEvent {
	el.mu.Lock()
	defer el.mu.Unlock()

	var result []AgentEvent
	for _, e := range el.events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

func (el *EventLog) Close() error {
	el.mu.Lock()
	defer el.mu.Unlock()

	for _, ch := range el.subs {
		close(ch)
	}
	el.subs = nil

	return el.file.Close()
}

func NewAgentEvent(agentID, agentName, eventType string, data interface{}) AgentEvent {
	raw, _ := json.Marshal(data)
	return AgentEvent{
		Timestamp: time.Now(),
		AgentID:   agentID,
		AgentName: agentName,
		Type:      eventType,
		Data:      raw,
	}
}
