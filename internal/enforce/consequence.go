// consequence.go — what KC9K does when a blacklist's device action fires or
// bypass prevention trips: close the browsers, lock the screen, log out,
// shut down, or start monk mode. Lock, logout and shutdown for a while go
// through Frozen Chicken (only ever extended); monk mode is in monk.go.
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
// mode blocks every website and cuts the connection. A warning is only the notification; close-tab is for the
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
