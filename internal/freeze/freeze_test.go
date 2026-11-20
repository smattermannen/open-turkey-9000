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
		{7 * 24 * time.Hour, true}, // R-FREEZE-3: at most a week (owner 2026-11-17)
		{7*24*time.Hour + time.Minute, false},
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

// R-FREEZE-1, §3.5: extending a running freeze never turns it into allowed
// apps only, which lets programs run; the other way is free.
func TestExtendMode(t *testing.T) {
	for _, c := range []struct{ running, next, want string }{
		{"", ModeApps, ModeApps},
		{ModeLock, ModeApps, ModeLock},
		{ModeLogout, ModeApps, ModeLogout},
		{ModeShutdown, ModeApps, ModeShutdown},
		{ModeApps, ModeLock, ModeLock},
		{ModeApps, ModeApps, ModeApps},
		{ModeLock, ModeShutdown, ModeShutdown},
	} {
		if got := ExtendMode(c.running, c.next); got != c.want {
			t.Errorf("ExtendMode(%q, %q) = %q; want %q", c.running, c.next, got, c.want)
		}
	}
}
