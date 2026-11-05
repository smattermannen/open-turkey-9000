package blocker

import (
	"strings"
	"testing"
)

func TestUnrestrict(t *testing.T) {
	script := `CLASS="--class gnu-linux --class gnu --class os"` + "\n" +
		`menuentry "Windows" --class windows --class os {`
	got := unrestrict(script)
	if !strings.Contains(got, `--class os --unrestricted"`) {
		t.Errorf("Linux entries stay restricted: %s", got)
	}
	if !strings.Contains(got, "--class windows --class os --unrestricted") {
		t.Errorf("Windows entries stay restricted: %s", got)
	}
	if unrestrict(got) != got {
		t.Error("patching twice changes the script")
	}
}

func TestGrubPasswordScript(t *testing.T) {
	got := grubPasswordScript("grub.pbkdf2.x")
	if !strings.Contains(got, "password_pbkdf2 kc9k grub.pbkdf2.x") {
		t.Errorf("no password line: %s", got)
	}
}
