package enforce

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/quota"
)

// fakeDevice replaces what consequences do to the computer and records it.
func fakeDevice(t *testing.T) *[]string {
	t.Helper()
	var did []string
	saved := []any{closeBrowsers, logoutUsers, powerOff, notifyDesktop, applyFreeze}
	closeBrowsers = func() error { did = append(did, "close-browsers"); return nil }
	logoutUsers = func() error { did = append(did, "logout"); return nil }
	powerOff = func() error { did = append(did, "poweroff"); return nil }
	notifyDesktop = func(title, _ string) { did = append(did, "notify") }
	applyFreeze = func(*db.DB, time.Time) { did = append(did, "freeze") }
	t.Cleanup(func() {
		closeBrowsers = saved[0].(func() error)
		logoutUsers = saved[1].(func() error)
		powerOff = saved[2].(func() error)
		notifyDesktop = saved[3].(func(string, string))
		applyFreeze = saved[4].(func(*db.DB, time.Time))
	})
	return &did
}

func testDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestRunConsequences(t *testing.T) {
	d := testDB(t)
	did := fakeDevice(t)

	// A one-off shutdown happens once; another within onceEvery only closes
	// the browsers, so a restored page can't loop the computer.
	if err := Run(d, Consequence{Kind: db.ConsequenceShutdown}, "test"); err != nil {
		t.Fatal(err)
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceShutdown}, "test"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(*did, " "); got != "notify poweroff notify close-browsers" {
		t.Errorf("did %q", got)
	}

	// Logging out for a while goes through Frozen Chicken, and only extends it.
	*did = nil
	if err := Run(d, Consequence{Kind: db.ConsequenceLogout, Minutes: 60}, ""); err != nil {
		t.Fatal(err)
	}
	until, _ := d.FreezeUntil()
	mode, _ := d.FreezeMode()
	if mode != "logout" || until < time.Now().Add(59*time.Minute).Unix() {
		t.Errorf("freeze %s until %d", mode, until)
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceLockScreen, Minutes: 5}, ""); err != nil {
		t.Fatal(err)
	}
	if again, _ := d.FreezeUntil(); again != until {
		t.Error("a shorter freeze cut the longer one short")
	}
	if got := strings.Join(*did, " "); got != "freeze freeze" {
		t.Errorf("did %q; want only the freeze, twice, and no notification", got)
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceMonk}, "test"); err == nil {
		t.Error("monk mode without a duration was accepted")
	}
}

