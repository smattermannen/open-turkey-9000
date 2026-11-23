//go:build linux

package blocker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeLibreWolfAuto replaces the daemon's LibreWolf installation: present
// and active are read on every check; start records each script and runs
// onStart.
type fakeLibreWolfAuto struct {
	mu       sync.Mutex
	present  bool
	active   int // checks the unit still reads as running
	starts   []string
	startErr error
	onStart  func(f *fakeLibreWolfAuto)
	logs     []string
}

func useFakeLibreWolfAuto(t *testing.T, f *fakeLibreWolfAuto) {
	t.Helper()
	oldDelay, oldPoll, oldRetry := libreWolfAutoDelay, libreWolfAutoPoll, libreWolfAutoRetry
	oldPresent, oldActive, oldStart := libreWolfPresent, libreWolfUnitActive, libreWolfUnitStart
	t.Cleanup(func() {
		libreWolfAutoDelay, libreWolfAutoPoll, libreWolfAutoRetry = oldDelay, oldPoll, oldRetry
		libreWolfPresent, libreWolfUnitActive, libreWolfUnitStart = oldPresent, oldActive, oldStart
	})
	libreWolfAutoDelay, libreWolfAutoPoll, libreWolfAutoRetry = time.Millisecond, time.Millisecond, 5*time.Millisecond
	libreWolfPresent = func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.present }
	libreWolfUnitActive = func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.active > 0 {
			f.active--
			return true
		}
		return false
	}
	libreWolfUnitStart = func(script string) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.starts = append(f.starts, script)
		if f.startErr != nil {
			return f.startErr
		}
		if f.onStart != nil {
			f.onStart(f)
		}
		return nil
	}
}

func (f *fakeLibreWolfAuto) logf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}

// runAuto runs AutoInstallLibreWolf until it returns (or fails the test
// after a few seconds).
func runAuto(t *testing.T, dir string, f *fakeLibreWolfAuto) {
	t.Helper()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { AutoInstallLibreWolf(dir, stop, f.logf); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(stop)
		<-done
		t.Fatalf("AutoInstallLibreWolf didn't finish; logs: %q", f.logs)
	}
}

func readMarker(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, LibreWolfAutoFile))
	if err != nil {
		t.Fatalf("marker: %v", err)
	}
	return string(data)
}

func TestAutoInstallLibreWolfMarkerLeavesItAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LibreWolfAutoFile), []byte("skipped: KC9K_NO_LIBREWOLF=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := &fakeLibreWolfAuto{}
	useFakeLibreWolfAuto(t, f)
	runAuto(t, dir, f)
	if len(f.starts) != 0 {
		t.Fatalf("installed despite the marker: %q", f.starts)
	}
}

func TestAutoInstallLibreWolfAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	f := &fakeLibreWolfAuto{present: true}
	useFakeLibreWolfAuto(t, f)
	runAuto(t, dir, f)
	if len(f.starts) != 0 {
		t.Fatalf("installed an installed LibreWolf: %q", f.starts)
	}
	if m := readMarker(t, dir); !strings.HasPrefix(m, "LibreWolf found installed") {
		t.Fatalf("marker %q", m)
	}
}

func TestAutoInstallLibreWolfInstallsOnce(t *testing.T) {
	dir := t.TempDir()
	f := &fakeLibreWolfAuto{onStart: func(f *fakeLibreWolfAuto) { f.active = 3; f.present = true }}
	useFakeLibreWolfAuto(t, f)
	runAuto(t, dir, f)
	if len(f.starts) != 1 {
		t.Fatalf("started %d times, want 1", len(f.starts))
	}
	if m := readMarker(t, dir); !strings.HasPrefix(m, "LibreWolf installed by the daemon") {
		t.Fatalf("marker %q", m)
	}
	// A later daemon start finds the marker and does nothing.
	f.present = false
	runAuto(t, dir, f)
	if len(f.starts) != 1 {
		t.Fatalf("installed again after the marker: %d starts", len(f.starts))
	}
}

func TestAutoInstallLibreWolfRetriesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	tries := 0
	f := &fakeLibreWolfAuto{onStart: func(f *fakeLibreWolfAuto) {
		tries++
		f.active = 1
		f.present = tries == 3 // offline twice, then it works
	}}
	useFakeLibreWolfAuto(t, f)
	runAuto(t, dir, f)
	if len(f.starts) != 3 {
		t.Fatalf("started %d times, want 3; logs %q", len(f.starts), f.logs)
	}
	failures := 0
	for _, l := range f.logs {
		if strings.Contains(l, "LibreWolf not installed") && strings.Contains(l, LibreWolfInstallUnit) {
			failures++
		}
	}
	if failures != 2 {
		t.Fatalf("%d failures logged, want 2: %q", failures, f.logs)
	}
	readMarker(t, dir)
}

func TestAutoInstallLibreWolfWaitsForARunningUnit(t *testing.T) {
	// A restarted daemon finds the unit its predecessor started still
	// running: it waits instead of starting another.
	dir := t.TempDir()
	f := &fakeLibreWolfAuto{active: 4}
	useFakeLibreWolfAuto(t, f)
	// The unit installs LibreWolf by the time it reads as ended.
	libreWolfUnitActive = func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.active == 0 {
			return false
		}
		f.active--
		if f.active == 0 {
			f.present = true
		}
		return true
	}
	runAuto(t, dir, f)
	if len(f.starts) != 0 {
		t.Fatalf("started a second unit: %q", f.starts)
	}
	readMarker(t, dir)
}

func TestAutoInstallLibreWolfStartErrorRetries(t *testing.T) {
	dir := t.TempDir()
	f := &fakeLibreWolfAuto{startErr: fmt.Errorf("no systemd-run")}
	useFakeLibreWolfAuto(t, f)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { AutoInstallLibreWolf(dir, stop, f.logf); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		n := len(f.starts)
		f.mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no retry after a start error")
		}
		time.Sleep(time.Millisecond)
	}
	close(stop)
	<-done
	if _, err := os.Stat(filepath.Join(dir, LibreWolfAutoFile)); err == nil {
		t.Fatal("marker written though nothing was installed")
	}
}

func TestLibreWolfScriptWaitsForTheLock(t *testing.T) {
	script, err := libreWolfScript()
	if err != nil {
		t.Fatal(err)
	}
	want := "'apt-get' '-o' 'DPkg::Lock::Timeout=600' 'install' '-y' 'extrepo' && 'extrepo' 'enable' 'librewolf' && " +
		"'apt-get' '-o' 'DPkg::Lock::Timeout=600' 'update' && 'apt-get' '-o' 'DPkg::Lock::Timeout=600' 'install' '-y' 'librewolf'"
	if script != want {
		t.Fatalf("script\n%s\nwant\n%s", script, want)
	}
}
