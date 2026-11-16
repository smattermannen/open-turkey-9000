package enforce

import (
	"log"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
	"golang.org/x/sys/unix"
)

// autostartWatch is what the daemon remembers between cycles to see a
// sign-in, a wake or a window opening (§3.6 autostart).
var autostartWatch = struct {
	started  bool
	sessions map[string]bool
	sleep    int64 // CLOCK_BOOTTIME - CLOCK_MONOTONIC: time spent asleep
	windows  map[string]bool
}{windows: map[string]bool{}}

// sleptSeconds is how long the computer has slept since boot. A variable
// for tests.
var sleptSeconds = func() int64 {
	var boot, mono unix.Timespec
	if unix.ClockGettime(unix.CLOCK_BOOTTIME, &boot) != nil || unix.ClockGettime(unix.CLOCK_MONOTONIC, &mono) != nil {
		return 0
	}
	return boot.Sec - mono.Sec
}

// runAutostarts switches on inactive blocks whose autostart is due. The
// first cycle only looks, so a daemon restart isn't taken for a sign-in.
func runAutostarts(d *db.DB, now time.Time) {
	rules, err := d.Autostarts("")
	if err != nil {
		log.Printf("autostart: %v", err)
		return
	}
	w := &autostartWatch
	sessions := blocker.GraphicalSessions()
	slept := sleptSeconds()
	signedIn := false
	if w.started {
		for id := range sessions {
			signedIn = signedIn || !w.sessions[id]
		}
		signedIn = signedIn || slept-w.sleep > 30 // woke up
	}
	w.sessions, w.sleep = sessions, slept
	opened := map[string]bool{}
	for _, a := range rules {
		if a.Kind != db.AutostartWindow {
			continue
		}
		schedules, err := d.ListSchedules(a.Block)
		if err != nil {
			continue
		}
		in := schedule.Enforced(db.Windows(schedules), now)
		if w.started && in && !w.windows[a.Block] {
			opened[a.Block] = true
		}
		w.windows[a.Block] = in
	}
	first := !w.started
	w.started = true
	if first {
		return
	}
	for _, a := range rules {
		if !db.AutostartDue(a, now, signedIn, opened[a.Block]) {
			continue
		}
		switch a.Kind {
		case db.AutostartOnce, db.AutostartWindow:
			_ = d.RemoveAutostart(a.ID) // one-shot
		case db.AutostartAt:
			_ = d.SetAutostartFired(a.ID, now.Format("2006-01-02"))
		}
		if active, err := d.IsBlockActive(a.Block); err != nil || active || d.DraftOf(a.Block) != "" {
			continue // on already, or a draft (§4.3: never runs)
		}
		if err := d.ActivateBlock(a.Block, false, 0, 0, false); err != nil {
			log.Printf("autostart: %v", err)
			continue
		}
		log.Printf("autostart: switched on %s (%s)", a.Block, a)
		noticeOnce("autostart "+a.Block+" "+now.Format("2006-01-02 15:04"), "'"+a.Block+"' is on",
			"Its autostart ("+a.String()+") switched it on.")
	}
}
