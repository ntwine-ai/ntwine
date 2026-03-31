package orchestrator

import "sync"

type MuteSet struct {
	mu    sync.RWMutex
	muted map[string]bool
}

func NewMuteSet() *MuteSet {
	return &MuteSet{muted: make(map[string]bool)}
}

func (s *MuteSet) Mute(modelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.muted[modelID] = true
}

func (s *MuteSet) Unmute(modelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.muted, modelID)
}

func (s *MuteSet) IsMuted(modelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.muted[modelID]
}

func (s *MuteSet) ActiveModels(models []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active := make([]string, 0, len(models))
	for _, m := range models {
		if !s.muted[m] {
			active = append(active, m)
		}
	}
	return active
}
