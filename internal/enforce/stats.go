package enforce

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// SettingStats turns statistics off when set to "off". They are on by default
// and never leave the computer.
const SettingStats = "stats"

const (
	// StatsKeepDays is how long statistics are kept.
	StatsKeepDays = 365
	// idleLimit is how long without input still counts as using an app
	// (use, §3.7: input in the last 3 minutes).
	idleLimit = 180
	// maxReportSeconds caps one report from the extension per site, so a
	// stray or forged request can't add more than a moment of use.
	maxReportSeconds = 120
)

// StatsOn reports whether statistics are recorded.
func StatsOn(d *db.DB) bool {
	value, err := d.GetSetting(SettingStats)
	if err != nil {
		return false
	}
	return value != "off"
}

// Day is the local calendar day statistics file time under.
func Day(t time.Time) string {
	return t.Format("2006-01-02")
}

// RecordApps adds elapsed seconds of use to every app in the foreground now,
// for statistics and for open quota sessions. The daemon calls it on its own
// cycle; elapsed is capped so a suspended or stalled daemon doesn't count the
// gap as use.
func RecordApps(d *db.DB, now time.Time, elapsed time.Duration) {
	seconds := int64(elapsed.Round(time.Second) / time.Second)
	if seconds > 10 {
		seconds = 10
	}
	apps := blocker.ForegroundApps(idleLimit)
	noteQuotaApps(d, apps, seconds)
	if !StatsOn(d) {
		return
	}
	for _, app := range apps {
		if err := d.AddUsage(Day(now), db.UsageApp, app, seconds); err != nil {
			log.Printf("stats: %v", err)
		}
	}
}

// PruneStats deletes statistics older than StatsKeepDays.
func PruneStats(d *db.DB, now time.Time) {
	if err := d.PruneUsage(Day(now.AddDate(0, 0, -StatsKeepDays))); err != nil {
		log.Printf("stats: %v", err)
	}
}

// usageReport is what the browser extension sends since its last report:
// seconds per site (statistics), and per block with an open quota session,
// the seconds its websites were in use.
type usageReport struct {
	Sites map[string]int64 `json:"sites"`
	Quota map[string]int64 `json:"quota"`
}

// usageHandler takes website time from the bundled extension. A web page can't
// send the custom header without a CORS preflight, which this server never
// answers, so only the extension (or a local program) can report.
func usageHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != RulesAddress || r.Method != http.MethodPost || r.Header.Get("X-KC9K-Usage") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var report usageReport
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&report); err != nil {
			http.Error(w, "bad report", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		if len(report.Quota) > 0 {
			open := map[string]bool{}
			for _, b := range openQuotaBlocks(d) {
				open[b.Name] = true
			}
			for block, seconds := range report.Quota {
				if open[block] && seconds > 0 {
					addQuotaUse(block, min(seconds, maxReportSeconds))
				}
			}
		}
		if !StatsOn(d) {
			return
		}
		day := Day(time.Now())
		for site, seconds := range report.Sites {
			site = blocker.NormalizeDomain(site)
			if !blocker.IsValidDomain(site) || seconds <= 0 {
				continue
			}
			if seconds > maxReportSeconds {
				seconds = maxReportSeconds
			}
			if err := d.AddUsage(day, db.UsageSite, site, seconds); err != nil {
				log.Printf("stats: %v", err)
			}
		}
	}
}
