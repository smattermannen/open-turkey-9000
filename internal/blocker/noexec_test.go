//go:build linux

package blocker

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMountOptions(t *testing.T) {
	info := "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n" +
		"30 22 8:1 /home /home rw,nosuid,nodev,noexec,relatime shared:1 - ext4 /dev/sda1 rw\n"
	mounts := mountOptions(info)
	if hasOption(mounts["/"], "noexec") {
		t.Error("/ is not noexec")
	}
	if !hasOption(mounts["/home"], "noexec") {
		t.Error("/home is noexec")
	}
	if hasOption("rw,noexecish", "noexec") {
		t.Error("only exact options match")
	}
}

// B3, §3.8 strict mode: nothing runs from what is mounted inside the noexec
// folders (a guest session's tmpfs home in /tmp) or from a user's runtime
// folder (/run/user/<uid>), except exempt folders and what is inside them.
func TestNoexecSubmounts(t *testing.T) {
	mounts := map[string]string{
		"/":                    "rw,relatime",
		"/home":                "rw,nosuid,nodev,noexec",
		"/tmp":                 "rw,nosuid,nodev,noexec",
		"/tmp/guest-331bld":    "rw,relatime",
		"/home/tester/games":   "rw,nosuid,nodev", // exempt
		"/home/tester/games/x": "rw",              // inside an exempt folder
		"/home/anna":           "rw,noexec",       // already noexec
		"/run":                 "rw,nosuid,nodev,noexec",
		"/run/user/1000":       "rw,nosuid,nodev,relatime",
		"/run/user/1000/doc":   "rw,nosuid,nodev,user_id=1000",
		"/run/lock":            "rw,nosuid,nodev,noexec",
		"/run/user/0":          "rw,nosuid,nodev", // root's
		"/run/user/120":        "rw,nosuid,nodev", // the display manager's
		"/media/usb":           "rw",
		`/tmp/odd\040name`:     "rw",
	}
	got := noexecSubmounts(mounts, []string{"/home/tester/games"})
	want := []string{"/run/user/1000", "/run/user/1000/doc", "/tmp/guest-331bld"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("noexecSubmounts = %v, want %v", got, want)
	}
}

// B3: the noexec folders are bound recursively, so a home already mounted
// inside one stays in view (then made noexec by noexecSubmounts).
func TestNoexecPlanRecursive(t *testing.T) {
	plan := noexecPlan(map[string]string{}, nil)
	if len(plan) == 0 || plan[0][0] != "--rbind" {
		t.Errorf("plan = %v, want --rbind first", plan)
	}
}

