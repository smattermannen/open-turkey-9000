// Package cloudtest is a fake of the parts of Supabase KC9K uses (Auth,
// Storage, the setups table), in memory, enforcing what row-level security
// does: an account reads and writes only its own setup.
package cloudtest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/smattermannen/kc9k/internal/cloud"
)

const AnonKey = "anon-test-key"

// Server is a running fake.
type Server struct {
	*httptest.Server
	mu       sync.Mutex
	users    map[string]user   // by email
	access   map[string]string // access token -> user id
	refresh  map[string]string // refresh token -> user id
	objects  map[string][]byte // object path -> bytes
	rows     map[string]json.RawMessage
	Requests []string // "METHOD path" of each request
	// ExpiresIn is the life of new access tokens (seconds).
	ExpiresIn int64

	// The accountability partner's tables (2.13), with the same row-level
	// security as cloud/supabase/schema.sql.
	partners   []cloud.Partner
	messages   []cloud.Message
	requests   []cloud.UnlockRequest
	Heartbeats map[string]string // "<owner id> <device>" -> seen_at
	// Mailer sends what the cloud emails: each invitation to the partner,
	// each message to the user's active partners (the provider in the
	// real cloud). Default: recorded in Mail.
	Mailer cloud.Mailer
	Mail   []cloud.Mail
	nextID int64
}

// mailRecorder is the default Mailer.
type mailRecorder struct{ s *Server }

func (m mailRecorder) Send(mail cloud.Mail) error {
	m.s.Mail = append(m.s.Mail, mail)
	return nil
}

// Sent returns the emails sent so far (Mail, read under the lock).
func (s *Server) Sent() []cloud.Mail {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]cloud.Mail(nil), s.Mail...)
}

type user struct{ id, email, password string }

// New starts a fake; Close it when done.
func New() *Server {
	s := &Server{users: map[string]user{}, access: map[string]string{}, refresh: map[string]string{},
		objects: map[string][]byte{}, rows: map[string]json.RawMessage{}, ExpiresIn: 3600,
		Heartbeats: map[string]string{}}
	s.Mailer = mailRecorder{s}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func token() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func (s *Server) session(w http.ResponseWriter, u user) {
	a, r := token(), token()
	s.access[a], s.refresh[r] = u.id, u.id
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": a, "refresh_token": r, "expires_in": s.ExpiresIn,
		"user": map[string]string{"id": u.id, "email": u.email}})
}

// Expire makes every access token stale (a refresh is then needed).
func (s *Server) Expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.access = map[string]string{}
}

