package cli

// guidedsetup_linux.go — the guided setup's Linux steps: root-equivalent
// groups, the root password, other systems in GRUB's menu.

import (
	"os"
	"os/user"
	"strings"
)

const (
	strictDetail        = "While anything is locked: no sudo or pkexec, no root login or su to root, nothing runs from /home or /tmp, no containers or virtual machines."
	escapeDetail        = "Last: kc9k.nofreeze and recovery mode stop working, and the boot menu gets a password nobody knows. Until then they are the way out of a bug (R-ESCAPE-1); try it in a virtual machine first."
	firmwareUnreachable = "This computer's firmware can't be changed from Linux. "
)

// Files the checklist reads; tests point them at generated ones.
var (
	shadowPath  = "/etc/shadow"
	grubCfgPath = "/boot/grub/grub.cfg"
)

// rootPasswordState tells whether root can log in with a password: "locked",
// "password", or "" when /etc/shadow can't be read.
func rootPasswordState() string {
	data, err := os.ReadFile(shadowPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 2 || fields[0] != "root" {
			continue
		}
		if fields[1] == "" || strings.HasPrefix(fields[1], "!") || strings.HasPrefix(fields[1], "*") {
			if fields[1] == "" {
				return "password" // no password at all: anyone may log in as root
			}
			return "locked"
		}
		return "password"
	}
	return ""
}

// otherSystems lists the boot entries of other operating systems (Windows,
// another Linux) in GRUB's menu.
func otherSystems() []string {
	data, err := os.ReadFile(grubCfgPath)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "menuentry ") || !strings.Contains(t, "osprober") && !strings.Contains(t, "--class windows") {
			continue
		}
		name := t
		if parts := strings.SplitN(t, "'", 3); len(parts) == 3 {
			name = parts[1]
		}
		out = append(out, name)
	}
	return out
}

// riskyGroups lists the groups of rootGroups name is in.
func riskyGroups(name string) []string {
	u, err := user.Lookup(name)
	if err != nil {
		return nil
	}
	ids, _ := u.GroupIds()
	var out []string
	for _, id := range ids {
		if g, err := user.LookupGroupId(id); err == nil {
			for _, risky := range rootGroups {
				if g.Name == risky {
					out = append(out, risky)
				}
			}
		}
	}
	return out
}

// accountSteps are the steps about who can be root.
func accountSteps(strictUser string) []setupStep {
	var steps []setupStep
	if strictUser != "" {
		groups := riskyGroups(strictUser)
		step := setupStep{ID: "groups", Title: "No root-equivalent groups", State: doneIf(len(groups) == 0),
			Detail: strictUser + " is in no group that can get around strict mode."}
		if len(groups) > 0 {
			step.Detail = strictUser + " is in " + strings.Join(groups, ", ") + ", which gives raw access to the drives."
			step.Command = "sudo gpasswd -d " + strictUser + " " + strings.Join(groups, ",")
		}
		steps = append(steps, step)
	}

	root := setupStep{ID: "root-password", Title: "No root password"}
	switch rootPasswordState() {
	case "locked":
		root.State, root.Detail = stepDone, "root has no password: nobody can log in as root."
	case "password":
		root.State, root.Command = stepTodo, "sudo passwd -l root"
		root.Detail = "root has a password. Strict mode keeps it out of login and su during a lock, but recovery mode asks for it until no-escape is on."
	default:
		root.State, root.Detail = stepUnknown, "Can't read /etc/shadow (run as root to check)."
	}
	steps = append(steps, root)

	return steps
}

// recoverySteps: Linux's recovery path is described by no-escape's step.
func recoverySteps() []setupStep { return nil }
