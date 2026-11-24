package enforce

import (
	"fmt"
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

// §4.10: with block embedded content off, a blacklist's paths and patterns
// stay out of the browser policy, which Chrome and Firefox apply to frames
// too: an embedded path-blocked video plays. Whole sites stay (hosts and
// the firewall block them anyway), and so do KC9K's own lists.
func TestEmbeddedOffKeepsPatternsOutOfPolicy(t *testing.T) {
	t.Cleanup(func() { embeddedOff.Store(false) })
	user := db.ActiveBlockDetail{BlockName: "b", Lists: db.Lists{
		blacklist("Blocked", db.Where{URL: true}, "reddit.com", "player.vimeo.com/video/1", "*casino*")}}
	blocks := []db.ActiveBlockDetail{user, monkBlock()}

	hostsOn, _, bp := Inputs(blocks)
	if !reflect.DeepEqual(bp.BlockPatterns, []string{"player.vimeo.com/video/1", "*casino*", "*.*"}) {
		t.Errorf("setting on: patterns %v", bp.BlockPatterns)
	}
	embeddedOff.Store(true)
	hostsOff, _, bp := Inputs(blocks)
	if !reflect.DeepEqual(bp.BlockPatterns, []string{"*.*"}) {
		t.Errorf("setting off: patterns %v; want only monk mode's", bp.BlockPatterns)
	}
	if !reflect.DeepEqual(bp.BlockDomains, []string{"reddit.com"}) || !reflect.DeepEqual(hostsOff, hostsOn) {
		t.Errorf("setting off: sites %v, hosts %v; want reddit.com in both as before", bp.BlockDomains, hostsOff)
	}

	// The setting is read from the database before each apply.
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	loadDNSChoice(d)
	if embeddedOff.Load() {
		t.Error("unset: block embedded content should be on")
	}
	if err := d.SetSetting(SettingEmbedded, "off"); err != nil {
		t.Fatal(err)
	}
	loadDNSChoice(d)
	if !embeddedOff.Load() {
		t.Error("set off: not loaded")
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

// Principle 9: the daemon works out the layer inputs once for the same
// blocks (100,000 sites took 150 ms every cycle); any change works them out
// again, an entry changed in place, an action or the file URL setting too.
func TestInputsKeptForTheSameBlocks(t *testing.T) {
	t.Cleanup(func() { fileURLsOn.Store(false) })
	sites := make([]string, 100000)
	for i := range sites {
		sites[i] = fmt.Sprintf("site%d.example", i)
	}
	big := []db.ActiveBlockDetail{{BlockName: "big", Lists: db.Lists{blacklist("Blocked", db.Where{URL: true}, sites...)}}}
	Inputs(big)
	// Timed as the fastest of 5 rounds: one round on a loaded CI runner
	// (other packages' tests at once) once took 108 ms (A17, run 748).
	fastest := func(round func()) time.Duration {
		best := time.Hour
		for range 5 {
			start := time.Now()
			round()
			best = min(best, time.Since(start))
		}
		return best
	}
	var hosts []string
	if took := fastest(func() { hosts, _, _ = Inputs(big) }); took > 100*time.Millisecond || len(hosts) != len(sites) {
		t.Errorf("the same blocks again: %d hosts in %s", len(hosts), took)
	}
	hosts[0] = "changed.by.a.caller"
	if again, _, _ := Inputs(big); again[0] == "changed.by.a.caller" {
		t.Error("a caller's change reached the kept result")
	}

	small := []db.ActiveBlockDetail{{BlockName: "b", Lists: db.Lists{blacklist("Blocked", db.Where{URL: true}, "a.example", "file:///home/me/x")}}}
	if hosts, _, _ := Inputs(small); !reflect.DeepEqual(hosts, []string{"a.example"}) {
		t.Fatalf("hosts %v", hosts)
	}
	small[0].Lists[0].Entries[0] = "b.example" // changed in place
	if hosts, _, _ := Inputs(small); !reflect.DeepEqual(hosts, []string{"b.example"}) {
		t.Errorf("entry changed in place: %v", hosts)
	}
	small[0].Lists[0].Actions = []db.ListAction{{Name: db.ActionGreyscale, Distinct: 1, Hits: 1}}
	if hosts, _, _ := Inputs(small); len(hosts) != 0 {
		t.Errorf("no block action: %v", hosts)
	}
	small[0].Lists[0].Actions = []db.ListAction{{Name: db.ActionBlock, Distinct: 1, Hits: 1}}
	if _, _, bp := Inputs(small); len(bp.BlockFiles) != 0 {
		t.Fatalf("files with the setting off: %v", bp.BlockFiles)
	}
	small[0].Lists = append(small[0].Lists, exceptionList("Exceptions", db.Where{URL: true}))
	fileURLsOn.Store(true)
	if _, _, bp := Inputs(small); !reflect.DeepEqual(bp.BlockFiles, []string{"file:///home/me/x"}) {
		t.Errorf("file URL setting turned on: %v", bp.BlockFiles)
	}
	// The daemon asks for all blocks and for each alone every cycle: each
	// set is kept.
	fileURLsOn.Store(false)
	both := append(append([]db.ActiveBlockDetail{}, big...), small...)
	if took := fastest(func() {
		Inputs(both)
		Inputs(big)
		Inputs(small)
	}); took > 100*time.Millisecond {
		t.Errorf("all blocks, then each alone, again: %s", took)
	}
}