// B3, §3.8: a Flatpak app from a user installation, or on a user-installed
// runtime, runs from a noexec folder and is closed; system ones and ones in
// an exempt folder are not.
func TestFlatpakFromNoexec(t *testing.T) {
	info := func(app, runtime string) string {
		return "[Application]\nname=org.kc9k.B3Test\nruntime=runtime/org.freedesktop.Platform/x86_64/24.08\n\n" +
			"[Instance]\ninstance-id=123\napp-path=" + app + "\nruntime-path=" + runtime + "\nbranch=master\n"
	}
	sys := "/var/lib/flatpak/runtime/org.freedesktop.Platform/x86_64/24.08/abc/files"
	cases := []struct {
		name, info string
		exempt     []string
		want       bool
	}{
		{"user app", info("/home/tester/.local/share/flatpak/app/org.kc9k.B3Test/x86_64/master/abc/files", sys), nil, true},
		{"user runtime", info("/var/lib/flatpak/app/org.kc9k.B3Test/x86_64/master/abc/files", "/home/tester/.local/share/flatpak/runtime/x/files"), nil, true},
		{"system", info("/var/lib/flatpak/app/org.kc9k.B3Test/x86_64/master/abc/files", sys), nil, false},
		{"exempt", info("/home/tester/.local/share/flatpak/app/x/files", sys), []string{"/home/tester/.local/share/flatpak"}, false},
		{"path only in another section", "[Application]\napp-path=/home/x\n", nil, false},
	}
	for _, c := range cases {
		if got := flatpakFromNoexec(c.info, c.exempt); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// B3: a guest's home mounted in /tmp during the lockdown is copied beneath
// the noexec bind of /tmp too (shared propagation); the copy inside the bind
// is the one in view, and only its options count.
func TestMountOptionsInView(t *testing.T) {
	info := "1 0 253:1 / / rw shared:1 - ext4 /dev/vda1 rw\n" +
		"30 1 0:5 / /run rw,nosuid,nodev,noexec shared:2 - tmpfs tmpfs rw\n" +
		"324 1 253:1 /tmp /tmp rw,nosuid,nodev,noexec shared:1 - ext4 /dev/vda1 rw\n" +
		"334 324 0:51 / /tmp/guest-331bld rw,nosuid,nodev,noexec shared:383 - tmpfs none rw\n" +
		"335 1 0:51 / /tmp/guest-331bld rw,relatime shared:383 - tmpfs none rw\n" +
		"400 1 253:1 /home /home rw,relatime shared:1 - ext4 /dev/vda1 rw\n" +
		"401 400 253:1 /home /home rw,noexec shared:1 - ext4 /dev/vda1 rw\n" +
		"402 1 0:60 / /hidden/x rw shared:9 - tmpfs t rw\n"
	got := mountOptions(info)
	if !hasOption(got["/tmp/guest-331bld"], "noexec") {
		t.Errorf("guest home in view = %q, want the noexec one", got["/tmp/guest-331bld"])
	}
	if !hasOption(got["/home"], "noexec") {
		t.Errorf("/home = %q, want the bind stacked on top", got["/home"])
	}
	if !hasOption(got["/run"], "noexec") || hasOption(got["/"], "noexec") {
		t.Errorf("mounts = %v", got)
	}
	if _, ok := got["/hidden/x"]; !ok {
		t.Error("a mount on / below a folder that isn't a mount point is in view")
	}
}

// B3, §3.8 strict mode: a person's own executable doesn't run from a noexec
// folder (started before the lockdown), a runtime folder, or below a folder
// anyone can write to on a mount that runs programs (/var/crash); an exempt
// folder, and a person's file elsewhere (a tarball in /opt), do.
func TestForbiddenPlace(t *testing.T) {
	useGuestPasswd(t, guestPasswd)
	exempt := []string{"/home/tester/games"}
	cases := []struct {
		path   string
		shared bool
		want   bool
	}{
		{"/home/tester/Downloads/browser/firefox", false, true},
		{"/tmp/x/app", true, true},
		{"/run/user/1000/b3true", false, true},
		{"/var/crash/b3true", true, true},
		{"/timeshift/snapshots/2026-11-20/localhost/tmp/x", true, true},
		{"/home/tester/games/steam/steam", false, false},
		{"/opt/Telegram/Telegram", false, false},
		{"/run/user/0/x", false, false},
	}
	for _, c := range cases {
		if got := forbiddenPlace(c.path, exempt, c.shared); got != c.want {
			t.Errorf("%s (shared %v): got %v, want %v", c.path, c.shared, got, c.want)
		}
	}
	dir := t.TempDir()
	open := dir + "/open"
	if err := os.Mkdir(open, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o1777); err != nil {
		t.Fatal(err)
	}
	if !worldWritableAbove(open+"/sub/x") || worldWritableAbove("/usr/bin/true") {
		t.Error("worldWritableAbove")
	}
}

// B3, §3.8: lifting strict mode takes away only KC9K's binds. Seen in the VM
// (2026-11-20): unmounting the shared /tmp bind unmounted every guest's home
// with it, and an exempt folder's bind left a noexec copy beneath. Exempt
// binds go first, then each bind is made private before it goes; a tmpfs
// /dev/shm mounted noexec of its own, and a user's own mount at an exempt
// folder, stay.
func TestRemoveNoexecPlan(t *testing.T) {
	info := "1 0 253:1 /@ / rw shared:1 - btrfs /dev/vda1 rw\n" +
		"30 1 0:5 / /dev/shm rw,nosuid,nodev,noexec shared:2 - tmpfs tmpfs rw\n" +
		"40 1 0:51 / /tmp/guest-a rw,relatime shared:9 - tmpfs none rw\n" +
		"181 1 253:1 /@/home /home rw,nosuid,nodev,noexec shared:1 - btrfs /dev/vda1 rw\n" +
		"190 1 253:1 /@/tmp /tmp rw,nosuid,nodev,noexec shared:1 - btrfs /dev/vda1 rw\n" +
		"191 190 0:51 / /tmp/guest-a rw,nosuid,nodev,noexec shared:9 - tmpfs none rw\n" +
		"706 181 253:1 /@/home/tester/games /home/tester/games rw,nosuid,nodev shared:1 - btrfs /dev/vda1 rw\n" +
		"707 1 253:1 /@/home/tester/games /home/tester/games rw,nosuid,nodev,noexec shared:1 - btrfs /dev/vda1 rw\n" +
		"710 181 0:60 / /home/tester/disk rw shared:20 - ext4 /dev/sdb1 rw\n"
	got := removeNoexecPlan(visibleMounts(info), []string{"/home/tester/games", "/home/tester/disk"})
	want := [][]string{
		{"umount", "-l", "/home/tester/games"},
		{"mount", "--make-rprivate", "/home"}, {"umount", "-l", "/home"},
		{"mount", "--make-rprivate", "/tmp"}, {"umount", "-l", "/tmp"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("plan = %v\nwant %v", got, want)
	}
}

// ourBind tells KC9K's bind of a noexec folder from a folder mounted on its
// own.
func TestOurBind(t *testing.T) {
	cases := map[string]bool{
		"1 0 8:1 / / rw - ext4 x rw\n9 1 8:1 /tmp /tmp rw,noexec - ext4 x rw":                                       true,
		"1 0 8:1 / / rw - ext4 x rw\n9 1 0:3 / /tmp rw,noexec - tmpfs x rw":                                         false,
		"1 0 8:1 / / rw - ext4 x rw\n5 1 0:3 / /dev/shm rw - tmpfs x rw\n9 5 0:3 / /dev/shm rw,noexec - tmpfs x rw": true,
		"1 0 8:1 /@ / rw - btrfs x rw\n9 1 8:1 /@/home /home rw,noexec - btrfs x rw":                                true,
		"1 0 8:1 /@ / rw - btrfs x rw\n9 1 8:1 /@home /home rw,noexec - btrfs x rw":                                 false,
	}
	for info, want := range cases {
		vis := visibleMounts(info)
		var m mount
		for p, v := range vis {
			if p != "/" {
				m = v
			}
		}
		if got := ourBind(m); got != want {
			t.Errorf("%q: ourBind = %v, want %v", info, got, want)
		}
	}
}
