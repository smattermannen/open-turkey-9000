// projectbackup.go — what only a backup holds (§3.13 "Backup: everything"):
// running locks, password hashes, hotkeys and statistics. A share leaves
// them out. Import only adds: a password where none is set, a hotkey whose
// keys are free, statistics days not already here, and running locks on the
// blocks it creates (those blocks come in switched on) and on settings not
// locked here; every lock is confirmed first (R-LOCK-12).
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// blockLockExport is a block's running locks (trusted unix times).
type blockLockExport struct {
	Block       string `json:"block"`
	LockedUntil int64  `json:"locked_until,omitempty"`
	TimerAfter  int64  `json:"timer_after_seconds,omitempty"`
	LockChars   int    `json:"lock_chars,omitempty"`
	Style       string `json:"lock_style,omitempty"`
	Password    string `json:"password_hash,omitempty"`
	Delay       int64  `json:"delay_seconds,omitempty"`
	Relock      int64  `json:"relock_seconds,omitempty"`
	Order       string `json:"order,omitempty"`
}

// backupExtras is the part of a project file only a backup has.
type backupExtras struct {
	BlockLocks       []blockLockExport `json:"block_locks,omitempty"`
	SettingLocks     []db.SettingLock  `json:"setting_locks,omitempty"`
	SettingsPassword string            `json:"settings_password_hash,omitempty"`
	PasswordForLocks bool              `json:"settings_password_for_locks,omitempty"`
	AccessPasswords  map[string]string `json:"access_password_hashes,omitempty"`
	Hotkeys          []db.Hotkey       `json:"hotkeys,omitempty"`
	BlockPasswords   map[string]string `json:"block_password_hashes,omitempty"` // §3.11, by block
	Statistics       []db.UsageRow     `json:"statistics,omitempty"`
}

// exportBackupExtras gathers what only a backup holds.
func exportBackupExtras(database *db.DB) (*backupExtras, error) {
	out := &backupExtras{AccessPasswords: map[string]string{}}
	trusted, err := database.TrustedNow(time.Now())
	if err != nil {
		return nil, err
	}
	boot, err := db.BootID()
	if err != nil {
		return nil, err
	}
	blocks, err := database.ListBlocks()
	if err != nil {
		return nil, err
	}
	for _, b := range blocks {
		detail, err := database.GetBlock(b.Name)
		if err != nil {
			return nil, err
		}
		e, err := database.GetLockExtras(b.Name)
		if err != nil {
			return nil, err
		}
		if !detail.Active || len(db.LayersOn(detail, e, trusted, boot)) == 0 {
			continue
		}
		l := blockLockExport{Block: b.Name, TimerAfter: e.TimerAfter, Delay: e.Delay, Relock: e.Relock, Order: e.Order}
		if detail.LockedUntil > trusted {
			l.LockedUntil = detail.LockedUntil
		}
		if db.LayerOn(db.LayerText, detail, e, trusted, boot) {
			l.LockChars, l.Style = detail.LockChars, e.Style
		}
		if db.LayerOn(db.LayerPassword, detail, e, trusted, boot) {
			l.Password = e.PasswordHash
		}
		out.BlockLocks = append(out.BlockLocks, l)
	}
	locks, err := database.SettingLocks()
	if err != nil {
		return nil, err
	}
	for _, l := range locks {
		if l.InForce(trusted, boot) && l.Name != enforce.StrictLockName {
			l.RestartBoot, l.LiftedUntil, l.LiftedKinds = "", 0, "" // restart locks belong to this boot
			out.SettingLocks = append(out.SettingLocks, l)
		}
	}
	if out.SettingsPassword, err = database.GetSetting(SettingSettingsPassword); err != nil {
		return nil, err
	}
	forLocks, _ := database.GetSetting(SettingPasswordForLocks)
	out.PasswordForLocks = forLocks == "on"
	for _, area := range accessAreas {
		if hash, _ := database.GetSetting(db.AccessPasswordSetting + area); hash != "" {
			out.AccessPasswords[area] = hash
		}
	}
	out.BlockPasswords = map[string]string{}
	for _, b := range blocks {
		if hash, _ := database.BlockViewPassword(b.Name); hash != "" {
			out.BlockPasswords[b.Name] = hash
		}
	}
	if out.Hotkeys, err = database.Hotkeys(); err != nil {
		return nil, err
	}
	for i := range out.Hotkeys {
		out.Hotkeys[i].ID = 0
	}
	if out.Statistics, err = database.UsageSince(""); err != nil {
		return nil, err
	}
	return out, nil
}

// backupLocks lists the locks importing would set, for confirmLocks.
func (b *backupExtras) backupLocks(renamed map[string]string, database *db.DB, trusted int64) []lockToSet {
	var out []lockToSet
	for _, l := range b.BlockLocks {
		name, ok := renamed[l.Block]
		if !ok {
			continue
		}
		target := "imported block '" + name + "'"
		if l.LockedUntil > trusted {
			out = append(out, lockToSet{target, "timer lock", timerLength(trusted, l.LockedUntil)})
		}
		if l.TimerAfter > 0 {
			out = append(out, lockToSet{target, "timer lock", (time.Duration(l.TimerAfter) * time.Second).String() + " after the locks before it"})
		}
		if l.LockChars > 0 {
			out = append(out, lockToSet{target, textLockKind(l.Style, l.LockChars), ""})
		}
		if l.Password != "" {
			out = append(out, lockToSet{target, "password lock (the backup's password)", ""})
		}
		if l.Delay > 0 {
			out = append(out, lockToSet{target, "delay lock", (time.Duration(l.Delay) * time.Second).String()})
		}
	}
	for _, l := range b.importableSettingLocks(database, trusted) {
		out = append(out, lockToSet{settingTarget(l.Name), "setting lock", strings.Join(l.Layers(), " → ")})
	}
	for _, h := range b.Hotkeys {
		if h.LockFor != "" && h.NoConfirm {
			out = append(out, lockToSet{"block '" + h.Block + "'", "timer lock, set at each press of " + h.Keys + " without asking", h.LockFor})
		}
	}
	return out
}

