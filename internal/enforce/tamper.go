// tamper.go — signs that someone works on KC9K itself, logged as events for
// bypass prevention:
//   - the daemon starting again within the same boot: it was killed, stopped
//     or restarted (an upgrade counts once too, which a sensible threshold
//     absorbs);
//   - the wall clock jumping against the boot clock, which `date` can't move.
package enforce

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"golang.org/x/sys/unix"
)

// settingDaemonBoot holds the boot the daemon last ran in.
const settingDaemonBoot = "daemon.boot"

// clockJump is how far the wall clock may drift from the boot clock between
// two cycles before it counts as moved (NTP corrections are far smaller).
const clockJump = 2 * time.Minute

// NoteDaemonStart logs a tamper event when the daemon already ran in this
// boot. The daemon calls it once when it starts.
func NoteDaemonStart(d *db.DB) {
	boot, err := db.BootID()
	if err != nil {
		log.Printf("tamper: %v", err)
		return
	}
	last, err := d.GetSetting(settingDaemonBoot)
	if err != nil {
		log.Printf("tamper: %v", err)
		return
	}
	if last == boot {
		log.Printf("tamper: the daemon started again within this boot")
		logEventFor(d, db.EventTamper, activeNames(d), "daemon restarted")
	}
	if err := d.SetSetting(settingDaemonBoot, boot); err != nil {
		log.Printf("tamper: %v", err)
	}
}

// clockWatch remembers the last cycle's wall and boot clocks.
var clockWatch struct {
	wall, boot time.Duration
}

func bootClock() (time.Duration, bool) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, false
	}
	return time.Duration(ts.Nano()), true
}

// watchClock logs a tamper event when the wall clock moved by more than
// clockJump against the boot clock since the last cycle.
func watchClock(d *db.DB, now time.Time) {
	boot, ok := bootClock()
	if !ok {
		return
	}
	wall := time.Duration(now.UnixNano())
	prevWall, prevBoot := clockWatch.wall, clockWatch.boot
	clockWatch.wall, clockWatch.boot = wall, boot
	if prevBoot == 0 {
		return
	}
	if jump := clockDrift(prevWall, prevBoot, wall, boot); jump > clockJump {
		log.Printf("tamper: the clock moved by %s", jump.Round(time.Second))
		logEventFor(d, db.EventTamper, activeNames(d), fmt.Sprintf("clock moved by %s", strings.TrimSuffix(jump.Round(time.Second).String(), "0s")))
	}
}

// clockDrift is how far the wall clock moved beyond the boot clock.
func clockDrift(prevWall, prevBoot, wall, boot time.Duration) time.Duration {
	drift := (wall - prevWall) - (boot - prevBoot)
	if drift < 0 {
		drift = -drift
	}
	return drift
}
