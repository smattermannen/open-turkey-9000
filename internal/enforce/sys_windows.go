package enforce

// sys_windows.go — the clocks and the apply lock on Windows.

import (
	"log"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/winagent"
	"golang.org/x/sys/windows"
)

var (
	modkernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procQueryUnbiasedInterruptTime = modkernel32.NewProc("QueryUnbiasedInterruptTime")
)

// unbiasedSinceBoot is the time since boot without sleep and hibernation.
func unbiasedSinceBoot() (time.Duration, bool) {
	var t uint64 // 100 ns units
	if r, _, _ := procQueryUnbiasedInterruptTime.Call(uintptr(unsafe.Pointer(&t))); r == 0 {
		return 0, false
	}
	return time.Duration(t) * 100, true
}

// sleptSeconds is how long the computer has slept since boot: the tick
// count counts sleep, the unbiased interrupt time doesn't. A variable for
// tests.
var sleptSeconds = func() int64 {
	awake, ok := unbiasedSinceBoot()
	if !ok {
		return 0
	}
	return int64((windows.DurationSinceBoot() - awake) / time.Second)
}

// bootClock is the time since boot, sleep included (GetTickCount64).
func bootClock() (time.Duration, bool) {
	return windows.DurationSinceBoot(), true
}

// applyLockPath is next to the database, in the service's own folder.
var applyLockPath = filepath.Join(os.Getenv("ProgramData"), "kc9k", "apply.lock")

// applyLock makes the service and the CLI take turns writing the layers
// (see the Linux one). Without access to the lock file, as in tests, there
// is nothing to coordinate with.
func applyLock() func() {
	file, err := os.OpenFile(applyLockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return func() {}
	}
	var ol windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		file.Close()
		return func() {}
	}
	return func() { file.Close() }
}

// platformTaskManagers are Windows' task managers and the tools that stop
// services or change what keeps KC9K running (tamper protection
// "task-managers": closed while anything is locked). mmc hosts Services,
// the Registry's and Group Policy's editors' consoles, Task Scheduler.
var platformTaskManagers = []string{"taskmgr", "procexp", "procexp64", "procexp64a", "processhacker",
	"systeminformer", "procmon", "procmon64", "resmon", "perfmon", "mmc", "regedit", "regedt32", "sc",
	"bcdedit", "reagentc", "schtasks", "autoruns", "autoruns64", "pskill", "pskill64", "taskkill", "msconfig"}

func init() {
	// Windows' USB image writers (factory reset protection).
	ResetGuardApps = append(ResetGuardApps, "rufus", "balenaetcher", "ventoy2disk", "mediacreationtool",
		"mediacreationtoolw1022h2", "win32diskimager", "rpi-imager", "unetbootin", "universal-usb-installer")
	circadianForgetBacklights = blocker.ForgetBacklights
}

// agentRestarts is the session agents started again so far, counted.
var agentRestarts int64

// guardPlatform puts back the service's protection and counts each thing
// found changed as a bypass (§3.10); factory reset protection turns the
// recovery environment off while it acts.
func guardPlatform(d *db.DB, trusted int64) {
	var fixed []string
	fixed = append(fixed, blocker.GuardService()...)
	blocker.LockdownRepairs.Lock()
	fixed = append(fixed, blocker.LockdownRepairs.What...)
	blocker.LockdownRepairs.What = nil
	blocker.LockdownRepairs.Unlock()
	if m := winagent.Default; m != nil {
		if n := m.Restarts.Load(); n > agentRestarts {
			agentRestarts = n
			fixed = append(fixed, "a session agent was ended")
		}
	}
	if again, err := blocker.SyncRecoveryGuard(resetGuardActive(d, trusted)); err != nil {
		log.Printf("tamper: recovery environment: %v", err)
	} else if again {
		fixed = append(fixed, "the recovery environment was turned on during a lock")
	}
	// Block time and date changes, while anything is locked (W23).
	clockOn := false
	if v, err := d.GetSetting(db.SettingTamperClock); err == nil && v == "on" {
		locked, err := AnyLock(d, time.Now(), trusted)
		clockOn = err == nil && locked
	}
	if again, err := blocker.SyncClockGuard(clockOn, func() (time.Time, bool) {
		at, err := internetTime()
		return at, err == nil
	}); err != nil {
		log.Printf("tamper: clock: %v", err)
	} else {
		fixed = append(fixed, again...)
	}
	for _, what := range fixed {
		log.Printf("tamper: put back %s", what)
		logEventFor(d, db.EventTamper, activeNames(d), what+" (put back)")
	}
}
