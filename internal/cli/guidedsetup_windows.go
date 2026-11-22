package cli

// guidedsetup_windows.go — the guided setup's Windows steps (W15): other
// administrator accounts and the built-in Administrator instead of Linux's
// root-equivalent groups and root password; Safe Mode, Windows' way out
// of a bug until no-escape; other systems in Windows' boot menu.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/smattermannen/kc9k/internal/winapi"
)

const (
	strictDetail        = "While anything is locked: KC9K's service, files, registry keys and boot settings are closed to administrators; no new services, tasks or programs as SYSTEM, no installers, no new accounts; nothing runs from your own folders, temporary folders or USB drives; no virtual machines, WSL or Windows Sandbox."
	escapeDetail        = "Last: Safe Mode stops being a way out, the recovery environment and the boot menu go off. Until then Safe Mode is the way out of a bug (R-ESCAPE-1); try it in a virtual machine first."
	firmwareUnreachable = "This computer's firmware offers no settings Windows can change. "
)

// localAdmins lists the enabled local accounts in Administrators: name and
// relative ID. A variable for tests.
var localAdmins = func() ([]winapi.Account, error) {
	members, err := winapi.AdministratorSIDs()
	if err != nil {
		return nil, err
	}
	in := map[string]bool{}
	for _, m := range members {
		in[m] = true
	}
	accounts, err := winapi.LocalAccounts()
	if err != nil {
		return nil, err
	}
	var out []winapi.Account
	for _, a := range accounts {
		if a.Disabled {
			continue
		}
		if sid, err := winapi.LocalAccountSID(a.RID); err == nil && in[sid] {
			out = append(out, a)
		}
	}
	return out, nil
}

// accountSteps: who else is an administrator.
func accountSteps(strictUser string) []setupStep {
	admins, err := localAdmins()
	if err != nil {
		return []setupStep{{ID: "other-admins", Title: "No other administrator accounts", State: stepUnknown,
			Detail: "Can't list the administrators: " + err.Error()}}
	}
	var others, cmds []string
	builtin := ""
	for _, a := range admins {
		if a.RID == 500 {
			builtin = a.Name
			continue
		}
		if !strings.EqualFold(a.Name, strictUser) {
			others = append(others, a.Name)
			cmds = append(cmds, `net user "`+a.Name+`" /active:no`)
		}
	}
	other := setupStep{ID: "other-admins", Title: "No other administrator accounts", State: doneIf(len(others) == 0),
		Detail: "Only " + strictUser + " is an administrator."}
	if strictUser == "" {
		other.Detail = "No other administrator accounts."
	}
	if len(others) > 0 {
		other.Detail = "Also administrators: " + strings.Join(others, ", ") + ". Strict mode denies them the same, but each has its own password and is another way in; turn off the ones you don't need."
		other.Command = strings.Join(cmds, "; ")
	}
	b := setupStep{ID: "builtin-admin", Title: "Built-in Administrator off", State: stepDone,
		Detail: "The built-in Administrator account is off."}
	if builtin != "" {
		b.State, b.Command = stepTodo, `net user "`+builtin+`" /active:no`
		b.Detail = "The built-in Administrator (" + builtin + ") is on: it never asks before running as administrator. Turn it off."
	}
	return []setupStep{other, b}
}

// recoverySteps: Safe Mode, the way out of a bug until no-escape.
func recoverySteps() []setupStep {
	return []setupStep{{ID: "safe-mode", Title: "Safe Mode", State: stepInfo,
		Detail: "Until no-escape is on, Safe Mode is the way out of a bug: freezes and strict mode don't hold there (blocks do), and kc9k recover removes a forgotten settings password."}}
}

// otherSystems lists the boot menu's other systems (bcdedit's loader
// entries other than the one running; its element names aren't translated).
func otherSystems() []string {
	out, err := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "bcdedit.exe"), "/enum", "osloader").Output()
	if err != nil {
		return nil
	}
	return parseOtherLoaders(string(out))
}

// parseOtherLoaders reads bcdedit's loader entries: the descriptions of
// those whose identifier isn't {current}.
func parseOtherLoaders(out string) []string {
	var names []string
	current := false
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch strings.ToLower(f[0]) {
		case "identifier":
			current = strings.EqualFold(f[1], "{current}")
		case "description":
			if !current {
				names = append(names, strings.Join(f[1:], " "))
			}
		}
	}
	return names
}
