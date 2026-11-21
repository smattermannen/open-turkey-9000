//go:build linux

// pamguard.go — strict mode's PAM root guard (§3.8, SPEC §6 2026-11-14).
//
// Strict mode's lockdown takes sudo and pkexec away, but a root password
// would still open a root login on a text console (login) and su to root
// (su, su-l). While the lockdown is in place, a guard line in those PAM
// files refuses root before any password is asked; everyone else logs in and
// uses su as before. It goes with the lockdown (ApplyLockdown,
// RemoveLockdown), so kc9k.nofreeze and recovery lift it too.
package blocker

import (
	"os"
	"path/filepath"
	"strings"
)

// PAMDir is where the PAM files live; tests point it at generated files.
var PAMDir = "/etc/pam.d"

// pamGuardFiles are the PAM services the guard goes into.
var pamGuardFiles = []string{"login", "su", "su-l"}

const (
	pamGuardMarker = "# kc9k strict mode: no root login or su to root during a lock (removed when it ends)"
	// pam_succeed_if checks the account being logged in to (for su: the
	// target). requisite fails at once, before a password is asked.
	pamGuardRule = "auth requisite pam_succeed_if.so uid ne 0"
)

// GuardPAMConfig adds the guard to one PAM file's text. It goes right after
// a pam_rootok line (root itself may still use su to become anyone), else
// before the first rule. Text that already has it is returned unchanged.
func GuardPAMConfig(config string) string {
	if strings.Contains(config, pamGuardMarker) {
		return config
	}
	lines := strings.SplitAfter(config, "\n")
	at := -1
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if at < 0 {
			at = i // the first rule
		}
		if strings.Contains(line, "pam_rootok.so") {
			at = i + 1
			break
		}
	}
	if at < 0 {
		at = len(lines)
	}
	guard := pamGuardMarker + "\n" + pamGuardRule + "\n"
	if at > 0 && lines[at-1] != "" && !strings.HasSuffix(lines[at-1], "\n") {
		guard = "\n" + guard
	}
	return strings.Join(lines[:at], "") + guard + strings.Join(lines[at:], "")
}

// UnguardPAMConfig takes the guard out of one PAM file's text, leaving every
// other line as it was.
func UnguardPAMConfig(config string) string {
	if !strings.Contains(config, pamGuardMarker) {
		return config
	}
	lines := strings.SplitAfter(config, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\n") == pamGuardMarker {
			if i+1 < len(lines) && strings.TrimRight(lines[i+1], "\n") == pamGuardRule {
				i++
			}
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "")
}

// rewritePAM applies change to each guarded PAM file that exists, keeping its
// mode. A file the change leaves as it is isn't written.
func rewritePAM(change func(string) string) error {
	for _, name := range pamGuardFiles {
		path := filepath.Join(PAMDir, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next := change(string(data))
		if next == string(data) {
			continue
		}
		if err := atomicWriteFile(path, []byte(next), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// ApplyPAMGuard puts the guard into login, su and su-l.
func ApplyPAMGuard() error { return rewritePAM(GuardPAMConfig) }

// RemovePAMGuard takes it out again.
func RemovePAMGuard() error { return rewritePAM(UnguardPAMConfig) }

// PAMGuardInstalled reports whether any of those files has the guard.
func PAMGuardInstalled() bool {
	for _, name := range pamGuardFiles {
		if data, err := os.ReadFile(filepath.Join(PAMDir, name)); err == nil && strings.Contains(string(data), pamGuardMarker) {
			return true
		}
	}
	return false
}
