package enforce

import (
	"slices"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// fakeLockdown records what enforceLockdown does instead of doing it.
func fakeLockdown(t *testing.T, cmdline string) *[]string {
	t.Helper()
	var did []string
	oldA, oldR, oldNA, oldNR, oldK := applyLockdown, removeLockdown, applyNoexec, removeNoexec, kernelCmdline
	t.Cleanup(func() {
		applyLockdown, removeLockdown, applyNoexec, removeNoexec, kernelCmdline = oldA, oldR, oldNA, oldNR, oldK
	})
	applyLockdown = func(user string) error { did = append(did, "lockdown "+user); return nil }
	removeLockdown = func() error { did = append(did, "remove"); return nil }
	applyNoexec = func(exempt ...string) error {
		did = append(did, "noexec "+slices.Concat(exempt, []string{""})[0])
		return nil
	}
	removeNoexec = func(...string) error { did = append(did, "remove noexec"); return nil }
	kernelCmdline = func() string { return cmdline }
	return &did
}

// §3.8 strict mode (R-STRICT-1, R-ESCAPE-1): the lockdown (sudo, pkexec,
// root login and su, noexec) is on only while strict mode is on and
// something other than its own lock is locked; kc9k.nofreeze lifts it for
// that boot unless no-escape is on.
func TestEnforceLockdownWhen(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	trusted := func() int64 {
		n, err := d.TrustedNow(now)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	run := func(cmdline string) []string {
		did := fakeLockdown(t, cmdline)
		enforceLockdown(d, now, trusted())
		return *did
	}
	removed := []string{"remove", "remove noexec"}

	if got := run(""); !slices.Equal(got, removed) {
		t.Errorf("strict off: %v", got)
	}
	_ = d.SetSetting(SettingStrict, "on")
	_ = d.SetSetting(SettingStrictUser, "me")
	_ = d.SetSetting(SettingStrictExempt, `["/home/me/bin"]`)
	if got := run(""); !slices.Equal(got, removed) {
		t.Errorf("strict on, nothing locked: %v", got)
	}
	// Its own lock only keeps strict mode on.
	if err := d.SetSettingLock(db.SettingLock{Name: StrictLockName, LockedUntil: trusted() + 3600}); err != nil {
		t.Fatal(err)
	}
	if got := run(""); !slices.Equal(got, removed) {
		t.Errorf("only strict mode's own lock: %v", got)
	}
	// Another setting locked: the lockdown holds.
	if err := d.SetSettingLock(db.SettingLock{Name: "web-only", LockedUntil: trusted() + 3600}); err != nil {
		t.Fatal(err)
	}
	if got := run(""); !slices.Equal(got, []string{"lockdown me", "noexec /home/me/bin"}) {
		t.Errorf("a setting locked: %v", got)
	}
	// kc9k.nofreeze: lifted for this boot, unless no-escape is on.
	if got := run("ro quiet kc9k.nofreeze"); !slices.Equal(got, removed) {
		t.Errorf("with kc9k.nofreeze: %v", got)
	}
	_ = d.SetSetting(SettingNoEscape, "on")
	if got := run("ro quiet kc9k.nofreeze"); !slices.Equal(got, []string{"lockdown me", "noexec /home/me/bin"}) {
		t.Errorf("kc9k.nofreeze with no-escape on: %v", got)
	}
}
