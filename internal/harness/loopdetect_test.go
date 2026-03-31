package harness

import "testing"

func TestLoopDetector_InitialCallOK(t *testing.T) {
	ld := NewLoopDetector()
	action := ld.Record("my_tool", `{"x":1}`)
	if action != LoopOK {
		t.Errorf("expected LoopOK on first call, got %v", action)
	}
}

func TestLoopDetector_DifferentArgsNeverLoop(t *testing.T) {
	ld := NewLoopDetector()
	for i := 0; i < 20; i++ {
		action := ld.Record("tool", `{"i":`+string(rune('0'+i))+`}`)
		if action != LoopOK {
			t.Errorf("call %d with unique args should be LoopOK, got %v", i, action)
		}
	}
}

func TestLoopDetector_WarnAtThree(t *testing.T) {
	ld := NewLoopDetector()
	args := `{"a":1}`

	ld.Record("t", args) // 1 — ok
	ld.Record("t", args) // 2 — ok
	action := ld.Record("t", args) // 3 — warn

	if action != LoopWarn {
		t.Errorf("expected LoopWarn at count 3, got %v", action)
	}
}

func TestLoopDetector_ForceAtFive(t *testing.T) {
	ld := NewLoopDetector()
	args := `{"b":2}`

	for i := 0; i < 4; i++ {
		ld.Record("t", args)
	}
	action := ld.Record("t", args) // 5th

	if action != LoopForceText {
		t.Errorf("expected LoopForceText at count 5, got %v", action)
	}
}

func TestLoopDetector_AbortAtEight(t *testing.T) {
	ld := NewLoopDetector()
	args := `{"c":3}`

	for i := 0; i < 7; i++ {
		ld.Record("t", args)
	}
	action := ld.Record("t", args) // 8th

	if action != LoopAbort {
		t.Errorf("expected LoopAbort at count 8, got %v", action)
	}
}

func TestLoopDetector_Reset_ClearsHistory(t *testing.T) {
	ld := NewLoopDetector()
	args := `{"d":4}`
	for i := 0; i < 8; i++ {
		ld.Record("t", args)
	}

	ld.Reset()

	action := ld.Record("t", args)
	if action != LoopOK {
		t.Errorf("after reset, first call should be LoopOK, got %v", action)
	}
}

func TestLoopDetector_DifferentToolsSameArgs_Independent(t *testing.T) {
	ld := NewLoopDetector()
	args := `{"x":1}`

	// 8 calls on tool_a
	for i := 0; i < 7; i++ {
		ld.Record("tool_a", args)
	}

	// tool_b with same args should start fresh
	action := ld.Record("tool_b", args)
	if action != LoopOK {
		t.Errorf("different tool with same args should be LoopOK, got %v", action)
	}
}

func TestLoopDetector_WindowSizeEvicts_OldCalls(t *testing.T) {
	ld := NewLoopDetector()
	sameArgs := `{"same":true}`
	otherArgs := `{"other":`

	// flood with 20 different calls to fill the window
	for i := 0; i < 20; i++ {
		unique := otherArgs + string(rune('0'+i%10)) + `}`
		ld.Record("t", unique)
	}

	// the same-args calls were never recorded; first one should be OK
	action := ld.Record("t", sameArgs)
	if action != LoopOK {
		t.Errorf("expected LoopOK after window eviction, got %v", action)
	}
}

func TestLoopDetector_WarningMessage(t *testing.T) {
	ld := NewLoopDetector()
	msg := ld.WarningMessage("my_tool", 5)
	if msg == "" {
		t.Error("expected non-empty warning message")
	}
}

func TestLoopDetector_EmptyArgsDistinctFromPopulated(t *testing.T) {
	ld := NewLoopDetector()
	ld.Record("t", "")
	ld.Record("t", "")
	action := ld.Record("t", "") // 3rd empty
	if action != LoopWarn {
		t.Errorf("empty args should also trigger loop detection at 3, got %v", action)
	}
}
