//go:build linux

package blocker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useUsernsFiles points the user namespace setting and its saved value at
// files for one test, the setting holding value ("" for a kernel without it).
func useUsernsFiles(t *testing.T, value string) (setting, saved string) {
	t.Helper()
	dir := t.TempDir()
	setting, saved = filepath.Join(dir, "restrict"), filepath.Join(dir, "saved")
	if value != "" {
		if err := os.WriteFile(setting, []byte(value+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldSetting, oldSaved := usernsRestrictPath, usernsStatePath
	usernsRestrictPath, usernsStatePath = setting, saved
	t.Cleanup(func() { usernsRestrictPath, usernsStatePath = oldSetting, oldSaved })
	return setting, saved
}

func readTrimT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

// B3, §3.8 strict mode: during the lockdown user namespaces get no
// capabilities (`unshare -Urm` can't mount an exec tmpfs), even on Linux
// Mint, which turns AppArmor's restriction off; the old value comes back.
func TestUserNamespacesRestricted(t *testing.T) {
	setting, saved := useUsernsFiles(t, "0")
	for i := 0; i < 2; i++ { // every cycle: the first saved value is kept
		if err := restrictUserNamespaces(); err != nil {
			t.Fatal(err)
		}
		if got := readTrimT(t, setting); got != "1" {
			t.Fatalf("setting = %s during the lockdown, want 1", got)
		}
	}
	if got := readTrimT(t, saved); got != "0" {
		t.Errorf("saved = %s, want 0", got)
	}
	if err := restoreUserNamespaces(); err != nil {
		t.Fatal(err)
	}
	if got := readTrimT(t, setting); got != "0" {
		t.Errorf("setting = %s after, want 0", got)
	}
	if _, err := os.Stat(saved); !os.IsNotExist(err) {
		t.Error("saved value left behind")
	}
}

// Ubuntu's own value (1) is left as it is, and a kernel without the setting
// is no error.
func TestUserNamespacesAlreadyRestricted(t *testing.T) {
	setting, saved := useUsernsFiles(t, "1")
	if err := restrictUserNamespaces(); err != nil {
		t.Fatal(err)
	}
	if err := restoreUserNamespaces(); err != nil {
		t.Fatal(err)
	}
	if got := readTrimT(t, setting); got != "1" {
		t.Errorf("setting = %s, want 1", got)
	}
	if _, err := os.Stat(saved); !os.IsNotExist(err) {
		t.Error("nothing to save, but a value was saved")
	}
	useUsernsFiles(t, "")
	if err := restrictUserNamespaces(); err != nil {
		t.Errorf("no such setting: %v", err)
	}
	if err := restoreUserNamespaces(); err != nil {
		t.Errorf("no such setting: %v", err)
	}
}
