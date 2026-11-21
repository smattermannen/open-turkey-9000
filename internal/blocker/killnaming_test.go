//go:build linux

package blocker

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// §3.2 affected users, fetch tools: a download tool given a blocked site's
// address is closed only in the processes of the users the block applies
// to. A script named curl, as the test's own user, plays the tool.
func TestKillNamingForUsers(t *testing.T) {
	uid := uint64(os.Getuid())
	if !regularUser(uid) {
		t.Skip("run as a regular user: only regular users' tools are closed")
	}
	// A copy of the shell named curl, given a site's address after a
	// command that keeps it running.
	tool := filepath.Join(t.TempDir(), "curl")
	sh, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(tool, sh, 0o755); err != nil {
		t.Fatal(err)
	}
	start := func() *exec.Cmd {
		cmd := exec.Command(tool, "-c", "sleep 30; :", "https://www.kc9k-test-site.invalid/r/x")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
		time.Sleep(200 * time.Millisecond)
		return cmd
	}
	running := func(cmd *exec.Cmd) bool {
		_, err := os.Stat("/proc/" + strconv.Itoa(cmd.Process.Pid))
		data, _ := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/stat")
		return err == nil && !slices.Contains([]string{"Z"}, statState(string(data)))
	}
	cmd := start()
	if comm := readComm(cmd.Process.Pid); comm != "curl" {
		t.Skipf("the stand-in tool is named %q here, not curl", comm)
	}
	if got := KillNamingFor([]string{"kc9k-test-site.invalid"}, []uint64{uid + 1}); len(got) != 0 || !running(cmd) {
		t.Errorf("another account's block closed this user's tool: %v", got)
	}
	if got := KillNamingFor([]string{"kc9k-test-site.invalid"}, []uint64{}); len(got) != 0 || !running(cmd) {
		t.Errorf("a block for no existing account closed a tool: %v", got)
	}
	if got := KillNamingFor([]string{"kc9k-other-site.invalid"}, []uint64{uid}); len(got) != 0 || !running(cmd) {
		t.Errorf("a tool given another site was closed: %v", got)
	}
	if got := KillNamingFor([]string{"kc9k-test-site.invalid"}, []uint64{uid}); !slices.Equal(got, []string{"curl"}) {
		t.Errorf("this user's tool given a blocked site: closed %v", got)
	}
	cmd.Wait()
	if running(cmd) {
		t.Error("still running")
	}
}

// statState is the state letter of a /proc/<pid>/stat line.
func statState(stat string) string {
	for i := len(stat) - 1; i > 0; i-- {
		if stat[i] == ')' && i+2 < len(stat) {
			return stat[i+2 : i+3]
		}
	}
	return ""
}
