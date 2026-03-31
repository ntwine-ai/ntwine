package orchestrator

import "github.com/ntwine-ai/ntwine/internal/openrouter"

type Injector struct {
	ch chan string
}

func NewInjector() *Injector {
	return &Injector{ch: make(chan string, 32)}
}

func (inj *Injector) Send(content string) {
	select {
	case inj.ch <- content:
	default:
	}
}

func (inj *Injector) Drain() []openrouter.ChatMessage {
	var msgs []openrouter.ChatMessage
	for {
		select {
		case content := <-inj.ch:
			msgs = append(msgs, openrouter.ChatMessage{
				Role:    "user",
				Content: "[God]: " + content,
			})
		default:
			return msgs
		}
	}
}
