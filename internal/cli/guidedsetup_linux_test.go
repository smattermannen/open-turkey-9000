package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// §4.10 guided setup: the checklist for strict mode and no-escape, in its
// order, follows the settings and says what to run.
func TestGuidedSetup(t *testing.T) {
	testDB(t)
	notTerminal(t)
	fakeFirmwareRoot(t)
	dir := t.TempDir()
	shadow, grub := filepath.Join(dir, "shadow"), filepath.Join(dir, "grub.cfg")
	_ = os.WriteFile(shadow, []byte("root:$6$abc:19000:0:99999:7:::\nme:!:1:::\n"), 0o600)
	_ = os.WriteFile(grub, []byte("menuentry 'Linux Mint 22' --class linuxmint --class gnu-linux {\n}\n"+
		"menuentry 'Windows Boot Manager (on /dev/nvme0n1p1)' --class windows --class os $menuentry_id_option 'osprober-efi-1' {\n}\n"), 0o644)
	oldShadow, oldGrub := shadowPath, grubCfgPath
	shadowPath, grubCfgPath = shadow, grub
	t.Cleanup(func() { shadowPath, grubCfgPath = oldShadow, oldGrub })

	steps := func() map[string]setupStep {
		var list []setupStep
		if err := json.Unmarshal([]byte(capture(t, "guided-setup", "--json")), &list); err != nil {
			t.Fatal(err)
		}
		if list[len(list)-1].ID != "no-escape" || list[0].ID != "settings-password" {
			t.Errorf("order: first %s, last %s", list[0].ID, list[len(list)-1].ID)
		}
		out := map[string]setupStep{}
		for _, s := range list {
			out[s.ID] = s
		}
		return out
	}
	s := steps()
	for _, id := range []string{"settings-password", "no-weakening", "tamper", "strict", "strict-lock", "reset-guard", "firmware", "no-escape"} {
		if s[id].State != stepTodo || s[id].Command == "" {
			t.Errorf("%s: %+v", id, s[id])
		}
	}
	if s["root-password"].State != stepTodo || s["firmware"].Command != "kc9k firmware-lock on" {
		t.Errorf("root %+v, firmware %+v", s["root-password"], s["firmware"])
	}
	if o := s["other-systems"]; o.State != stepInfo || !strings.Contains(o.Detail, "Windows Boot Manager") || strings.Contains(o.Detail, "Mint") {
		t.Errorf("other systems: %+v", o)
	}
	for _, args := range [][]string{{"strict", "on"}, {"reset-guard", "on"}, {"no-weakening", "on"},
		{"tamper", "clock", "on"}, {"tamper", "task-managers", "on"}, {"tamper", "private", "on"}} {
		mustRun(t, args...)
	}
	_ = os.WriteFile(shadow, []byte("root:!:19000:0:99999:7:::\n"), 0o600)
	s = steps()
	for _, id := range []string{"strict", "reset-guard", "no-weakening", "root-password"} {
		if s[id].State != stepDone {
			t.Errorf("%s not done: %+v", id, s[id])
		}
	}
	if s["tamper"].State != stepTodo || !strings.Contains(s["tamper"].Detail, "unsupported-browsers") || strings.Contains(s["tamper"].Detail, "clock") {
		t.Errorf("tamper: %+v", s["tamper"])
	}
	_ = os.Remove(shadow)
	blocker.FirmwareRoot = filepath.Join(dir, "none")
	s = steps()
	if s["root-password"].State != stepUnknown || s["firmware"].State != stepByHand {
		t.Errorf("root %+v, firmware %+v", s["root-password"], s["firmware"])
	}
	if out := capture(t, "guided-setup"); !strings.Contains(out, "[x]") || !strings.Contains(out, "→ kc9k no-escape on --yes") {
		t.Errorf("text checklist:\n%s", out)
	}
}
