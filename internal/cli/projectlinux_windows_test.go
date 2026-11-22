package cli

import "testing"

// The Linux build's project imported by the real Windows build, no seam
// (W56; windows.yml's unit job runs it).
func TestLinuxProjectOnRealWindows(t *testing.T) {
	if projectOS != "windows" {
		t.Fatalf("projectOS = %q on Windows", projectOS)
	}
	checkLinuxProjectOnWindows(t)
}
