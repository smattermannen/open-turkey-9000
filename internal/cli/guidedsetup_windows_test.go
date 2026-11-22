package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/winapi"
)

// W15: the guided setup's Windows steps, from the accounts given.
func TestGuidedSetupWindows(t *testing.T) {
	testDB(t)
	notTerminal(t)
	old := localAdmins
	localAdmins = func() ([]winapi.Account, error) {
		return []winapi.Account{{Name: "Administrator", RID: 500}, {Name: "me", RID: 1001}, {Name: "kid", RID: 1002}}, nil
	}
	t.Cleanup(func() { localAdmins = old })
	var list []setupStep
	if err := json.Unmarshal([]byte(capture(t, "guided-setup", "--json")), &list); err != nil {
		t.Fatal(err)
	}
	s := map[string]setupStep{}
	for _, step := range list {
		s[step.ID] = step
	}
	if list[0].ID != "settings-password" || list[len(list)-1].ID != "no-escape" {
		t.Errorf("order: first %s, last %s", list[0].ID, list[len(list)-1].ID)
	}
	if _, linux := s["root-password"]; linux {
		t.Error("Linux's root password step on Windows")
	}
	if o := s["other-admins"]; o.State != stepTodo || !strings.Contains(o.Detail, "kid") || strings.Contains(o.Detail, "Administrator,") {
		t.Errorf("other administrators: %+v", o)
	}
	if b := s["builtin-admin"]; b.State != stepTodo || !strings.Contains(b.Command, "Administrator") {
		t.Errorf("built-in Administrator: %+v", b)
	}
	if s["safe-mode"].State != stepInfo || strings.Contains(s["no-escape"].Detail, "kc9k.nofreeze") {
		t.Errorf("recovery: %+v, %+v", s["safe-mode"], s["no-escape"])
	}
	if strings.Contains(s["strict"].Detail, "sudo") {
		t.Errorf("strict mode's step names sudo: %s", s["strict"].Detail)
	}
}

func TestParseOtherLoaders(t *testing.T) {
	out := "Windows Boot Loader\r\n-------------------\r\nidentifier              {current}\r\ndevice                  partition=C:\r\ndescription             Windows 11\r\n\r\n" +
		"Windows Boot Loader\r\n-------------------\r\nidentifier              {0a1b}\r\ndescription             Windows 10 (old)\r\n"
	if got := parseOtherLoaders(out); len(got) != 1 || got[0] != "Windows 10 (old)" {
		t.Errorf("other loaders: %v", got)
	}
}

// W15: the real accounts of this computer can be listed.
func TestLocalAdminsForReal(t *testing.T) {
	admins, err := localAdmins()
	if err != nil {
		t.Fatal(err)
	}
	if len(admins) == 0 {
		t.Error("no administrator found (the runner's account is one)")
	}
	t.Logf("administrators: %v", admins)
}
