package blocker

import (
	"path/filepath"
	"testing"
)

func TestWithDeny(t *testing.T) {
	cases := []struct{ in, want string }{
		{"O:SYG:SYD:PAI(A;;FA;;;SY)(A;;0x1200a9;;;BU)", "O:SYG:SYD:PAI" + denyFileACE + "(A;;FA;;;SY)(A;;0x1200a9;;;BU)"},
		// Inherited entries come back by themselves: not written as explicit.
		{"O:BAD:AI(A;ID;FA;;;SY)(A;;FR;;;BU)", "O:BAD:AI" + denyFileACE + "(A;;FR;;;BU)"},
		// Already there: once only, first.
		{"D:(A;;FA;;;SY)" + denyFileACE, "D:" + denyFileACE + "(A;;FA;;;SY)"},
		// A SACL stays after the DACL.
		{"D:(A;;FA;;;SY)S:(AU;FA;FA;;;WD)", "D:" + denyFileACE + "(A;;FA;;;SY)S:(AU;FA;FA;;;WD)"},
	}
	for _, c := range cases {
		if got := withDeny(c.in, denyFileACE); got != c.want {
			t.Errorf("withDeny(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestAccelToVK(t *testing.T) {
	for accel, want := range map[string][2]uint32{
		"<Primary><Alt>s": {0x3, 'S'}, "<Super>F9": {0x8, 0x78}, "<Shift>7": {0x4, '7'},
	} {
		mods, vk, err := accelToVK(accel)
		if err != nil || mods != want[0] || vk != want[1] {
			t.Errorf("accelToVK(%q) = %x %x %v; want %x %x", accel, mods, vk, err, want[0], want[1])
		}
	}
	if _, _, err := accelToVK("<Primary>Menu"); err == nil {
		t.Error("an unknown key was taken")
	}
}

func TestUserProgram(t *testing.T) {
	drive := filepath.VolumeName(systemRoot())
	for path, want := range map[string]bool{
		drive + `\Users\me\Downloads\portable-browser\browser.exe`:              true,
		drive + `\Users\me\Desktop\game.exe`:                                     true,
		drive + `\Users\me\AppData\Local\Temp\setup\x.exe`:                       true,
		drive + `\Users\me\AppData\Local\Programs\Microsoft VS Code\Code.exe`:    false,
		drive + `\Users\me\AppData\Local\Microsoft\OneDrive\OneDrive.exe`:        false,
		drive + `\Program Files\App\app.exe`:                                       false,
		drive + `\Users\me\Downloads\allowed\tool.exe`:                            false,
	} {
		p := &winProc{Exe: path}
		if got := userProgram(p, []string{drive + `\Users\me\Downloads\allowed`}); got != want {
			t.Errorf("userProgram(%s) = %v; want %v", path, got, want)
		}
	}
}
