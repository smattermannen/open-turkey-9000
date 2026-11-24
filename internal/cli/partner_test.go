package cli

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
	"github.com/smattermannen/kc9k/internal/db"
)

// §4.15, two devices on two databases, one fake cloud: the user invites
// Anna, who accepts on her KC9K; a password lock only she knows; removing
// her, alerts off and stopping the block are refused while it is locked;
// she approves the unlock request and the next 'partner requests' unlocks
// it; alerts and the daily report reach her through the daemon's tick; she
// holds the settings password, which keeps her from being removed until
// she has given it back.
func TestPartner(t *testing.T) {
	notTerminal(t)
	f := cloudtest.New()
	defer f.Close()
	paths := map[*db.DB]string{}
	device := func() *db.DB {
		path := filepath.Join(t.TempDir(), "t.db")
		t.Setenv("KC9K_DB", path)
		d, err := db.OpenDB(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		mustRun(t, "account", "server", f.URL, cloudtest.AnonKey)
		paths[d] = path
		return d
	}
	me := device()
	mePath := paths[me]
	if err := runWith(t, 0, []string{"secret123"}, "account", "signup", "me@example.org"); err != nil {
		t.Fatal(err)
	}
	// Nothing to send to without partners: the queue is emptied, unsent.
	_ = me.QueuePartnerAlert("early", "")
	partnerTick(me, time.Now())
	if a, _ := me.PartnerAlerts(); len(a) != 0 || len(f.Queued()) != 0 {
		t.Errorf("alerts without partners: kept %v, sent %v", a, f.Queued())
	}
	mustRun(t, "partner", "invite", "anna@example.org", "--reports", "daily")
	if err := runCLI(t, "partner", "lock", "work", "anna@example.org", "--yes"); err == nil {
		t.Error("a lock for a block that doesn't exist")
	}
	mustRun(t, "block", "create", "work")
	mustRun(t, "block", "add", "work", "reddit.com")
	if err := runCLI(t, "partner", "lock", "work", "anna@example.org", "--yes"); err == nil || !strings.Contains(err.Error(), "accepted") {
		t.Errorf("a lock held by someone who hasn't accepted: %v", err)
	}

	anna := device()
	if err := runWith(t, 0, []string{"secret456"}, "account", "signup", "anna@example.org"); err != nil {
		t.Fatal(err)
	}
	if out := capture(t, "partner", "inbox"); !strings.Contains(out, "Invitation from me@example.org") {
		t.Errorf("anna's inbox: %s", out)
	}
	mustRun(t, "partner", "accept", "me@example.org")

	t.Setenv("KC9K_DB", mePath)
	if err := runCLI(t, "partner", "lock", "work", "anna@example.org"); err == nil {
		t.Error("a partner lock without confirming (R-LOCK-12)")
	}
	mustRun(t, "partner", "lock", "work", "anna@example.org", "--yes")
	if b, _ := me.GetBlock("work"); !b.Active {
		t.Fatalf("the block isn't on: %+v", b)
	}
	if e, _ := me.GetLockExtras("work"); e.PasswordHash == "" {
		t.Fatalf("no password lock: %+v", e)
	}
	if err := runCLI(t, "stop", "work"); err == nil {
		t.Error("stopped a block with a partner's lock")
	}
	for _, args := range [][]string{
		{"partner", "remove", "anna@example.org"},
		{"partner", "set", "anna@example.org", "--alerts=false"},
		{"partner", "set", "anna@example.org", "--reports", "weekly"},
	} {
		if err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "refused while") {
			t.Errorf("%v while locked: %v", args, err)
		}
	}
	if out := capture(t, "partner", "list"); !strings.Contains(out, "anna@example.org: accepted; reports daily, alerts on; holds lock:work") {
		t.Errorf("list: %s", out)
	}
	mustRun(t, "partner", "set", "anna@example.org", "--reports", "daily") // the same: free

	// Alerts and the report through the daemon's tick.
	_ = me.QueuePartnerAlert("Block 'games' was switched off", "")
	partnerTick(me, time.Now())
	var kinds []string
	for _, m := range f.Queued() {
		if m.To == "anna@example.org" {
			kinds = append(kinds, m.Kind+": "+m.Subject)
		}
	}
	if !slices.ContainsFunc(kinds, func(k string) bool { return strings.Contains(k, "alert: Killer Chicken 9000: Block 'games' was switched off") }) ||
		!slices.ContainsFunc(kinds, func(k string) bool { return strings.HasPrefix(k, "report: Killer Chicken 9000 daily report") }) ||
		slices.ContainsFunc(kinds, func(k string) bool { return strings.Contains(k, "weekly") }) {
		t.Errorf("anna got: %v", kinds)
	}
	if a, _ := me.PartnerAlerts(); len(a) != 0 {
		t.Errorf("sent alerts kept: %v", a)
	}
	n := len(f.Queued())
	partnerTick(me, time.Now())
	if len(f.Queued()) != n {
		t.Errorf("the report went twice: %v", f.Queued()[n:])
	}

	mustRun(t, "partner", "request", "work", "--reason", "the exam is over")
	if out := capture(t, "partner", "requests"); !strings.Contains(out, "waiting for their answer") {
		t.Errorf("before the answer: %s", out)
	}
	t.Setenv("KC9K_DB", paths[anna])
	out := capture(t, "partner", "inbox")
	id := regexp.MustCompile(`Request (\d+) from me@example.org to unlock lock:work: "the exam is over"`).FindStringSubmatch(out)
	if id == nil || !strings.Contains(out, "You hold me@example.org's lock:work: ") {
		t.Fatalf("anna's inbox: %s", out)
	}
	mustRun(t, "partner", "approve", id[1])
	t.Setenv("KC9K_DB", mePath)
	if out := capture(t, "partner", "requests"); !strings.Contains(out, "block 'work' unlocked") {
		t.Errorf("after approval: %s", out)
	}
	if e, _ := me.GetLockExtras("work"); e.PasswordHash != "" {
		t.Error("the approved request didn't unlock the block")
	}
	if holds := partnerHolds(me); len(holds) != 0 {
		t.Errorf("still held after use: %v", holds)
	}

	// The settings password, held by Anna: she can't be removed until she
	// gives it back.
	mustRun(t, "partner", "hold", "settings-password", "anna@example.org", "--yes")
	if set, _ := settingsPasswordSet(me); !set {
		t.Fatal("no settings password")
	}
	t.Setenv("KC9K_DB", paths[anna])
	held := regexp.MustCompile(`You hold me@example.org's settings-password: (\S+)`).FindStringSubmatch(capture(t, "partner", "inbox"))
	if held == nil {
		t.Fatal("anna doesn't hold the settings password")
	}
	t.Setenv("KC9K_DB", mePath)
	if err := runWith(t, 0, []string{held[1]}, "partner", "remove", "anna@example.org"); err == nil || !strings.Contains(err.Error(), "holds your settings password") {
		t.Errorf("removed the holder of the settings password: %v", err)
	}
	mustRun(t, "partner", "request", "settings-password")
	t.Setenv("KC9K_DB", paths[anna])
	reqs := regexp.MustCompile(`Request (\d+) from`).FindStringSubmatch(capture(t, "partner", "inbox"))
	mustRun(t, "partner", "approve", reqs[1])
	t.Setenv("KC9K_DB", mePath)
	if out := capture(t, "partner", "requests"); !strings.Contains(out, "Your settings password is: "+held[1]) {
		t.Errorf("the approved settings password isn't given: %s", out)
	}
	if err := runWith(t, 0, []string{held[1]}, "partner", "remove", "anna@example.org"); err != nil {
		t.Errorf("removing anna once nothing is locked or held: %v", err)
	}
	c := cloud.New(f.URL, cloudtest.AnonKey)
	s, _ := c.Login("me@example.org", "secret123")
	if ps, _ := c.Partners(s); len(ps) != 0 {
		t.Errorf("partners after removal: %+v", ps)
	}
}

