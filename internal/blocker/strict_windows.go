package blocker

// strict_windows.go — what strict mode stops on Windows beyond its deny
// entries (ROADMAP W20–W22; SPEC §6 2026-11-22 By Opus). Linux's strict
// mode takes root away (sudo, pkexec, root logins, container sockets);
// Windows' administrator becomes SYSTEM other ways, and SYSTEM is never
// denied, so while strict mode holds:
//
//   - W20: the service manager refuses people's logons creating services
//     (sc create, PsExec, drivers) or changing boot settings; every service
//     refuses them changing its configuration, deleting it or its
//     permissions, and a few KC9K relies on refuse being stopped; the
//     Services key tree is denied like KC9K's own (lockdownTargets); Task
//     Scheduler's root folder refuses them registering tasks, and tasks
//     made meanwhile to run as SYSTEM or a service account are deleted and
//     counted; WMI's subscription namespace refuses them writing, and
//     event consumers made meanwhile are deleted and counted; Windows
//     Installer is off (DisableMSI 2: strict mode is Linux's "no sudo
//     apt").
//   - W21: the privileges that pass by deny entries or reach SYSTEM
//     (backup, restore, take ownership, debug, drivers, firmware
//     variables, volumes) are removed from every person's running
//     process's token, and from Administrators' and Backup Operators' user
//     rights (from the next sign-in); the rights are given back afterwards
//     (running tokens keep the loss until their next sign-in).
//   - W22: accounts made or turned on meanwhile are turned off and counted,
//     and turned on again afterwards if strict mode turned them off.
//
// Everything is put back by removeStrictExtras; its state is
// ProgramData\kc9k\strict-extras.json.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/mgr"
)

// strictState is what strict mode's extras found and changed.
type strictState struct {
	Since      int64               `json:"since"`
	MSIBefore  *uint64             `json:"msi_before,omitempty"` // DisableMSI before; nil: none
	Consumers  []string            `json:"consumers"`            // WMI event consumers at the start
	Accounts   map[string]bool     `json:"accounts"`             // name -> disabled, at the start
	TurnedOff  []string            `json:"turned_off"`           // accounts strict mode turned off
	Rights     map[string][]string `json:"rights"`               // SID -> user rights removed
	RightsDone bool                `json:"rights_done"`
}

var (
	strictStatePath = filepath.Join(platform.DataDir, "strict-extras.json")
	strictMu        sync.Mutex
	strictHeavyAt   time.Time
	strippedTokens  = map[uint32]time.Time{} // pid -> start time, stripped
)

func loadStrictState() (strictState, bool) {
	var s strictState
	data, err := os.ReadFile(strictStatePath)
	if err != nil {
		return s, false
	}
	_ = json.Unmarshal(data, &s)
	return s, true
}

func saveStrictState(s strictState) error {
	data, _ := json.Marshal(s)
	return os.WriteFile(strictStatePath, data, 0o600)
}

// strictDenyWho are the logons strict mode denies (denyWho).
func strictDenies(mask, flags string) []string {
	var out []string
	for _, who := range denyWho {
		out = append(out, "(D;"+flags+";"+mask+";;;"+who+")")
	}
	return out
}

const (
	// SC_MANAGER_CREATE_SERVICE, SC_MANAGER_MODIFY_BOOT_CONFIG, WRITE_DAC,
	// WRITE_OWNER.
	scmDenyMask = "0xc0022"
	// SERVICE_CHANGE_CONFIG, DELETE, WRITE_DAC, WRITE_OWNER.
	serviceDenyMask = "0xd0002"
	// ... and SERVICE_STOP, SERVICE_PAUSE_CONTINUE for the services below.
	serviceDenyStopMask = "0xd0062"
	// Task folders: write data, append, write extended attributes and
	// attributes, delete, WRITE_DAC, WRITE_OWNER.
	taskFolderDenyMask = "0xd0116"
	// WMI namespaces: full and partial write, write provider, WRITE_DAC,
	// WRITE_OWNER.
	wmiDenyMask = "0xc001c"
)

// strictUnstoppable are services KC9K relies on, which strict mode keeps
// people from stopping: the firewall engine, DNS client, WMI, Task
// Scheduler, the event log, Wi-Fi (places) and time.
var strictUnstoppable = map[string]bool{"bfe": true, "dnscache": true, "winmgmt": true, "schedule": true,
	"eventlog": true, "wlansvc": true, "w32time": true}

