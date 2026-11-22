package blocker

// noexec_windows.go — strict mode's noexec on Windows (ROADMAP W25; SPEC
// §6 2026-11-22 By Opus): Software Restriction Policies, kept in the
// registry only, so Windows Home has them (AppLocker is Pro-only). While
// strict mode holds, programs may not start from where downloaded and
// portable programs land, as userProgram tells them: each profile's own
// folders but AppData (its temporary folder included), the Public profile
// and removable drives; exempt folders may. Windows' own Temp stays open:
// SYSTEM's programs (KC9K's PowerShell too) use it.
// Rules apply to every user, administrators included. KC9K's rules are
// told by their key names ({4B43394B-…}); SRP set up by someone else keeps
// its own default level and rules. CloseNoexecPrograms still closes what
// runs already.

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	saferKey       = `SOFTWARE\Policies\Microsoft\Windows\Safer`
	saferCodeIDs   = saferKey + `\CodeIdentifiers`
	saferDisallow  = "0"
	saferAllow     = "262144"
	srpRulePrefix  = "{4B43394B-"
	srpDescription = "Killer Chicken 9000 strict mode"
)

// srpMadePath notes that KC9K made the Safer key (so it removes it all).
var srpMadePath = filepath.Join(platform.DataDir, "srp-made")

// srpRule is one path rule: its level (saferDisallow or saferAllow).
type srpRule struct{ Path, Level string }

