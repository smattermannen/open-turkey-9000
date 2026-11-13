// tamper.go — signs that someone works on KC9K itself, logged as events for
// bypass prevention:
//   - the daemon starting again within the same boot: it was killed, stopped
//     or restarted. Not an upgrade (the program file changed since the last
//     start) and not after a crash the daemon reported itself (§3.10); a
//     normal shutdown starts a new boot;
//   - the wall clock jumping against the boot clock, which `date` can't move.
package enforce

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"golang.org/x/sys/unix"
)

// Settings that tell a daemon start from tampering.
const (
	settingDaemonBoot   = "daemon.boot"   // the boot the daemon last ran in
	settingDaemonBinary = "daemon.binary" // a fingerprint of the program it ran
	settingDaemonCrash  = "daemon.crash"  // the boot of a crash it reported
)

// daemonBinary fingerprints the running program, so a start after an
// upgrade is told from a restart. Replaced in tests.
var daemonBinary = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(exe)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// NoteDaemonCrash records that the daemon is going down from its own fault,
// so the next start isn't taken for tampering.
func NoteDaemonCrash(d *db.DB, why any) {
	log.Printf("daemon: crashed: %v", why)
	boot, err := db.BootID()
	if err != nil {
		return
	}
	if err := d.SetSetting(settingDaemonCrash, boot); err != nil {
		log.Printf("tamper: %v", err)
	}
}

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
	binary := daemonBinary()
	if last == boot {
		lastBinary, _ := d.GetSetting(settingDaemonBinary)
		crash, _ := d.GetSetting(settingDaemonCrash)
		switch {
		case crash == boot:
			log.Printf("daemon: started again after a crash it reported")
		case binary != "" && binary != lastBinary:
			// No fingerprint stored: the last run was a version from before
			// fingerprints, so this one is an upgrade too.
			log.Printf("daemon: started again after an upgrade")
		default:
			log.Printf("tamper: the daemon started again within this boot")
			logEventFor(d, db.EventTamper, activeNames(d), "daemon restarted")
		}
	}
	for key, value := range map[string]string{settingDaemonBoot: boot, settingDaemonBinary: binary, settingDaemonCrash: ""} {
		if err := d.SetSetting(key, value); err != nil {
			log.Printf("tamper: %v", err)
		}
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
