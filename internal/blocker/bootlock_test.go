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

// Lines as they appear in Ubuntu/Mint's /etc/grub.d/30_os-prober, where the
// Windows class is only known at runtime.
func TestUnrestrictOsProber(t *testing.T) {
	script := `menuentry '$(echo "${LONGNAME} $onstr" | grub_quote)' $CLASS --class os \$menuentry_id_option 'osprober-chain-x' {` + "\n" +
		`menuentry '$(echo "${LONGNAME} $onstr" | grub_quote)' $CLASS --class os \$menuentry_id_option 'osprober-efi-x' {`
	got := unrestrict(script)
	if strings.Count(got, "$CLASS --class os --unrestricted ") != 2 {
		t.Errorf("os-prober entries stay restricted: %s", got)
	}
}

func TestRestrictedEntries(t *testing.T) {
	config := "menuentry 'Linux Mint' --class os --unrestricted $menuentry_id_option 'a' {\n" +
		"submenu 'Advanced options' $menuentry_id_option 'b' {\n" +
		"\tmenuentry 'Linux Mint (recovery mode)' --class os $menuentry_id_option 'c' {\n" +
		"menuentry 'Windows Boot Manager' --class windows --class os $menuentry_id_option 'd' {\n"
	got := restrictedEntries(config)
	if len(got) != 1 || !strings.Contains(got[0], "Windows") {
		t.Errorf("want only the Windows entry flagged, got %q", got)
	}
}

func TestGrubPasswordScript(t *testing.T) {
	got := grubPasswordScript("grub.pbkdf2.x")
	if !strings.Contains(got, "password_pbkdf2 kc9k grub.pbkdf2.x") {
		t.Errorf("no password line: %s", got)
	}
}