// Object returns what is stored for a user id.
func (s *Server) Object(userID string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.objects["setups/"+userID+"/setup.json.gz"]
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Requests = append(s.Requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("apikey") != AnonKey {
		fail(w, 401, "Invalid API key")
		return
	}
	var body map[string]string
	if strings.HasPrefix(r.URL.Path, "/auth/") && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	switch {
	case r.Method == "POST" && r.URL.Path == "/auth/v1/signup":
		if _, ok := s.users[body["email"]]; ok {
			fail(w, 422, "User already registered")
			return
		}
		if len(body["password"]) < 6 {
			fail(w, 422, "Password should be at least 6 characters.")
			return
		}
		u := user{id: token()[:8] + "-uid", email: body["email"], password: body["password"]}
		s.users[u.email] = u
		s.session(w, u)
		return
	case r.Method == "POST" && r.URL.Path == "/auth/v1/token":
		switch r.URL.Query().Get("grant_type") {
		case "password":
			u, ok := s.users[body["email"]]
			if !ok || u.password != body["password"] {
				fail(w, 400, "Invalid login credentials")
				return
			}
			s.session(w, u)
		case "refresh_token":
			id, ok := s.refresh[body["refresh_token"]]
			if !ok {
				fail(w, 400, "Invalid Refresh Token")
				return
			}
			delete(s.refresh, body["refresh_token"])
			for _, u := range s.users {
				if u.id == id {
					s.session(w, u)
					return
				}
			}
		}
		return
	}
	uid, ok := s.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	if !ok {
		fail(w, 401, "JWT expired")
		return
	}
	switch {
	case r.Method == "POST" && r.URL.Path == "/auth/v1/logout":
		delete(s.access, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		w.WriteHeader(204)
	case strings.HasPrefix(r.URL.Path, "/storage/v1/object/"):
		name := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/storage/v1/object/"), "authenticated/")
		parts := strings.Split(name, "/")
		if len(parts) != 3 || parts[0] != "setups" || parts[1] != uid {
			fail(w, 403, "new row violates row-level security policy")
			return
		}
		if r.Method == "POST" {
			if _, exists := s.objects[name]; exists && r.Header.Get("X-Upsert") != "true" {
				fail(w, 409, "The resource already exists")
				return
			}
			data, _ := io.ReadAll(r.Body)
			s.objects[name] = data
			_, _ = w.Write([]byte(`{"Key":"` + name + `"}`))
			return
		}
		data, ok := s.objects[name]
		if !ok {
			fail(w, 400, "Object not found")
			return
		}
		_, _ = w.Write(data)
	case r.URL.Path == "/rest/v1/setups":
		if r.Method == "POST" {
			var row map[string]any
			raw, _ := io.ReadAll(r.Body)
			if json.Unmarshal(raw, &row) != nil || row["user_id"] != uid {
				fail(w, 403, "new row violates row-level security policy")
				return
			}
			s.rows[uid] = raw
			w.WriteHeader(201)
			return
		}
		if r.URL.Query().Get("user_id") != "eq."+uid {
			_, _ = w.Write([]byte("[]")) // others' rows are invisible
			return
		}
		if row, ok := s.rows[uid]; ok {
			_, _ = w.Write([]byte("[" + string(row) + "]"))
			return
		}
		_, _ = w.Write([]byte("[]"))
	case strings.HasPrefix(r.URL.Path, "/rest/v1/partners"), strings.HasPrefix(r.URL.Path, "/rest/v1/partner_messages"),
		strings.HasPrefix(r.URL.Path, "/rest/v1/unlock_requests"), strings.HasPrefix(r.URL.Path, "/rest/v1/heartbeats"):
		s.partnerTables(w, r, uid)
	default:
		fail(w, 404, "not found")
	}
}

// emailOf is a user id's email.
func (s *Server) emailOf(uid string) string {
	for _, u := range s.users {
		if u.id == uid {
			return u.email
		}
	}
	return ""
}

// partnerOf reports whether uid is an active partner of owner.
func (s *Server) partnerOf(uid, owner string) bool {
	for _, p := range s.partners {
		if p.OwnerID == owner && p.PartnerID == uid && p.Status == cloud.PartnerActive {
			return true
		}
	}
	return false
}

func newRows(r *http.Request, one any, many any) bool {
	raw, _ := io.ReadAll(r.Body)
	if len(raw) > 0 && raw[0] == '[' {
		return json.Unmarshal(raw, many) == nil
	}
	return json.Unmarshal(raw, one) == nil
}

// partnerTables serves the partners, partner_messages, unlock_requests and
// heartbeats tables as PostgREST does, with schema.sql's row-level
// security.
func (s *Server) partnerTables(w http.ResponseWriter, r *http.Request, uid string) {
	email := strings.ToLower(s.emailOf(uid))
	table := strings.TrimPrefix(r.URL.Path, "/rest/v1/")
	id := strings.TrimPrefix(r.URL.Query().Get("id"), "eq.")
	denied := func() { fail(w, 403, "new row violates row-level security policy") }
	reply := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	s.nextID++
	newID := hex.EncodeToString([]byte{byte(s.nextID >> 8), byte(s.nextID)}) + "-" + token()[:8]
	switch table + " " + r.Method {
	case "partners POST":
		var p cloud.Partner
		if !newRows(r, &p, &[]cloud.Partner{}) || p.OwnerID != uid || p.Status != cloud.PartnerInvited || p.PartnerID != "" {
			denied()
			return
		}
		for _, q := range s.partners {
			if q.OwnerID == uid && strings.EqualFold(q.PartnerEmail, p.PartnerEmail) {
				fail(w, 409, "duplicate key value violates unique constraint \"partners_owner_id_partner_email_key\"")
				return
			}
		}
		p.ID = newID
		s.partners = append(s.partners, p)
		_ = s.Mailer.Send(cloud.Mail{To: p.PartnerEmail, Subject: p.OwnerEmail + " asks you to be their accountability partner",
			Body: "Sign in to Killer Chicken 9000 with this email and accept: kc9k partner accept " + p.OwnerEmail})
		w.WriteHeader(201)
		reply([]cloud.Partner{p})
	case "partners GET":
		var out []cloud.Partner
		for _, p := range s.partners {
			if p.OwnerID == uid || strings.EqualFold(p.PartnerEmail, email) {
				out = append(out, p)
			}
		}
		reply(append([]cloud.Partner{}, out...))
	case "partners PATCH":
		var change map[string]string
		_ = json.NewDecoder(r.Body).Decode(&change)
		for i, p := range s.partners {
			if p.ID != id {
				continue
			}
			// Only the invited partner accepts, as themselves, changing
			// nothing else (schema.sql's keep_partner_row).
			if !strings.EqualFold(p.PartnerEmail, email) || change["partner_id"] != uid || change["status"] != cloud.PartnerActive ||
				len(change) != 2 {
				denied()
				return
			}
			s.partners[i].Status, s.partners[i].PartnerID = cloud.PartnerActive, uid
		}
		w.WriteHeader(204)
	case "partners DELETE":
		s.partners = slicesDelete(s.partners, func(p cloud.Partner) bool {
			return p.ID == id && (p.OwnerID == uid || strings.EqualFold(p.PartnerEmail, email))
		})
		w.WriteHeader(204)
	case "partner_messages POST":
		var one cloud.Message
		var many []cloud.Message
		if !newRows(r, &one, &many) {
			fail(w, 400, "bad JSON")
			return
		}
		if many == nil {
			many = []cloud.Message{one}
		}
		for _, m := range many {
			if m.OwnerID != uid {
				denied()
				return
			}
		}
		for _, m := range many {
			s.nextID++
			m.ID = s.nextID
			s.messages = append(s.messages, m)
			for _, p := range s.partners {
				if p.OwnerID == uid && p.Status == cloud.PartnerActive {
					body := m.Body
					if m.Secret != "" {
						body += "\n\nPassword: " + m.Secret
					}
					_ = s.Mailer.Send(cloud.Mail{To: p.PartnerEmail, Subject: m.Subject, Body: body})
				}
			}
		}
		w.WriteHeader(201)
		reply(many)
	case "partner_messages GET":
		var out []cloud.Message
		for i := len(s.messages) - 1; i >= 0; i-- {
			m := s.messages[i]
			if (m.OwnerID == uid && m.Kind != cloud.MessagePassword) || s.partnerOf(uid, m.OwnerID) {
				out = append(out, m)
			}
		}
		reply(append([]cloud.Message{}, out...))
	case "unlock_requests POST":
		var q cloud.UnlockRequest
		if !newRows(r, &q, &[]cloud.UnlockRequest{}) || q.OwnerID != uid || q.Status != cloud.RequestPending || q.Secret != "" {
			denied()
			return
		}
		q.ID = newID
		s.requests = append(s.requests, q)
		for _, p := range s.partners {
			if p.OwnerID == uid && p.Status == cloud.PartnerActive {
				_ = s.Mailer.Send(cloud.Mail{To: p.PartnerEmail, Subject: q.OwnerEmail + " asks to unlock " + q.Target,
					Body: q.Reason + "\n\nkc9k partner approve " + q.ID + "   or   kc9k partner deny " + q.ID})
			}
		}
		w.WriteHeader(201)
		reply([]cloud.UnlockRequest{q})
	case "unlock_requests GET":
		var out []cloud.UnlockRequest
		for i := len(s.requests) - 1; i >= 0; i-- {
			q := s.requests[i]
			if q.OwnerID == uid || s.partnerOf(uid, q.OwnerID) {
				out = append(out, q)
			}
		}
		reply(append([]cloud.UnlockRequest{}, out...))
	case "unlock_requests PATCH":
		var change map[string]any
		_ = json.NewDecoder(r.Body).Decode(&change)
		for i, q := range s.requests {
			if q.ID != id {
				continue
			}
			status, _ := change["status"].(string)
			for k := range change { // only the decision changes (schema.sql's keep_request_row)
				if k != "status" && k != "secret" && k != "decided_by" && k != "decided_at" {
					denied()
					return
				}
			}
			switch {
			case s.partnerOf(uid, q.OwnerID) && q.Status == cloud.RequestPending &&
				(status == cloud.RequestApproved || status == cloud.RequestDenied):
				s.requests[i].Status = status
				s.requests[i].Secret, _ = change["secret"].(string)
				s.requests[i].DecidedBy, _ = change["decided_by"].(string)
			case q.OwnerID == uid && q.Status == cloud.RequestApproved && status == cloud.RequestUsed:
				s.requests[i].Status, s.requests[i].Secret = cloud.RequestUsed, ""
			default:
				denied()
				return
			}
		}
		w.WriteHeader(204)
	case "heartbeats POST":
		var hb map[string]any
		_ = json.NewDecoder(r.Body).Decode(&hb)
		if hb["owner_id"] != uid {
			denied()
			return
		}
		device, _ := hb["device"].(string)
		seen, _ := hb["seen_at"].(string)
		s.Heartbeats[uid+" "+device] = seen
		w.WriteHeader(201)
	default:
		fail(w, 405, "method not allowed")
	}
}

func slicesDelete[T any](in []T, del func(T) bool) []T {
	out := in[:0]
	for _, v := range in {
		if !del(v) {
			out = append(out, v)
		}
	}
	return out
}
