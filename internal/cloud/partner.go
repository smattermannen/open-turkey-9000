// partner.go — the accountability partner (SPEC §4.15, ROADMAP 2.13) in
// KC9K's cloud: a user invites partners by email; a partner, signed in to
// their own KC9K account, accepts; the user's devices send reports, alerts
// and held passwords as messages to every active partner; the user asks
// for unlocks, a partner approves or denies them. Row-level security
// (cloud/supabase/schema.sql) lets a user see their own partners, requests
// and messages but never a held password, and a partner see only what the
// users they partner sent.
//
// Email: the cloud sends each new message and invitation by email (a
// database webhook to an email provider, set up by the owner of KC9K's
// Supabase project: ROADMAP 2.13). Mailer is that sender's interface;
// cloudtest's fake records what it would send.
package cloud

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Partner states.
const (
	PartnerInvited = "invited"
	PartnerActive  = "active"
)

// Message kinds.
const (
	MessageReport   = "report"
	MessageAlert    = "alert"
	MessagePassword = "password" // a password a partner holds (Secret); its sender can't read it back
)

// Unlock request states.
const (
	RequestPending  = "pending"
	RequestApproved = "approved"
	RequestDenied   = "denied"
	RequestUsed     = "used"
)

// Partner is one partnership: OwnerID's KC9K shared with PartnerEmail.
type Partner struct {
	ID           string    `json:"id,omitempty"`
	OwnerID      string    `json:"owner_id"`
	OwnerEmail   string    `json:"owner_email"`
	PartnerEmail string    `json:"partner_email"`
	PartnerID    string    `json:"partner_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// Message is a report, an alert or a held password sent to a user's
// partners.
type Message struct {
	ID         int64     `json:"id,omitempty"`
	OwnerID    string    `json:"owner_id"`
	OwnerEmail string    `json:"owner_email"`
	Kind       string    `json:"kind"`
	Target     string    `json:"target,omitempty"` // a held password's lock: "block:<name>", "settings", "setting:<name>"
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Secret     string    `json:"secret,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// UnlockRequest is a user asking their partners to lift a lock whose
// password a partner holds. Approving hands the password over in Secret,
// for one use.
type UnlockRequest struct {
	ID         string     `json:"id,omitempty"`
	OwnerID    string     `json:"owner_id"`
	OwnerEmail string     `json:"owner_email"`
	Target     string     `json:"target"`
	Reason     string     `json:"reason,omitempty"`
	Status     string     `json:"status"`
	Secret     string     `json:"secret,omitempty"`
	DecidedBy  string     `json:"decided_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	DecidedAt  *time.Time `json:"decided_at,omitempty"`
}

// Mail is one email the cloud sends.
type Mail struct {
	To, Subject, Body string
}

// Mailer sends the cloud's email (the provider; a fake in tests).
type Mailer interface {
	Send(Mail) error
}

// rest sends one PostgREST request as s.
func (c *Client) rest(method, table, query string, s *Session, body any, out any) error {
	if s == nil {
		return ErrSignedOut
	}
	var header http.Header
	path := "/rest/v1/" + table
	if query != "" {
		path += "?" + query
	}
	if body != nil {
		r, h := jsonBody(body)
		header = h
		header.Set("Prefer", "return=representation")
		_, err := c.do(method, path, s, header, r, out)
		return err
	}
	_, err := c.do(method, path, s, header, nil, out)
	return err
}

// InvitePartner invites email to be s's accountability partner.
func (c *Client) InvitePartner(s *Session, email string) (Partner, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return Partner{}, fmt.Errorf("%q isn't an email address", email)
	}
	if strings.EqualFold(email, s.Email) {
		return Partner{}, fmt.Errorf("you can't be your own accountability partner")
	}
	var rows []Partner
	err := c.rest("POST", "partners", "", s, Partner{OwnerID: s.UserID, OwnerEmail: s.Email, PartnerEmail: email,
		Status: PartnerInvited, CreatedAt: c.Now().UTC()}, &rows)
	if err != nil {
		return Partner{}, err
	}
	if len(rows) == 0 {
		return Partner{}, fmt.Errorf("KC9K's cloud didn't keep the invitation")
	}
	return rows[0], nil
}

// Partners lists every partnership s is in: as the user (OwnerID is s) or
// as the partner (PartnerEmail is s's).
func (c *Client) Partners(s *Session) ([]Partner, error) {
	var rows []Partner
	err := c.rest("GET", "partners", "select=*&order=created_at", s, nil, &rows)
	return rows, err
}

// AcceptPartner makes s the active partner of an invitation sent to s.
func (c *Client) AcceptPartner(s *Session, id string) error {
	return c.rest("PATCH", "partners", "id=eq."+url.QueryEscape(id), s,
		map[string]string{"status": PartnerActive, "partner_id": s.UserID}, nil)
}

// RemovePartner ends a partnership (the user's, or the partner leaving).
func (c *Client) RemovePartner(s *Session, id string) error {
	return c.rest("DELETE", "partners", "id=eq."+url.QueryEscape(id), s, nil, nil)
}

// Send sends messages to s's partners.
func (c *Client) Send(s *Session, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	for i := range msgs {
		msgs[i].OwnerID, msgs[i].OwnerEmail = s.UserID, s.Email
		if msgs[i].CreatedAt.IsZero() {
			msgs[i].CreatedAt = c.Now().UTC()
		}
	}
	return c.rest("POST", "partner_messages", "", s, msgs, nil)
}

// Inbox lists the messages s can read, newest first: as a partner, what the
// users s partners sent (held passwords included); as a user, what s's own
// devices sent (never a held password).
func (c *Client) Inbox(s *Session) ([]Message, error) {
	var rows []Message
	err := c.rest("GET", "partner_messages", "select=*&order=created_at.desc", s, nil, &rows)
	return rows, err
}

// RequestUnlock asks s's partners to lift target's lock.
func (c *Client) RequestUnlock(s *Session, target, reason string) (UnlockRequest, error) {
	var rows []UnlockRequest
	err := c.rest("POST", "unlock_requests", "", s, UnlockRequest{OwnerID: s.UserID, OwnerEmail: s.Email,
		Target: target, Reason: reason, Status: RequestPending, CreatedAt: c.Now().UTC()}, &rows)
	if err != nil {
		return UnlockRequest{}, err
	}
	if len(rows) == 0 {
		return UnlockRequest{}, fmt.Errorf("KC9K's cloud didn't keep the request")
	}
	return rows[0], nil
}

// UnlockRequests lists the requests s made or may decide, newest first.
func (c *Client) UnlockRequests(s *Session) ([]UnlockRequest, error) {
	var rows []UnlockRequest
	err := c.rest("GET", "unlock_requests", "select=*&order=created_at.desc", s, nil, &rows)
	return rows, err
}

// DecideUnlock approves (handing over secret) or denies a request, as a
// partner of its user.
func (c *Client) DecideUnlock(s *Session, id string, approve bool, secret string) error {
	change := map[string]any{"status": RequestDenied, "decided_by": s.Email, "decided_at": c.Now().UTC()}
	if approve {
		change["status"], change["secret"] = RequestApproved, secret
	}
	return c.rest("PATCH", "unlock_requests", "id=eq."+url.QueryEscape(id), s, change, nil)
}

// UseUnlock marks an approved request used, its password gone from the
// cloud.
func (c *Client) UseUnlock(s *Session, id string) error {
	return c.rest("PATCH", "unlock_requests", "id=eq."+url.QueryEscape(id), s,
		map[string]string{"status": RequestUsed, "secret": ""}, nil)
}

// Heartbeat says device is running KC9K now: the cloud alerts the partners
// when a device the user has is not heard from (SPEC §4.15).
func (c *Client) Heartbeat(s *Session, device string) error {
	if s == nil {
		return ErrSignedOut
	}
	r, h := jsonBody(map[string]any{"owner_id": s.UserID, "device": device, "seen_at": c.Now().UTC()})
	h.Set("Prefer", "resolution=merge-duplicates")
	_, err := c.do("POST", "/rest/v1/heartbeats?on_conflict=owner_id,device", s, h, r, nil)
	return err
}
