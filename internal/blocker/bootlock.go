package blocker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// The boot lock closes the routes around strict mode that go through GRUB. It
// sets a random GRUB password that is never stored, hides recovery entries and
// keeps normal entries bootable. Nobody can then edit a boot entry, including
// the owner, so the only way past it is booting other media or reinstalling.
const (
	grubPasswordFile = "/etc/grub.d/09_kc9k"
	grubDefaultsFile = "/etc/default/grub.d/99-kc9k.cfg"
	grubLinuxScript  = "/etc/grub.d/10_linux"
	grubProberScript = "/etc/grub.d/30_os-prober"
	grubBackupSuffix = ".kc9k-backup"
)

// unrestrict makes a script's menu entries bootable without the password.
func unrestrict(script string) string {
	if strings.Contains(script, "--unrestricted") {
		return script
	}
	script = strings.Replace(script, `CLASS="--class gnu-linux --class gnu --class os`,
		`CLASS="--class gnu-linux --class gnu --class os --unrestricted`, 1)
	// os-prober builds the class at runtime ($CLASS is e.g. "--class windows"),
	// so Windows and other chainloaded systems only match on "$CLASS --class os ".
	script = strings.ReplaceAll(script, "$CLASS --class os ", "$CLASS --class os --unrestricted ")
	return strings.ReplaceAll(script, "--class windows --class os", "--class windows --class os --unrestricted")
}

// restrictedEntries lists the top-level boot entries that would need the
// password. Entries inside submenus or conditionals are indented and may stay
// restricted (recovery, advanced options, firmware settings).
func restrictedEntries(config string) []string {
	var restricted []string
	for _, line := range strings.Split(config, "\n") {
		if strings.HasPrefix(line, "menuentry ") && !strings.Contains(line, "--unrestricted") {
			restricted = append(restricted, line)
		}
	}
	return restricted
}

func grubPasswordScript(hash string) string {
	return "#!/bin/sh\ncat <<'EOF'\nset superusers=\"kc9k\"\npassword_pbkdf2 kc9k " + hash + "\nEOF\n"
}

func randomGrubHash() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	password := hex.EncodeToString(raw)
	cmd := exec.Command("grub-mkpasswd-pbkdf2")
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("grub-mkpasswd-pbkdf2: %w", err)
	}
	for _, field := range strings.Fields(string(output)) {
		if strings.HasPrefix(field, "grub.pbkdf2.") {
			return field, nil
		}
	}
	return "", fmt.Errorf("grub-mkpasswd-pbkdf2 returned no hash")
}

func patchScript(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), "--unrestricted") {
		if err := os.WriteFile(path+grubBackupSuffix, data, 0644); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(unrestrict(string(data))), 0755)
}

func restoreScript(path string) error {
	backup := path + grubBackupSuffix
	data, err := os.ReadFile(backup)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0755); err != nil {
		return err
	}
	return os.Remove(backup)
}

func updateGrub() error {
	if output, err := exec.Command("update-grub").CombinedOutput(); err != nil {
		return fmt.Errorf("update-grub: %s (error: %w)", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// grubAcceptsConfig checks the generated menu, so a broken one is undone
// instead of left for the next boot.
func grubAcceptsConfig() error {
	config, err := os.ReadFile("/boot/grub/grub.cfg")
	if err != nil {
		return err
	}
	text := string(config)
	if !strings.Contains(text, "--unrestricted") || !strings.Contains(text, "password_pbkdf2") {
		return fmt.Errorf("the boot menu lacks unrestricted entries or the password")
	}
	if restricted := restrictedEntries(text); len(restricted) > 0 {
		return fmt.Errorf("boot entries would need the password: %s", strings.Join(restricted, "; "))
	}
	if output, err := exec.Command("grub-script-check", "/boot/grub/grub.cfg").CombinedOutput(); err != nil {
		return fmt.Errorf("grub-script-check: %s (error: %w)", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ApplyBootLock installs the boot lock and rolls back on any failure.
func ApplyBootLock() error {
	hash, err := randomGrubHash()
	if err != nil {
		return err
	}
	if err := patchScript(grubLinuxScript); err != nil {
		return err
	}
	if err := patchScript(grubProberScript); err != nil {
		_ = RemoveBootLock()
		return err
	}
	if err := os.MkdirAll("/etc/default/grub.d", 0755); err != nil {
		_ = RemoveBootLock()
		return err
	}
	if err := os.WriteFile(grubDefaultsFile, []byte("GRUB_DISABLE_RECOVERY=\"true\"\n"), 0644); err != nil {
		_ = RemoveBootLock()
		return err
	}
	if err := os.WriteFile(grubPasswordFile, []byte(grubPasswordScript(hash)), 0700); err != nil {
		_ = RemoveBootLock()
		return err
	}
	if err := updateGrub(); err != nil {
		_ = RemoveBootLock()
		return err
	}
	if err := grubAcceptsConfig(); err != nil {
		_ = RemoveBootLock()
		return err
	}
	return nil
}

// BootLockInstalled reports whether the boot lock is in place.
func BootLockInstalled() bool {
	_, err := os.Stat(grubPasswordFile)
	return err == nil
}

// RemoveBootLock restores normal GRUB behaviour.
func RemoveBootLock() error {
	for _, path := range []string{grubPasswordFile, grubDefaultsFile} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := restoreScript(grubLinuxScript); err != nil {
		return err
	}
	if err := restoreScript(grubProberScript); err != nil {
		return err
	}
	return updateGrub()
}
