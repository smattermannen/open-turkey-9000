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
)

const AnonKey = "anon-test-key"

// ServiceKey stands for the project's service role key (the mail sender's).
const ServiceKey = "service-test-key"

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
	partnerStore
}

type user struct{ id, email, password string }

// New starts a fake; Close it when done.
func New() *Server {
	s := &Server{users: map[string]user{}, access: map[string]string{}, refresh: map[string]string{},
		objects: map[string][]byte{}, rows: map[string]json.RawMessage{}, ExpiresIn: 3600,
		partnerStore: newPartnerStore()}
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
	if k := r.Header.Get("apikey"); k != AnonKey && k != ServiceKey {
		fail(w, 401, "Invalid API key")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/rest/v1/rpc/outbox_") {
		// The mail sender's functions: the service role key only.
		if r.Header.Get("Authorization") != "Bearer "+ServiceKey {
			fail(w, 403, "permission denied")
			return
		}
		s.rpc(w, r, "")
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
	case strings.HasPrefix(r.URL.Path, "/rest/v1/rpc/"):
		s.rpc(w, r, uid)
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
	default:
		fail(w, 404, "not found")
	}
}