func TestStartMonk(t *testing.T) {
	d := testDB(t)
	if err := StartMonk(d, time.Hour); err != nil {
		t.Fatal(err)
	}
	b, err := d.GetBlock(MonkBlock)
	if err != nil {
		t.Fatal(err)
	}
	l, _ := b.Lists.Find(db.DefaultBlockedList)
	trusted, _ := d.TrustedNow(time.Now())
	extras, _ := d.GetLockExtras(MonkBlock)
	if !b.Active || len(l.Entries) != 1 || l.Entries[0] != "*.*" || b.LockedUntil < trusted+3500 || extras.StopAt != b.LockedUntil {
		t.Fatalf("monk block = %+v, stops at %d", b, extras.StopAt)
	}
	// A longer run extends it; a shorter one changes nothing.
	if err := StartMonk(d, 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	longer, _ := d.GetBlock(MonkBlock)
	if err := StartMonk(d, time.Minute); err != nil {
		t.Fatal(err)
	}
	same, _ := d.GetBlock(MonkBlock)
	extras, _ = d.GetLockExtras(MonkBlock)
	if longer.LockedUntil <= b.LockedUntil || same.LockedUntil != longer.LockedUntil || extras.StopAt != longer.LockedUntil {
		t.Errorf("until %d, then %d, then %d; stops at %d", b.LockedUntil, longer.LockedUntil, same.LockedUntil, extras.StopAt)
	}
}

// at returns events n seconds after base, as RecordEvent stores them.
func addEvents(t *testing.T, d *db.DB, kind, block string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if err := d.RecordEvent(kind, block, ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNepsisLadder(t *testing.T) {
	d := testDB(t)
	did := fakeDevice(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	ladder := db.Nepsis{
		Detectors: []string{db.EventPage, db.EventRefused},
		Steps: []db.NepsisStep{
			{Count: 2, Minutes: 10, Actions: []db.NepsisAction{{Kind: db.ConsequenceWarning}, {Kind: db.ConsequenceCloseTab}}},
			{Count: 2, Minutes: 10, Actions: []db.NepsisAction{{Kind: db.ConsequenceCloseBrowser}}},
		},
		QuietMinutes: 60,
	}
	if err := d.SetBlockNepsis("b", ladder); err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	// Apps aren't among the detectors; another block's events don't count.
	addEvents(t, d, db.EventApp, "b", 5)
	addEvents(t, d, db.EventPage, "other", 5)
	addEvents(t, d, db.EventPage, "b", 1)
	if checkScopes(d, now, []string{"b"}) || len(*did) != 0 {
		t.Fatalf("fired early: %q", *did)
	}
	// Step 1: a warning and the tab.
	addEvents(t, d, db.EventRefused, "b", 1)
	if !checkScopes(d, now.Add(time.Second), []string{"b"}) {
		t.Fatal("step 1 didn't ask to close the tab")
	}
	if got := strings.Join(*did, " "); got != "notify" {
		t.Fatalf("step 1 did %q", got)
	}
	// The same events don't count twice; two new ones climb to step 2.
	if checkScopes(d, now.Add(2*time.Second), []string{"b"}) {
		t.Fatal("fired again on the same events")
	}
	time.Sleep(2100 * time.Millisecond) // events after the firing second (now+1s)
	addEvents(t, d, db.EventPage, "b", 2)
	if checkScopes(d, time.Now(), []string{"b"}) {
		t.Fatal("step 2 asked to close the tab")
	}
	if got := strings.Join(*did, " "); got != "notify notify close-browsers" {
		t.Fatalf("after step 2: %q", got)
	}
	n, _ := d.BlockNepsis("b")
	if level, _, _ := NepsisState(d, time.Now(), "b", n); level != 1 {
		t.Errorf("level after the last step = %d; want it to stay on the last (1)", level)
	}
	// After the quiet time it falls back to step 1.
	if level, _, _ := NepsisState(d, time.Now().Add(61*time.Minute), "b", n); level != 0 {
		t.Errorf("level after an hour's quiet = %d; want 0", level)
	}
}

// A bypass prevention stored before the ladder becomes one step that closes
// the browsers first, as it did.
func TestNepsisFromBeforeTheLadder(t *testing.T) {
	d := testDB(t)
	if err := d.SetSetting(db.SettingNepsis, `{"count":4,"minutes":10,"consequence":"monk","action_minutes":30}`); err != nil {
		t.Fatal(err)
	}
	n, err := d.GlobalNepsis()
	if err != nil {
		t.Fatal(err)
	}
	want := []db.NepsisStep{{Count: 4, Minutes: 10, Actions: []db.NepsisAction{
		{Kind: db.ConsequenceCloseBrowser}, {Kind: db.ConsequenceMonk, Minutes: 30}}}}
	if !reflect.DeepEqual(n.Steps, want) || n.Count != 0 {
		t.Errorf("read as %+v", n)
	}
}

func TestNepsisStricter(t *testing.T) {
	step := func(count, minutes int, actions ...db.NepsisAction) db.NepsisStep {
		return db.NepsisStep{Count: count, Minutes: minutes, Actions: actions}
	}
	monk := func(m int) db.NepsisAction { return db.NepsisAction{Kind: db.ConsequenceMonk, Minutes: m} }
	warn := db.NepsisAction{Kind: db.ConsequenceWarning}
	old := db.Nepsis{Detectors: []string{"page"}, QuietMinutes: 60, Steps: []db.NepsisStep{step(3, 10, warn), step(3, 10, monk(5))}}
	cases := []struct {
		name string
		n    db.Nepsis
		want bool
	}{
		{"the same", old, true},
		{"fewer events, longer, more", db.Nepsis{Detectors: []string{"page", "app"}, QuietMinutes: 120,
			Steps: []db.NepsisStep{step(2, 20, warn, monk(1)), step(3, 10, monk(10)), step(1, 5, warn)}}, true},
		{"every detector", db.Nepsis{QuietMinutes: 60, Steps: old.Steps}, true},
		{"never falls back", db.Nepsis{Detectors: []string{"page"}, Steps: old.Steps}, true},
		{"falls back sooner", db.Nepsis{Detectors: []string{"page"}, QuietMinutes: 30, Steps: old.Steps}, false},
		{"a detector less", db.Nepsis{Detectors: []string{"app"}, QuietMinutes: 60, Steps: old.Steps}, false},
		{"a step less", db.Nepsis{Detectors: []string{"page"}, QuietMinutes: 60, Steps: old.Steps[:1]}, false},
		{"a shorter monk", db.Nepsis{Detectors: []string{"page"}, QuietMinutes: 60, Steps: []db.NepsisStep{step(3, 10, warn), step(3, 10, monk(4))}}, false},
		{"off", db.Nepsis{}, false},
	}
	for _, c := range cases {
		if got := c.n.StricterThan(old); got != c.want {
			t.Errorf("%s: got %v; want %v", c.name, got, c.want)
		}
	}
}

func TestHitHandler(t *testing.T) {
	d := testDB(t)
	fakeDevice(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	id, _ := d.ListID("b", db.DefaultBlockedList)
	handler := hitHandler(d)
	post := func(header bool, body string) int {
		r := httptest.NewRequest(http.MethodPost, "http://"+RulesAddress+"/hit", strings.NewReader(body))
		r.Host = RulesAddress
		if header {
			r.Header.Set(ExtensionHeader, "1")
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w.Code
	}
	count := func() int {
		n, _ := d.CountEvents("b", 0)
		return n
	}
	hit := `{"list": ` + itoa(id) + `, "actions": ["block"], "url": "https://reddit.com/"}`
	if post(false, hit) != http.StatusForbidden || count() != 0 {
		t.Fatal("a hit without the extension's header counted")
	}
	if post(true, hit) != http.StatusOK || count() != 1 {
		t.Fatalf("a hit didn't count: %d events", count())
	}
	// Actions the list doesn't take, and lists that aren't on, count nothing.
	post(true, `{"list": `+itoa(id)+`, "actions": ["greyscale", "shutdown"]}`)
	post(true, `{"list": 999, "actions": ["block"]}`)
	if count() != 1 {
		t.Errorf("%d events; want 1", count())
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestTamperSigns(t *testing.T) {
	d := testDB(t)
	NoteDaemonStart(d)
	if n, _ := d.CountEvents("", 0); n != 0 {
		t.Fatal("the first start in a boot counted as tampering")
	}
	NoteDaemonStart(d)
	if n, _ := d.CountEvents("", 0); n != 1 {
		t.Errorf("a second start in the same boot logged %d events; want 1", n)
	}
	if got := clockDrift(0, 0, 10*time.Minute, 5*time.Second); got != 10*time.Minute-5*time.Second {
		t.Errorf("drift = %s", got)
	}
	if got := clockDrift(0, 0, 5*time.Second, 5*time.Second); got != 0 {
		t.Errorf("drift with both clocks moving = %s", got)
	}
}

// An open quota session spends only the time its block was in use.
func TestQuotaSpendsUse(t *testing.T) {
	d := testDB(t)
	if err := d.CreateBlock("games"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddApps("games", []string{"steam"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s := quota.New(600, now.Unix(), quota.DayOf(now))
	s.Open = true
	if err := d.SaveQuota("games", s); err != nil {
		t.Fatal(err)
	}
	// Twenty seconds pass with nothing in use: nothing is spent.
	if err := reconcileQuotas(d, now.Add(20*time.Second), now.Unix()); err != nil {
		t.Fatal(err)
	}
	if q, _ := d.GetQuota("games"); q.Used != 0 || !q.Open {
		t.Fatalf("idle session: %+v", q)
	}
	// Steam in front for 5 seconds, then a forged report for a closed
	// session's block.
	noteQuotaApps(d, []string{"firefox", "steam"}, 5)
	addQuotaUse("other", 50)
	if err := reconcileQuotas(d, now.Add(25*time.Second), now.Unix()); err != nil {
		t.Fatal(err)
	}
	if q, _ := d.GetQuota("games"); q.Used != 5 {
		t.Errorf("used = %d; want 5", q.Used)
	}
}
