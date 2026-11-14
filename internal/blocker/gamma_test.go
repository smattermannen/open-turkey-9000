package blocker

import (
	"os"
	"testing"
)

// halfRamp is a ramp at half brightness, warm: less green, even less blue.
func halfRamp(size int) (red, green, blue []uint16) {
	red, green, blue = identityRamp(size)
	for i := range red {
		red[i] /= 2
		green[i] /= 3
		blue[i] /= 5
	}
	return
}

// TestGammaOnVirtualScreen sets, reads and restores gamma ramps on a
// virtual X server (Xvfb), never on a real screen.
func TestGammaOnVirtualScreen(t *testing.T) {
	display, xauth := startX(t)
	uid := uint64(os.Getuid())
	if !trustedX11Socket(displayNumber(display), uid) {
		t.Skip("the X socket directory isn't one KC9K trusts here")
	}
	old := gammaSessions
	gammaSessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth, uid: uid}} }
	defer func() { gammaSessions = old }()

	c, err := openUserDisplay(display, xauth, uid)
	if err != nil {
		t.Fatal(err)
	}
	// This connection stays open: an X server resets when its last client
	// leaves, and a real desktop always has clients.
	defer c.Close()
	if _, err := c.randrOpcode(); err != nil {
		t.Skipf("no RandR: %v", err)
	}
	n, err := SetGamma(halfRamp)
	if err != nil || n == 0 {
		t.Fatalf("SetGamma: %d screens, %v", n, err)
	}
	if got := ReadGamma(halfRamp); got != GammaOurs {
		t.Errorf("after setting: %v", got)
	}
	if got := ReadGamma(identityRamp); got != GammaOther {
		t.Errorf("someone else's ramp reads %v", got)
	}
	if _, err := SetGamma(nil); err != nil {
		t.Fatal(err)
	}
	if got := ReadGamma(halfRamp); got != GammaIdentity {
		t.Errorf("after restoring: %v", got)
	}
}

func TestParseDDC(t *testing.T) {
	out := "Display 1\n   I2C bus:  /dev/i2c-4\n   Monitor:  DEL:DELL U2415:ABC123\n\nInvalid display\n   I2C bus: /dev/i2c-5\n\nDisplay 2\n   Monitor: GSM::\n"
	screens := parseDDCDetect(out)
	if len(screens) != 2 || screens[0].Display != "1" || screens[0].Name != "DELL U2415" || screens[1].Name != "Display 2" {
		t.Errorf("parsed %+v", screens)
	}
	if cur, max, err := parseVCP("VCP 10 C 37 100\n"); err != nil || cur != 37 || max != 100 {
		t.Errorf("parseVCP: %d %d %v", cur, max, err)
	}
	if _, _, err := parseVCP("VCP 10 ERR"); err == nil {
		t.Error("an error answer was read as a level")
	}
}

func TestBacklightsPreferFirmware(t *testing.T) {
	dir := t.TempDir()
	make := func(name, kind string, max int) {
		os.MkdirAll(dir+"/"+name, 0o755)
		os.WriteFile(dir+"/"+name+"/type", []byte(kind+"\n"), 0o644)
		os.WriteFile(dir+"/"+name+"/max_brightness", []byte(itoa(max)+"\n"), 0o644)
		os.WriteFile(dir+"/"+name+"/brightness", []byte("10\n"), 0o644)
	}
	make("intel_backlight", "raw", 1000)
	make("acpi_video0", "firmware", 15)
	old := backlightDir
	backlightDir = dir
	defer func() { backlightDir = old }()
	got := Backlights()
	if len(got) != 1 || got[0].Name != "acpi_video0" || got[0].Max != 15 {
		t.Fatalf("Backlights() = %+v", got)
	}
	if err := got[0].SetRaw(99); err != nil {
		t.Fatal(err)
	}
	if level, _ := got[0].Raw(); level != 15 {
		t.Errorf("level above the maximum was kept: %d", level)
	}
	if PercentToRaw(0.01, 1000) != 1 || PercentToRaw(50, 1000) != 500 {
		t.Error("PercentToRaw")
	}
}

func itoa(n int) string {
	return string(rune('0'+n/1000%10)) + string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}
