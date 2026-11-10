package enforce

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	if err := Run(d, Consequence{Kind: db.ConsequenceShutdown}, "test", true); err != nil {
		t.Fatal(err)
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceShutdown}, "test", true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(*did, " "); got != "notify close-browsers poweroff notify close-browsers" {
		t.Errorf("did %q", got)
	}

	// Logging out for a while goes through Frozen Chicken, and only extends it.
	*did = nil
	if err := Run(d, Consequence{Kind: db.ConsequenceLogout, Minutes: 60}, "test", false); err != nil {
		t.Fatal(err)
	}
	until, _ := d.FreezeUntil()
	mode, _ := d.FreezeMode()
	if mode != "logout" || until < time.Now().Add(59*time.Minute).Unix() {
		t.Errorf("freeze %s until %d", mode, until)
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceLockScreen, Minutes: 5}, "test", false); err != nil {
		t.Fatal(err)
	}
	if again, _ := d.FreezeUntil(); again != until {
		t.Error("a shorter freeze cut the longer one short")
	}
	if strings.Contains(strings.Join(*did, " "), "close-browsers") {
		t.Error("closed the browsers when asked not to")
	}
	if err := Run(d, Consequence{Kind: db.ConsequenceMonk}, "test", false); err == nil {
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

func TestNepsis(t *testing.T) {
	d := testDB(t)
	did := fakeDevice(t)
	if err := d.CreateBlock("b"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "b", db.DefaultBlockedList, "reddit.com")
	if err := d.ActivateBlock("b", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBlockNepsis("b", db.Nepsis{Count: 3, Minutes: 10, Consequence: db.ConsequenceCloseBrowser}); err != nil {
		t.Fatal(err)
	}
	active, _ := d.GetActiveBlocks()
	now := time.Now()
	for i := 0; i < 2; i++ {
		if err := d.RecordEvent(db.EventPage, "b", "reddit.com"); err != nil {
			t.Fatal(err)
		}
	}
	// Another block's events don't count towards b's.
	if err := d.RecordEvent(db.EventApp, "other", ""); err != nil {
		t.Fatal(err)
	}
	enforceNepsis(d, now, active)
	if len(*did) != 0 {
		t.Fatalf("fired at 2 of 3: %q", *did)
	}
	if err := d.RecordEvent(db.EventRefused, "b", "stop"); err != nil {
		t.Fatal(err)
	}
	enforceNepsis(d, now.Add(time.Second), active)
	if got := strings.Join(*did, " "); got != "notify close-browsers" {
		t.Fatalf("at 3 of 3 did %q", got)
	}
	// After firing, only newer events count.
	enforceNepsis(d, now.Add(2*time.Second), active)
	if len(*did) != 2 {
		t.Errorf("fired again on the same events: %q", *did)
	}

	// The global one counts every event.
	if err := d.SetGlobalNepsis(db.Nepsis{Count: 4, Minutes: 10, Consequence: db.ConsequenceMonk, ActionMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	enforceNepsis(d, now.Add(3*time.Second), nil)
	if m, err := d.GetBlock(MonkBlock); err != nil || !m.Active {
		t.Errorf("global bypass prevention didn't start monk mode: %v", err)
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
	if post(true, hit) != http.StatusNoContent || count() != 1 {
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
