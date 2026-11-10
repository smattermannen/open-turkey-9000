package quota

import "testing"

func TestOnBreak(t *testing.T) {
	cases := []struct {
		now  int64
		want bool
	}{
		{99, false}, {100, false}, {124, false}, {125, true}, {129, true}, {130, false}, {155, true},
	}
	for _, c := range cases {
		if got := OnBreak(25, 5, 100, c.now); got != c.want {
			t.Errorf("now %d: got %v", c.now, got)
		}
	}
	if OnBreak(0, 5, 0, 10) {
		t.Error("an unset cycle has no break")
	}
}

func TestLongBreak(t *testing.T) {
	// 25 work, 5 break, a 15-minute break after every 4th work period.
	c := Cycle{On: 25 * 60, Off: 5 * 60, Long: 15 * 60, Every: 4, Anchor: 1000}
	at := func(min int64) int64 { return c.Anchor + min*60 }
	cases := []struct {
		min  int64
		left int64 // minutes of break left; 0: working
	}{
		{0, 0}, {24, 0}, {25, 5}, {29, 1}, {30, 0}, // first period and short break
		{55, 5}, {85, 5}, // second and third breaks
		{90, 0}, {114, 0}, {115, 15}, {129, 1}, // fourth work period, then the long break
		{130, 0}, {155, 5}, // the round starts over
	}
	for _, tc := range cases {
		if got := c.BreakLeft(at(tc.min)); got != tc.left*60 {
			t.Errorf("minute %d: %d s of break left; want %d", tc.min, got, tc.left*60)
		}
	}
	// Without a long break it is the plain cycle.
	plain := Cycle{On: 60, Off: 30, Anchor: 0}
	if plain.BreakLeft(59) != 0 || plain.BreakLeft(60) != 30 || plain.BreakLeft(89) != 1 || plain.BreakLeft(90) != 0 {
		t.Error("plain cycle is off")
	}
	if c.BreakLeft(c.Anchor-1) != 0 {
		t.Error("a break before the cycle began")
	}
}
