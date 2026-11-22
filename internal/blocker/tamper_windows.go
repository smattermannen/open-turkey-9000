package blocker

// tamper_windows.go — bypass prevention against an administrator on
// Windows (ROADMAP W5, SPEC §6 2026-11-21). An administrator can always
// win in the end; every way here is meant to be slow, visible, and counted
// as a bypass: whatever is found changed is put back each cycle and
// reported (GuardService's list; enforce logs each as tampering).
//
//   - Always: the service's security descriptor lets administrators query
//     and start it, never stop, pause, reconfigure or delete it; its
//     settings (automatic start, restart on failure, its program), its
//     Safe Mode registration (it runs in Safe Mode too) and the SYSTEM
//     watchdog task that starts it again are checked each cycle; KC9K's
//     folders keep their security.
//   - Strict mode (ApplyLockdown, while anything is locked): deny entries
//     for Administrators on what would stop KC9K: the service's registry
//     key, the hosts file, the browsers' policy keys, Safe Mode's
//     registration, the boot configuration (BCD) and the recovery
//     environment's configuration; the originals are kept and put back
//     when the lock ends. People's own programs (run from their profile
//     or a removable drive) are closed, as noexec does on Linux.
//   - No-escape (ApplyBootLock): the recovery environment off, automatic
//     repair off, the boot menu hidden; undone by RemoveBootLock.
// Nothing here hides from Windows Defender or any security software.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/smattermannen/kc9k/internal/freeze"
	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/mgr"
)

// serviceName is the Windows service's name (winsvc.ServiceName).
const serviceName = "kc9k"

// ServiceSDDL: SYSTEM may do anything; Administrators query, start and
// read it; signed-in users query it. Nobody else stops, pauses,
// reconfigures or deletes it.
const ServiceSDDL = "O:SYG:SYD:(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;SY)(A;;CCLCSWRPLOCRRC;;;BA)(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)(A;;CCLCSWLOCRRC;;;AU)"

// Folder security: see winsvc (kept here too: the service checks them).
const (
	dataSDDL    = "O:SYG:SYD:P(A;OICI;FA;;;SY)"
	programSDDL = "O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;0x1200a9;;;BA)(A;OICI;0x1200a9;;;BU)"
)

// Deny entries strict mode adds. Not for Administrators: LocalSystem's
// token holds that group too, and the service would deny itself. A person's
// programs carry how they signed in instead (Interactive at the screen,
// Network over SSH or remote PowerShell, Batch for scheduled tasks); a
// service never does. On files and folders: writing, deleting, and
// changing permissions or owner; on registry keys: setting values, adding
// keys, deleting, and changing permissions or owner.
var denyWho = []string{"IU", "NU", "S-1-5-3"}

// denyACEs are the deny entries for one kind of object: "dir", "file" or
// "key".
func denyACEs(kind string) []string {
	flags, mask := "OICI", "0xd0156"
	switch kind {
	case "file":
		flags = ""
	case "key":
		flags, mask = "CI", "0xd0006"
	}
	var out []string
	for _, who := range denyWho {
		out = append(out, "(D;"+flags+";"+mask+";;;"+who+")")
	}
	return out
}

// watchdogTask is the scheduled task that starts the service again.
const watchdogTask = `\KC9K\Watchdog`

