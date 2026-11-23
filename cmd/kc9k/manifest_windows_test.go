package main

// manifest_windows_test.go — kc9k.exe's manifest at work (ROADMAP §4.3).
// This test binary links the same rsrc_windows_*.syso as kc9k.exe, so it
// runs under the same manifest: per-monitor DPI aware, common controls v6;
// and the challenge window opens, sizes itself for its screen, takes keys
// and is cancelled by closing.

import (
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/winagent"
	"golang.org/x/sys/windows"
)

var (
	user32                              = windows.NewLazySystemDLL("user32.dll")
	procGetThreadDpiAwarenessContext    = user32.NewProc("GetThreadDpiAwarenessContext")
	procGetAwarenessFromDpiAwarenessCtx = user32.NewProc("GetAwarenessFromDpiAwarenessContext")
	procAreDpiAwarenessContextsEqual    = user32.NewProc("AreDpiAwarenessContextsEqual")
	procFindWindowW                     = user32.NewProc("FindWindowW")
	procPostMessageW                    = user32.NewProc("PostMessageW")
	procGetWindowRect                   = user32.NewProc("GetWindowRect")
	procGetDpiForWindow                 = user32.NewProc("GetDpiForWindow")
	procGetWindowDpiAwarenessContext    = user32.NewProc("GetWindowDpiAwarenessContext")
)

func TestManifestDPIAware(t *testing.T) {
	ctx, _, _ := procGetThreadDpiAwarenessContext.Call()
	aw, _, _ := procGetAwarenessFromDpiAwarenessCtx.Call(ctx)
	const perMonitorAware = 2
	if aw != perMonitorAware {
		t.Fatalf("DPI awareness %d, want per-monitor (%d): the manifest isn't applied", aw, perMonitorAware)
	}
	perMonitorV2 := ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	if eq, _, _ := procAreDpiAwarenessContextsEqual.Call(ctx, perMonitorV2); eq == 0 {
		t.Errorf("not per-monitor v2 (Windows 10 1703+ reads PerMonitorV2)")
	}
}

func TestManifestCommonControls6(t *testing.T) {
	dll, err := windows.LoadLibrary("comctl32.dll")
	if err != nil {
		t.Fatal(err)
	}
	defer windows.FreeLibrary(dll)
	p, err := windows.GetProcAddress(dll, "DllGetVersion")
	if err != nil {
		t.Fatal(err)
	}
	v := struct{ size, major, minor, build, platform uint32 }{size: 20}
	if hr, _, _ := syscallN(p, uintptr(unsafe.Pointer(&v))); hr != 0 {
		t.Fatalf("DllGetVersion: %#x", hr)
	}
	if v.major != 6 {
		t.Errorf("comctl32 %d.%d loaded, want 6 (the manifest's common controls)", v.major, v.minor)
	}
}

func TestChallengeWindowUnderManifest(t *testing.T) {
	type res struct {
		out string
		err error
	}
	done := make(chan res, 1)
	go func() {
		out, err := winagent.RunChallenge(winagent.ChallengeArgs{Pages: []string{"ab", "c"}, Accuracy: 90, Width: 60})
		done <- res{out, err}
	}()
	cls := windows.StringToUTF16Ptr("KC9KChallenge")
	var hwnd uintptr
	for i := 0; i < 100 && hwnd == 0; i++ {
		select {
		case r := <-done:
			t.Fatalf("challenge ended before its window was seen: %q %v", r.out, r.err)
		default:
		}
		time.Sleep(100 * time.Millisecond)
		hwnd, _, _ = procFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0)
	}
	if hwnd == 0 {
		t.Fatal("no challenge window")
	}
	ctx, _, _ := procGetWindowDpiAwarenessContext.Call(hwnd)
	if aw, _, _ := procGetAwarenessFromDpiAwarenessCtx.Call(ctx); aw != 2 {
		t.Errorf("window DPI awareness %d, want per-monitor", aw)
	}
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	var r struct{ l, t, r, b int32 }
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	wantW := int32((900*int(dpi) + 48) / 96)
	if w := r.r - r.l; w < wantW-2 || w > wantW+2 {
		t.Errorf("window %d px wide at %d DPI, want %d", w, dpi, wantW)
	}
	t.Logf("challenge window %dx%d at %d DPI", r.r-r.l, r.b-r.t, dpi)
	const wmChar, wmClose = 0x0102, 0x0010
	for _, k := range []uintptr{'a', 'b', 0x0D} {
		procPostMessageW.Call(hwnd, wmChar, k, 0)
	}
	time.Sleep(300 * time.Millisecond)
	procPostMessageW.Call(hwnd, wmClose, 0, 0)
	var got res
	select {
	case got = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("challenge window didn't close")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	lines := strings.Fields(strings.ReplaceAll(got.out, "\n", " | "))
	out := strings.Join(lines, " ")
	if !strings.HasPrefix(got.out, "k 97 ") || !strings.Contains(got.out, "\nk 98 ") || !strings.Contains(got.out, "\nn ") || !strings.HasSuffix(got.out, "c\n") {
		t.Errorf("events %q: want a, b, Enter, then cancelled", out)
	}
}
