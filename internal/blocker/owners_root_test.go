//go:build roottest

// Run as root on a disposable machine: go test -tags roottest -run Kill ./internal/blocker
package blocker

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// A blocked name kills the user's process but leaves the same program alone
// when it runs as root.
func TestKillSparesSystemProcesses(t *testing.T) {
	asRoot := exec.Command("sleep", "30")
	asUser := exec.Command("sleep", "30")
	asUser.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1500, Gid: 1500}}
	for _, c := range []*exec.Cmd{asRoot, asUser} {
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
	}
	defer asRoot.Process.Kill()
	defer asUser.Process.Kill()
	time.Sleep(200 * time.Millisecond)

	if _, err := KillBlocked([]string{"sleep"}); err != nil {
		t.Fatal(err)
	}
	userDone := make(chan error, 1)
	go func() { userDone <- asUser.Wait() }()
	select {
	case <-userDone:
	case <-time.After(3 * time.Second):
		t.Fatal("the user's blocked process is still running")
	}
	if err := asRoot.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("the root process was killed: %v", err)
	}
}
