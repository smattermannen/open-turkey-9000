package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// §4.12: toggle switches a block on (no locks) or off; a locked block stays
// on. §4.2: a duplicate starts inactive.
func TestToggleAndDuplicate(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "toggle", "b")
	if detail, _ := d.GetBlock("b"); !detail.Active || detail.Locked || detail.LockedUntil != 0 {
		t.Fatalf("toggle on: %+v", detail)
	}
	mustRun(t, "toggle", "b")
	if active, _ := d.IsBlockActive("b"); active {
		t.Fatal("toggle off left it on")
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "toggle", "b"); err == nil {
		t.Error("toggle switched a locked block off")
	}
	mustRun(t, "block", "duplicate", "b", "copy")
	copied, err := d.GetBlock("copy")
	if err != nil || copied.Active || len(copied.Lists) != 2 {
		t.Errorf("duplicate of an active locked block: %+v, %v", copied, err)
	}
}

// §4.10: notification choices from the CLI.
func TestNotificationsCommand(t *testing.T) {
	d := testDB(t)
	mustRun(t, "notifications", "window-end", "off")
	mustRun(t, "notifications", "advance", "5,2")
	o := enforce.LoadNoticeOptions(d)
	if o.On("window-end") || !o.On("timer-end") || len(o.Advance) != 2 || o.Advance[0] != 2 {
		t.Errorf("options = %+v", o)
	}
	if err := runCLI(t, "notifications", "advance", "0"); err == nil {
		t.Error("advance 0 accepted")
	}
	if err := runCLI(t, "notifications", "nonsense", "on"); err == nil {
		t.Error("unknown kind accepted")
	}
	mustRun(t, "notifications", "window-end", "on")
	if !enforce.LoadNoticeOptions(d).On("window-end") {
		t.Error("not back on")
	}
}

// §3.7: strict use counting on is free; back to normal weakens, refused
// while locked with no weakening on (R-WEAK-4).
func TestUseCounting(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "use-counting", "strict")
	if !enforce.StrictUseOn(d) {
		t.Fatal("not on")
	}
	if r, _ := enforce.BuildRules(d); !r.StrictUse {
		t.Error("not in the rules")
	}
	mustRun(t, "no-weakening", "on")
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "use-counting", "normal"); err == nil {
		t.Error("weakened while locked with no weakening on")
	}
}

// §3.8: adding an exempt folder weakens strict mode; removing is free.
func TestStrictExempt(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	dir := t.TempDir()
	mustRun(t, "strict", "exempt", "add", dir)
	if got := enforce.ExemptFolders(d); len(got) != 1 || got[0] != dir {
		t.Fatalf("exempt = %v", got)
	}
	if err := runCLI(t, "strict", "exempt", "add", "/usr/bin"); err == nil {
		t.Error("a system folder was exempted")
	}
	mustRun(t, "strict", "on", "--for", "1h", "--yes")
	other := t.TempDir()
	if err := runCLI(t, "strict", "exempt", "add", other); err == nil {
		t.Error("exempted a folder while strict mode is locked")
	}
	mustRun(t, "strict", "exempt", "remove", dir)
	if got := enforce.ExemptFolders(d); len(got) != 0 {
		t.Errorf("after remove: %v", got)
	}
}

// §4.1: the windows still to come (or open) today.
func TestWindowsLeftToday(t *testing.T) {
	mon := time.Date(2026, 11, 16, 12, 30, 0, 0, time.Local)
	ws := []schedule.Window{
		{Days: schedule.AllDays, Start: 9 * 60, End: 10 * 60},
		{Days: schedule.AllDays, Start: 12 * 60, End: 13 * 60},
		{Days: 1 << uint(time.Monday), Start: 18 * 60, End: 24 * 60, Allow: true},
		{Days: 1 << uint(time.Tuesday), Start: 14 * 60, End: 15 * 60},
	}
	got := strings.Join(windowsLeftToday(ws, mon), " | ")
	if got != "12:00-13:00 | allow 18:00-24:00" {
		t.Errorf("left today = %q", got)
	}
}