// §4.15 report: the last whole day, time per site and app, times blocked.
func TestPartnerReport(t *testing.T) {
	d := testDB(t)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.Local) // a Wednesday
	for _, r := range []struct {
		day, kind, name string
		n               int64
	}{
		{"2026-11-24", db.UsageSite, "youtube.com", 3600},
		{"2026-11-24", db.UsageApp, "steam", 600},
		{"2026-11-24", db.UsageBlocked, "reddit.com", 7},
		{"2026-11-25", db.UsageSite, "today.example", 60}, // today: not yet
		{"2026-11-17", db.UsageSite, "lastweek.example", 60},
	} {
		if err := d.AddUsage(r.day, r.kind, r.name, r.n); err != nil {
			t.Fatal(err)
		}
	}
	subject, body, err := partnerReport(d, "daily", now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject, "daily report: Tue 24 Nov") || !strings.Contains(body, "youtube.com") ||
		!strings.Contains(body, "steam") || !strings.Contains(body, "Times blocked: 7") ||
		strings.Contains(body, "today.example") || strings.Contains(body, "lastweek.example") {
		t.Errorf("daily: %s\n%s", subject, body)
	}
	subject, body, _ = partnerReport(d, "weekly", now)
	if !strings.Contains(subject, "Mon 16 Nov to Sun 22 Nov") || !strings.Contains(body, "lastweek.example") || strings.Contains(body, "youtube.com") {
		t.Errorf("weekly: %s\n%s", subject, body)
	}
}
