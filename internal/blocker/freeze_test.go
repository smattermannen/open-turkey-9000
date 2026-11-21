package blocker

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// §3.8 freeze mode logout: every regular user is ended, but one already
// closing is not ended again (systemd 255's logind spun at 100% CPU on that).
func TestUsersToLogOut(t *testing.T) {
	out := "1000 tester no active\n 109 lightdm no active\n1001 anna no closing\n1002 bo\n\n"
	if got, want := usersToLogOut(out), []string{"tester", "bo"}; !reflect.DeepEqual(got, want) {
		t.Errorf("usersToLogOut = %v, want %v", got, want)
	}
}

// R-FREEZE-2: during a logout freeze no one logs in again; the file is
// removed after it, and the system's own nologin file is left alone.
func TestR_FREEZE_2_HoldLogins(t *testing.T) {
	old := NologinPath
	NologinPath = filepath.Join(t.TempDir(), "nologin")
	t.Cleanup(func() { NologinPath = old })

	until := time.Date(2026, 11, 20, 6, 30, 0, 0, time.Local)
	if err := HoldLogins(until); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(NologinPath)
	if err != nil || !strings.Contains(string(data), "2026-11-20 06:30") {
		t.Fatalf("nologin = %q, %v", data, err)
	}
	if err := ReleaseLogins(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(NologinPath); !os.IsNotExist(err) {
		t.Fatalf("nologin still there after the freeze: %v", err)
	}

	system := "System is going down.\n"
	if err := os.WriteFile(NologinPath, []byte(system), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := HoldLogins(until); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseLogins(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(NologinPath); string(data) != system {
		t.Errorf("the system's nologin file became %q", data)
	}
}
