package cloud_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
)

// 2.13: accepting an invitation changes only partner_id and status, as
// cloud/supabase/schema.sql's keep_partner_row enforces: an account that
// rewrites the row's owner while accepting doesn't become that owner's
// partner (it would read their held passwords).
func TestAcceptCantRewriteOwner(t *testing.T) {
	f := cloudtest.New()
	defer f.Close()
	c := cloud.New(f.URL, cloudtest.AnonKey)
	me, _ := c.SignUp("me@example.org", "secret123")
	bob, _ := c.SignUp("bob@example.org", "secret123")
	eve, _ := c.SignUp("eve@example.org", "secret123")
	if _, err := c.InvitePartner(me, "anna@example.org"); err != nil {
		t.Fatal(err)
	}
	p, err := c.InvitePartner(bob, "eve@example.org")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"status": cloud.PartnerActive, "partner_id": eve.UserID,
		"owner_id": me.UserID, "owner_email": "me@example.org"})
	req, _ := http.NewRequest("PATCH", f.URL+"/rest/v1/partners?id=eq."+p.ID, bytes.NewReader(body))
	req.Header.Set("apikey", cloudtest.AnonKey)
	req.Header.Set("Authorization", "Bearer "+eve.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode < 400 {
		t.Errorf("a rewriting accept was taken: %v %v", resp, err)
	}
	if ps, _ := c.Partners(me); len(ps) != 1 || ps[0].PartnerEmail != "anna@example.org" || ps[0].Status == cloud.PartnerActive {
		t.Errorf("me's partners after eve's try: %+v", ps)
	}
	if err := c.AcceptPartner(eve, p.ID); err != nil {
		t.Errorf("eve can't accept bob's invitation as it is: %v", err)
	}
}
