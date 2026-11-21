package winagent

// notify_windows.go — notifications from the agent: a notification-area
// icon with a balloon, which Windows 10 and 11 show as a toast. No
// PowerShell, no AppX identity needed.

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procLoadIconW       = user32.NewProc("LoadIconW")
)

type notifyIconData struct {
	cbSize           uint32
	hwnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

var _ = [1]int{}[unsafe.Sizeof(notifyIconData{})-976]

var notifyMu sync.Mutex

// notify shows title and body as a notification, for about 10 s.
func notify(title, body string) error {
	notifyMu.Lock()
	errc := make(chan error, 1)
	go func() {
		defer notifyMu.Unlock()
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		class, _ := windows.UTF16PtrFromString("STATIC")
		const hwndMessage = ^uintptr(2) // HWND_MESSAGE (-3)
		hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, 0, 0)
		if hwnd == 0 {
			errc <- err
			return
		}
		defer procDestroyWindow.Call(hwnd)
		const idiInformation = 32516
		icon, _, _ := procLoadIconW.Call(0, idiInformation)
		d := notifyIconData{hwnd: hwnd, uID: 1, hIcon: icon}
		d.cbSize = uint32(unsafe.Sizeof(d))
		const nifIcon, nifTip, nifInfo, nimAdd, nimModify, nimDelete, niifInfo = 0x2, 0x4, 0x10, 0, 1, 2, 0x1
		copy(d.szTip[:127], windows.StringToUTF16("Killer Chicken 9000"))
		d.uFlags = nifIcon | nifTip
		if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&d))); r == 0 {
			errc <- err
			return
		}
		defer procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&d)))
		copy(d.szInfoTitle[:63], windows.StringToUTF16(truncate(title, 63)))
		copy(d.szInfo[:255], windows.StringToUTF16(truncate(body, 255)))
		d.uFlags, d.dwInfoFlags = nifInfo, niifInfo
		r, _, err := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&d)))
		if r == 0 {
			errc <- err
			return
		}
		errc <- nil
		time.Sleep(10 * time.Second) // the icon goes with the toast
	}()
	return <-errc
}

// truncate shortens s to at most n UTF-16 units' worth of runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