// GuardService puts back what was changed of the service's protection and
// returns what it put back, for enforce to count as bypasses. Only the
// service (SYSTEM) calls it.
func GuardService() []string {
	if !platform.Privileged() {
		return nil
	}
	var fixed []string
	note := func(what string, err error) {
		if err != nil {
			log.Printf("tamper: %s: %v", what, err)
			return
		}
		fixed = append(fixed, what)
	}
	exe := filepath.Join(platform.InstallDir, "kc9k.exe")
	m, err := mgr.Connect()
	if err != nil {
		log.Printf("tamper: service manager: %v", err)
		return nil
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return nil // not installed (development): nothing to guard
	}
	defer s.Close()
	if cur, err := serviceSecurity(s.Handle); err == nil && cur != canonicalSDDL(ServiceSDDL) {
		note("the service's permissions", setServiceSecurity(s.Handle, ServiceSDDL))
	}
	if cfg, err := s.Config(); err == nil {
		want := windows.EscapeArg(exe) + " service"
		if cfg.StartType != mgr.StartAutomatic || !strings.EqualFold(cfg.BinaryPathName, want) {
			cfg.StartType, cfg.BinaryPathName = mgr.StartAutomatic, want
			note("the service's settings", s.UpdateConfig(cfg))
		}
	}
	if acts, err := s.RecoveryActions(); err == nil && (len(acts) == 0 || acts[0].Type != mgr.ServiceRestart) {
		err := s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart}, {Type: mgr.ServiceRestart, Delay: 5e9}, {Type: mgr.ServiceRestart, Delay: 10e9}}, 60)
		if err == nil {
			err = s.SetRecoveryActionsOnNonCrashFailures(true)
		}
		note("the service's restart on failure", err)
	}
	for _, mode := range []string{"Minimal", "Network"} {
		key := `SYSTEM\CurrentControlSet\Control\SafeBoot\` + mode + `\` + serviceName
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
		if err == nil {
			v, _, _ := k.GetStringValue("")
			k.Close()
			if v == "Service" {
				continue
			}
		}
		k, _, err = registry.CreateKey(registry.LOCAL_MACHINE, key, registry.SET_VALUE)
		if err == nil {
			err = k.SetStringValue("", "Service")
			k.Close()
		}
		note("the service's Safe Mode ("+mode+") registration", err)
	}
	if !taskExists(watchdogTask) {
		note("the watchdog task", createWatchdogTask(exe))
	}
	for path, sddl := range map[string]string{platform.DataDir: dataSDDL, platform.InstallDir: programSDDL} {
		if cur, err := pathSecurity(path, false); err == nil && !strings.HasPrefix(cur, canonicalSDDL(sddl)) && !lockdownActive() {
			note("the permissions of "+path, SetPathSecurity(path, sddl))
		}
	}
	return fixed
}

// canonicalSDDL is sddl as Windows prints it back.
func canonicalSDDL(sddl string) string {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return sddl
	}
	return sd.String()
}

func serviceSecurity(h windows.Handle) (string, error) {
	sd, err := windows.GetSecurityInfo(h, windows.SE_SERVICE, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	return sd.String(), nil
}

func setServiceSecurity(h windows.Handle, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	owner, _, _ := sd.Owner()
	group, _, _ := sd.Group()
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(h, windows.SE_SERVICE,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION, owner, group, dacl, nil)
}

// SetServiceSecurity gives the installed service KC9K's permissions.
func SetServiceSecurity(h windows.Handle) error { return setServiceSecurity(h, ServiceSDDL) }

// pathSecurity is the owner and DACL of a file, folder or (key) registry
// key, as SDDL.
func pathSecurity(path string, key bool) (string, error) {
	var kind windows.SE_OBJECT_TYPE = windows.SE_FILE_OBJECT
	if key {
		kind = windows.SE_REGISTRY_KEY
	}
	sd, err := windows.GetNamedSecurityInfo(path, kind, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	return sd.String(), nil
}

// SetPathSecurity puts sddl's owner and DACL on path (a file or folder).
func SetPathSecurity(path, sddl string) error { return setSecurity(path, false, sddl) }

func setSecurity(path string, key bool, sddl string) error {
	var kind windows.SE_OBJECT_TYPE = windows.SE_FILE_OBJECT
	if key {
		kind = windows.SE_REGISTRY_KEY
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
	var owner, group *windows.SID
	if o, _, err := sd.Owner(); err == nil && o != nil {
		owner, info = o, info|windows.OWNER_SECURITY_INFORMATION
	}
	if g, _, err := sd.Group(); err == nil && g != nil {
		group, info = g, info|windows.GROUP_SECURITY_INFORMATION
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if control, _, err := sd.Control(); err == nil && control&windows.SE_DACL_PROTECTED != 0 {
		info |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		info |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	return windows.SetNamedSecurityInfo(path, kind, info, owner, group, dacl, nil)
}

// withDeny adds ace at the start of sddl's DACL (deny entries come first),
// keeping only the explicit entries: inherited ones come back by
// themselves.
func withDeny(sddl, ace string) string {
	i := strings.Index(sddl, "D:")
	if i < 0 {
		return sddl + "D:" + ace
	}
	rest := sddl[i+2:]
	flags := ""
	for len(rest) > 0 && rest[0] != '(' && rest[0] != 'S' {
		flags += rest[:1]
		rest = rest[1:]
	}
	end := strings.Index(rest, "S:")
	dacl, sacl := rest, ""
	if end >= 0 {
		dacl, sacl = rest[:end], rest[end:]
	}
	var kept []string
	canon := canonACE(ace)
	for _, a := range splitACEs(dacl) {
		if !strings.Contains(strings.SplitN(a, ";", 3)[1], "ID") && a != ace && a != canon {
			kept = append(kept, a)
		}
	}
	return sddl[:i] + "D:" + flags + ace + strings.Join(kept, "") + sacl
}

func splitACEs(dacl string) []string {
	var out []string
	for len(dacl) > 0 && dacl[0] == '(' {
		end := strings.IndexByte(dacl, ')')
		if end < 0 {
			break
		}
		out = append(out, dacl[:end+1])
		dacl = dacl[end+1:]
	}
	return out
}

// canonACE is ace as Windows prints it back (rights in letters where they
// have them: 0xd0156 reads DCLCRPDTCRSDWDWO), so entries compare.
func canonACE(ace string) string {
	sd, err := windows.SecurityDescriptorFromString("D:" + ace)
	if err != nil {
		return ace
	}
	out := sd.String()
	if i := strings.Index(out, "D:"); i >= 0 {
		out = out[i+2:]
	}
	if i := strings.IndexByte(out, '('); i >= 0 {
		out = out[i:]
	}
	return out
}

// lockdownTarget is one object strict mode denies administrators.
type lockdownTarget struct {
	Path string `json:"path"`
	Key  bool   `json:"key"`
	SDDL string `json:"sddl"` // its security before, to put back
}

// lockdownPath keeps strict mode's targets and their old security.
var lockdownPath = filepath.Join(platform.DataDir, "lockdown.json")

var lockdownMu sync.Mutex

func lockdownActive() bool {
	_, err := os.Stat(lockdownPath)
	return err == nil
}

// lockdownTargets are what strict mode denies administrators.
func lockdownTargets() []lockdownTarget {
	var out []lockdownTarget
	add := func(path string, key bool) { out = append(out, lockdownTarget{Path: path, Key: key}) }
	// The whole Services tree (W20): a service's ImagePath written there
	// runs as SYSTEM at the next boot. Windows Installer's policy key too.
	for _, k := range []string{`SYSTEM\CurrentControlSet\Services\` + serviceName, `SYSTEM\CurrentControlSet\Services`,
		`SYSTEM\CurrentControlSet\Control\SafeBoot`, `BCD00000000`, msiPolicyKey} {
		add(`MACHINE\`+k, true)
	}
	for _, b := range chromiumBrowsers {
		add(`MACHINE\`+b.key, true)
	}
	add(hostsPath, false)
	add(filepath.Join(systemRoot(), "System32", "Recovery", "ReAgent.xml"), false)
	add(platform.InstallDir, false)
	for _, dir := range firefoxInstallDirs() {
		add(filepath.Join(dir, "distribution"), false)
	}
	return out
}

// LockdownRepairs counts deny entries strict mode had to put back (an
// administrator removed them); enforce reads and resets it.
var LockdownRepairs struct {
	sync.Mutex
	What []string
}

// ApplyLockdown puts strict mode's deny entries in place, and back where
// they were removed (user is Linux's; unused here).
func ApplyLockdown(user string) error {
	lockdownMu.Lock()
	defer lockdownMu.Unlock()
	saved := map[string]lockdownTarget{}
	if data, err := os.ReadFile(lockdownPath); err == nil {
		var list []lockdownTarget
		_ = json.Unmarshal(data, &list)
		for _, t := range list {
			saved[t.Path] = t
		}
	}
	fresh := len(saved) == 0
	var list []lockdownTarget
	var firstErr error
	for _, t := range lockdownTargets() {
		cur, err := pathSecurity(t.Path, t.Key)
		if err != nil {
			continue // not there (no Brave policy key, say)
		}
		kind := "dir"
		if t.Key {
			kind = "key"
		} else if info, err := os.Stat(t.Path); err == nil && !info.IsDir() {
			kind = "file"
		}
		if old, ok := saved[t.Path]; ok {
			t.SDDL = old.SDDL
		} else {
			t.SDDL = cur
		}
		list = append(list, t)
		next := cur
		for _, ace := range denyACEs(kind) {
			if !strings.Contains(next, canonACE(ace)) {
				next = withDeny(next, ace)
			}
		}
		if next == cur {
			continue
		}
		if err := setSecurity(t.Path, t.Key, next); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("deny %s: %w", t.Path, err)
			}
			continue
		}
		if !fresh {
			LockdownRepairs.Lock()
			LockdownRepairs.What = append(LockdownRepairs.What, "strict mode's protection of "+t.Path)
			LockdownRepairs.Unlock()
		}
	}
	data, _ := json.Marshal(list)
	if err := os.WriteFile(lockdownPath, data, 0o600); err != nil && firstErr == nil {
		firstErr = err
	}
	if repaired := applyStrictExtras(); len(repaired) > 0 {
		LockdownRepairs.Lock()
		LockdownRepairs.What = append(LockdownRepairs.What, repaired...)
		LockdownRepairs.Unlock()
	}
	return firstErr
}

// RemoveLockdown takes strict mode's deny entries out again. It removes
// only its own entries from the security as it is now (an install or
// Windows may have changed the rest meanwhile); the security found at the
// start is kept in the state file for a person to compare.
func RemoveLockdown() error {
	lockdownMu.Lock()
	defer lockdownMu.Unlock()
	// Strict mode's extras are put back on their own: one that can't be
	// (logged, tried again next cycle) never keeps the deny entries on.
	_ = removeStrictExtras()
	data, err := os.ReadFile(lockdownPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []lockdownTarget
	_ = json.Unmarshal(data, &list)
	var firstErr error
	for _, t := range list {
		cur, err := pathSecurity(t.Path, t.Key)
		if err != nil {
			continue // gone
		}
		next := cur
		for _, kind := range []string{"dir", "file", "key"} {
			for _, ace := range denyACEs(kind) {
				next = withoutACE(next, ace)
			}
		}
		// Entries of the first build (for Administrators, which denied
		// the service too).
		for _, ace := range []string{"(D;OICI;0xd0156;;;BA)", "(D;;0xd0156;;;BA)", "(D;CI;0xd0006;;;BA)"} {
			next = withoutACE(next, ace)
		}
		if next == cur {
			continue
		}
		if err := setSecurity(t.Path, t.Key, next); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("restore %s: %w", t.Path, err)
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return os.Remove(lockdownPath)
}

// withoutACE removes ace from sddl's DACL, and the inherited entries
// (they come back by themselves).
func withoutACE(sddl, ace string) string {
	canon := canonACE(ace)
	if !strings.Contains(sddl, ace) && !strings.Contains(sddl, canon) {
		return sddl
	}
	i := strings.Index(sddl, "D:")
	rest := sddl[i+2:]
	flags := ""
	for len(rest) > 0 && rest[0] != '(' && rest[0] != 'S' {
		flags += rest[:1]
		rest = rest[1:]
	}
	end := strings.Index(rest, "S:")
	dacl, sacl := rest, ""
	if end >= 0 {
		dacl, sacl = rest[:end], rest[end:]
	}
	var kept []string
	for _, a := range splitACEs(dacl) {
		if !strings.EqualFold(a, ace) && a != canon && !strings.Contains(strings.SplitN(a, ";", 3)[1], "ID") {
			kept = append(kept, a)
		}
	}
	return sddl[:i] + "D:" + flags + strings.Join(kept, "") + sacl
}

// LockdownInstalled reports whether strict mode's deny entries are on.
func LockdownInstalled() bool { return lockdownActive() }

// SafeModeBoot reports whether Windows was started in Safe Mode (with or
// without networking): Windows writes SafeBoot\Option only then. A
// variable for tests.
var SafeModeBoot = func() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SafeBoot\Option`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("OptionValue")
	return err == nil && v != 0
}