// srpRules are the rules strict mode wants; profiles and drives are read
// now. A variable for tests.
var srpRules = func(exempt []string) []srpRule {
	var out []srpRule
	users := filepath.Join(filepath.VolumeName(systemRoot())+`\`, "Users")
	entries, _ := os.ReadDir(users)
	for _, e := range entries {
		if !e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := strings.ToLower(e.Name())
		if name == "default" || name == "default user" || name == "all users" {
			continue
		}
		profile := filepath.Join(users, e.Name())
		out = append(out, srpRule{profile, saferDisallow})
		if name != "public" {
			out = append(out, srpRule{filepath.Join(profile, "AppData"), saferAllow},
				srpRule{filepath.Join(profile, "AppData", "Local", "Temp"), saferDisallow})
		}
	}
	for _, drive := range removableDrives() {
		out = append(out, srpRule{drive, saferDisallow})
	}
	for _, e := range exempt {
		if e != "" {
			out = append(out, srpRule{longPath(filepath.Clean(e)), saferAllow})
		}
	}
	return out
}

// removableDrives are the removable drives' roots ("E:\").
func removableDrives() []string {
	var out []string
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, _ := windows.UTF16PtrFromString(root)
		if windows.GetDriveType(p) == windows.DRIVE_REMOVABLE {
			out = append(out, root)
		}
	}
	return out
}

// srpRuleName is a rule's key name: KC9K's prefix and a hash of its path
// and level, so the same rule keeps its name.
func srpRuleName(r srpRule) string {
	sum := sha1.Sum([]byte(strings.ToLower(r.Path) + "|" + r.Level))
	h := strings.ToUpper(hex.EncodeToString(sum[:]))
	return srpRulePrefix + h[0:4] + "-" + h[4:8] + "-" + h[8:12] + "-" + h[12:24] + "}"
}

// applySRP puts strict mode's rules in place (every cycle: drives and
// profiles come and go).
func applySRP(exempt []string) error {
	if !platform.Privileged() {
		return nil
	}
	if err := ensureSRPOn(); err != nil {
		return err
	}
	want := map[string]srpRule{}
	for _, r := range srpRules(exempt) {
		want[srpRuleName(r)] = r
	}
	changed := false
	defer func() {
		if changed {
			refreshPolicy()
		}
	}()
	have := map[string]string{} // name -> level
	for _, level := range []string{saferDisallow, saferAllow} {
		for _, name := range srpOurRules(level) {
			have[name] = level
		}
	}
	for name, level := range have {
		if r, ok := want[name]; !ok || r.Level != level {
			changed = true
			if err := registry.DeleteKey(registry.LOCAL_MACHINE, saferCodeIDs+`\`+level+`\Paths\`+name); err != nil {
				return err
			}
		}
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := have[name]; ok {
			continue
		}
		r := want[name]
		changed = true
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, saferCodeIDs+`\`+r.Level+`\Paths\`+name, registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("SRP rule %s: %w", r.Path, err)
		}
		err = k.SetExpandStringValue("ItemData", r.Path)
		if err == nil {
			err = k.SetDWordValue("SaferFlags", 0)
		}
		if err == nil {
			err = k.SetStringValue("Description", srpDescription)
		}
		k.Close()
		if err != nil {
			return fmt.Errorf("SRP rule %s: %w", r.Path, err)
		}
	}
	return nil
}

// srpOurRules lists KC9K's rules of a level.
func srpOurRules(level string) []string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, saferCodeIDs+`\`+level+`\Paths`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadSubKeyNames(-1)
	var out []string
	for _, n := range names {
		if strings.HasPrefix(strings.ToUpper(n), srpRulePrefix) {
			out = append(out, n)
		}
	}
	return out
}

// srpMade is what KC9K added to make SRP act: the key itself, or the
// values it lacked (Windows Server ships the key with only
// authenticodeenabled, which leaves SRP off).
type srpMade struct {
	Key    bool     `json:"key"`
	Values []string `json:"values"`
}

// ensureSRPOn sets what SRP needs to act and is missing: unrestricted by
// default, programs (not DLLs) checked, for every user. Someone else's
// settings are left as they are.
func ensureSRPOn() error {
	var made srpMade
	if data, err := os.ReadFile(srpMadePath); err == nil {
		_ = json.Unmarshal(data, &made)
	}
	k, existed, err := registry.CreateKey(registry.LOCAL_MACHINE, saferCodeIDs, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()
	if !existed {
		made.Key = true
	}
	added := false
	for _, v := range []struct {
		name  string
		value uint32
	}{{"DefaultLevel", 262144}, {"TransparentEnabled", 1}, {"PolicyScope", 0}} {
		if _, _, err := k.GetIntegerValue(v.name); err == nil {
			continue
		}
		if err := k.SetDWordValue(v.name, v.value); err != nil {
			return err
		}
		made.Values, added = append(made.Values, v.name), true
	}
	if _, _, err := k.GetStringsValue("ExecutableTypes"); err != nil {
		_ = k.SetStringsValue("ExecutableTypes", []string{"ADE", "ADP", "BAS", "BAT", "CHM", "CMD", "COM", "CPL", "CRT", "EXE",
			"HLP", "HTA", "INF", "INS", "ISP", "LNK", "MDB", "MDE", "MSC", "MSI", "MSP", "MST", "OCX", "PCD", "PIF", "PS1",
			"REG", "SCR", "SHS", "URL", "VB", "VBS", "WSC", "WSF", "WSH", "JS", "JSE"})
		made.Values, added = append(made.Values, "ExecutableTypes"), true
	}
	if !added && existed {
		return nil
	}
	data, _ := json.Marshal(made)
	return os.WriteFile(srpMadePath, data, 0o600)
}

// removeSRP takes KC9K's rules out, and what ensureSRPOn added. With
// nothing of KC9K's there (the normal case, every cycle) it touches
// nothing and refreshes nothing (W50); policy is refreshed only after a
// rule or value really went.
func removeSRP() error {
	if !platform.Privileged() {
		return nil
	}
	changed := false
	defer func() {
		if changed {
			refreshPolicy()
		}
	}()
	for _, level := range []string{saferDisallow, saferAllow} {
		for _, name := range srpOurRules(level) {
			if err := registry.DeleteKey(registry.LOCAL_MACHINE, saferCodeIDs+`\`+level+`\Paths\`+name); err != nil {
				return err
			}
			changed = true
		}
	}
	data, err := os.ReadFile(srpMadePath)
	if err != nil {
		return nil
	}
	var made srpMade
	if json.Unmarshal(data, &made) != nil {
		made.Key = true // the first build's note: KC9K made the key
	}
	if made.Key {
		err := deleteRegTree(registry.LOCAL_MACHINE, saferKey)
		if err != nil && err != registry.ErrNotExist {
			return err
		}
		if err == nil {
			changed = true
		}
	} else if k, err := registry.OpenKey(registry.LOCAL_MACHINE, saferCodeIDs, registry.SET_VALUE); err == nil {
		for _, v := range made.Values {
			if k.DeleteValue(v) == nil {
				changed = true
			}
		}
		k.Close()
	}
	return os.Remove(srpMadePath)
}

// longPath is path with 8.3 short names (RUNNER~1) spelled out: SRP
// matches the long form.
func longPath(path string) string {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetLongPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return path
	}
	return windows.UTF16ToString(buf[:n])
}

var procRefreshPolicyEx = windows.NewLazySystemDLL("userenv.dll").NewProc("RefreshPolicyEx")

// refreshPolicy tells running programs that policy changed, so they read
// the SRP rules again (each process keeps what it read). Called only
// after KC9K's rules really changed. RP_FORCE is needed then: the rules
// are written to the registry, not through a Group Policy object, so an
// unforced refresh finds no GPO changed and tells no program. A variable
// for tests (refreshPolicyCalls counts on windows-latest).
var refreshPolicy = func() {
	const rpForce = 1
	procRefreshPolicyEx.Call(1, rpForce)
}
