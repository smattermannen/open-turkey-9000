//go:build roottest

// Run as root on a disposable machine: go test -tags roottest -run Notify ./internal/blocker
package blocker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A notification reaches every desktop session, sent as that session's user
// on its session bus.
func TestNotifyDesktopReachesSessions(t *testing.T) {
	const name, uid = "kc9knotify", "1777"
	if err := exec.Command("useradd", "-u", uid, "-M", name).Run(); err != nil {
		t.Skipf("can't add a test user: %v", err)
	}
	defer exec.Command("userdel", name).Run()

	runtime := "/run/user/" + uid
	if err := os.MkdirAll(runtime, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(runtime)
	if err := os.WriteFile(runtime+"/bus", nil, 0600); err != nil {
		t.Fatal(err)
	}

	// Go's test folders are private to root; the user needs to reach these.
	bin, err := os.MkdirTemp("/tmp", "kc9k-notify-bin-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(bin)
	outDir, err := os.MkdirTemp("/tmp", "kc9k-notify-out-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)
	out := filepath.Join(outDir, "sent")
	if err := os.Chmod(outDir, 0777); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$(id -u)\" \"$DBUS_SESSION_BUS_ADDRESS\" \"$*\" >> " + out + "\n"
	if err := os.WriteFile(filepath.Join(bin, "notify-send"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	NotifyDesktop("Frozen Chicken starts in 5 minutes", "Save your work.")
	deadline := time.Now().Add(3 * time.Second)
	var sent string
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(out)
		if sent = string(data); sent != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.HasPrefix(sent, uid+"|unix:path=/run/user/"+uid+"/bus|") || !strings.Contains(sent, "Frozen Chicken starts in 5 minutes Save your work.") {
		t.Fatalf("notification not delivered as the user on their bus: %q", sent)
	}
}
