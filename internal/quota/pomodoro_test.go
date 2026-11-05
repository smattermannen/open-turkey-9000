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
