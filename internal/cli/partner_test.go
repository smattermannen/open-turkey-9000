package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
	"github.com/smattermannen/kc9k/internal/db"
)

// §4.15 (2.13): a user and their accountability partner, each on their own
// computer (database), one fake cloud. Invitation and acceptance; alerts
// and a due report reach the partner (by email too); a password lock and
// the settings password only the partner holds, unlocked through an
// approved request; removing the partner and fewer reports refused while
// anything is locked.
func TestAccountabilityPartner(t *testing.T) {
	notTerminal(t)
	f := cloudtest.New()
	defer f.Close()
	dbs := map[string]string{}
	device := func(name string) *db.DB {
		path, ok := dbs[name]
		if !ok {
			path = filepath.Join(t.TempDir(), name+".db")
			dbs[name] = path
		}
		t.Setenv("KC9K_DB", path)
		d, err := db.OpenDB(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		if !ok {
			mustRun(t, "account", "server", f.URL, cloudtest.AnonKey)
		}
		return d
	}
	mailTo := func(to, text string) bool {
		for _, m := range f.Sent() {
			if m.To == to && strings.Contains(m.Subject+"\n"+m.Body, text) {
				return true
			}
		}
		return false
	}

	// The partner makes an account on their computer.
	device("partner")
	if err := runWith(t, 0, []string{"secret123"}, "account", "signup", "pat@example.org"); err != nil {
		t.Fatal(err)
	}
	// The user invites them.
	u := device("user")
	if err := runWith(t, 0, []string{"secret456"}, "account", "signup", "me@example.org"); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "start", "nothing"); err == nil {
		t.Fatal("start of a missing block worked")
	}
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add", "b", "reddit.com")
	if err := runCLI(t, "start", "b", "--password-stdin", "--partner-holds", "--yes"); err == nil ||
		!strings.Contains(err.Error(), "active accountability partner") {
		t.Fatalf("--partner-holds without a partner: %v", err)
	}
	if a, _ := u.GetBlock("b"); a.Active {
		t.Fatal("the block came on without its password lock")
	}
	mustRun(t, "partner", "invite", "pat@example.org")
	if !mailTo("pat@example.org", "kc9k partner accept me@example.org") {
		t.Errorf("no invitation email: %+v", f.Sent())
	}
	mustRun(t, "block", "create", "c")
	mustRun(t, "block", "add", "c", "x.com")
	mustRun(t, "start", "c")
	mustRun(t, "stop", "c")
	if out, _ := u.PartnerOutbox(); len(out) != 0 {
		t.Errorf("an alert was queued before anyone accepted: %+v", out)
	}

	// The partner accepts.
	device("partner")
	if out := capture(t, "partner"); !strings.Contains(out, "me@example.org invites you") {
		t.Errorf("invitation not listed: %s", out)
	}
	mustRun(t, "partner", "accept", "me@example.org")

	// Back on the user's computer: a password lock only the partner holds.
	u = device("user")
	var view partnerView
	if err := json.Unmarshal([]byte(capture(t, "partner", "--json")), &view); err != nil ||
		len(view.Partners) != 1 || view.Partners[0].Status != cloud.PartnerActive {
		t.Fatalf("partners: %+v %v", view, err)
	}
	out := capture(t, "start", "b", "--password-stdin", "--partner-holds", "--yes")
	if !strings.Contains(out, "you won't see it") {
		t.Errorf("start said: %s", out)
	}
	passwordLocked := func() bool {
		h, err := blockHold(u, "b", time.Now())
		return err == nil && h.active && slices.Contains(h.layers, db.LayerPassword)
	}
	if !passwordLocked() {
		t.Fatal("block b not on with a password lock")
	}
	if !mailTo("pat@example.org", "Password: ") {
		t.Error("the partner didn't get the password by email")
	}
	// The user's own account can't read it back.
	c, s, err := signedIn(u)
	if err != nil {
		t.Fatal(err)
	}
	if mine, _ := c.Inbox(s); len(mine) != 0 {
		t.Errorf("the user reads their own held password: %+v", mine)
	}
	// Alerts: a block switched off, a block deleted; sent by partner send.
	mustRun(t, "start", "c")
	mustRun(t, "stop", "c")
	mustRun(t, "block", "remove", "c")
	mustRun(t, "partner", "send")
	if out, _ := u.PartnerOutbox(); len(out) != 0 {
		t.Errorf("still queued after send: %+v", out)
	}
	if !mailTo("pat@example.org", "Block 'c' was switched off") || !mailTo("pat@example.org", "Block 'c' was deleted") {
		t.Errorf("alerts not emailed: %+v", f.Sent())
	}
	if len(f.Heartbeats) != 1 {
		t.Errorf("heartbeats: %v", f.Heartbeats)
	}
	// A report once one is due.
	mustRun(t, "partner", "reports", "daily")
	if err := u.SetSetting(settingPartnerLastReport, "1"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "partner", "send")
	if !mailTo("pat@example.org", "Times blocked") {
		t.Error("no report emailed")
	}

	// Weakening the partnership while a lock runs: refused.
	for _, args := range [][]string{{"partner", "remove", "pat@example.org"}, {"partner", "reports", "weekly"},
		{"partner", "reports", "off"}} {
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v while locked", args)
		}
	}

	// Unlock request: asked, approved by the partner, used once.
	if err := runCLI(t, "partner", "unlock", "b"); err == nil {
		t.Error("unlocked without an approved request")
	}
	mustRun(t, "partner", "request", "b", "--reason", "work needs reddit")
	device("partner")
	inbox := capture(t, "partner", "inbox")
	if !strings.Contains(inbox, "You hold the password of block 'b'") || !strings.Contains(inbox, "switched off") {
		t.Errorf("partner's inbox: %s", inbox)
	}
	var reqs []cloud.UnlockRequest
	if err := json.Unmarshal([]byte(capture(t, "partner", "requests", "--json")), &reqs); err != nil || len(reqs) != 1 {
		t.Fatalf("requests: %+v %v", reqs, err)
	}
	mustRun(t, "partner", "approve", reqs[0].ID)
	if err := runCLI(t, "partner", "approve", reqs[0].ID); err == nil {
		t.Error("approved twice")
	}
	u = device("user")
	mustRun(t, "partner", "unlock", "b")
	if passwordLocked() {
		t.Error("block b's password lock stayed")
	}
	if err := runCLI(t, "partner", "unlock", "b"); err == nil {
		t.Error("an approved request worked twice")
	}

	// The settings password, held by the partner.
	mustRun(t, "password", "set", "--partner-holds")
	if h, _ := u.GetSetting(SettingSettingsPassword); h == "" {
		t.Fatal("no settings password set")
	}
	mustRun(t, "partner", "request", "settings")
	device("partner")
	if err := json.Unmarshal([]byte(capture(t, "partner", "requests", "--json")), &reqs); err != nil || len(reqs) != 2 {
		t.Fatalf("requests: %+v %v", reqs, err)
	}
	mustRun(t, "partner", "approve", reqs[0].ID) // newest first
	u = device("user")
	mustRun(t, "partner", "unlock", "settings")
	if h, _ := u.GetSetting(SettingSettingsPassword); h != "" {
		t.Error("the settings password stayed")
	}

	// Nothing locked now (block b is on, unlocked): the partner may go, and
	// hears it.
	mustRun(t, "stop", "b")
	mustRun(t, "partner", "remove", "pat@example.org")
	if !mailTo("pat@example.org", "removed pat@example.org") {
		t.Error("the removed partner wasn't told")
	}
	if v, _ := u.GetSetting(db.SettingPartnerActive); v != "off" {
		t.Errorf("still an active partner: %q", v)
	}
}
