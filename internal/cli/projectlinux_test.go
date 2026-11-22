package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// linuxProjects are the project exported by the Linux build (testdata/,
// W56) and its CRLF and UTF-8 BOM copies.
var linuxProjects = []string{"linux-project.json", "linux-project-crlf.json", "linux-project-bom.json"}

// linuxProjectApps are the file's apps, as the Linux build wrote them.
var linuxProjectApps = []string{"/opt/games/", "/usr/lib/librewolf/librewolf", "gnome-terminal-server", "org.gnome.Nautilus", "steam"}

func linuxProject(name string) string { return filepath.Join("..", "..", "testdata", name) }

// checkLinuxProjectOnWindows imports the Linux project as Windows does:
// read as it is, its Linux-only parts listed by the dry run and kept, only
// added beside a locked block, its lock confirmed (W56, R-WEAK-1/2,
// R-LOCK-12).
func checkLinuxProjectOnWindows(t *testing.T) {
	notTerminal(t)
	d := testDB(t)
	mustRun(t, "block", "create", "linux-desk")
	mustRun(t, "block", "add-app", "linux-desk", "discord")
	mustRun(t, "start", "linux-desk", "--for", "2h", "--yes")
	before, _ := d.GetBlock("linux-desk")

	var first string
	for _, name := range linuxProjects {
		out := capture(t, "project", "import", "--dry-run", linuxProject(name))
		for _, want := range []string{
			"Not used on Windows",
			"app gnome-terminal-server (a Linux program)",
			"app /usr/lib/librewolf/librewolf (a Linux path)",
			"app /opt/games/ (a Linux path)",
			"app org.gnome.Nautilus (a Linux app ID or X11 window class)",
			`custom action "run systemctl suspend" (a Linux program)`,
			`custom action "open /home/tester/Documents/focus.txt" (a Linux path)`,
			"Would: set a lock: ",
			"Would: add block 'linux-desk' as 'linux-desk-2'",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: dry run lacks %q:\n%s", name, want, out)
			}
		}
		if strings.Contains(out, "app steam") || strings.Contains(out, "C:\\") || strings.Contains(out, ".exe") {
			t.Errorf("%s: a used app listed, or a Windows name guessed:\n%s", name, out)
		}
		if first == "" {
			first = out
		} else if out != first {
			t.Errorf("%s reads differently from %s:\n%s\n---\n%s", name, linuxProjects[0], out, first)
		}
	}

	if err := runCLI(t, "project", "import", linuxProject(linuxProjects[2])); err == nil {
		t.Fatal("imported the file's schedule lock without confirmation")
	}
	if _, err := d.GetBlock("linux-desk-2"); err == nil {
		t.Fatal("a refused import wrote a block")
	}
	capture(t, "project", "import", "--yes", linuxProject(linuxProjects[1]))
	got, err := d.GetBlock("linux-desk-2")
	if err != nil {
		t.Fatal(err)
	}
	if apps := slices.Sorted(slices.Values(got.Apps)); !slices.Equal(apps, linuxProjectApps) {
		t.Errorf("apps not kept as they are: %v", apps)
	}
	after, _ := d.GetBlock("linux-desk")
	if !after.Active || after.LockedUntil != before.LockedUntil || !slices.Equal(after.Apps, before.Apps) {
		t.Errorf("the locked block changed: %+v", after)
	}
}

func TestLinuxProjectOnWindows(t *testing.T) {
	old := projectOS
	projectOS = "windows"
	t.Cleanup(func() { projectOS = old })
	checkLinuxProjectOnWindows(t)
}

// On Linux nothing is "not used", and the BOM copy reads as well.
func TestLinuxProjectOnLinux(t *testing.T) {
	old := projectOS
	projectOS = "linux"
	t.Cleanup(func() { projectOS = old })
	testDB(t)
	out := capture(t, "project", "import", "--dry-run", linuxProject("linux-project-bom.json"))
	if strings.Contains(out, "Not used") || !strings.Contains(out, "Would: add block 'linux-desk'") {
		t.Errorf("dry run on Linux:\n%s", out)
	}
}
