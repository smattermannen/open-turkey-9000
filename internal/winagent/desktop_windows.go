package winagent

// desktop_windows.go — the desktop work the agent does for the service
// (ROADMAP W3): list windows with their titles and processes, ask windows
// to close (WM_CLOSE: polite close), the app in front, lock the session,
// notifications, yes/no questions, hotkeys. Everything runs as the
// signed-in user on their own desktop.

import (
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	shell32                      = windows.NewLazySystemDLL("shell32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procLockWorkStation          = user32.NewProc("LockWorkStation")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procRegisterHotKey           = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey         = user32.NewProc("UnregisterHotKey")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procPostThreadMessageW       = user32.NewProc("PostThreadMessageW")
	procGetWindow                = user32.NewProc("GetWindow")
	procShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
)

// Ops of W3.
const (
	OpWindows    = "windows"    // → []Window
	OpClose      = "close"      // CloseArgs → number asked
	OpForeground = "foreground" // → Foreground
	OpLock       = "lock"       // lock the session
	OpNotify     = "notify"     // NotifyArgs
	OpConfirm    = "confirm"    // ConfirmArgs → bool
	OpHotkeys    = "hotkeys"    // []Hotkey: register these, drop the rest
	OpChallenge  = "challenge"  // ChallengeArgs → the key events, one per line

	EventHotkey = "hotkey" // data: the hotkey's ID
)

// Window is one top-level visible window.
type Window struct {
	HWND  uintptr `json:"hwnd"`
	PID   uint32  `json:"pid"`
	Title string  `json:"title"`
	Class string  `json:"class"`
}

// CloseArgs names windows to ask to close: these handles, and every window
// of these processes.
type CloseArgs struct {
	HWNDs []uintptr `json:"hwnds,omitempty"`
	PIDs  []uint32  `json:"pids,omitempty"`
}

// Foreground is the window in front.
type Foreground struct {
	PID   uint32 `json:"pid"`
	Title string `json:"title"`
}

// NotifyArgs is a notification.
type NotifyArgs struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// ConfirmArgs is a yes/no question.
type ConfirmArgs struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Hotkey is one key combination: Mods are MOD_* flags, Key a virtual key.
type Hotkey struct {
	ID   int    `json:"id"`
	Mods uint32 `json:"mods"`
	Key  uint32 `json:"key"`
}

func init() {
	Register(OpWindows, func(json.RawMessage) (any, error) { return listWindows(), nil })
	Register(OpClose, func(raw json.RawMessage) (any, error) {
		var a CloseArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return closeWindows(a), nil
	})
	Register(OpForeground, func(json.RawMessage) (any, error) {
		h, _, _ := procGetForegroundWindow.Call()
		if h == 0 {
			return Foreground{}, nil
		}
		var pid uint32
		procGetWindowThreadProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
		return Foreground{PID: pid, Title: windowText(h)}, nil
	})
	Register(OpLock, func(json.RawMessage) (any, error) {
		if r, _, err := procLockWorkStation.Call(); r == 0 {
			return nil, err
		}
		return nil, nil
	})
	Register(OpNotify, func(raw json.RawMessage) (any, error) {
		var a NotifyArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, notify(a.Title, a.Body)
	})
	Register(OpConfirm, func(raw json.RawMessage) (any, error) {
		var a ConfirmArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		t, _ := windows.UTF16PtrFromString(a.Title)
		x, _ := windows.UTF16PtrFromString(a.Text)
		const mbYesNo, mbIconQuestion, mbTopmost, mbSetForeground, idYes = 0x4, 0x20, 0x40000, 0x10000, 6
		r, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(x)), uintptr(unsafe.Pointer(t)), mbYesNo|mbIconQuestion|mbTopmost|mbSetForeground)
		return r == idYes, nil
	})
	Register(OpHotkeys, func(raw json.RawMessage) (any, error) {
		var keys []Hotkey
		if err := json.Unmarshal(raw, &keys); err != nil {
			return nil, err
		}
		return nil, hotkeys.set(keys)
	})
}

