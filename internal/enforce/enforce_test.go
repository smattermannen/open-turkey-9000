package enforce

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/schedule"
)

// enabledAt reports whether a block enforces at t: active and not lifted by
// its windows or places (§3.3).
func enabledAt(d *db.DB, name string, t time.Time) bool {
	b, err := d.GetBlock(name)
	return err == nil && b.Active && !scheduleLifted(d, t)[name]
}

// TestReconcileSchedules checks the daemon's scheduling core (R-SCHED-1, no
// layers, so no root needed): an inactive block never enforces; an active
// one is enabled inside its window and lifted outside it; the daemon never
// switches it on or off.
func TestReconcileSchedules(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("s"); err != nil {
		t.Fatal(err)
	}
	if err := addEntries(d, "s", db.DefaultBlockedList, "x.com"); err != nil {
		t.Fatal(err)
	}
	// Every day, 09:00–17:00.
	if err := d.AddSchedule("s", schedule.Window{Days: 0x7f, Start: 9 * 60, End: 17 * 60}); err != nil {
		t.Fatal(err)
	}

	inWindow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local)  // Mon 10:00
	outWindow := time.Date(2024, 1, 1, 18, 0, 0, 0, time.Local) // Mon 18:00

	if err := reconcileSchedules(d, inWindow); err != nil {
		t.Fatal(err)
	}
	if enabledAt(d, "s", inWindow) {
		t.Fatal("an inactive block enforces in its window")
	}
	if err := d.ActivateBlock("s", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if !enabledAt(d, "s", inWindow) || enabledAt(d, "s", outWindow) {
		t.Fatal("active block: want enabled in its window, lifted outside it")
	}
	if err := reconcileSchedules(d, outWindow); err != nil {
		t.Fatal(err)
	}
	if b, _ := d.GetBlock("s"); !b.Active {
		t.Fatal("the schedule switched the block off")
	}
}

