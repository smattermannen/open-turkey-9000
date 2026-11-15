package blocker

import (
	"os"
	"path/filepath"
	"testing"
)

// §3.8: strict mode's exempt folders: real folders inside the noexec
// folders, mounted exec again after the noexec folders.
func TestExemptFolders(t *testing.T) {
	dir := t.TempDir() // under /tmp
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		dir string
		ok  bool
	}{{dir, true}, {"/home", false}, {"/usr/bin", false}, {"relative", false}, {link, false}, {dir + "/../x", false}} {
		if (ExemptFolderError(c.dir) == nil) != c.ok {
			t.Errorf("%s: ok = %v", c.dir, !c.ok)
		}
	}
	plan := noexecPlan(map[string]string{"/tmp": "rw,noexec"}, []string{dir})
	last := plan[len(plan)-1]
	if last[len(last)-1] != dir || last[1] != "remount,bind,exec,nosuid,nodev" {
		t.Errorf("plan = %v", plan)
	}
	for _, step := range plan {
		if step[len(step)-1] == "/tmp" {
			t.Error("an already noexec folder was mounted again")
		}
	}
	if got := noexecPlan(map[string]string{"/home": "noexec", "/tmp": "noexec", "/var/tmp": "noexec", "/dev/shm": "noexec", dir: "rw"}, []string{dir}); len(got) != 0 {
		t.Errorf("everything in place, plan = %v", got)
	}
}