// KernelCmdline stands for Linux's kernel command line: Windows has none,
// and Safe Mode is its recovery boot (W13, SPEC §6 2026-11-22 By Opus), so
// in Safe Mode it carries kc9k.nofreeze: freezes and strict mode's
// lockdown are skipped for that boot unless no-escape is on.
func KernelCmdline() string {
	if SafeModeBoot() {
		return freeze.BypassParam
	}
	return ""
}

// recoveryMenuOn keeps a way to Safe Mode while the recovery environment
// is off and no-escape isn't on (R-ESCAPE-1): the F8 boot menu.
func recoveryMenuOn(on bool) error {
	policy := "Standard"
	if on {
		policy = "Legacy"
	}
	_, err := bcdedit("/set", "{current}", "bootmenupolicy", policy)
	return err
}

// taskExists reports whether the scheduled task exists: its file in the
// Tasks folder (no schtasks process each cycle), else schtasks' answer.
func taskExists(name string) bool {
	file := filepath.Join(systemRoot(), "System32", "Tasks", strings.TrimPrefix(name, `\`))
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return exec.Command(filepath.Join(systemRoot(), "System32", "schtasks.exe"), "/Query", "/TN", name).Run() == nil
}

// watchdogXML is the task: as SYSTEM, at startup and every minute, start
// the service if it isn't running; administrators may read it, not change
// or delete it.
func watchdogXML(exe string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Starts the Killer Chicken 9000 service again if it was stopped.</Description>
    <SecurityDescriptor>D:P(A;;FA;;;SY)(A;;FR;;;BA)(A;;FR;;;AU)</SecurityDescriptor>
  </RegistrationInfo>
  <Triggers>
    <BootTrigger><Enabled>true</Enabled></BootTrigger>
    <TimeTrigger>
      <Repetition><Interval>PT1M</Interval><StopAtDurationEnd>false</StopAtDurationEnd></Repetition>
      <StartBoundary>2020-01-01T00:00:00</StartBoundary>
      <Enabled>true</Enabled>
    </TimeTrigger>
  </Triggers>
  <Principals><Principal id="System"><UserId>S-1-5-18</UserId><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <StartWhenAvailable>true</StartWhenAvailable>
    <ExecutionTimeLimit>PT5M</ExecutionTimeLimit>
    <Hidden>false</Hidden>
    <Enabled>true</Enabled>
  </Settings>
  <Actions Context="System"><Exec><Command>` + xmlEscape(exe) + `</Command><Arguments>watchdog</Arguments></Exec></Actions>
</Task>`
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// createWatchdogTask registers the watchdog task.
func createWatchdogTask(exe string) error {
	f, err := os.CreateTemp(platform.DataDir, "watchdog-*.xml")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	// schtasks reads the XML as UTF-16 with a byte order mark.
	u := windows.StringToUTF16(watchdogXML(exe))
	buf := []byte{0xFF, 0xFE}
	for _, c := range u[:len(u)-1] {
		buf = append(buf, byte(c), byte(c>>8))
	}
	if _, err := f.Write(buf); err != nil {
		f.Close()
		return err
	}
	f.Close()
	out, err := exec.Command(filepath.Join(systemRoot(), "System32", "schtasks.exe"), "/Create", "/TN", watchdogTask, "/XML", name, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveWatchdogTask deletes the watchdog task (uninstall).
func RemoveWatchdogTask() error {
	if !taskExists(watchdogTask) {
		return nil
	}
	out, err := exec.Command(filepath.Join(systemRoot(), "System32", "schtasks.exe"), "/Delete", "/TN", watchdogTask, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveSafeBoot takes the service out of Safe Mode (uninstall).
func RemoveSafeBoot() {
	for _, mode := range []string{"Minimal", "Network"} {
		_ = registry.DeleteKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SafeBoot\`+mode+`\`+serviceName)
	}
}

// bootLockPath keeps the boot settings no-escape found, to put back.
var bootLockPath = filepath.Join(platform.DataDir, "bootlock.json")

type bootState struct {
	RecoveryWasOn bool `json:"recovery_was_on"`
}

func bcdedit(args ...string) (string, error) {
	out, err := exec.Command(filepath.Join(systemRoot(), "System32", "bcdedit.exe"), args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("bcdedit %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func reagentc(arg string) (string, error) {
	out, err := exec.Command(filepath.Join(systemRoot(), "System32", "ReAgentc.exe"), arg).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("reagentc %s: %v: %s", arg, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// recoveryOn reports whether the recovery environment is enabled (its
// configuration file says so; reagentc's answer is translated).
func recoveryOn() bool {
	data, err := os.ReadFile(filepath.Join(systemRoot(), "System32", "Recovery", "ReAgent.xml"))
	if err != nil {
		return false
	}
	text := strings.ToLower(string(data))
	return strings.Contains(text, `<winreinstalled state="1"`)
}

// ApplyBootLock turns the ways around a lock at boot off: the recovery
// environment (Reset this PC, Startup Repair, its command prompt), the
// automatic repair that opens it, and the boot menu. KC9K's service also
// runs in Safe Mode, so Safe Mode is no way around.
func ApplyBootLock() error {
	state := bootState{RecoveryWasOn: recoveryOn()}
	if _, err := os.Stat(bootLockPath); err != nil {
		data, _ := json.Marshal(state)
		if err := os.WriteFile(bootLockPath, data, 0o600); err != nil {
			return err
		}
	}
	if state.RecoveryWasOn {
		if _, err := reagentc("/disable"); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"/set", "{current}", "recoveryenabled", "No"},
		{"/set", "{current}", "bootstatuspolicy", "IgnoreAllFailures"},
		{"/set", "{bootmgr}", "displaybootmenu", "No"},
		{"/set", "{current}", "bootmenupolicy", "Standard"},
	} {
		if _, err := bcdedit(args...); err != nil {
			return err
		}
	}
	return nil
}

// RemoveBootLock puts the boot settings back.
func RemoveBootLock() error {
	var state bootState
	data, err := os.ReadFile(bootLockPath)
	if err != nil {
		return nil
	}
	_ = json.Unmarshal(data, &state)
	for _, args := range [][]string{
		{"/set", "{current}", "recoveryenabled", "Yes"},
		{"/deletevalue", "{current}", "bootstatuspolicy"},
	} {
		if _, err := bcdedit(args...); err != nil {
			return err
		}
	}
	if _, err := os.Stat(recoveryGuardPath); err == nil {
		// Factory reset protection still keeps the recovery environment
		// off: Safe Mode stays reachable by F8.
		if err := recoveryMenuOn(true); err != nil {
			return err
		}
	} else if state.RecoveryWasOn {
		if _, err := reagentc("/enable"); err != nil {
			return err
		}
	}
	return os.Remove(bootLockPath)
}

// BootLockInstalled reports whether no-escape's boot settings are on.
func BootLockInstalled() bool {
	_, err := os.Stat(bootLockPath)
	return err == nil
}

// userProgram reports whether a process runs from where downloaded and
// portable programs land: a profile's own folders (Downloads, Desktop,
// Documents… anything but AppData), its temporary folder, or a removable
// drive. Programs installed per user (AppData\Local\Programs, OneDrive,
// Teams, a per-user Chrome) keep running: they were installed, not
// brought along (SPEC §6 2026-11-21, By Opus).
func userProgram(p *winProc, exempt []string) bool {
	if p.Exe == "" {
		return false
	}
	lower := strings.ToLower(filepath.Clean(p.Exe))
	for _, e := range exempt {
		if e != "" && strings.HasPrefix(lower, strings.ToLower(filepath.Clean(e))+`\`) {
			return false
		}
	}
	users := strings.ToLower(filepath.Join(filepath.VolumeName(systemRoot())+`\`, "Users")) + `\`
	if rest, ok := strings.CutPrefix(lower, users); ok {
		parts := strings.Split(rest, `\`)
		if len(parts) < 3 { // a program right in a profile folder
			return len(parts) == 2
		}
		if parts[1] != "appdata" {
			return true // Downloads, Desktop, Documents, …
		}
		return parts[2] == "local" && len(parts) > 3 && parts[3] == "temp"
	}
	vol := filepath.VolumeName(p.Exe)
	if vol == "" {
		return false
	}
	root, _ := windows.UTF16PtrFromString(vol + `\`)
	return windows.GetDriveType(root) == windows.DRIVE_REMOVABLE
}

// ApplyNoexec: Windows mounts nothing; CloseNoexecPrograms does the work.
func ApplyNoexec(exempt ...string) error { return nil }

// RemoveNoexec: nothing to undo.
func RemoveNoexec(exempt ...string) error { return nil }

// CloseNoexecPrograms closes people's programs run from where downloaded
// and portable programs land (userProgram), except in exempt folders.
func CloseNoexecPrograms(exempt []string) int {
	procs := processSnapshot()
	protected := protectedPIDs(procs)
	n := 0
	for _, p := range procs {
		if p.person() && !protected[p.PID] && userProgram(p, exempt) && killProcess(p.PID) == nil {
			log.Printf("strict: closed %s (pid %d): run from %s", p.Name, p.PID, filepath.Dir(p.Exe))
			n++
		}
	}
	return n
}

// ExemptFolderError checks a folder to exempt: a folder in a profile.
func ExemptFolderError(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("give the folder's full path")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}
	return nil
}

// UnexemptFolder: nothing was mounted for it.
func UnexemptFolder(dir string) error { return nil }

// recoveryGuardPath notes that factory reset protection turned the
// recovery environment off (and whether it was on before).
var recoveryGuardPath = filepath.Join(platform.DataDir, "recovery-guard.json")

// SyncRecoveryGuard turns the recovery environment (Reset this PC, its
// command prompt) off while factory reset protection acts, and back on
// after; it reports a change made by hand while it acts.
func SyncRecoveryGuard(on bool) (fixed bool, err error) {
	_, statErr := os.Stat(recoveryGuardPath)
	active := statErr == nil
	switch {
	case on && !active:
		state := bootState{RecoveryWasOn: recoveryOn()}
		data, _ := json.Marshal(state)
		if err := os.WriteFile(recoveryGuardPath, data, 0o600); err != nil {
			return false, err
		}
		if state.RecoveryWasOn {
			_, err = reagentc("/disable")
		}
		if err == nil && !BootLockInstalled() {
			err = recoveryMenuOn(true)
		}
		return false, err
	case on && active:
		if recoveryOn() {
			_, err = reagentc("/disable")
			return err == nil, err
		}
		return false, nil
	case !on && active:
		var state bootState
		if data, err := os.ReadFile(recoveryGuardPath); err == nil {
			_ = json.Unmarshal(data, &state)
		}
		if !BootLockInstalled() {
			if err := recoveryMenuOn(false); err != nil {
				return false, err
			}
			if state.RecoveryWasOn {
				if _, err := reagentc("/enable"); err != nil {
					return false, err
				}
			}
		}
		return false, os.Remove(recoveryGuardPath)
	}
	return false, nil
}
