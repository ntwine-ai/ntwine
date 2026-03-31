package orchestrator

import "sync"

type PinSet struct {
	mu   sync.RWMutex
	pins []string
}

func NewPinSet() *PinSet {
	return &PinSet{pins: []string{}}
}

func (p *PinSet) Add(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pins = append(p.pins, message)
}

func (p *PinSet) All() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, len(p.pins))
	copy(out, p.pins)
	return out
}
