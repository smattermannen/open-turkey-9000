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

// useGuestPasswd points guest detection at a passwd file for one test.
func useGuestPasswd(t *testing.T, passwd string) {
	t.Helper()
	path := t.TempDir() + "/passwd"
	if err := os.WriteFile(path, []byte(passwd), 0o644); err != nil {
		t.Fatal(err)
	}
	old := passwdPath
	passwdPath = path
	t.Cleanup(func() {
		passwdPath = old
		guestCache.Lock()
		guestCache.uids, guestCache.stamp = nil, ""
		guestCache.Unlock()
	})
}

const guestPasswd = "root:x:0:0:root:/root:/bin/bash\n" +
	"lightdm:x:109:116:Light Display Manager:/var/lib/lightdm:/bin/false\n" +
	"guest-331bld:x:999:988:Guest:/tmp/guest-331bld:/bin/bash\n" +
	"guest-fake:x:998:987::/home/guest-fake:/bin/bash\n" +
	"tester:x:1000:1000::/home/tester:/bin/bash\n"

// B3, §3.8 strict mode: a LightDM guest session runs as a system account
// (uid 999 here); it is a person all the same, so blocks reach its programs.
// A guest-named account elsewhere, and system accounts, are not.
func TestGuestIsPerson(t *testing.T) {
	useGuestPasswd(t, guestPasswd)
	cases := map[uint64]bool{999: true, 998: false, 109: false, 0: false, 1000: true}
	for uid, want := range cases {
		if got := personUser(uid); got != want {
			t.Errorf("personUser(%d) = %v, want %v", uid, got, want)
		}
	}
	if got := parsePasswdUsers(guestPasswd); len(got) != 1 || got[0] != "tester" {
		t.Errorf("accounts to pick for a block = %v, want only tester (a guest comes and goes)", got)
	}
}

// B3: a logout freeze ends the guest's session too.
func TestUsersToLogOutGuest(t *testing.T) {
	useGuestPasswd(t, guestPasswd)
	out := "1000 tester no active\n 999 guest-331bld no active\n 109 lightdm no active\n"
	if got := usersToLogOut(out); len(got) != 2 || got[0] != "tester" || got[1] != "guest-331bld" {
		t.Errorf("usersToLogOut = %v, want [tester guest-331bld]", got)
	}
}