// §3.6: autostart rules from the CLI; removing one weakens.
func TestAutostartCommand(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "autostart", "b", "add", "at", "08:00", "mon-fri")
	mustRun(t, "autostart", "b", "add", "sign-in")
	if err := runCLI(t, "autostart", "b", "add", "once", "2001-01-01", "08:00"); err == nil {
		t.Error("a time in the past accepted")
	}
	rules, _ := d.Autostarts("b")
	if len(rules) != 2 || rules[0].String() != "at 08:00 weekdays" {
		t.Fatalf("rules = %+v", rules)
	}
	out := capture(t, "project", "export")
	if !strings.Contains(out, `"kind": "sign-in"`) {
		t.Error("autostart not in the project")
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "autostart", "b", "remove", "1"); err == nil {
		t.Error("removed an autostart of a locked block")
	}
}

// §3.6: a delay break weakens when set or loosened, is free to remove.
func TestDelayBreakCommand(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "break", "delay", "b", "10m", "30m")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "break", "delay", "b", "20m", "30m"); err == nil {
		t.Error("a longer delay break while locked")
	}
	if err := runCLI(t, "break", "delay", "b", "10m", "5m"); err == nil {
		t.Error("a shorter wait while locked")
	}
	mustRun(t, "break", "request", "b")
	if k, _ := d.GetBreaks("b"); k.DelayAt == 0 {
		t.Error("request didn't start the countdown")
	}
	if err := runCLI(t, "break", "request", "b"); err == nil {
		t.Error("asked twice")
	}
	mustRun(t, "break", "delay", "b", "off")
}

// §3.6 reward kept longer weakens.
func TestRewardKeepCommand(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "day")
	if k, _ := d.GetBreaks("b"); k.Keep != "day" {
		t.Fatalf("keep = %q", k.Keep)
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "month"); err == nil {
		t.Error("a longer keep while locked")
	}
	mustRun(t, "break", "reward", "b", "10m", "--per", "1h", "--keep", "one")
}

// §3.6: allowance per week with a refill time; changing the period weakens.
func TestAllowancePeriod(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "allowance", "set", "b", "5h", "--per", "week", "--refill", "04:00")
	q, _ := d.GetQuota("b")
	if q == nil || q.Period != "week" || q.Refill != 240 || q.Limit != 5*3600 {
		t.Fatalf("quota = %+v", q)
	}
	if err := runCLI(t, "allowance", "set", "b", "30h"); err == nil {
		t.Error("30h per day accepted")
	}
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "allowance", "set", "b", "5h", "--per", "month", "--refill", "04:00"); err == nil {
		t.Error("changed the period of a locked block")
	}
}

// §3.6 session limit from the CLI.
func TestSessionLimitCommand(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add-app", "b", "steam")
	mustRun(t, "allowance", "set", "b", "1h", "--sessions", "1")
	mustRun(t, "allowance", "start", "b")
	mustRun(t, "allowance", "end", "b")
	if err := runCLI(t, "allowance", "start", "b"); err == nil {
		t.Error("a second session with a limit of one")
	}
	if q, _ := d.GetQuota("b"); q.MaxSessions != 1 || q.Sessions != 1 {
		t.Errorf("quota = %+v", q)
	}
}

