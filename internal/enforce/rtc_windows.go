package enforce

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

var bootEvent struct {
	sync.Mutex
	read          bool
	start, logged time.Time
	ok            bool
}

// readHardwareClock is the hardware clock read at boot (Kernel-General
// event 12, read once per service run) plus the boot clock since. A cleared
// log: none (the clock check then uses internet time alone).
func readHardwareClock() (int64, bool) {
	bootEvent.Lock()
	defer bootEvent.Unlock()
	if !bootEvent.read {
		bootEvent.read = true
		out, err := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "wevtutil.exe"), "qe", "System",
			"/q:*[System[Provider[@Name='Microsoft-Windows-Kernel-General'] and EventID=12]]", "/c:1", "/rd:true", "/f:xml").Output()
		if err == nil {
			bootEvent.start, bootEvent.logged, bootEvent.ok = parseBootEvent(string(out))
		}
	}
	if !bootEvent.ok {
		return 0, false
	}
	return hardwareClockAt(bootEvent.start, windows.DurationSinceBoot()), true
}
