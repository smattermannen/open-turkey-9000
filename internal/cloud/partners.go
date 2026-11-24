package cloud

// partners.go — the accountability partner (SPEC §4.15; ROADMAP 2.13).
// Everything goes through Postgres functions (POST /rest/v1/rpc/<name>,
// cloud/supabase/schema.sql): the tables are closed to direct access, and
// each function checks who calls. The user (owner) invites partners by
// email, sends them reports and alerts, gives them passwords to hold and
// asks them to approve unlocking; a partner, signed in to their own KC9K
// account with the invited address, accepts, reads their inbox, holds the
// passwords and approves or denies. Email goes out from the queue
// (DeliverOutbox), never from a device.

import (
	"errors"
	"time"
)

// Partner is one of the owner's partners.
type Partner struct {
	ID       string    `json:"id"`
	Email    string    `json:"email"`
	Accepted bool      `json:"accepted"`
	Reports  string    `json:"reports"` // "daily", "weekly" or "off"
	Alerts   bool      `json:"alerts"`
	Holds    []string  `json:"holds"` // what the partner holds a password for
	Created  time.Time `json:"created_at"`
}

// Message is something sent to a partner: an invitation, a report, an
// alert, an unlock request or its answer.
type Message struct {
	ID      int64     `json:"id"`
	From    string    `json:"from"` // the owner's email
	Kind    string    `json:"kind"`
	Subject string    `json:"subject"`
	Body    string    `json:"body"`
	At      time.Time `json:"created_at"`
}

// Message kinds.
const (
	KindInvite  = "invite"
	KindReport  = "report"
	KindAlert   = "alert"
	KindRequest = "unlock-request"
	KindRemoved = "removed"
)

// Held is a password a partner holds for someone (partner's side).
type Held struct {
	From   string `json:"from"` // the owner's email
	What   string `json:"what"` // "settings-password" or "lock:<block>"
	Secret string `json:"secret"`
}

// Request is an unlock request: the owner's side sees its status (and the
// secret once approved), the partner's side the requests to answer.
type Request struct {
	ID       int64     `json:"id"`
	From     string    `json:"from,omitempty"`    // partner's side: the owner's email
	Partner  string    `json:"partner,omitempty"` // owner's side: the partner's email
	What     string    `json:"what"`
	Reason   string    `json:"reason"`
	Status   string    `json:"status"` // "pending", "approved", "denied", "used"
	Secret   string    `json:"secret,omitempty"`
	At       time.Time `json:"created_at"`
	Answered time.Time `json:"answered_at,omitempty"`
}

// Watched is someone the signed-in account is a partner for, with when
// their KC9K was last heard from.
type Watched struct {
	Email    string    `json:"email"`
	LastSeen time.Time `json:"last_seen,omitempty"`
	Device   string    `json:"device,omitempty"`
}

// Invitation is an invitation waiting for the signed-in account.
type Invitation struct {
	ID   string `json:"id"`
	From string `json:"from"`
}

// Inbox is the partner's side of everything.
type Inbox struct {
	Invitations []Invitation `json:"invitations"`
	Messages    []Message    `json:"messages"`
	Held        []Held       `json:"held"`
	Requests    []Request    `json:"requests"`
	Watching    []Watched    `json:"watching"`
}

// ErrNotFound is a partner, invitation or request the account doesn't have.
var ErrNotFound = errors.New("not found")

func (c *Client) rpc(s *Session, name string, args map[string]any, out any) error {
	if args == nil {
		args = map[string]any{}
	}
	body, h := jsonBody(args)
	_, err := c.do("POST", "/rest/v1/rpc/"+name, s, h, body, out)
	var ce *Error
	if errors.As(err, &ce) && ce.Status == 404 {
		return ErrNotFound
	}
	return err
}

// InvitePartner invites email as a partner (an invitation email is queued).
func (c *Client) InvitePartner(s *Session, email, reports string, alerts bool) (Partner, error) {
	var p Partner
	err := c.rpc(s, "partner_invite", map[string]any{"p_email": email, "p_reports": reports, "p_alerts": alerts}, &p)
	return p, err
}

// Partners lists the account's partners.
func (c *Client) Partners(s *Session) ([]Partner, error) {
	var out []Partner
	err := c.rpc(s, "partner_list", nil, &out)
	return out, err
}

// SetPartner changes what a partner gets.
func (c *Client) SetPartner(s *Session, id, reports string, alerts bool) error {
	return c.rpc(s, "partner_set", map[string]any{"p_id": id, "p_reports": reports, "p_alerts": alerts}, nil)
}

// RemovePartner removes a partner (they are told), with what they hold.
func (c *Client) RemovePartner(s *Session, id string) error {
	return c.rpc(s, "partner_remove", map[string]any{"p_id": id}, nil)
}

// SendToPartners queues a report or an alert for every accepted partner
// who gets that kind (a report only to those with that period); it
// returns how many it went to.
func (c *Client) SendToPartners(s *Session, kind, period, subject, body string) (int, error) {
	var n int
	err := c.rpc(s, "partner_send", map[string]any{"p_kind": kind, "p_period": period, "p_subject": subject, "p_body": body}, &n)
	return n, err
}

// Hold gives a partner a password to hold (replacing what they held for
// the same thing).
func (c *Client) Hold(s *Session, partnerID, what, secret string) error {
	return c.rpc(s, "partner_hold", map[string]any{"p_id": partnerID, "p_what": what, "p_secret": secret}, nil)
}

// Release takes a held password back from every partner (the thing it
// guarded is gone or unlocked).
func (c *Client) Release(s *Session, what string) error {
	return c.rpc(s, "partner_release", map[string]any{"p_what": what}, nil)
}

// RequestUnlock asks the partners holding what's password to approve.
func (c *Client) RequestUnlock(s *Session, what, reason string) ([]Request, error) {
	var out []Request
	err := c.rpc(s, "unlock_request", map[string]any{"p_what": what, "p_reason": reason}, &out)
	return out, err
}

// Requests lists the account's unlock requests, with the secret of an
// approved one.
func (c *Client) Requests(s *Session) ([]Request, error) {
	var out []Request
	err := c.rpc(s, "unlock_status", nil, &out)
	return out, err
}

// UsedRequest marks an approved request used (its secret is then gone).
func (c *Client) UsedRequest(s *Session, id int64) error {
	return c.rpc(s, "unlock_used", map[string]any{"p_id": id}, nil)
}

// Heartbeat says this device's KC9K is running.
func (c *Client) Heartbeat(s *Session, device string) error {
	return c.rpc(s, "heartbeat", map[string]any{"p_device": device}, nil)
}

// PartnerInbox is the partner's side: invitations, messages, held
// passwords, requests to answer, who they watch.
func (c *Client) PartnerInbox(s *Session) (Inbox, error) {
	var in Inbox
	err := c.rpc(s, "partner_inbox", nil, &in)
	return in, err
}

// AcceptInvitation accepts an invitation sent to the account's address.
func (c *Client) AcceptInvitation(s *Session, id string) error {
	return c.rpc(s, "partner_accept", map[string]any{"p_id": id}, nil)
}

// AnswerRequest approves or denies an unlock request the partner holds
// the password for.
func (c *Client) AnswerRequest(s *Session, id int64, approve bool) error {
	return c.rpc(s, "unlock_answer", map[string]any{"p_id": id, "p_approve": approve}, nil)
}
