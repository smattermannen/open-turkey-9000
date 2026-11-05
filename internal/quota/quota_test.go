package quota

import "testing"

func TestSpendAndClose(t *testing.T) {
	s := New(20, 100, "d1")
	s.Open = true
	s = s.Step(110, "d1")
	if s.Used != 10 || !s.Open {
		t.Fatalf("after 10s: %+v", s)
	}
	s = s.Step(120, "d1")
	if s.Remaining() != 0 || s.Open {
		t.Fatalf("spent quota should close the session: %+v", s)
	}
}

func TestClosedSessionSpendsNothing(t *testing.T) {
	if s := New(20, 100, "d1").Step(500, "d1"); s.Used != 0 {
		t.Fatalf("used = %d", s.Used)
	}
}

func TestStepIsCapped(t *testing.T) {
	s := New(600, 100, "d1")
	s.Open = true
	if s = s.Step(100000, "d1"); s.Used != MaxStep {
		t.Fatalf("forward jump: used = %d", s.Used)
	}
	if s = s.Step(5, "d1"); s.Used != MaxStep {
		t.Fatalf("backward jump: used = %d", s.Used)
	}
}

func TestSessionEnd(t *testing.T) {
	s := New(600, 100, "d1")
	s.Open, s.SessionEnd = true, 105
	if s = s.Step(106, "d1"); s.Open {
		t.Fatal("session should end at SessionEnd")
	}
}

func TestNewDay(t *testing.T) {
	s := New(60, 100, "d1").SetLimit(120)
	if s.Limit != 60 {
		t.Fatal("a raise must wait for midnight")
	}
	s.Used, s.Open = 30, true
	s = s.Step(110, "d2")
	if s.Used != 0 || s.Open || s.Limit != 120 || s.PendingLimit != NoPending {
		t.Fatalf("new day: %+v", s)
	}
}

func TestLowerAppliesNow(t *testing.T) {
	if s := New(120, 0, "d1").SetLimit(60); s.Limit != 60 {
		t.Fatalf("limit = %d", s.Limit)
	}
}

func TestRemovalAtMidnight(t *testing.T) {
	s := New(60, 0, "d1")
	s.PendingLimit = 0
	if s = s.Step(1, "d2"); s.Limit != 0 {
		t.Fatalf("limit = %d", s.Limit)
	}
}