// §3.8, R-FREEZE-2: a scheduled freeze can have the lock kinds of §3.4;
// while one holds it can't be removed, and it counts as a lock.
func TestFreezeScheduleLocks(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	day := strings.ToLower(time.Now().AddDate(0, 0, 3).Weekday().String()[:3])
	mustRun(t, "freeze", "schedule", "add", "--days", day, "--from", "03:00", "--to", "03:30", "--yes")
	if err := runWith(t, 0, []string{"pw"}, "freeze", "schedule", "lock", "1", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	trusted, _ := d.TrustedNow(time.Now())
	if locked, _ := enforce.AnySettingLocked(d, trusted); !locked {
		t.Error("a locked freeze schedule doesn't count")
	}
	if err := runCLI(t, "freeze", "schedule", "remove", "1"); err == nil {
		t.Error("removed a locked freeze schedule")
	}
	if err := runWith(t, 0, []string{"wrong"}, "freeze", "schedule", "unlock", "1"); err == nil {
		t.Error("unlocked with a wrong password")
	}
	if err := runWith(t, 0, []string{"pw"}, "freeze", "schedule", "unlock", "1"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "freeze", "schedule", "remove", "1")
}

// §3.2: a template is a saved block definition; blocks made from it start
// inactive and are independent.
func TestTemplates(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "work")
	mustRun(t, "block", "add", "work", "reddit.com")
	mustRun(t, "block", "add-app", "work", "steam")
	mustRun(t, "template", "save", "work", "Focus")
	mustRun(t, "block", "create", "study", "--template", "focus")
	b, err := d.GetBlock("study")
	if err != nil || b.Active || len(b.Apps) != 1 {
		t.Fatalf("from template: %+v, %v", b, err)
	}
	found := false
	for _, l := range b.Lists {
		for _, e := range l.Entries {
			found = found || e == "reddit.com"
		}
	}
	if !found {
		t.Error("the template's entries are missing")
	}
	if err := runCLI(t, "block", "create", "x", "--template", "nope"); err == nil {
		t.Error("an unknown template accepted")
	}
	mustRun(t, "template", "remove", "Focus")
	if names, _ := d.Templates(); len(names) != 0 {
		t.Errorf("templates = %v", names)
	}
}

// §4.3 import from KC9K: copy another block's list into a list.
func TestListCopy(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "a")
	mustRun(t, "block", "add", "a", "reddit.com", "x.com")
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "copy", "a", "Blocked", "b", "Blocked")
	if l := list(t, d, "b", "Blocked"); len(l.Entries) != 2 {
		t.Errorf("copied entries = %v", l.Entries)
	}
	if err := runWith(t, 0, []string{"pw12"}, "block", "password", "a", "set"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"nope"}, "list", "copy", "a", "Blocked", "b", "Blocked"); err == nil {
		t.Error("copied from a hidden block without its password")
	}
}

// §4.9: stats top N, search and a block's own rows.
func TestStatsFilters(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "social")
	mustRun(t, "block", "add", "social", "reddit.com")
	day := time.Now().Format("2006-01-02")
	_ = d.AddUsage(day, db.UsageSite, "old.reddit.com", 600)
	_ = d.AddUsage(day, db.UsageSite, "wikipedia.org", 900)
	_ = d.AddUsage(day, db.UsageBlocked, "reddit.com", 3)
	out := capture(t, "stats", "--block", "social")
	if !strings.Contains(out, "old.reddit.com") || strings.Contains(out, "wikipedia") || !strings.Contains(out, "3×") {
		t.Errorf("--block social:\n%s", out)
	}
	if out := capture(t, "stats", "--search", "wiki"); strings.Contains(out, "reddit") {
		t.Errorf("--search wiki:\n%s", out)
	}
	if out := capture(t, "stats", "--top", "1"); !strings.Contains(out, "and 1 more") {
		t.Errorf("--top 1:\n%s", out)
	}
}

