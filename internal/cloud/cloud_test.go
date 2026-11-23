package cloud_test

import (
	"errors"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
)

func TestAccountAndSetup(t *testing.T) {
	f := cloudtest.New()
	defer f.Close()
	c := cloud.New(f.URL+"/", cloudtest.AnonKey)
	s, err := c.SignUp("me@example.org", "secret123")
	if err != nil || s == nil || s.UserID == "" {
		t.Fatalf("sign-up: %+v %v", s, err)
	}
	if _, err := c.SignUp("me@example.org", "secret123"); err == nil {
		t.Error("the same address signed up twice")
	}
	if _, err := c.Login("me@example.org", "wrong"); err == nil {
		t.Error("a wrong password signed in")
	}
	if s, err = c.Login("me@example.org", "secret123"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Latest(s); !errors.Is(err, cloud.ErrNoUpload) {
		t.Errorf("before any upload: %v", err)
	}
	if _, err := c.Get(s); !errors.Is(err, cloud.ErrNoUpload) {
		t.Errorf("download before any upload: %v", err)
	}
	for _, data := range []string{"first", "second"} {
		if _, err := c.Put(s, []byte(data), "laptop", 1); err != nil {
			t.Fatal(err)
		}
	}
	got, err := c.Get(s)
	if err != nil || string(got) != "second" {
		t.Errorf("download: %q %v", got, err)
	}
	u, err := c.Latest(s)
	if err != nil || u.Device != "laptop" || u.Size != 6 || u.Format != 1 {
		t.Errorf("latest: %+v %v", u, err)
	}

	// Another account sees nothing of the first.
	other, _ := c.SignUp("you@example.org", "secret456")
	if _, err := c.Get(other); !errors.Is(err, cloud.ErrNoUpload) {
		t.Errorf("another account's download: %v", err)
	}
	spy := *other
	spy.UserID = s.UserID
	if _, err := c.Get(&spy); err == nil {
		t.Error("an account read another's setup")
	}

	// An expired session is refreshed; a refused refresh means signed out.
	f.Expire()
	s.ExpiresAt = time.Now().Unix()
	fresh, changed, err := c.Fresh(s)
	if err != nil || !changed || fresh.UserID != s.UserID {
		t.Fatalf("refresh: %+v %v %v", fresh, changed, err)
	}
	if _, err := c.Get(fresh); err != nil {
		t.Errorf("after refresh: %v", err)
	}
	if _, _, err := c.Fresh(s); !errors.Is(err, cloud.ErrSignedOut) {
		t.Errorf("a used refresh token: %v", err)
	}
	if err := c.Logout(fresh); err != nil {
		t.Error(err)
	}
	if _, err := c.Get(fresh); err == nil {
		t.Error("a logged-out session still reads")
	}
}

func TestUnreachable(t *testing.T) {
	c := cloud.New("http://127.0.0.1:1", "k")
	if _, err := c.Login("a@b.c", "x"); err == nil {
		t.Error("no server, no error")
	}
}
