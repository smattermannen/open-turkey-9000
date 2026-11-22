package blocker

import (
	"strings"
	"testing"
)

// W20: tasks running as SYSTEM or a service account are told by their
// principal, in either encoding Task Scheduler writes.
func TestStrictTaskPrincipal(t *testing.T) {
	sys := `<Task><Principals><Principal id="Author"><UserId>S-1-5-18</UserId><RunLevel>HighestAvailable</RunLevel></Principal></Principals></Task>`
	me := `<Task><Principals><Principal><UserId>S-1-5-21-1-2-3-1001</UserId></Principal></Principals></Task>`
	named := `<Task><Principals><Principal><UserId>NT AUTHORITY\SYSTEM</UserId></Principal></Principals></Task>`
	if !taskRunsAsService(sys) || taskRunsAsService(me) || !taskRunsAsService(named) {
		t.Error("principals misread")
	}
	u := []byte{0xFF, 0xFE}
	for _, r := range sys {
		u = append(u, byte(r), 0)
	}
	if !taskRunsAsService(decodeTaskXML(u)) {
		t.Error("a UTF-16 task file misread")
	}
}

// W20: deny entries added to and taken from a DACL-only descriptor, as
// Windows prints it.
func TestStrictDenies(t *testing.T) {
	cur := "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)"
	aces := strictDenies(serviceDenyMask, "")
	next := addDenies(cur, aces)
	for _, a := range aces {
		if !containsACE(next, a) {
			t.Errorf("%s not added: %s", a, next)
		}
	}
	if again := addDenies(next, aces); again != next {
		t.Errorf("added twice: %s", again)
	}
	if back := removeDenies(next, aces); canonicalSDDL(back) != canonicalSDDL(cur) {
		t.Errorf("not taken out: %s", back)
	}
}

func containsACE(sddl, ace string) bool {
	return strings.Contains(sddl, ace) || strings.Contains(sddl, canonACE(ace))
}

// W24: which processes strict mode closes as virtual machines.
func TestStrictVMProcesses(t *testing.T) {
	win := systemRoot()
	person := func(name, exe string) *winProc { return &winProc{Name: name, Exe: exe, UID: 1001} }
	cases := []struct {
		p    *winProc
		want bool
	}{
		{person("virtualboxvm", `C:\Program Files\Oracle\VirtualBox\VirtualBoxVM.exe`), true},
		{person("wsl", win + `\System32\wsl.exe`), true},
		{person("qemu-system-x86_64", `C:\qemu\qemu-system-x86_64.exe`), true},
		{person("bash", win + `\System32\bash.exe`), true},
		{person("bash", `C:\Program Files\Git\usr\bin\bash.exe`), false},
		{person("notepad", win + `\System32\notepad.exe`), false},
		{&winProc{Name: "vmwp", Exe: win + `\System32\vmwp.exe`, UID: -1, System: true}, true},
		{&winProc{Name: "dockerd", Exe: `C:\Program Files\Docker\dockerd.exe`, UID: -1, System: true}, false},
	}
	for _, c := range cases {
		if got := isVMProcess(c.p); got != c.want {
			t.Errorf("%s (%s): %v, want %v", c.p.Name, c.p.Exe, got, c.want)
		}
	}
}

// W25: a rule keeps its key name; another path or level gets another.
func TestSRPRuleNames(t *testing.T) {
	a := srpRuleName(srpRule{`C:\Users\me`, saferDisallow})
	if a != srpRuleName(srpRule{`c:\users\ME`, saferDisallow}) || !strings.HasPrefix(a, srpRulePrefix) || len(a) != 38 {
		t.Errorf("rule name %q", a)
	}
	if a == srpRuleName(srpRule{`C:\Users\me`, saferAllow}) || a == srpRuleName(srpRule{`C:\Users\you`, saferDisallow}) {
		t.Error("two rules share a name")
	}
	rules := srpRules([]string{`C:\Users\me\Games`})
	found := map[string]string{}
	for _, r := range rules {
		found[strings.ToLower(r.Path)] = r.Level
	}
	if found[`c:\users\me\games`] != saferAllow {
		t.Errorf("exempt folder not allowed: %v", found)
	}
}
