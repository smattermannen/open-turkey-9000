package blocker

import (
	"os"
	"testing"
)

func TestRegularUser(t *testing.T) {
	cases := map[uint64]bool{
		0: false, 1: false, 999: false, // root and system accounts
		1000: true, 1001: true, 59999: true, 60000: true, // people
		65534: false,               // nobody
		61184: false, 65519: false, // systemd dynamic service accounts
		61183: true, 65520: true,
		100000: true, 165535: true, // rootless containers: the user's own
		4294967295: false, // "no user"
	}
	for uid, want := range cases {
		if got := regularUser(uid); got != want {
			t.Errorf("regularUser(%d) = %v, want %v", uid, got, want)
		}
	}
}

func TestParseStatusUIDs(t *testing.T) {
	status := "Name:\tsudo\nUmask:\t0022\nState:\tS (sleeping)\nUid:\t1000\t0\t0\t0\nGid:\t1000\t1000\t1000\t1000\n"
	r, e, ok := parseStatusUIDs(status)
	if !ok || r != 1000 || e != 0 {
		t.Fatalf("got %d %d %v, want 1000 0 true", r, e, ok)
	}
	if _, _, ok := parseStatusUIDs("Name:\tx\n"); ok {
		t.Error("status without a Uid line parsed")
	}
}

// The test itself runs as some user; a process is "owned by a user" exactly
// when that user is a regular one.
func TestOwnedByUserSelf(t *testing.T) {
	uid := uint64(os.Getuid())
	if got, want := ownedByUser(os.Getpid()), regularUser(uid); got != want {
		t.Errorf("ownedByUser(self, uid %d) = %v, want %v", uid, got, want)
	}
	if ownedByUser(-1) {
		t.Error("a missing process counted as a user's")
	}
	if got, want := ownedBy(os.Getpid(), uid), regularUser(uid); got != want {
		t.Errorf("ownedBy(self, %d) = %v, want %v", uid, got, want)
	}
	if ownedBy(os.Getpid(), uid+1) {
		t.Error("a process counted as another user's")
	}
}

func TestEssentialApps(t *testing.T) {
	refused := []string{"Xorg", "cinnamon", "CINNAMON-SESSION", "csd-media-keys", "python3", "python3.12",
		"/usr/bin/Xorg", "pipewire", "kc9k-gui", "dbus-daemon"}
	for _, entry := range refused {
		if err := ValidAppEntry(entry); err == nil {
			t.Errorf("%s was accepted", entry)
		}
	}
	allowed := []string{"discord", "steam", "com.discordapp.Discord", "/opt/games/", "bash", "firefox", "python-game"}
	for _, entry := range allowed {
		if err := ValidAppEntry(entry); err != nil {
			t.Errorf("%s was refused: %v", entry, err)
		}
	}
	got := withoutEssentialApps([]string{"steam", "Xorg", "discord", "python3"})
	if len(got) != 2 || got[0] != "steam" || got[1] != "discord" {
		t.Errorf("withoutEssentialApps = %v", got)
	}
}
