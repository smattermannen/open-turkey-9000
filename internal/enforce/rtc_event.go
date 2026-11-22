package enforce

// rtc_event.go — reading Windows' boot event (ROADMAP W17): event 12 of
// Microsoft-Windows-Kernel-General, "The operating system started at system
// time …", whose StartTime is the hardware clock as read at boot. Pure, so
// it is tested on every system.

import (
	"regexp"
	"time"
)

var (
	startTimeData   = regexp.MustCompile(`<Data Name=['"]StartTime['"]>([^<]+)</Data>`)
	timeCreatedAttr = regexp.MustCompile(`<TimeCreated SystemTime=['"]([^'"]+)['"]`)
)

// parseBootEvent reads the event's StartTime and when it was logged
// (wevtutil's XML).
func parseBootEvent(xml string) (start, logged time.Time, ok bool) {
	m := startTimeData.FindStringSubmatch(xml)
	c := timeCreatedAttr.FindStringSubmatch(xml)
	if m == nil || c == nil {
		return time.Time{}, time.Time{}, false
	}
	start, err1 := time.Parse(time.RFC3339Nano, m[1])
	logged, err2 := time.Parse(time.RFC3339Nano, c[1])
	return start, logged, err1 == nil && err2 == nil && start.Year() > 2000
}

// hardwareClockAt is the hardware clock's count: its reading at boot plus
// the boot clock since. The newest event 12 is this boot's: every boot logs
// one, and a cleared log gives none. The wall clock plays no part (it may
// have been moved).
func hardwareClockAt(start time.Time, sinceBoot time.Duration) int64 {
	return start.Add(sinceBoot).Unix()
}
