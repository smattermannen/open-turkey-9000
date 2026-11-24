package cloudtest

// partners.go — the fake's accountability partner functions (SPEC §4.15),
// as cloud/supabase/schema.sql defines them: who may call each and what it
// changes. Called with s.mu held.

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

type fakePartner struct {
	id, owner, email, partnerUID, reports string
	accepted, alerts, removed             bool
	created                               time.Time
}

type fakeMessage struct {
	id                      int64
	partner                 string // partner id
	to, kind, subject, body string
	at                      time.Time
	emailed                 bool
}

type fakeRequest struct {
	id                            int64
	partner, what, reason, status string
	secret                        string
	at, answered                  time.Time
}

type partnerStore struct {
	partners   []*fakePartner
	held       map[[2]string]string // {partner id, what} -> secret
	messages   []*fakeMessage
	requests   []*fakeRequest
	heartbeats map[string][2]string // owner uid -> {device, RFC 3339 time}
	nextID     int64
}

func newPartnerStore() partnerStore {
	return partnerStore{held: map[[2]string]string{}, heartbeats: map[string][2]string{}}
}

// Mail is what the fake has queued as email, to whom and what.
type Mail struct{ To, Kind, Subject, Body string }

// Queued returns every message queued so far (emailed or not).
func (s *Server) Queued() []Mail {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Mail
	for _, m := range s.messages {
		out = append(out, Mail{m.to, m.kind, m.subject, m.body})
	}
	return out
}

func (s *Server) emailOf(uid string) string {
	for _, u := range s.users {
		if u.id == uid {
			return u.email
		}
	}
	return ""
}

func (s *Server) partnerByID(id string) *fakePartner {
	for _, p := range s.partners {
		if p.id == id && !p.removed {
			return p
		}
	}
	return nil
}

func (s *Server) queue(p *fakePartner, kind, subject, body string) {
	s.nextID++
	s.messages = append(s.messages, &fakeMessage{id: s.nextID, partner: p.id, to: p.email, kind: kind,
		subject: subject, body: body, at: time.Now().UTC()})
}

func (s *Server) holds(p *fakePartner) []string {
	out := []string{}
	for k := range s.held {
		if k[0] == p.id {
			out = append(out, k[1])
		}
	}
	slices.Sort(out)
	return out
}