// strictPrivileges pass by deny entries or reach SYSTEM.
var strictPrivileges = []string{"SeBackupPrivilege", "SeRestorePrivilege", "SeTakeOwnershipPrivilege",
	"SeDebugPrivilege", "SeLoadDriverPrivilege", "SeSystemEnvironmentPrivilege", "SeManageVolumePrivilege",
	"SeRelabelPrivilege", "SeTcbPrivilege", "SeCreateTokenPrivilege", "SeAssignPrimaryTokenPrivilege"}

// strictRightsGroups lose those rights: Administrators, Backup Operators.
var strictRightsGroups = []windows.WELL_KNOWN_SID_TYPE{windows.WinBuiltinAdministratorsSid, windows.WinBuiltinBackupOperatorsSid}

// applyStrictExtras puts strict mode's Windows extras in place, and back
// where they were undone; it returns what it put back (bypasses).
func applyStrictExtras() []string {
	if !platform.Privileged() {
		return nil
	}
	strictMu.Lock()
	defer strictMu.Unlock()
	s, ok := loadStrictState()
	fresh := !ok
	if fresh {
		s = strictState{Since: time.Now().Unix(), Accounts: map[string]bool{}, Rights: map[string][]string{}}
		if v, err := regDWORD(msiPolicyKey, "DisableMSI"); err == nil {
			s.MSIBefore = &v
		}
		s.Consumers, _ = wmiConsumers()
		if list, err := winapi.LocalAccounts(); err == nil {
			for _, a := range list {
				s.Accounts[a.Name] = a.Disabled
			}
		}
		if err := saveStrictState(s); err != nil {
			log.Printf("strict: %v", err)
			return nil
		}
	}
	var fixed []string
	note := func(what string, repaired bool, err error) {
		if err != nil {
			log.Printf("strict: %s: %v", what, err)
		} else if repaired && !fresh {
			fixed = append(fixed, what)
		}
	}
	// Every cycle: what is cheap.
	changed, err := denySCM(true)
	note("strict mode's protection of the service manager", changed, err)
	changed, err = ensureMSIOff()
	note("strict mode's Windows Installer policy", changed, err)
	stripPrivileges()
	if !s.RightsDone {
		s.RightsDone = true
		note("Administrators' user rights", false, removeStrictRights(&s))
	}
	off := strictAccounts(&s)
	for _, name := range off {
		fixed = append(fixed, "the account "+name+" made or turned on during strict mode (turned off)")
	}
	// Every 30 s: what walks many objects.
	if fresh || time.Since(strictHeavyAt) > 30*time.Second {
		strictHeavyAt = time.Now()
		n, err := denyServices(true)
		note("strict mode's protection of services", n > 0, err)
		changed, err = denyTaskFolder(true)
		note("strict mode's protection of Task Scheduler", changed, err)
		for _, task := range systemTasksSince(s.Since) {
			if err := deleteTask(task); err != nil {
				log.Printf("strict: deleting task %s: %v", task, err)
				continue
			}
			fixed = append(fixed, "the task "+task+" made to run as SYSTEM (deleted)")
		}
		changed, err = denyWMI(true)
		note("strict mode's protection of WMI subscriptions", changed, err)
		if now, err := wmiConsumers(); err == nil {
			for _, c := range newItems(s.Consumers, now) {
				if err := deleteWMIConsumer(c); err != nil {
					log.Printf("strict: deleting WMI consumer %s: %v", c, err)
					continue
				}
				fixed = append(fixed, "the WMI event consumer "+c+" (deleted)")
			}
		}
	}
	if err := saveStrictState(s); err != nil {
		log.Printf("strict: %v", err)
	}
	return fixed
}

// removeStrictExtras undoes applyStrictExtras.
func removeStrictExtras() error {
	strictMu.Lock()
	defer strictMu.Unlock()
	s, ok := loadStrictState()
	if !ok {
		return nil
	}
	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	_, err := denySCM(false)
	keep(err)
	_, err = denyServices(false)
	keep(err)
	_, err = denyTaskFolder(false)
	keep(err)
	_, err = denyWMI(false)
	keep(err)
	keep(restoreMSI(s.MSIBefore))
	keep(restoreStrictRights(&s))
	for _, name := range s.TurnedOff {
		keep(winapi.SetAccountDisabled(name, false))
	}
	if firstErr != nil {
		log.Printf("strict: putting back: %v", firstErr)
		return firstErr
	}
	strippedTokens = map[uint32]time.Time{}
	return os.Remove(strictStatePath)
}

