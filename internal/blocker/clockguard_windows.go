package blocker

// clockguard_windows.go — tamper protection's "block time and date
// changes" on Windows while anything is locked (ROADMAP W23; SPEC §6
// 2026-11-22 By Opus). Trusted time already ignores the clock; schedules
// read local time, which an administrator (and, for the time zone, any
// user) may change. So: a changed time zone is put back, and a jump of the
// system clock against the boot clock that internet time doesn't confirm
// is put back. Each is counted as tamper.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows"
)

var (
	procGetDynamicTimeZoneInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDynamicTimeZoneInformation")
	procSetDynamicTimeZoneInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetDynamicTimeZoneInformation")
	procSetSystemTime                 = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetSystemTime")
)

// dynamicTZ is DYNAMIC_TIME_ZONE_INFORMATION.
type dynamicTZ struct {
	Bias                        int32
	StandardName                [32]uint16
	StandardDate                windows.Systemtime
	StandardBias                int32
	DaylightName                [32]uint16
	DaylightDate                windows.Systemtime
	DaylightBias                int32
	TimeZoneKeyName             [128]uint16
	DynamicDaylightTimeDisabled uint8
}

// clockGuard is what was found when the guard began.
type clockGuard struct {
	Zone   dynamicTZ `json:"zone"`
	Offset int64     `json:"offset"` // wall clock minus boot clock, seconds
	// AskedAt: the last internet check of a jump (once a minute at most).
	AskedAt int64 `json:"asked_at"`
}

var clockGuardPath = filepath.Join(platform.DataDir, "clock-guard.json")

// clockJump is how far the clock may move against the boot clock before
// internet time is asked (Windows' own time sync moves it by seconds).
const clockJump = 120 * time.Second

func currentZone() (dynamicTZ, error) {
	var z dynamicTZ
	if r, _, err := procGetDynamicTimeZoneInformation.Call(uintptr(unsafe.Pointer(&z))); r == 0xFFFFFFFF {
		return z, err
	}
	return z, nil
}

func wallMinusBoot(now time.Time) int64 {
	return now.Unix() - int64(windows.DurationSinceBoot()/time.Second)
}

// SyncClockGuard keeps the time zone and the clock while on; internet
// tells the internet's time (false: not known). It returns what it put
// back.
func SyncClockGuard(on bool, internet func() (time.Time, bool)) ([]string, error) {
	var g clockGuard
	data, err := os.ReadFile(clockGuardPath)
	active := err == nil
	if !on {
		if active {
			return nil, os.Remove(clockGuardPath)
		}
		return nil, nil
	}
	now := time.Now()
	if !active {
		if g.Zone, err = currentZone(); err != nil {
			return nil, err
		}
		g.Offset = wallMinusBoot(now)
		data, _ := json.Marshal(g)
		return nil, os.WriteFile(clockGuardPath, data, 0o600)
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, os.Remove(clockGuardPath)
	}
	var fixed []string
	if z, err := currentZone(); err == nil && (z.TimeZoneKeyName != g.Zone.TimeZoneKeyName || z.DynamicDaylightTimeDisabled != g.Zone.DynamicDaylightTimeDisabled) {
		_ = enablePrivilege("SeTimeZonePrivilege")
		if r, _, err := procSetDynamicTimeZoneInformation.Call(uintptr(unsafe.Pointer(&g.Zone))); r == 0 {
			return fixed, err
		}
		fixed = append(fixed, "the time zone")
	}
	drift := time.Duration(wallMinusBoot(now)-g.Offset) * time.Second
	if drift > clockJump || drift < -clockJump {
		if now.Unix()-g.AskedAt < 60 {
			return fixed, nil
		}
		g.AskedAt = now.Unix()
		if real, ok := internet(); ok && real.Sub(now).Abs() < clockJump {
			g.Offset = wallMinusBoot(now) // a true correction (time sync)
		} else {
			want := time.Unix(g.Offset, 0).Add(windows.DurationSinceBoot()).UTC()
			st := windows.Systemtime{Year: uint16(want.Year()), Month: uint16(want.Month()), Day: uint16(want.Day()),
				DayOfWeek: uint16(want.Weekday()), Hour: uint16(want.Hour()), Minute: uint16(want.Minute()),
				Second: uint16(want.Second()), Milliseconds: uint16(want.Nanosecond() / 1e6)}
			_ = enablePrivilege("SeSystemtimePrivilege")
			if r, _, err := procSetSystemTime.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
				return fixed, err
			}
			fixed = append(fixed, "the system clock")
		}
		data, _ := json.Marshal(g)
		_ = os.WriteFile(clockGuardPath, data, 0o600)
	}
	return fixed, nil
}
