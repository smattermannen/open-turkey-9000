package winagent

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestMain lets this test binary act as an agent that crashes, ends by
// itself or waits to be ended (KC9K_AGENT_HELPER).
func TestMain(m *testing.M) {
	switch os.Getenv("KC9K_AGENT_HELPER") {
	case "crash":
		done := make(chan struct{})
		go func() {
			var m map[string]int
			m["x"] = 1 // panic in a goroutine: Go's crash report, exit code 2
			close(done)
		}()
		<-done
	case "done":
		fmt.Fprintln(os.Stderr, markerDone)
		os.Exit(0)
	case "wait":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func spawnHelper(t *testing.T, mode string) (windows.Handle, *agentOutput) {
	t.Setenv("KC9K_AGENT_HELPER", mode)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h, out, err := spawnWithOutput(0, self, syscall.EscapeArg(self), nil, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	return h, out
}

func waitEnded(t *testing.T, h windows.Handle) {
	if ev, _ := windows.WaitForSingleObject(h, 30000); ev != windows.WAIT_OBJECT_0 {
		t.Fatal("the helper didn't end")
	}
}

// §3.10 / W50: an agent that crashed (and said so on its output) is not
// tamper; one ended from outside is, whatever exit code it was given; one
// that ended by itself is not.
func TestAgentEndCrashVsKill(t *testing.T) {
	h, out := spawnHelper(t, "crash")
	waitEnded(t, h)
	if end, why := agentEnded(h, out); end != endCrash {
		t.Errorf("a crash counted as %v%s; output %q", end, why, out.text())
	}
	windows.CloseHandle(h)

	for _, code := range []uint32{1, 2, 0} {
		h, out = spawnHelper(t, "wait")
		time.Sleep(500 * time.Millisecond)
		if err := windows.TerminateProcess(h, code); err != nil {
			t.Fatal(err)
		}
		waitEnded(t, h)
		if end, why := agentEnded(h, out); end != endOutside {
			t.Errorf("an agent ended from outside (code %d) counted as %v%s", code, end, why)
		}
		windows.CloseHandle(h)
	}

	h, out = spawnHelper(t, "done")
	waitEnded(t, h)
	if end, why := agentEnded(h, out); end != endSelf {
		t.Errorf("an agent that ended by itself counted as %v%s", end, why)
	}
	windows.CloseHandle(h)
}
