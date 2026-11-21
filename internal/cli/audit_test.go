package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Decision 2026-11-14: every sudo-allowed command is checked for hostile
// arguments. kc9k-bin runs as root, so a path given on the command line is
// read as the calling user (readAsCaller), never opened directly; the only
// direct reads are KC9K's own files. A new os.Open/os.ReadFile in the CLI
// must be added here on purpose, after checking its path can't come from
// the caller.
func TestNoDirectFileReads(t *testing.T) {
	allowed := map[string]int{
		"readcaller_linux.go":   1, // readAsCallerUpTo itself, when not run through sudo
		"windows_windows.go":    1, // the service's log, a fixed path in its data folder
		"readcaller_windows.go": 2, // readAsCallerUpTo itself: impersonating the caller, or as the caller
		"projectcircadian.go":   1, // wallpapers dir, base name only
		"projectwallpapers.go":  1, // wallpapers dir, base name only (modes', the default's)
		"master.go":             1, // the master hash file (devmaster builds)
		"project.go":            2, // images dir, names checked by db.IsImageName
		"recover.go":            1, // /proc/cmdline
		"guidedsetup.go":        2, // /etc/shadow and /boot/grub/grub.cfg, fixed paths (shadowPath, grubCfgPath)
	}
	re := regexp.MustCompile(`os\.(ReadFile|Open|OpenFile)\(`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if n := len(re.FindAll(data, -1)); n != allowed[f] {
			t.Errorf("%s opens files directly %d times (allowed %d): read caller paths with readAsCaller", f, n, allowed[f])
		}
	}
}
