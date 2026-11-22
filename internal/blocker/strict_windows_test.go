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
