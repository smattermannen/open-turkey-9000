package cli

import (
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

// A setting's locks can be added to and made stronger, never weaker.
func TestStrengthenSettingLock(t *testing.T) {
	const now, boot = 1000, "boot-a"
	state := func(l db.SettingLock) settingLockState {
		l.Name = "safesearch"
		return settingLockState{lock: l, trusted: now, bootID: boot}
	}

	next, changes, err := strengthenSettingLock(state(db.SettingLock{}), db.SettingLock{LockedUntil: 2000, LockChars: 100}, false)
	if err != nil || next.LockedUntil != 2000 || next.LockChars != 100 || len(changes) != 2 {
		t.Fatalf("new locks = %+v, %v, %v", next, changes, err)
	}
	if _, _, err := strengthenSettingLock(state(next), db.SettingLock{LockedUntil: 1500}, false); err == nil {
		t.Error("an earlier end should be refused")
	}
	if _, _, err := strengthenSettingLock(state(next), db.SettingLock{LockChars: 50}, true); err == nil {
		t.Error("a shorter challenge should be refused")
	}
	if got, _, err := strengthenSettingLock(state(next), db.SettingLock{LockChars: 400}, true); err != nil || got.LockChars != 400 {
		t.Errorf("a longer challenge = %+v, %v", got, err)
	}
	if got, changes, err := strengthenSettingLock(state(next), db.SettingLock{LockChars: 30}, false); err != nil || got.LockChars != 100 || len(changes) != 0 {
		t.Errorf("--lock without --lock-chars on a locked setting = %+v, %v, %v; want unchanged", got, changes, err)
	}

	withPassword := db.SettingLock{PasswordHash: "x"}
	if _, _, err := strengthenSettingLock(state(withPassword), db.SettingLock{PasswordHash: "y"}, false); err == nil {
		t.Error("replacing a password should be refused")
	}

	// Locks that ran out don't count: an expired time lock can be set again
	// to any later time, and a restart lock from an earlier boot is dropped.
	stale := db.SettingLock{LockedUntil: 500, RestartBoot: "boot-old"}
	got, _, err := strengthenSettingLock(state(stale), db.SettingLock{LockedUntil: 1200}, false)
	if err != nil || got.LockedUntil != 1200 || got.RestartBoot != "" {
		t.Errorf("over stale locks = %+v, %v", got, err)
	}
}
