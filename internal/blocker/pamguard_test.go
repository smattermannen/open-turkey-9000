//go:build linux

package blocker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Generated PAM files shaped like Linux Mint's and Ubuntu's (never the real
// /etc/pam.d: PAMDir points at a scratch folder).
const (
	pamLogin = `#
# The PAM configuration file for the Shadow ` + "`login'" + ` service
#
auth       optional   pam_faildelay.so  delay=3000000
auth [success=ok new_authtok_reqd=ok ignore=ignore user_unknown=bad default=die] pam_securetty.so
auth       requisite  pam_nologin.so
session [success=ok ignore=ignore module_unknown=ignore default=bad] pam_selinux.so close
@include common-auth
auth       optional   pam_group.so
@include common-account
@include common-session
@include common-password
`
	pamSu = `#
# The PAM configuration file for the Shadow ` + "`su'" + ` service
#

# This allows root to su without passwords (normal operation)
auth       sufficient pam_rootok.so

# Uncomment this to force users to be a member of group wheel
# auth       required   pam_wheel.so

session       required   pam_env.so readenv=1
@include common-auth
@include common-account
@include common-session`
	pamSuL = `#%PAM-1.0
auth		include		su
account		include		su
password	include		su
session		optional	pam_keyinit.so force revoke
session		include		su
`
)

func pamRules(config string) []string {
	var rules []string
	for _, line := range strings.Split(config, "\n") {
		if f := strings.Fields(line); len(f) > 0 && !strings.HasPrefix(f[0], "#") {
			rules = append(rules, strings.Join(f, " "))
		}
	}
	return rules
}

// SPEC §6 2026-11-14: the root guard goes into login, su and su-l, before
// any password is asked; root itself keeps su (after pam_rootok).
func TestGuardPAMConfigPlacement(t *testing.T) {
	for name, tc := range map[string]struct {
		config string
		before string // the rule the guard comes right before
	}{
		"login": {pamLogin, "auth optional pam_faildelay.so delay=3000000"},
		"su":    {pamSu, "session required pam_env.so readenv=1"},
		"su-l":  {pamSuL, "auth include su"},
		"empty": {"", ""},
	} {
		guarded := GuardPAMConfig(tc.config)
		rules := pamRules(guarded)
		idx := -1
		for i, r := range rules {
			if r == pamGuardRule {
				idx = i
			}
		}
		if idx < 0 {
			t.Fatalf("%s: no guard in\n%s", name, guarded)
		}
		if tc.before != "" && (idx+1 >= len(rules) || rules[idx+1] != tc.before) {
			t.Errorf("%s: guard not right before %q:\n%s", name, tc.before, guarded)
		}
		if name == "su" && rules[idx-1] != "auth sufficient pam_rootok.so" {
			t.Errorf("su: guard should follow pam_rootok:\n%s", guarded)
		}
		if again := GuardPAMConfig(guarded); again != guarded {
			t.Errorf("%s: guarding twice changed it", name)
		}
		if back := UnguardPAMConfig(guarded); back != tc.config {
			t.Errorf("%s: unguard didn't give back the file:\n%q\nwant\n%q", name, back, tc.config)
		}
	}
	// A file ending without a newline keeps its last line intact.
	if g := GuardPAMConfig("auth required pam_unix.so"); !strings.Contains(g, "pam_unix.so\n"+pamGuardMarker) && !strings.HasPrefix(g, pamGuardMarker) {
		t.Errorf("guard glued to a line: %q", g)
	}
}

// Unguarding never touches lines that only look like ours.
func TestUnguardPAMConfigLeavesOthers(t *testing.T) {
	config := "auth requisite pam_succeed_if.so uid ne 0\n" + pamSu
	if got := UnguardPAMConfig(config); got != config {
		t.Errorf("a file without the marker changed")
	}
}

// ApplyPAMGuard/RemovePAMGuard on a generated /etc/pam.d: each file keeps
// its mode, a missing file is skipped, others aren't touched.
func TestApplyRemovePAMGuardOnGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	old := PAMDir
	PAMDir = dir
	t.Cleanup(func() { PAMDir = old })
	files := map[string]string{"login": pamLogin, "su": pamSu, "sudo": "@include common-auth\n"}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if PAMGuardInstalled() {
		t.Fatal("guard reported before it was applied")
	}
	if err := ApplyPAMGuard(); err != nil {
		t.Fatal(err)
	}
	if !PAMGuardInstalled() {
		t.Fatal("guard not reported after applying it")
	}
	for name, content := range files {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		guarded := strings.Contains(string(data), pamGuardRule)
		if want := name != "sudo"; guarded != want {
			t.Errorf("%s guarded = %v, want %v", name, guarded, want)
		}
		if name == "sudo" && string(data) != content {
			t.Errorf("sudo changed")
		}
		info, _ := os.Stat(filepath.Join(dir, name))
		if info.Mode().Perm() != 0644 {
			t.Errorf("%s mode %v", name, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "su-l")); !os.IsNotExist(err) {
		t.Errorf("a missing su-l was made")
	}
	if err := ApplyPAMGuard(); err != nil { // the daemon applies it every cycle
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "su")); strings.Count(string(data), pamGuardRule) != 1 {
		t.Errorf("applied twice:\n%s", data)
	}
	if err := RemovePAMGuard(); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if data, _ := os.ReadFile(filepath.Join(dir, name)); string(data) != content {
			t.Errorf("%s not restored:\n%s", name, data)
		}
	}
	if PAMGuardInstalled() {
		t.Error("guard still reported")
	}
}
