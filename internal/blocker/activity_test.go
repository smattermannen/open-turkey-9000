package blocker

import (
	"errors"
	"os/user"
	"testing"
)

func TestLastField(t *testing.T) {
	cases := map[string]string{
		"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3e00049\n": "0x3e00049",
		"_NET_WM_PID(CARDINAL) = 7276\n":                      "7276",
		"(uint32 42,)\n":                                      "42",
		"":                                                    "",
	}
	for in, want := range cases {
		if got := lastField(in); got != want {
			t.Errorf("lastField(%q) = %q, want %q", in, got, want)
		}
	}
}

// B8 (§3.7 Use): a locked screen is asked of logind (LockedHint), never of
// the session bus, where any program the user runs can answer for
// org.cinnamon.ScreenSaver. Every desktop session must be locked; a text
// console doesn't count, and without logind nobody is away for a lock.
func TestScreenLockedByLogind(t *testing.T) {
	fake := func(sessions string, props map[string]string) func(...string) (string, error) {
		return func(args ...string) (string, error) {
			switch args[0] {
			case "show-user":
				if args[1] != "1000" {
					return "", errors.New("no such user")
				}
				return sessions, nil
			case "show-session":
				if p, ok := props[args[1]]; ok {
					return p, nil
				}
			}
			return "", errors.New("no such session")
		}
	}
	for _, c := range []struct {
		name     string
		uid      string
		sessions string
		props    map[string]string
		locked   bool
	}{
		{"locked desktop", "1000", "3\n", map[string]string{"3": "Type=x11\nLockedHint=yes\n"}, true},
		{"unlocked desktop", "1000", "3\n", map[string]string{"3": "Type=x11\nLockedHint=no\n"}, false},
		{"wayland locked", "1000", "5", map[string]string{"5": "Type=wayland\nLockedHint=yes"}, true},
		{"console beside a locked desktop", "1000", "3 c2", map[string]string{"3": "Type=x11\nLockedHint=yes", "c2": "Type=tty\nLockedHint=no"}, true},
		{"one of two desktops unlocked", "1000", "3 7", map[string]string{"3": "Type=x11\nLockedHint=yes", "7": "Type=x11\nLockedHint=no"}, false},
		{"only a console", "1000", "c2", map[string]string{"c2": "Type=tty\nLockedHint=yes"}, false},
		{"no sessions", "1000", "", nil, false},
		{"unknown user", "1001", "3", map[string]string{"3": "Type=x11\nLockedHint=yes"}, false},
		{"not a uid", "-1000", "3", map[string]string{"3": "Type=x11\nLockedHint=yes"}, false},
		{"odd session id ignored", "1000", "--all", map[string]string{"--all": "Type=x11\nLockedHint=yes"}, false},
	} {
		if got := lockedByLogind(c.uid, fake(c.sessions, c.props)); got != c.locked {
			t.Errorf("%s: locked %v, want %v", c.name, got, c.locked)
		}
	}
	none := func(...string) (string, error) { return "", errors.New("loginctl not installed") }
	if lockedByLogind("1000", none) {
		t.Error("without logind the screen counts as locked")
	}

	// awayFromDesktop: the lock alone makes the user away, input or not.
	savedIdle, savedLocked := x11IdleSeconds, screenLocked
	t.Cleanup(func() { x11IdleSeconds, screenLocked = savedIdle, savedLocked })
	x11IdleSeconds = func(*user.User) (uint64, bool) { return 5, true }
	u := &user.User{Uid: "1000", HomeDir: "/nonexistent"}
	screenLocked = func(uid string) bool { return uid == "1000" }
	if !awayFromDesktop(u, "", 180) {
		t.Error("locked screen with recent input: not away")
	}
	screenLocked = func(string) bool { return false }
	if awayFromDesktop(u, "", 180) {
		t.Error("unlocked screen with recent input: away")
	}
}