func windowText(h uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(h)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return windows.UTF16ToString(buf)
}

func windowClass(h uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetClassNameW.Call(h, uintptr(unsafe.Pointer(&buf[0])), 256)
	return windows.UTF16ToString(buf[:n])
}

var enumMu sync.Mutex
var enumOut []Window
var enumCallback = syscall.NewCallback(func(h, _ uintptr) uintptr {
	if v, _, _ := procIsWindowVisible.Call(h); v == 0 {
		return 1
	}
	const gwOwner = 4
	if owner, _, _ := procGetWindow.Call(h, gwOwner); owner != 0 {
		return 1 // dialogs and tool windows belong to their owner's
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
	enumOut = append(enumOut, Window{HWND: h, PID: pid, Title: windowText(h), Class: windowClass(h)})
	return 1
})

// listWindows lists the desktop's top-level visible windows.
func listWindows() []Window {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumOut = nil
	procEnumWindows.Call(enumCallback, 0)
	out := enumOut
	enumOut = nil
	return out
}

// closeWindows posts WM_CLOSE to the windows named, and returns how many.
func closeWindows(a CloseArgs) int {
	const wmClose = 0x0010
	targets := map[uintptr]bool{}
	for _, h := range a.HWNDs {
		targets[h] = true
	}
	if len(a.PIDs) > 0 {
		pids := map[uint32]bool{}
		for _, p := range a.PIDs {
			pids[p] = true
		}
		for _, w := range listWindows() {
			if pids[w.PID] {
				targets[w.HWND] = true
			}
		}
	}
	n := 0
	for h := range targets {
		if r, _, _ := procPostMessageW.Call(h, wmClose, 0, 0); r != 0 {
			n++
		}
	}
	return n
}

// hotkeys keeps the registered hotkeys on one thread with a message loop
// (RegisterHotKey belongs to the thread that calls it).
var hotkeys = &hotkeyThread{}

type hotkeyThread struct {
	once     sync.Once
	threadID uint32
	requests chan []Hotkey
	results  chan error
}

func (t *hotkeyThread) start() {
	t.requests = make(chan []Hotkey)
	t.results = make(chan error)
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		t.threadID = windows.GetCurrentThreadId()
		close(ready)
		registered := map[int]bool{}
		type msg struct {
			hwnd    uintptr
			message uint32
			wParam  uintptr
			lParam  uintptr
			time    uint32
			pt      [2]int32
			private uint32
		}
		const wmHotkey, wmApp = 0x0312, 0x8000
		for {
			var m msg
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			switch m.message {
			case wmHotkey:
				Emit(EventHotkey, int(m.wParam))
			case wmApp:
				keys := <-t.requests
				for id := range registered {
					procUnregisterHotKey.Call(0, uintptr(id))
					delete(registered, id)
				}
				var firstErr error
				for _, k := range keys {
					const modNoRepeat = 0x4000
					if r, _, err := procRegisterHotKey.Call(0, uintptr(k.ID), uintptr(k.Mods|modNoRepeat), uintptr(k.Key)); r == 0 {
						if firstErr == nil {
							firstErr = err
						}
						continue
					}
					registered[k.ID] = true
				}
				t.results <- firstErr
			}
		}
	}()
	<-ready
}

func (t *hotkeyThread) set(keys []Hotkey) error {
	t.once.Do(t.start)
	const wmApp = 0x8000
	for i := 0; ; i++ {
		if r, _, _ := procPostThreadMessageW.Call(uintptr(t.threadID), wmApp, 0, 0); r != 0 {
			break
		}
		if i > 50 {
			return errors.New("the hotkey thread doesn't answer")
		}
		time.Sleep(10 * time.Millisecond) // its message queue isn't there yet
	}
	t.requests <- keys
	return <-t.results
}