func newItems(before, now []string) []string {
	had := map[string]bool{}
	for _, b := range before {
		had[b] = true
	}
	var out []string
	for _, n := range now {
		if !had[n] {
			out = append(out, n)
		}
	}
	return out
}

// addDenies adds the deny entries to a DACL-only SDDL; removeDenies takes
// them out.
func addDenies(sddl string, aces []string) string {
	for _, ace := range aces {
		if !strings.Contains(sddl, ace) && !strings.Contains(sddl, canonACE(ace)) {
			sddl = withDeny(sddl, ace)
		}
	}
	return sddl
}

func removeDenies(sddl string, aces []string) string {
	for _, ace := range aces {
		sddl = withoutACE(sddl, ace)
	}
	return sddl
}

func handleDACL(h windows.Handle, kind windows.SE_OBJECT_TYPE) (string, error) {
	sd, err := windows.GetSecurityInfo(h, kind, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	return sd.String(), nil
}

func setHandleDACL(h windows.Handle, kind windows.SE_OBJECT_TYPE, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(h, kind, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// changeDACL adds (on) or removes deny entries on a handle's DACL; it
// reports whether it changed anything.
func changeDACL(h windows.Handle, kind windows.SE_OBJECT_TYPE, aces []string, on bool) (bool, error) {
	cur, err := handleDACL(h, kind)
	if err != nil {
		return false, err
	}
	next := removeDenies(cur, aces)
	if on {
		next = addDenies(cur, aces)
	}
	if next == cur {
		return false, nil
	}
	return true, setHandleDACL(h, kind, next)
}

// denySCM: the service manager (W20).
func denySCM(on bool) (bool, error) {
	h, err := windows.OpenSCManager(nil, nil, windows.READ_CONTROL|windows.WRITE_DAC)
	if err != nil {
		return false, err
	}
	defer windows.CloseServiceHandle(h)
	return changeDACL(h, windows.SE_SERVICE, strictDenies(scmDenyMask, ""), on)
}

// denyServices: every service but KC9K's own (GuardService keeps its
// permissions) and those whose permissions SYSTEM can't change (protected
// services). It returns how many it changed.
func denyServices(on bool) (int, error) {
	m, err := mgr.Connect()
	if err != nil {
		return 0, err
	}
	defer m.Disconnect()
	names, err := m.ListServices()
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, name := range names {
		if strings.EqualFold(name, serviceName) {
			continue
		}
		mask := serviceDenyMask
		if strictUnstoppable[strings.ToLower(name)] {
			mask = serviceDenyStopMask
		}
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		h, err := windows.OpenService(m.Handle, p, windows.READ_CONTROL|windows.WRITE_DAC)
		if err != nil {
			continue
		}
		aces := strictDenies(mask, "")
		if !on { // either mask may be on it
			aces = append(aces, strictDenies(serviceDenyStopMask, "")...)
			aces = append(aces, strictDenies(serviceDenyMask, "")...)
		}
		if ok, err := changeDACL(h, windows.SE_SERVICE, aces, on); err == nil && ok {
			changed++
		}
		windows.CloseServiceHandle(h)
	}
	return changed, nil
}

// powershellIn runs a PowerShell script with stdin; a variable for tests.
var powershellIn = func(script string, stdin string) ([]byte, error) {
	ps := filepath.Join(systemRoot(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// changeSDDLBy reads an SDDL with get, adds or removes the deny entries,
// and writes it back with set (the SDDL on stdin) when it changed.
func changeSDDLBy(get, set string, aces []string, on bool) (bool, error) {
	out, err := powershellIn(get, "")
	if err != nil {
		return false, err
	}
	cur := strings.TrimSpace(string(out))
	if !strings.Contains(cur, "D:") {
		return false, fmt.Errorf("no DACL read: %q", cur)
	}
	next := removeDenies(cur, aces)
	if on {
		next = addDenies(cur, aces)
	}
	if next == cur {
		return false, nil
	}
	_, err = powershellIn(set, next)
	return true, err
}

// denyTaskFolder: Task Scheduler's root folder, whose permissions it
// checks when a task is registered in it or below (W20).
func denyTaskFolder(on bool) (bool, error) {
	const get = `$s = New-Object -ComObject Schedule.Service; $s.Connect(); $s.GetFolder('\').GetSecurityDescriptor(4)`
	const set = `$sddl = [Console]::In.ReadToEnd().Trim(); $s = New-Object -ComObject Schedule.Service; $s.Connect(); $s.GetFolder('\').SetSecurityDescriptor($sddl, 0)`
	return changeSDDLBy(get, set, strictDenies(taskFolderDenyMask, "OICI"), on)
}

// denyWMI: the namespace where permanent event subscriptions live (W20).
func denyWMI(on bool) (bool, error) {
	const get = `$s = Get-WmiObject -Namespace root/subscription -Class __SystemSecurity; $b = $s.GetSD().SD; ([wmiclass]'root/cimv2:Win32_SecurityDescriptorHelper').BinarySDToSDDL($b).SDDL`
	const set = `$sddl = [Console]::In.ReadToEnd().Trim(); $b = ([wmiclass]'root/cimv2:Win32_SecurityDescriptorHelper').SDDLToBinarySD($sddl).BinarySD; $s = Get-WmiObject -Namespace root/subscription -Class __SystemSecurity; $r = $s.SetSD($b); if ($r.ReturnValue -ne 0) { throw "SetSD $($r.ReturnValue)" }`
	return changeSDDLBy(get, set, strictDenies(wmiDenyMask, "CI"), on)
}

// wmiConsumers lists the permanent event consumers.
func wmiConsumers() ([]string, error) {
	out, err := powershellIn(`Get-WmiObject -Namespace root/subscription -Class __EventConsumer | ForEach-Object { $_.__RELPATH }`, "")
	if err != nil {
		return nil, err
	}
	var list []string
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r", ""), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			list = append(list, line)
		}
	}
	return list, nil
}

// deleteWMIConsumer deletes a consumer and the bindings to it.
func deleteWMIConsumer(relpath string) error {
	_, err := powershellIn(`$p = [Console]::In.ReadToEnd().Trim()
Get-WmiObject -Namespace root/subscription -Class __FilterToConsumerBinding | Where-Object { $_.Consumer -like "*$p" } | Remove-WmiObject
Get-WmiObject -Namespace root/subscription -Class __EventConsumer | Where-Object { $_.__RELPATH -eq $p } | Remove-WmiObject`, relpath)
	return err
}

// serviceAccounts are the principals a task runs as SYSTEM or a service
// account by.
var serviceAccounts = []string{"S-1-5-18", "S-1-5-19", "S-1-5-20", "SYSTEM", "LOCALSYSTEM", "LOCAL SERVICE", "NETWORK SERVICE",
	`NT AUTHORITY\SYSTEM`, `NT AUTHORITY\LOCALSERVICE`, `NT AUTHORITY\NETWORKSERVICE`}

// systemTasksSince lists tasks (Task Scheduler paths) written after since
// that run as SYSTEM or a service account, outside Windows' own folder
// (\Microsoft\Windows, which Windows Update writes) and KC9K's.
func systemTasksSince(since int64) []string {
	root := filepath.Join(systemRoot(), "System32", "Tasks")
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.ModTime().Unix() < since {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		low := strings.ToLower(rel)
		if strings.HasPrefix(low, `microsoft\windows\`) || strings.HasPrefix(low, `kc9k\`) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !taskRunsAsService(decodeTaskXML(data)) {
			return nil
		}
		out = append(out, `\`+rel)
		return nil
	})
	return out
}

// decodeTaskXML reads a task file (UTF-16 with a byte order mark, or UTF-8).
func decodeTaskXML(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		u := make([]uint16, (len(data)-2)/2)
		for i := range u {
			u[i] = uint16(data[2+2*i]) | uint16(data[3+2*i])<<8
		}
		return string(utf16.Decode(u))
	}
	return string(data)
}

// taskRunsAsService reports whether a task's XML has a principal that is
// SYSTEM or a service account.
func taskRunsAsService(xml string) bool {
	upper := strings.ToUpper(xml)
	for {
		i := strings.Index(upper, "<USERID>")
		if i < 0 {
			return false
		}
		upper = upper[i+len("<USERID>"):]
		end := strings.Index(upper, "</USERID>")
		if end < 0 {
			return false
		}
		who := strings.TrimSpace(upper[:end])
		for _, a := range serviceAccounts {
			if who == strings.ToUpper(a) {
				return true
			}
		}
	}
}

func deleteTask(name string) error {
	out, err := exec.Command(filepath.Join(systemRoot(), "System32", "schtasks.exe"), "/Delete", "/TN", name, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Windows Installer's policy (W20).
const msiPolicyKey = `SOFTWARE\Policies\Microsoft\Windows\Installer`

func regDWORD(path, name string) (uint64, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(name)
	return v, err
}

// ensureMSIOff sets DisableMSI to 2 (always off); it reports a change.
func ensureMSIOff() (bool, error) {
	if v, err := regDWORD(msiPolicyKey, "DisableMSI"); err == nil && v == 2 {
		return false, nil
	}
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, msiPolicyKey, registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	return true, k.SetDWordValue("DisableMSI", 2)
}

func restoreMSI(before *uint64) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, msiPolicyKey, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	if before == nil {
		if err := k.DeleteValue("DisableMSI"); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetDWordValue("DisableMSI", uint32(*before))
}

// privilegeLUIDs are strictPrivileges' local IDs.
var privilegeLUIDs = sync.OnceValue(func() map[windows.LUID]bool {
	out := map[windows.LUID]bool{}
	for _, name := range strictPrivileges {
		var luid windows.LUID
		p, _ := windows.UTF16PtrFromString(name)
		if windows.LookupPrivilegeValue(nil, p, &luid) == nil {
			out[luid] = true
		}
	}
	return out
})

// stripPrivileges removes strictPrivileges from every person's running
// process's token (W21), once per process.
func stripPrivileges() {
	procs := processSnapshot()
	for pid, at := range strippedTokens {
		if p := procs[pid]; p == nil || !p.Started.Equal(at) {
			delete(strippedTokens, pid)
		}
	}
	for _, p := range procs {
		if !p.person() {
			continue
		}
		if at, done := strippedTokens[p.PID]; done && at.Equal(p.Started) {
			continue
		}
		if err := stripToken(p.PID); err == nil {
			strippedTokens[p.PID] = p.Started
		}
	}
}

func stripToken(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, &tok); err != nil {
		return err
	}
	defer tok.Close()
	var n uint32
	_ = windows.GetTokenInformation(tok, windows.TokenPrivileges, nil, 0, &n)
	if n == 0 {
		return fmt.Errorf("no privileges read")
	}
	buf := make([]byte, n)
	if err := windows.GetTokenInformation(tok, windows.TokenPrivileges, &buf[0], n, &n); err != nil {
		return err
	}
	privs := (*windows.Tokenprivileges)(unsafe.Pointer(&buf[0]))
	want := privilegeLUIDs()
	for _, la := range privs.AllPrivileges() {
		if !want[la.Luid] {
			continue
		}
		state := windows.Tokenprivileges{PrivilegeCount: 1}
		state.Privileges[0] = windows.LUIDAndAttributes{Luid: la.Luid, Attributes: windows.SE_PRIVILEGE_REMOVED}
		if err := windows.AdjustTokenPrivileges(tok, false, &state, 0, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

// removeStrictRights takes strictPrivileges from the groups' user rights,
// noting which each had (W21).
func removeStrictRights(s *strictState) error {
	var firstErr error
	for _, g := range strictRightsGroups {
		sid, err := windows.CreateWellKnownSid(g)
		if err != nil {
			continue
		}
		have, err := winapi.AccountRights(sid)
		if err != nil {
			firstErr = err
			continue
		}
		var take []string
		for _, r := range have {
			for _, p := range strictPrivileges {
				if r == p {
					take = append(take, r)
				}
			}
		}
		if len(take) == 0 {
			continue
		}
		if err := winapi.SetAccountRights(sid, take, false); err != nil {
			firstErr = err
			continue
		}
		s.Rights[sid.String()] = append(s.Rights[sid.String()], take...)
	}
	return firstErr
}

func restoreStrictRights(s *strictState) error {
	for sidText, rights := range s.Rights {
		sid, err := windows.StringToSid(sidText)
		if err != nil {
			continue
		}
		if err := winapi.SetAccountRights(sid, rights, true); err != nil {
			return err
		}
		delete(s.Rights, sidText)
	}
	return nil
}

// strictAccounts turns off accounts made, or turned on, since strict mode
// began (W22), and returns their names.
func strictAccounts(s *strictState) []string {
	list, err := winapi.LocalAccounts()
	if err != nil {
		return nil
	}
	var off []string
	for _, a := range list {
		if a.Disabled {
			continue
		}
		wasDisabled, known := s.Accounts[a.Name]
		if known && !wasDisabled {
			continue
		}
		if err := winapi.SetAccountDisabled(a.Name, true); err != nil {
			log.Printf("strict: turning off account %s: %v", a.Name, err)
			continue
		}
		off = append(off, a.Name)
		if !known {
			s.TurnedOff = append(s.TurnedOff, a.Name) // made meanwhile: on again after
		}
	}
	return off
}
