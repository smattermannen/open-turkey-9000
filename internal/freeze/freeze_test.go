package freeze

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		d  time.Duration
		ok bool
	}{
		{30 * time.Second, false},
		{time.Minute, true},
		{24 * time.Hour, true},
		{25 * time.Hour, false},
	}
	for _, c := range cases {
		if err := Validate(c.d); (err == nil) != c.ok {
			t.Errorf("Validate(%s) = %v; want ok=%v", c.d, err, c.ok)
		}
	}
}

func TestEngaged(t *testing.T) {
	if !Engaged(10, 20, "quiet splash") {
		t.Error("should be engaged before the end time")
	}
	if Engaged(20, 20, "quiet splash") {
		t.Error("should end at the end time")
	}
	if Engaged(10, 20, "quiet kc9k.nofreeze splash") {
		t.Error("the kernel parameter should bypass the freeze")
	}
	if !Engaged(10, 20, "xkc9k.nofreeze") {
		t.Error("only the exact parameter should bypass the freeze")
	}
}