func answer(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) rpc(w http.ResponseWriter, r *http.Request, uid string) {
	var a map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &a)
	str := func(k string) string { v, _ := a[k].(string); return v }
	num := func(k string) int64 { v, _ := a[k].(float64); return int64(v) }
	flag := func(k string) bool { v, _ := a[k].(bool); return v }
	me := s.emailOf(uid)
	own := func() *fakePartner {
		p := s.partnerByID(str("p_id"))
		if p == nil || p.owner != uid {
			return nil
		}
		return p
	}
	partnerJSON := func(p *fakePartner) map[string]any {
		return map[string]any{"id": p.id, "email": p.email, "accepted": p.accepted, "reports": p.reports,
			"alerts": p.alerts, "holds": s.holds(p), "created_at": p.created}
	}
	switch strings.TrimPrefix(r.URL.Path, "/rest/v1/rpc/") {
	case "partner_invite":
		email := strings.ToLower(strings.TrimSpace(str("p_email")))
		if !strings.Contains(email, "@") || email == strings.ToLower(me) {
			fail(w, 400, "a partner is someone else's email address")
			return
		}
		if !slices.Contains([]string{"daily", "weekly", "off"}, str("p_reports")) {
			fail(w, 400, "reports are daily, weekly or off")
			return
		}
		for _, p := range s.partners {
			if p.owner == uid && p.email == email && !p.removed {
				fail(w, 409, "already a partner")
				return
			}
		}
		p := &fakePartner{id: token()[:12], owner: uid, email: email, reports: str("p_reports"), alerts: flag("p_alerts"),
			created: time.Now().UTC()}
		s.partners = append(s.partners, p)
		s.queue(p, "invite", me+" asks you to be their accountability partner",
			me+" uses Killer Chicken 9000 and asks you to be their accountability partner. Sign in to KC9K with this address to accept.")
		answer(w, partnerJSON(p))
	case "partner_list":
		out := []map[string]any{}
		for _, p := range s.partners {
			if p.owner == uid && !p.removed {
				out = append(out, partnerJSON(p))
			}
		}
		answer(w, out)
	case "partner_set":
		p := own()
		if p == nil {
			fail(w, 404, "no such partner")
			return
		}
		p.reports, p.alerts = str("p_reports"), flag("p_alerts")
		w.WriteHeader(204)
	case "partner_remove":
		p := own()
		if p == nil {
			fail(w, 404, "no such partner")
			return
		}
		p.removed = true
		for k := range s.held {
			if k[0] == p.id {
				delete(s.held, k)
			}
		}
		for _, q := range s.requests {
			if q.partner == p.id && q.status == "pending" {
				q.status = "denied"
			}
		}
		s.queue(p, "removed", me+" removed you as their accountability partner", "")
		w.WriteHeader(204)
	case "partner_send":
		kind := str("p_kind")
		if kind != "report" && kind != "alert" {
			fail(w, 400, "only reports and alerts")
			return
		}
		n := 0
		for _, p := range s.partners {
			if p.owner != uid || p.removed || !p.accepted || kind == "alert" && !p.alerts ||
				kind == "report" && p.reports != str("p_period") {
				continue
			}
			s.queue(p, kind, str("p_subject"), str("p_body"))
			n++
		}
		answer(w, n)
	case "partner_hold":
		p := own()
		if p == nil || !p.accepted {
			fail(w, 404, "no such partner, or not accepted yet")
			return
		}
		for k := range s.held {
			if k[1] == str("p_what") && s.partnerByID(k[0]) != nil && s.partnerByID(k[0]).owner == uid {
				delete(s.held, k) // one holder per thing
			}
		}
		s.held[[2]string{p.id, str("p_what")}] = str("p_secret")
		w.WriteHeader(204)
	case "partner_release":
		for k := range s.held {
			if p := s.partnerByID(k[0]); p != nil && p.owner == uid && k[1] == str("p_what") {
				delete(s.held, k)
			}
		}
		w.WriteHeader(204)
	case "unlock_request":
		out := []map[string]any{}
		for k := range s.held {
			p := s.partnerByID(k[0])
			if p == nil || p.owner != uid || k[1] != str("p_what") {
				continue
			}
			s.nextID++
			q := &fakeRequest{id: s.nextID, partner: p.id, what: k[1], reason: str("p_reason"), status: "pending", at: time.Now().UTC()}
			s.requests = append(s.requests, q)
			s.queue(p, "unlock-request", me+" asks you to approve unlocking "+k[1], q.reason)
			out = append(out, map[string]any{"id": q.id, "partner": p.email, "what": q.what, "reason": q.reason,
				"status": q.status, "created_at": q.at})
		}
		if len(out) == 0 {
			fail(w, 404, "no partner holds that password")
			return
		}
		answer(w, out)
	case "unlock_status":
		out := []map[string]any{}
		for _, q := range s.requests {
			p := s.partnerByID(q.partner)
			if p == nil || p.owner != uid {
				continue
			}
			row := map[string]any{"id": q.id, "partner": p.email, "what": q.what, "reason": q.reason,
				"status": q.status, "created_at": q.at}
			if q.status == "approved" {
				row["secret"], row["answered_at"] = q.secret, q.answered
			}
			out = append(out, row)
		}
		answer(w, out)
	case "unlock_used":
		for _, q := range s.requests {
			if p := s.partnerByID(q.partner); p != nil && p.owner == uid && q.id == num("p_id") {
				q.status, q.secret = "used", ""
			}
		}
		w.WriteHeader(204)
	case "heartbeat":
		s.heartbeats[uid] = [2]string{str("p_device"), time.Now().UTC().Format(time.RFC3339)}
		w.WriteHeader(204)
	case "partner_accept":
		p := s.partnerByID(str("p_id"))
		if p == nil || p.email != strings.ToLower(me) || p.owner == uid {
			fail(w, 404, "no such invitation")
			return
		}
		p.accepted, p.partnerUID = true, uid
		w.WriteHeader(204)
	case "partner_inbox":
		in := map[string]any{"invitations": []any{}, "messages": []any{}, "held": []any{}, "requests": []any{}, "watching": []any{}}
		for _, p := range s.partners {
			if p.removed {
				continue
			}
			from := s.emailOf(p.owner)
			if !p.accepted && p.email == strings.ToLower(me) {
				in["invitations"] = append(in["invitations"].([]any), map[string]any{"id": p.id, "from": from})
			}
			if !p.accepted || p.partnerUID != uid {
				continue
			}
			watched := map[string]any{"email": from}
			if hb, ok := s.heartbeats[p.owner]; ok {
				watched["device"], watched["last_seen"] = hb[0], hb[1]
			}
			in["watching"] = append(in["watching"].([]any), watched)
			for k, secret := range s.held {
				if k[0] == p.id {
					in["held"] = append(in["held"].([]any), map[string]any{"from": from, "what": k[1], "secret": secret})
				}
			}
			for _, m := range s.messages {
				if m.partner == p.id && m.kind != "invite" {
					in["messages"] = append(in["messages"].([]any), map[string]any{"id": m.id, "from": from, "kind": m.kind,
						"subject": m.subject, "body": m.body, "created_at": m.at})
				}
			}
			for _, q := range s.requests {
				if q.partner == p.id && q.status == "pending" {
					in["requests"] = append(in["requests"].([]any), map[string]any{"id": q.id, "from": from, "what": q.what,
						"reason": q.reason, "status": q.status, "created_at": q.at})
				}
			}
		}
		answer(w, in)
	case "unlock_answer":
		for _, q := range s.requests {
			p := s.partnerByID(q.partner)
			if q.id != num("p_id") || p == nil || p.partnerUID != uid || q.status != "pending" {
				continue
			}
			q.answered = time.Now().UTC()
			q.status = "denied"
			if flag("p_approve") {
				q.status, q.secret = "approved", s.held[[2]string{p.id, q.what}]
			}
			w.WriteHeader(204)
			return
		}
		fail(w, 404, "no such request")
	case "outbox_take":
		out := []map[string]any{}
		for _, m := range s.messages {
			if !m.emailed && len(out) < int(num("p_limit")) {
				out = append(out, map[string]any{"id": m.id, "to": m.to, "subject": m.subject, "body": m.body})
			}
		}
		answer(w, out)
	case "outbox_done":
		ids, _ := a["p_ids"].([]any)
		for _, m := range s.messages {
			for _, id := range ids {
				if f, _ := id.(float64); int64(f) == m.id {
					m.emailed = true
				}
			}
		}
		w.WriteHeader(204)
	default:
		fail(w, 404, "no such function")
	}
}
