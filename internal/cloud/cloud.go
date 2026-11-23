// Package cloud is KC9K's cloud (SPEC §4.14, principle 6's one exception;
// §6 2026-11-23: Supabase): the KC9K account (Supabase Auth) and the one
// uploaded setup per account (Supabase Storage, bucket "setups", object
// "<user id>/setup.json.gz", plus a "setups" row naming when and from which
// device). Plain HTTPS with net/http, no SDK; the anon key is public and
// safe behind row-level security (cloud/supabase/schema.sql).
package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Bucket and Object name where an account's setup lives.
const (
	Bucket     = "setups"
	objectName = "setup.json.gz"
)

// ErrNoUpload is returned by Download and Latest when the account has never
// uploaded a setup.
var ErrNoUpload = errors.New("nothing was uploaded from this KC9K account yet")

// ErrSignedOut is returned when a call needs a signed-in account.
var ErrSignedOut = errors.New("not signed in to a KC9K account ('kc9k account login')")

// Client talks to one Supabase project.
type Client struct {
	URL     string // https://<project>.supabase.co
	AnonKey string
	HTTP    *http.Client
	Now     func() time.Time
}

// New returns a client for the project at baseURL.
func New(baseURL, anonKey string) *Client {
	return &Client{URL: strings.TrimRight(baseURL, "/"), AnonKey: anonKey,
		HTTP: &http.Client{Timeout: 60 * time.Second}, Now: time.Now}
}

// Session is a signed-in account: kept root-only in the database.
type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"` // unix seconds
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
}

// Upload says when and from where the account's setup was last uploaded.
type Upload struct {
	UserID     string    `json:"user_id"`
	UploadedAt time.Time `json:"uploaded_at"`
	Device     string    `json:"device"`
	Format     int       `json:"format"`
	Size       int64     `json:"size"`
}

// Error is an answer the cloud refused with.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("KC9K's cloud answered %d", e.Status)
	}
	return fmt.Sprintf("KC9K's cloud: %s", e.Message)
}

// do sends one request; with a session it is sent as that account.
func (c *Client) do(method, path string, s *Session, header http.Header, body io.Reader, out any) ([]byte, error) {
	req, err := http.NewRequest(method, c.URL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.AnonKey)
	if s != nil {
		req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.AnonKey)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("KC9K's cloud can't be reached: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return data, &Error{Status: resp.StatusCode, Message: message(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return data, fmt.Errorf("KC9K's cloud answered something unreadable: %w", err)
		}
	}
	return data, nil
}

// message reads the reason out of an error answer (Auth, Storage and
// PostgREST each name it differently).
func message(data []byte) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return strings.TrimSpace(string(data))
	}
	for _, k := range []string{"error_description", "msg", "message", "error"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func jsonBody(v any) (io.Reader, http.Header) {
	data, _ := json.Marshal(v)
	return bytes.NewReader(data), http.Header{"Content-Type": {"application/json"}}
}

type authAnswer struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	// Sign-up without a session (email confirmation on) answers the user
	// itself.
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (c *Client) session(a authAnswer) *Session {
	if a.AccessToken == "" {
		return nil
	}
	exp := a.ExpiresAt
	if exp == 0 {
		exp = c.Now().Unix() + a.ExpiresIn
	}
	return &Session{AccessToken: a.AccessToken, RefreshToken: a.RefreshToken, ExpiresAt: exp,
		UserID: a.User.ID, Email: a.User.Email}
}

// SignUp makes an account. Its session is nil when the project asks the
// address to be confirmed first (then Login after the email's link).
func (c *Client) SignUp(email, password string) (*Session, error) {
	body, h := jsonBody(map[string]string{"email": email, "password": password})
	var a authAnswer
	if _, err := c.do("POST", "/auth/v1/signup", nil, h, body, &a); err != nil {
		return nil, err
	}
	return c.session(a), nil
}

// Login signs in with an email address and password.
func (c *Client) Login(email, password string) (*Session, error) {
	body, h := jsonBody(map[string]string{"email": email, "password": password})
	var a authAnswer
	if _, err := c.do("POST", "/auth/v1/token?grant_type=password", nil, h, body, &a); err != nil {
		return nil, err
	}
	s := c.session(a)
	if s == nil {
		return nil, errors.New("KC9K's cloud signed in without a session")
	}
	return s, nil
}

// Fresh returns s, refreshed first when it has expired or nearly has;
// changed says the caller should store the new one.
func (c *Client) Fresh(s *Session) (fresh *Session, changed bool, err error) {
	if s == nil || s.RefreshToken == "" {
		return nil, false, ErrSignedOut
	}
	if c.Now().Unix() < s.ExpiresAt-60 {
		return s, false, nil
	}
	body, h := jsonBody(map[string]string{"refresh_token": s.RefreshToken})
	var a authAnswer
	if _, err := c.do("POST", "/auth/v1/token?grant_type=refresh_token", nil, h, body, &a); err != nil {
		var ce *Error
		if errors.As(err, &ce) && ce.Status >= 400 && ce.Status < 500 {
			return nil, false, ErrSignedOut
		}
		return nil, false, err
	}
	n := c.session(a)
	if n == nil {
		return nil, false, ErrSignedOut
	}
	if n.UserID == "" {
		n.UserID, n.Email = s.UserID, s.Email
	}
	return n, true, nil
}

// Logout ends the session on the cloud too (the device forgets it either
// way).
func (c *Client) Logout(s *Session) error {
	_, err := c.do("POST", "/auth/v1/logout", s, nil, nil, nil)
	return err
}

func (c *Client) objectPath(s *Session) string {
	return "/" + Bucket + "/" + url.PathEscape(s.UserID) + "/" + objectName
}

// Put uploads setup (already gzipped) as the account's one setup, replacing
// the last, and records when and from which device.
func (c *Client) Put(s *Session, setup []byte, device string, format int) (Upload, error) {
	h := http.Header{"Content-Type": {"application/gzip"}, "X-Upsert": {"true"}, "Cache-Control": {"no-store"}}
	if _, err := c.do("POST", "/storage/v1/object"+c.objectPath(s), s, h, bytes.NewReader(setup), nil); err != nil {
		return Upload{}, err
	}
	u := Upload{UserID: s.UserID, UploadedAt: c.Now().UTC().Truncate(time.Second), Device: device, Format: format, Size: int64(len(setup))}
	body, jh := jsonBody(u)
	jh.Set("Prefer", "resolution=merge-duplicates,return=minimal")
	if _, err := c.do("POST", "/rest/v1/setups?on_conflict=user_id", s, jh, body, nil); err != nil {
		return Upload{}, err
	}
	return u, nil
}

// Latest says when and from where the account last uploaded.
func (c *Client) Latest(s *Session) (Upload, error) {
	var rows []Upload
	q := "/rest/v1/setups?select=user_id,uploaded_at,device,format,size&user_id=eq." + url.QueryEscape(s.UserID)
	if _, err := c.do("GET", q, s, nil, nil, &rows); err != nil {
		return Upload{}, err
	}
	if len(rows) == 0 {
		return Upload{}, ErrNoUpload
	}
	return rows[0], nil
}

// Get downloads the account's last upload (gzipped).
func (c *Client) Get(s *Session) ([]byte, error) {
	data, err := c.do("GET", "/storage/v1/object/authenticated"+c.objectPath(s), s, nil, nil, nil)
	var ce *Error
	if errors.As(err, &ce) && (ce.Status == 404 || ce.Status == 400 && strings.Contains(strings.ToLower(ce.Message), "not found")) {
		return nil, ErrNoUpload
	}
	return data, err
}
