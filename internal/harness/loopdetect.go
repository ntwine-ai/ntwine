package harness

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

type LoopAction int

const (
	LoopOK LoopAction = iota
	LoopWarn
	LoopForceText
	LoopAbort
)

type LoopDetector struct {
	mu         sync.Mutex
	history    []callSignature
	windowSize int
	warnAt     int
	forceAt    int
	abortAt    int
}

type callSignature struct {
	toolName string
	argsHash string
}

func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		windowSize: 20,
		warnAt:     3,
		forceAt:    5,
		abortAt:    8,
	}
}

func (ld *LoopDetector) Record(toolName string, argsJSON string) LoopAction {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	h := sha256.Sum256([]byte(argsJSON))
	sig := callSignature{
		toolName: toolName,
		argsHash: fmt.Sprintf("%x", h[:8]),
	}

	ld.history = append(ld.history, sig)

	if len(ld.history) > ld.windowSize {
		ld.history = ld.history[len(ld.history)-ld.windowSize:]
	}

	count := 0
	for _, s := range ld.history {
		if s.toolName == sig.toolName && s.argsHash == sig.argsHash {
			count++
		}
	}

	if count >= ld.abortAt {
		return LoopAbort
	}
	if count >= ld.forceAt {
		return LoopForceText
	}
	if count >= ld.warnAt {
		return LoopWarn
	}
	return LoopOK
}

func (ld *LoopDetector) Reset() {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.history = nil
}

func (ld *LoopDetector) WarningMessage(toolName string, count int) string {
	return fmt.Sprintf("you've called %s with the same arguments %d times. try a different approach.", toolName, count)
}