// importableSettingLocks are the backup's setting locks whose setting has no
// lock on here, still running.
func (b *backupExtras) importableSettingLocks(database *db.DB, trusted int64) []db.SettingLock {
	var out []db.SettingLock
	for _, l := range b.SettingLocks {
		if !enforce.IsLockableSetting(l.Name) && l.Name != db.NepsisLockName {
			continue
		}
		if l.LockedUntil != 0 && l.LockedUntil <= trusted && l.TimerAfter == 0 {
			l.LockedUntil = 0
		}
		if len(l.LayersOn(trusted, "")) == 0 {
			continue
		}
		if cur, err := loadSettingLock(database, l.Name); err != nil || cur.lock.InForce(cur.trusted, cur.bootID) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// importBackupExtras adds what a backup holds; say reports each step and
// dry changes nothing. renamed maps the file's block names to the blocks
// just created.
func importBackupExtras(database *db.DB, b *backupExtras, renamed map[string]string, dry bool, say func(string, ...any)) error {
	trusted, err := database.TrustedNow(time.Now())
	if err != nil {
		return err
	}
	for _, l := range b.BlockLocks {
		name, ok := renamed[l.Block]
		if !ok {
			continue
		}
		until := l.LockedUntil
		if until <= trusted {
			until = 0
		}
		e := db.LockExtras{PasswordHash: l.Password, Delay: l.Delay, Relock: l.Relock, Style: l.Style, Order: l.Order, TimerAfter: l.TimerAfter}
		detail := &db.BlockDetail{Active: true, Locked: l.LockChars > 0, LockChars: l.LockChars, LockedUntil: until}
		if len(db.LayersOn(detail, e, trusted, "")) == 0 {
			continue
		}
		say("switch block '%s' on with its running locks", name)
		if dry {
			continue
		}
		if err := database.ActivateBlock(name, l.LockChars > 0, l.LockChars, until, false); err != nil {
			return err
		}
		if err := database.SetLockExtras(name, e); err != nil {
			return err
		}
	}
	for _, l := range b.importableSettingLocks(database, trusted) {
		say("lock %s (%s)", settingTarget(l.Name), strings.Join(l.Layers(), " → "))
		if dry {
			continue
		}
		if l.Name != db.NepsisLockName {
			if err := database.SetSetting(enforce.SettingKey(l.Name), "on"); err != nil {
				return err
			}
		}
		if err := database.SetSettingLock(l); err != nil {
			return err
		}
	}
	if b.SettingsPassword != "" {
		if have, _ := database.GetSetting(SettingSettingsPassword); have == "" {
			say("set the settings password from the backup")
			if !dry {
				if err := database.SetSetting(SettingSettingsPassword, b.SettingsPassword); err != nil {
					return err
				}
				if b.PasswordForLocks {
					if err := database.SetSetting(SettingPasswordForLocks, "on"); err != nil {
						return err
					}
				}
			}
		} else {
			fmt.Println("A settings password is set here: left as it is.")
		}
	}
	for _, area := range accessAreas {
		hash := b.AccessPasswords[area]
		if hash == "" {
			continue
		}
		if have, _ := database.GetSetting(db.AccessPasswordSetting + area); have != "" {
			fmt.Printf("The access password (%s) is set here: left as it is.\n", area)
			continue
		}
		say("set the access password (%s) from the backup", area)
		if !dry {
			if err := database.SetSetting(db.AccessPasswordSetting+area, hash); err != nil {
				return err
			}
		}
	}
	for from, hash := range b.BlockPasswords {
		if name, ok := renamed[from]; ok && hash != "" {
			say("hide block '%s' behind its block password", name)
			if !dry {
				if err := database.SetBlockViewPassword(name, hash); err != nil {
					return err
				}
			}
		}
	}
	have, err := database.Hotkeys()
	if err != nil {
		return err
	}
	added := false
	for _, h := range b.Hotkeys {
		if name, ok := renamed[h.Block]; ok {
			h.Block = name
		}
		taken := false
		for _, x := range have {
			taken = taken || x.Keys == h.Keys
		}
		if taken {
			fmt.Printf("The hotkey %s is in use here: left out.\n", h.Keys)
			continue
		}
		if _, err := database.GetBlock(h.Block); err != nil {
			fmt.Printf("The hotkey %s starts '%s', which isn't here: left out.\n", h.Keys, h.Block)
			continue
		}
		say("add the hotkey %s (start %s%s)", h.Keys, h.Block, lockText(h.LockFor))
		if !dry {
			if _, err := database.AddHotkey(h); err != nil {
				return err
			}
			added = true
		}
	}
	if added {
		if err := syncHotkeys(database); err != nil {
			fmt.Printf("The hotkeys are saved but not set on the desktop yet: %v\n", err)
		}
	}
	if len(b.Statistics) > 0 {
		say("add %d statistics rows (days already here are kept)", len(b.Statistics))
		if !dry {
			if err := database.ImportUsage(b.Statistics); err != nil {
				return err
			}
		}
	}
	return nil
}
