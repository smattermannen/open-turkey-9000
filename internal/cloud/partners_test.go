package cloud_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
)

type sentMail struct{ to, subject string }

type fakeMailer struct {
	sent []sentMail
	fail string // an address that bounces
}

func (m *fakeMailer) Send(to, subject, body string) error {
	if to == m.fail {
		return errors.New("bounced")
	}
	m.sent = append(m.sent, sentMail{to, subject})
	return nil
}

// §4.15: invite, accept, reports and alerts to those who get them, a held
// password whose secret only the partner reads, an unlock request approved
// (the owner then gets the secret once), removal; email only from the
// queue, with the service role key.
func TestPartnerRoundTrip(t *testing.T) {
	f := cloudtest.New()
	defer f.Close()
	c := cloud.New(f.URL, cloudtest.AnonKey)
	me, _ := c.SignUp("me@example.org", "secret123")
	anna, _ := c.SignUp("anna@example.org", "secret123")
	bob, _ := c.SignUp("bob@example.org", "secret123")

	if _, err := c.InvitePartner(me, "me@example.org", "daily", true); err == nil {
		t.Error("invited oneself")
	}
	pa, err := c.InvitePartner(me, "Anna@example.org", "daily", true)
	if err != nil || pa.Email != "anna@example.org" || pa.Accepted {
		t.Fatalf("invite: %+v %v", pa, err)
	}
	if _, err := c.InvitePartner(me, "bob@example.org", "weekly", false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.InvitePartner(me, "anna@example.org", "daily", true); err == nil {
		t.Error("the same partner twice")
	}
	in, _ := c.PartnerInbox(anna)
	if len(in.Invitations) != 1 || in.Invitations[0].From != "me@example.org" {
		t.Fatalf("anna's invitations: %+v", in)
	}
	if err := c.AcceptInvitation(bob, in.Invitations[0].ID); err == nil {
		t.Error("bob accepted anna's invitation")
	}
	if err := c.Hold(me, pa.ID, "settings-password", "x"); err == nil {
		t.Error("a password held before the invitation was accepted")
	}
	if err := c.AcceptInvitation(anna, pa.ID); err != nil {
		t.Fatal(err)
	}
	bobIn, _ := c.PartnerInbox(bob)
	if err := c.AcceptInvitation(bob, bobIn.Invitations[0].ID); err != nil {
		t.Fatal(err)
	}

	if n, _ := c.SendToPartners(me, cloud.KindAlert, "", "Block 'work' switched off", "at 10:00"); n != 1 {
		t.Errorf("alert went to %d partners, want anna only", n)
	}
	if n, _ := c.SendToPartners(me, cloud.KindReport, "weekly", "Your week", "…"); n != 1 {
		t.Errorf("weekly report went to %d partners, want bob only", n)
	}
	if err := c.Hold(me, pa.ID, "lock:work", "s3cret-pw"); err != nil {
		t.Fatal(err)
	}
	if err := c.Heartbeat(me, "laptop"); err != nil {
		t.Fatal(err)
	}
	in, _ = c.PartnerInbox(anna)
	if len(in.Held) != 1 || in.Held[0].Secret != "s3cret-pw" || len(in.Messages) != 1 ||
		len(in.Watching) != 1 || in.Watching[0].Device != "laptop" || in.Watching[0].LastSeen.IsZero() {
		t.Errorf("anna's inbox: %+v", in)
	}
	if bobIn, _ = c.PartnerInbox(bob); len(bobIn.Held) != 0 {
		t.Errorf("bob sees a password he doesn't hold: %+v", bobIn.Held)
	}
	ps, _ := c.Partners(me)
	if len(ps) != 2 || !slices.Equal(ps[0].Holds, []string{"lock:work"}) {
		t.Errorf("partners: %+v", ps)
	}

	if _, err := c.RequestUnlock(me, "lock:other", "please"); err == nil {
		t.Error("a request for what nobody holds")
	}
	reqs, err := c.RequestUnlock(me, "lock:work", "the exam is over")
	if err != nil || len(reqs) != 1 || reqs[0].Partner != "anna@example.org" {
		t.Fatalf("request: %+v %v", reqs, err)
	}
	if err := c.AnswerRequest(bob, reqs[0].ID, true); err == nil {
		t.Error("bob approved a request anna holds")
	}
	if st, _ := c.Requests(me); len(st) != 1 || st[0].Status != "pending" || st[0].Secret != "" {
		t.Errorf("before the answer: %+v", st)
	}
	in, _ = c.PartnerInbox(anna)
	if len(in.Requests) != 1 || in.Requests[0].Reason != "the exam is over" {
		t.Fatalf("anna's requests: %+v", in.Requests)
	}
	if err := c.AnswerRequest(anna, reqs[0].ID, true); err != nil {
		t.Fatal(err)
	}
	st, _ := c.Requests(me)
	if len(st) != 1 || st[0].Status != "approved" || st[0].Secret != "s3cret-pw" {
		t.Fatalf("after approval: %+v", st)
	}
	if err := c.UsedRequest(me, st[0].ID); err != nil {
		t.Fatal(err)
	}
	if st, _ = c.Requests(me); st[0].Status != "used" || st[0].Secret != "" {
		t.Errorf("a used request keeps its secret: %+v", st)
	}

	if err := c.RemovePartner(me, pa.ID); err != nil {
		t.Fatal(err)
	}
	if in, _ = c.PartnerInbox(anna); len(in.Held) != 0 || len(in.Watching) != 0 {
		t.Errorf("a removed partner still holds or watches: %+v", in)
	}

	// The queue: accounts can't take it; the sender delivers it once.
	if _, err := cloud.DeliverOutbox(c, &fakeMailer{}, 100); err == nil {
		t.Error("the anon key read the mail queue")
	}
	sender := cloud.New(f.URL, cloudtest.ServiceKey)
	m := &fakeMailer{fail: "bob@example.org"}
	n, err := cloud.DeliverOutbox(sender, m, 100)
	if err == nil || !strings.Contains(err.Error(), "bob@example.org") {
		t.Errorf("a bounce isn't said: %v", err)
	}
	to := map[string]int{}
	for _, s := range m.sent {
		to[s.to]++
	}
	// anna: invite, alert, unlock request, removed; bob's two bounced.
	if n != 4 || to["anna@example.org"] != 4 {
		t.Errorf("sent %d: %+v", n, m.sent)
	}
	m.fail = ""
	m.sent = nil
	if n, err := cloud.DeliverOutbox(sender, m, 100); err != nil || n != 2 {
		t.Errorf("the bounced ones again: %d %v %+v", n, err, m.sent)
	}
	if n, _ := cloud.DeliverOutbox(sender, m, 100); n != 0 {
		t.Errorf("sent twice: %d", n)
	}
}