// §3.11, R-ESCAPE-1: from recovery mode, root removes a forgotten settings
// password and password locks, unless no-escape is on.
func TestRecover(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	oldRoot, oldRec := isRoot, inRecovery
	t.Cleanup(func() { isRoot, inRecovery = oldRoot, oldRec })
	isRoot, inRecovery = func() bool { return true }, func() bool { return false }
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runWith(t, 0, []string{"pw1"}, "start", "b", "--password-stdin", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runWith(t, 0, []string{"settings1"}, "password", "set"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "recover"); err == nil {
		t.Error("recovered outside a recovery boot")
	}
	inRecovery = func() bool { return true }
	_ = d.SetSetting(enforce.SettingNoEscape, "on")
	if err := runCLI(t, "recover"); err == nil {
		t.Error("recovered with no-escape on")
	}
	_ = d.SetSetting(enforce.SettingNoEscape, "off")
	mustRun(t, "recover")
	if v, _ := d.GetSetting(SettingSettingsPassword); v != "" {
		t.Error("settings password kept")
	}
	if e, _ := d.GetLockExtras("b"); e.PasswordHash != "" {
		t.Error("password lock kept")
	}
	mustRun(t, "stop", "b")
}

// §3.6 break per block window: one break each time the window is open.
func TestWindowBreak(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	now := time.Now()
	start := (now.Hour()*60 + now.Minute()) / 5 * 5
	end := start + 30
	if end > 24*60 {
		t.Skip("too close to midnight")
	}
	clock := func(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }
	to := clock(end)
	if end == 24*60 {
		to = "24:00"
	}
	mustRun(t, "schedule", "add", "b", "--days", "daily", "--from", clock(start), "--to", to)
	mustRun(t, "schedule", "break", "b", "1", "10m")
	mustRun(t, "start", "b")
	mustRun(t, "break", "start", "b")
	trusted, _ := d.TrustedNow(time.Now())
	if until, _ := d.PausedUntil("b"); until < trusted+590 {
		t.Fatalf("paused until %d (now %d)", until, trusted)
	}
	mustRun(t, "break", "end", "b")
	if err := runCLI(t, "break", "start", "b"); err == nil {
		t.Error("a second break in the same window")
	}
	mustRun(t, "schedule", "break", "b", "1", "off")
}

// §3.8: the allowed apps list; adding weakens (refused while locked).
func TestFreezeApps(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "freeze", "apps", "add", "libreoffice")
	if got := enforce.FreezeApps(d); len(got) != 1 || got[0] != "libreoffice" {
		t.Fatalf("apps = %v", got)
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	mustRun(t, "start", "b", "--for", "1h", "--yes")
	if err := runCLI(t, "freeze", "apps", "add", "steam"); err == nil {
		t.Error("an allowed app added while locked")
	}
	mustRun(t, "freeze", "apps", "remove", "libreoffice")
	if got := enforce.FreezeApps(d); len(got) != 0 {
		t.Errorf("after remove: %v", got)
	}
}

// §3.6 allowance per window: each opening of a block window is a period;
// outside windows the period stays (nothing refills).
func TestAllowancePerWindow(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	now := time.Now()
	start := (now.Hour()*60 + now.Minute()) / 5 * 5
	end := start + 30
	if end >= 24*60 {
		t.Skip("too close to midnight")
	}
	clock := func(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }
	mustRun(t, "schedule", "add", "b", "--days", "daily", "--from", clock(start), "--to", clock(end))
	mustRun(t, "allowance", "set", "b", "20m", "--per", "window")
	q, err := d.GetQuota("b")
	if err != nil || q == nil {
		t.Fatal(q, err)
	}
	want := fmt.Sprintf("%s %d", now.Format("2006-01-02"), start)
	if got := enforce.QuotaPeriod(d, "b", *q, now); got != want || q.Day != want {
		t.Errorf("period = %q, stored %q; want %q", got, q.Day, want)
	}
	later := now.Add(time.Duration(end-start+5) * time.Minute)
	if later.Day() == now.Day() {
		if got := enforce.QuotaPeriod(d, "b", *q, later); got != want {
			t.Errorf("outside the window the period changed to %q", got)
		}
	}
	tomorrow := now.Add(24 * time.Hour)
	if got := enforce.QuotaPeriod(d, "b", *q, tomorrow); got == want {
		t.Error("the next opening is the same period")
	}
}

// §4.9: private windows can be left out of statistics.
func TestStatsSkipPrivate(t *testing.T) {
	d := testDB(t)
	if rules, _ := enforce.BuildRules(d); rules.StatsSkipPrivate {
		t.Error("private windows skipped by default")
	}
	mustRun(t, "stats", "private", "skip")
	if rules, _ := enforce.BuildRules(d); !rules.StatsSkipPrivate {
		t.Error("stats private skip didn't reach the rules")
	}
	if err := runCLI(t, "stats", "private", "maybe"); err == nil {
		t.Error("a bad value was taken")
	}
	mustRun(t, "stats", "private", "count")
	if rules, _ := enforce.BuildRules(d); rules.StatsSkipPrivate {
		t.Error("count didn't bring them back")
	}
}
