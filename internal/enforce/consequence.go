// consequence.go — what KC9K does when a blacklist's device action fires or
// bypass prevention trips: close the browsers, lock the screen, log out,
// shut down, or start monk mode. Lock, logout and shutdown for a while go
// through Frozen Chicken (only ever extended); monk mode is the block
// MonkBlock, started with a time lock that ends it.
package enforce

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/freeze"
)

// MonkBlock is the block monk mode starts. It is an ordinary block: made with
// every website blocked (*.* in Blocked) the first time it is needed, and
// open to exceptions, apps and anything else afterwards.
const MonkBlock = "monk-mode"

// onceEvery is how often a one-off logout or shutdown may happen. A page that
// comes back after the restart (a restored session) can't loop the computer.
const onceEvery = 10 * time.Minute

// What consequences do to the computer. Tests replace these, so they never
// close the browsers, log out or shut down the machine running them.
var (
	closeBrowsers = func() error { _, err := blocker.KillBlocked(blocker.BrowserNames()); return err }
	logoutUsers   = blocker.LogoutUsers
	powerOff      = blocker.PowerOff
	notifyDesktop = blocker.NotifyDesktop
	applyFreeze   = enforceFreeze
)

// Consequence is one thing to do, with how long it lasts where that applies.
type Consequence struct {
	Kind    string // a db.Consequence* name
	Minutes int
}

func (c Consequence) String() string {
	if c.Minutes > 0 {
		return fmt.Sprintf("%s for %s", c.Kind, time.Duration(c.Minutes)*time.Minute)
	}
	return c.Kind
}

// consequenceMu keeps two consequences from running at once (the rules
// server and the daemon cycle both run them).
var consequenceMu sync.Mutex

// Run carries out a consequence, telling the user why first (unless why is
// ""). Each does only what it says: close-browser closes every browser,
// lock-screen, logout and shutdown leave them to the session ending, monk
// mode blocks them. A warning is only the notification; close-tab is for the
// extension (see hits.go) and does nothing here.
func Run(d *db.DB, c Consequence, why string) error {
	consequenceMu.Lock()
	defer consequenceMu.Unlock()
	if err := db.CheckConsequence(c.Kind, c.Minutes); err != nil {
		return err
	}
	now := time.Now()
	if c.Minutes == 0 && (c.Kind == db.ConsequenceLogout || c.Kind == db.ConsequenceShutdown) {
		key := "consequence.last." + c.Kind
		if last, _ := d.GetSetting(key); last != "" {
			if unix, err := strconv.ParseInt(last, 10, 64); err == nil && now.Sub(time.Unix(unix, 0)) < onceEvery {
				log.Printf("consequence: %s skipped, the last one was under %s ago; closing browsers instead", c.Kind, onceEvery)
				c = Consequence{Kind: db.ConsequenceCloseBrowser}
			}
		}
		if c.Kind != db.ConsequenceCloseBrowser {
			if err := d.SetSetting(key, strconv.FormatInt(now.Unix(), 10)); err != nil {
				return err
			}
		}
	}
	log.Printf("consequence: %s (%s)", c, why)
	if why != "" {
		notifyDesktop("Killer Chicken 9000: "+c.String(), why)
	}
	switch c.Kind {
	case db.ConsequenceWarning, db.ConsequenceCloseTab:
		return nil
	case db.ConsequenceCloseBrowser:
		return closeBrowsers()
	case db.ConsequenceMonk:
		return StartMonk(d, time.Duration(c.Minutes)*time.Minute)
	}
	mode := map[string]string{db.ConsequenceLockScreen: freeze.ModeLock, db.ConsequenceLogout: freeze.ModeLogout,
		db.ConsequenceShutdown: freeze.ModeShutdown}[c.Kind]
	if c.Minutes == 0 {
		if mode == freeze.ModeLogout {
			return logoutUsers()
		}
		return powerOff()
	}
	until := now.Add(time.Duration(c.Minutes) * time.Minute).Unix()
	current, err := d.FreezeUntil()
	if err != nil {
		return err
	}
	if until > current {
		if err := d.SetFreeze(until, mode); err != nil {
			return err
		}
	}
	applyFreeze(d, now) // at once, not at the next cycle
	return nil
}

// StartMonk turns monk mode on for a while, or extends it: the block
// MonkBlock with a time lock, ending when the lock does. A monk-mode block
// someone started by hand without an end keeps going after the lock.
func StartMonk(d *db.DB, length time.Duration) error {
	if length <= 0 || length > time.Duration(db.MaxMonkMinutes)*time.Minute {
		return fmt.Errorf("monk mode lasts from 1 minute to %d days", db.MaxMonkMinutes/(24*60))
	}
	if err := ensureMonkBlock(d); err != nil {
		return err
	}
	now := time.Now()
	trusted, err := d.TrustedNow(now)
	if err != nil {
		return err
	}
	until := trusted + int64(length/time.Second)
	detail, err := d.GetBlock(MonkBlock)
	if err != nil {
		return err
	}
	extras, err := d.GetLockExtras(MonkBlock)
	if err != nil && detail.Active {
		return err
	}
	switch {
	case !detail.Active:
		if err := d.ActivateBlock(MonkBlock, false, 0, until, false); err != nil {
			return err
		}
		extras = db.LockExtras{StopAt: until}
	case until > detail.LockedUntil:
		if err := d.StrengthenLocks(MonkBlock, detail.Locked, detail.LockChars, until); err != nil {
			return err
		}
		if extras.StopAt != 0 && until > extras.StopAt {
			extras.StopAt = until
		}
	default:
		return nil // already on for longer
	}
	return d.SetLockExtras(MonkBlock, extras)
}

// ensureMonkBlock makes the monk-mode block if it is missing, and puts *.*
// back if it blocks nothing.
func ensureMonkBlock(d *db.DB) error {
	detail, err := d.GetBlock(MonkBlock)
	if err != nil {
		if err := d.CreateBlock(MonkBlock); err != nil {
			return err
		}
		if detail, err = d.GetBlock(MonkBlock); err != nil {
			return err
		}
	}
	if !detail.Empty() {
		return nil
	}
	l, ok := detail.Lists.Find(db.DefaultBlockedList)
	if !ok {
		id, err := d.CreateList(MonkBlock, db.ListBlacklist, db.DefaultBlockedList, db.Where{URL: true})
		if err != nil {
			return err
		}
		l.ID = id
	}
	if err := d.SetListWhere(l.ID, db.Where{URL: true}); err != nil {
		return err
	}
	if err := d.SetListAction(l.ID, db.ActionBlock, true); err != nil {
		return err
	}
	return d.AddListEntries(l.ID, []string{"*.*"})
}
