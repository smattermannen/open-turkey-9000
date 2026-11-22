// clockcheck.go — the clock check (§4.10 tamper protection "clock: check
// against hardware clock and internet time"; SPEC §6 2026-11-18 agreed the
// internet query). While "block time and date changes" is on and a timer
// lock runs, the daemon asks the internet for the time once per boot and
// reads the hardware clock; db.CheckClock gives powered-off time back from
// the anchors (see db/clock.go). Nothing is sent but the request itself.
package enforce

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// clockSources answer HTTPS with a Date header. Not the DNS resolvers' IP
// addresses: the firewall layer shuts those (DNS over HTTPS). A source a
// block stops is skipped.
var clockSources = []string{"https://www.google.com/", "https://www.cloudflare.com/", "https://www.microsoft.com/"}

// clockAgree is how far two sources may differ and still count as one time.
const clockAgree = 2 * time.Minute

// agreedTime is the earlier of the first two times within clockAgree of
// each other. A cache can answer with an old Date, never a future one, so
// the earlier is the strict side; with no two agreeing there is no time.
func agreedTime(times []time.Time) (time.Time, bool) {
	sorted := append([]time.Time(nil), times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	var best time.Time
	for i := 0; i+1 < len(sorted); i++ {
		if sorted[i+1].Sub(sorted[i]) <= clockAgree {
			best = sorted[i] // the latest agreeing pair: older ones are caches
		}
	}
	return best, !best.IsZero()
}

// internetTime asks every source for the time and needs two to agree (a
// variable for tests).
var internetTime = func() (time.Time, error) {
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	var times []time.Time
	var errs []string
	for _, url := range clockSources {
		req, err := http.NewRequest(http.MethodHead, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := client.Do(req)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		resp.Body.Close()
		at, err := http.ParseTime(resp.Header.Get("Date"))
		if err != nil {
			errs = append(errs, url+": no Date")
			continue
		}
		times = append(times, at)
	}
	if at, ok := agreedTime(times); ok {
		return at, nil
	}
	return time.Time{}, fmt.Errorf("no two sources agree on the time (%d answered; %s)", len(times), strings.Join(errs, "; "))
}

// hardwareClock reads the hardware clock in seconds (a variable for tests);
// false when there is none. Linux reads /sys/class/rtc, Windows what it
// logged of the hardware clock at boot (rtc_*.go).
var hardwareClock = readHardwareClock

// CheckClock runs the clock check when it is due. The daemon calls it every
// few minutes; it asks the internet only when due (once per boot while a
// timer lock runs).
func CheckClock(d *db.DB, now time.Time) {
	due, err := d.ClockCheckDue(now)
	if err != nil {
		log.Printf("clock check: %v", err)
		return
	}
	if !due {
		return
	}
	real, err := internetTime()
	if err != nil {
		log.Printf("clock check: %v (tried again in a few minutes)", err)
		return
	}
	rtc, rtcOK := hardwareClock()
	credited, err := d.CheckClock(now, real, rtc, rtcOK)
	if err != nil {
		log.Printf("clock check: %v", err)
		return
	}
	if credited > 0 {
		log.Printf("clock check: %s of powered-off time counted for timer locks (internet time and hardware clock)", time.Duration(credited)*time.Second)
	}
}