// TestLockNeverChangesWhatIsBlocked: a locked block outside its window stays
// on (it can't be switched off) but is lifted: a lock never changes what is
// blocked (§3.4).
func TestLockNeverChangesWhatIsBlocked(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("s"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSchedule("s", schedule.Window{Days: 0x7f, Start: 9 * 60, End: 17 * 60}); err != nil {
		t.Fatal(err)
	}
	day := func(h int) time.Time { return time.Date(2024, 1, 1, h, 0, 0, 0, time.Local) }
	if err := d.ActivateBlock("s", true, 300, day(19).Unix(), false); err != nil {
		t.Fatal(err)
	}
	if err := reconcileSchedules(d, day(18)); err != nil {
		t.Fatal(err)
	}
	if b, _ := d.GetBlock("s"); !b.Active || enabledAt(d, "s", day(18)) || !enabledAt(d, "s", day(10)) {
		t.Errorf("locked block at 18:00: active %v, enabled %v; want on and lifted", b.Active, enabledAt(d, "s", day(18)))
	}
}

// TestApplyDueEdits checks that the daemon applies queued edits once due, but
// not while the block is char- or time-locked.
func TestApplyDueEdits(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("games"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddApps("games", []string{"steam", "lutris"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)
	if err := d.QueueEdit("games", db.Edit{Action: db.EditRemoveApps, Values: []string{"steam"}}, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	apps := func() []string { b, _ := d.GetBlock("games"); return b.Apps }

	// Not due yet.
	if err := applyDueEdits(d, now); err != nil {
		t.Fatal(err)
	}
	if len(apps()) != 2 {
		t.Fatal("edit applied before it was due")
	}

	// Due, but the block is time-locked: stays queued.
	if err := d.ActivateBlock("games", false, 0, now.Add(3*time.Hour).Unix(), false); err != nil {
		t.Fatal(err)
	}
	if err := applyDueEdits(d, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(apps()) != 2 {
		t.Fatal("edit applied while the block was time-locked")
	}

	// Lock expired: applied and removed from the queue.
	if err := applyDueEdits(d, now.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(apps(), []string{"lutris"}) {
		t.Fatalf("apps = %v; want [lutris]", apps())
	}
	if pending, _ := d.ListPendingEdits(); len(pending) != 0 {
		t.Errorf("pending = %+v; want none", pending)
	}
}

// TestReconcileDropsUnscheduledBlock: once its windows are all removed a
// block is continuous: enabled whenever active.
func TestReconcileDropsUnscheduledBlock(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("games"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddApps("games", []string{"steam"}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddSchedule("games", schedule.Window{Days: schedule.Friday, Start: 18 * 60, End: 24 * 60, Allow: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("games", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	friday := time.Date(2024, 1, 5, 20, 0, 0, 0, time.Local)
	if enabledAt(d, "games", friday) {
		t.Fatal("enabled in its allow window")
	}
	if err := d.RemoveSchedule("games", 1); err != nil {
		t.Fatal(err)
	}
	if !enabledAt(d, "games", friday) {
		t.Fatal("a block with no windows left should be enabled whenever active")
	}
}

// TestReconcileAllowSchedule checks allow windows: the active block is
// enabled outside them and lifted inside them.
func TestReconcileAllowSchedule(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.CreateBlock("games"); err != nil {
		t.Fatal(err)
	}
	// Allowed Friday and Saturday 18:00–24:00.
	if err := d.AddSchedule("games", schedule.Window{Days: schedule.Friday | schedule.Saturday, Start: 18 * 60, End: 24 * 60, Allow: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("games", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}

	fri := func(h, m int) time.Time { return time.Date(2024, 1, 5, h, m, 0, 0, time.Local) } // a Friday
	steps := []struct {
		t    time.Time
		want bool
	}{
		{fri(12, 0), true},                   // Friday afternoon: blocked
		{fri(18, 0), false},                  // allow window open
		{fri(23, 59), false},                 // still open up to midnight
		{fri(12, 0).AddDate(0, 0, 2), true},  // Sunday: blocked
		{fri(20, 0).AddDate(0, 0, 1), false}, // Saturday evening
		{fri(20, 0).AddDate(0, 0, 3), true},  // Monday evening: blocked
	}
	for _, st := range steps {
		if err := reconcileSchedules(d, st.t); err != nil {
			t.Fatal(err)
		}
		if got := enabledAt(d, "games", st.t); got != st.want {
			t.Errorf("%s: enabled = %v; want %v", st.t.Format("Mon 15:04"), got, st.want)
		}
		if on, _ := ScheduleActiveNow(d, "games", st.t); on != st.want {
			t.Errorf("%s: ScheduleActiveNow = %v; want %v", st.t.Format("Mon 15:04"), on, st.want)
		}
	}
}

func TestInputsBlocklist(t *testing.T) {
	active := []db.ActiveBlockDetail{
		{BlockName: "a", Lists: db.Lists{blacklist("Blocked", db.Where{URL: true}, "facebook.com", "*ads*")}, Apps: []string{"discord"}},
		{BlockName: "b", Lists: db.Lists{blacklist("Blocked", db.Where{URL: true}, "reddit.com")}, Apps: []string{"discord", "slack"}},
	}
	hosts, apps, bp := Inputs(active)

	if !reflect.DeepEqual(hosts, []string{"facebook.com", "reddit.com"}) {
		t.Errorf("hosts = %v", hosts)
	}
	if !reflect.DeepEqual(apps, []string{"discord", "slack"}) { // deduped, sorted
		t.Errorf("apps = %v", apps)
	}
	if !reflect.DeepEqual(bp.BlockPatterns, []string{"*ads*"}) {
		t.Errorf("patterns = %v", bp.BlockPatterns)
	}
}

// Blocking everything but some sites is *.* with exceptions: browser policy
// only, and the exceptions go with it.
func TestInputsEverythingExcept(t *testing.T) {
	active := []db.ActiveBlockDetail{
		{BlockName: "block", Lists: db.Lists{blacklist("Blocked", db.Where{URL: true}, "facebook.com")}},
		{BlockName: "allow", Lists: db.Lists{blacklist("Blocked", db.Where{URL: true}, "*.*"),
			exceptionList("Exceptions", db.Where{URL: true}, "work.com", "docs.com")}},
	}
	hosts, _, bp := Inputs(active)
	if !reflect.DeepEqual(hosts, []string{"facebook.com"}) {
		t.Errorf("hosts = %v; want [facebook.com]", hosts)
	}
	if !reflect.DeepEqual(bp.BlockPatterns, []string{"*.*"}) || !reflect.DeepEqual(bp.Exceptions, []string{"work.com", "docs.com"}) {
		t.Errorf("policy = %+v; want *.* except work.com and docs.com", bp)
	}
}

// §3.1, §4.10: with "allow file:// blocking" on, a blacklist's file entries
// reach the browser policy unless the block's file exceptions (file://* by
// default, R-LIST-3) spare them; off, they never do.
func TestFileEntriesInPolicy(t *testing.T) {
	t.Cleanup(func() { fileURLsOn.Store(false) })
	url := db.Where{URL: true}
	block := func(except ...string) []db.ActiveBlockDetail {
		return []db.ActiveBlockDetail{{BlockName: "b", Lists: db.Lists{
			blacklist("Blocked", url, "reddit.com", "file:///home/me/games/*"),
			exceptionList("Exceptions", url, except...)}}}
	}
	if _, _, bp := Inputs(block()); len(bp.BlockFiles) != 0 {
		t.Errorf("setting off: %v", bp.BlockFiles)
	}
	fileURLsOn.Store(true)
	if _, _, bp := Inputs(block()); len(bp.BlockFiles) != 1 {
		t.Errorf("setting on, no exception: %v", bp.BlockFiles)
	}
	if _, _, bp := Inputs(block("file://*")); len(bp.BlockFiles) != 0 {
		t.Errorf("spared by file://*: %v", bp.BlockFiles)
	}
	if _, _, bp := Inputs(block("file:///tmp/*")); len(bp.BlockFiles) != 1 {
		t.Errorf("an unrelated file exception spared it: %v", bp.BlockFiles)
	}
}

// §4.10: the SafeSearch browser setting reaches the extension's rules.
func TestRulesSafeSearch(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if r, _ := BuildRules(d); r.SafeSearch {
		t.Error("on without the setting")
	}
	if err := d.SetSetting(SettingKey("safesearch-google"), "on"); err != nil {
		t.Fatal(err)
	}
	if r, _ := BuildRules(d); !r.SafeSearch {
		t.Error("the setting didn't reach the rules")
	}
}

// R-LOCK-9: while anything is locked, the extension is required, even for
// a block of apps only or a locked setting alone.
func TestExtensionRequiredWhileLocked(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.CreateBlock("apps"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddApps("apps", []string{"steam"}); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("apps", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	active, _ := d.GetActiveBlocks()
	if ExtensionRequired(d, active) {
		t.Error("required for an unlocked app-only block")
	}
	if err := d.StrengthenLocks("apps", false, 0, time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	active, _ = d.GetActiveBlocks()
	if !ExtensionRequired(d, active) {
		t.Error("not required while a block is locked")
	}
}
