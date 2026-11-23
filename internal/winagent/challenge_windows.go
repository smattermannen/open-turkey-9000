package winagent

// challenge_windows.go — the text lock's challenge window on Windows
// (R-LOCK-5, ROADMAP W3), a plain Win32 window the agent opens in the
// person's session when the command that asks has no console. It decides
// nothing: it shows the pages and reports each key, as the GTK window does
// on Linux (lock/challenge_window.py), one line per event:
//
//	k <code> <ms>  a printable ASCII key typed (code), message time in ms
//	b <ms>         Backspace
//	n <ms>         Enter: the page is done (the next one is shown)
//	p              a paste was tried (Ctrl+V, Shift+Insert)
//	s              a key that wasn't typed on a keyboard (injected input)
//	c              cancelled (Escape, or the window closed)
//
// There is no text field: nothing can be pasted into it. Every key also
// arrives as Raw Input; one with no device behind it (SendInput,
// on-screen keyboards, automation) is reported as synthetic.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ChallengeArgs are the pages to type.
type ChallengeArgs struct {
	Pages    []string `json:"pages"`
	Accuracy float64  `json:"accuracy"`
	Width    int      `json:"width"`
}

var (
	gdi32                       = windows.NewLazySystemDLL("gdi32.dll")
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procShowWindow              = user32.NewProc("ShowWindow")
	procUpdateWindow            = user32.NewProc("UpdateWindow")
	procSetForegroundWindow     = user32.NewProc("SetForegroundWindow")
	procTranslateMessage        = user32.NewProc("TranslateMessage")
	procDispatchMessageW        = user32.NewProc("DispatchMessageW")
	procPostQuitMessage         = user32.NewProc("PostQuitMessage")
	procInvalidateRect          = user32.NewProc("InvalidateRect")
	procBeginPaint              = user32.NewProc("BeginPaint")
	procEndPaint                = user32.NewProc("EndPaint")
	procDrawTextW               = user32.NewProc("DrawTextW")
	procGetClientRect           = user32.NewProc("GetClientRect")
	procRegisterRawInputDevices = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData         = user32.NewProc("GetRawInputData")
	procGetMessageTime          = user32.NewProc("GetMessageTime")
	procGetKeyState             = user32.NewProc("GetKeyState")
	procGetStockObject          = gdi32.NewProc("GetStockObject")
	procSelectObject            = gdi32.NewProc("SelectObject")
	procFillRect                = user32.NewProc("FillRect")
	procGetSysColorBrush        = user32.NewProc("GetSysColorBrush")
	procGetDpiForWindow         = user32.NewProc("GetDpiForWindow")
	procSetWindowPos            = user32.NewProc("SetWindowPos")
	procCreateFontW             = gdi32.NewProc("CreateFontW")
	procDeleteObject            = gdi32.NewProc("DeleteObject")
)

// The window's size and text at 96 DPI (100 %). kc9k.exe is per-monitor
// DPI aware (its manifest, cmd/kc9k/kc9k.manifest): the window scales
// these to its screen and again when moved to another (WM_DPICHANGED).
const (
	challengeWidth   = 900
	challengeHeight  = 560
	challengeMargin  = 16
	challengeFontPx  = 16
	challengeFont    = "Consolas"
	challengeBaseDPI = 96
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc       uintptr
	erase     int32
	rcPaint   rect
	restore   int32
	incUpdate int32
	reserved  [32]byte
}

type rawInputDevice struct {
	usagePage uint16
	usage     uint16
	flags     uint32
	target    uintptr
}

type rawInputHeader struct {
	kind   uint32
	size   uint32
	device uintptr
	wParam uintptr
}

type rawKeyboard struct {
	makeCode uint16
	flags    uint16
	reserved uint16
	vkey     uint16
	message  uint32
	extra    uint32
}

// challengeState is the open window's state (one challenge at a time).
type challengeState struct {
	args   ChallengeArgs
	page   int
	typed  []byte
	events []string
	done   bool
	hwnd   uintptr
	dpi    uint32
	font   uintptr
}

var (
	challengeMu  sync.Mutex // one challenge window at a time
	challengeCur *challengeState
	wndProcOnce  sync.Once
	wndProcPtr   uintptr
	classOnce    sync.Once
	classErr     error
	className    = windows.StringToUTF16Ptr("KC9KChallenge")
)

// challengeHwnd is the open challenge window, 0 when none (read from other
// goroutines: OpChallengeCancel).
var challengeHwnd atomic.Uintptr

func init() {
	Register(OpChallengeCancel, func(json.RawMessage) (any, error) {
		if h := challengeHwnd.Load(); h != 0 {
			const wmClose = 0x0010
			procPostMessageW.Call(h, wmClose, 0, 0)
			return true, nil
		}
		return false, nil
	})
	Register(OpChallenge, func(raw json.RawMessage) (any, error) {
		var a ChallengeArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		if len(a.Pages) == 0 {
			return nil, errors.New("no pages")
		}
		return runChallenge(a)
	})
}

func (c *challengeState) say(line string) { c.events = append(c.events, line) }

func (c *challengeState) finish(line string) {
	if !c.done {
		c.done = true
		c.say(line)
	}
	procPostQuitMessage.Call(0)
}

// windowDPI is the DPI of the screen hwnd is on (96 where Windows is too
// old to say).
func windowDPI(hwnd uintptr) uint32 {
	if procGetDpiForWindow.Find() != nil {
		return challengeBaseDPI
	}
	if d, _, _ := procGetDpiForWindow.Call(hwnd); d != 0 {
		return uint32(d)
	}
	return challengeBaseDPI
}

// scale turns a size at 96 DPI into pixels at dpi.
func scale(px int, dpi uint32) int32 {
	return int32((px*int(dpi) + challengeBaseDPI/2) / challengeBaseDPI)
}

// setDPI makes the font for dpi (a fixed-width one, so pages line up as in
// the console).
func (c *challengeState) setDPI(dpi uint32) {
	c.dpi = dpi
	if c.font != 0 {
		procDeleteObject.Call(c.font)
	}
	const fwNormal, defaultCharset, cleartypeQuality, fixedPitchModern = 400, 1, 5, 1 | 0x30
	face := windows.StringToUTF16Ptr(challengeFont)
	h := -scale(challengeFontPx, dpi)
	c.font, _, _ = procCreateFontW.Call(uintptr(h), 0, 0, 0, fwNormal, 0, 0, 0, defaultCharset, 0, 0, cleartypeQuality, fixedPitchModern, uintptr(unsafe.Pointer(face)))
}

func messageTime() int64 {
	t, _, _ := procGetMessageTime.Call()
	return int64(uint32(t))
}

func keyDown(vk uintptr) bool {
	s, _, _ := procGetKeyState.Call(vk)
	return int16(s) < 0
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	const (
		wmDestroy = 0x0002
		wmPaint   = 0x000F
		wmClose   = 0x0010
		wmKeyDown = 0x0100
		wmChar    = 0x0102
		wmInput   = 0x00FF
		wmDPI     = 0x02E0
		vkInsert  = 0x2D
		vkShift   = 0x10
		vkControl = 0x11
	)
	c := challengeCur
	if c == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
	switch msg {
	case wmDPI:
		// Moved to a screen with another scale: the same size at its scale.
		c.setDPI(uint32(wParam & 0xFFFF))
		const swpNoMove, swpNoZOrder, swpNoActivate = 0x2, 0x4, 0x10
		procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(scale(challengeWidth, c.dpi)), uintptr(scale(challengeHeight, c.dpi)), swpNoMove|swpNoZOrder|swpNoActivate)
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmInput:
		var size uint32 = 64
		buf := make([]byte, 64)
		const ridInput = 0x10000003
		hdr := uint32(unsafe.Sizeof(rawInputHeader{}))
		n, _, _ := procGetRawInputData.Call(lParam, ridInput, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), uintptr(hdr))
		if int32(n) > 0 {
			h := (*rawInputHeader)(unsafe.Pointer(&buf[0]))
			const rimTypeKeyboard = 1
			if h.kind == rimTypeKeyboard {
				kb := (*rawKeyboard)(unsafe.Pointer(&buf[hdr]))
				const riKeyBreak = 0x1
				if kb.flags&riKeyBreak == 0 && h.device == 0 {
					c.finish("s")
					return 0
				}
			}
		}
	case wmKeyDown:
		if (wParam == 'V' && keyDown(vkControl)) || (wParam == vkInsert && keyDown(vkShift)) {
			c.finish("p")
			return 0
		}
	case wmChar:
		switch {
		case wParam == 0x1B: // Escape
			c.finish("c")
		case wParam == 0x08:
			if len(c.typed) > 0 {
				c.typed = c.typed[:len(c.typed)-1]
			}
			c.say(fmt.Sprintf("b %d", messageTime()))
		case wParam == 0x0D:
			c.say(fmt.Sprintf("n %d", messageTime()))
			c.page++
			c.typed = nil
			if c.page >= len(c.args.Pages) {
				c.done = true
				procPostQuitMessage.Call(0)
			}
		case wParam == 0x16: // Ctrl+V as a character
			c.finish("p")
		case wParam >= 0x20 && wParam < 0x7f:
			c.typed = append(c.typed, byte(wParam))
			c.say(fmt.Sprintf("k %d %d", wParam, messageTime()))
		}
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var r rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		const colorWindow = 5
		brush, _, _ := procGetSysColorBrush.Call(colorWindow)
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
		font := c.font
		if font == 0 {
			const ansiFixedFont = 11
			font, _, _ = procGetStockObject.Call(ansiFixedFont)
		}
		old, _, _ := procSelectObject.Call(hdc, font)
		defer procSelectObject.Call(hdc, old)
		m := scale(challengeMargin, c.dpi)
		r.left, r.top, r.right = r.left+m, r.top+m, r.right-m
		const dtWordBreak, dtNoPrefix = 0x10, 0x800
		page := ""
		if c.page < len(c.args.Pages) {
			page = c.args.Pages[c.page]
		}
		text := fmt.Sprintf("Unlock challenge, page %d of %d. Type the characters below, then press Enter.\r\nYou need %.0f%% accuracy over all pages. Backspace works; Escape cancels.\r\n\r\n%s\r\n\r\n%s_",
			min(c.page+1, len(c.args.Pages)), len(c.args.Pages), c.args.Accuracy, wrapText(page, c.args.Width), wrapText(string(c.typed), c.args.Width))
		t := windows.StringToUTF16(text)
		procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1), uintptr(unsafe.Pointer(&r)), dtWordBreak|dtNoPrefix)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmClose:
		c.finish("c")
		return 0
	case wmDestroy:
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func wrapText(s string, width int) string {
	if width <= 0 {
		width = 60
	}
	var lines []string
	for len(s) > width {
		lines = append(lines, s[:width])
		s = s[width:]
	}
	lines = append(lines, s)
	return strings.Join(lines, "\r\n")
}

// RunChallenge is OpChallenge run in this process (cmd/kc9k's manifest
// test opens it under kc9k.exe's manifest).
func RunChallenge(a ChallengeArgs) (string, error) { return runChallenge(a) }

// runChallenge opens the window, runs it until the last page or a cancel,
// and returns the events.
func runChallenge(a ChallengeArgs) (string, error) {
	if !challengeMu.TryLock() {
		return "", errors.New("a challenge window is open already")
	}
	defer challengeMu.Unlock()
	result := make(chan error, 1)
	state := &challengeState{args: a}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		wndProcOnce.Do(func() { wndProcPtr = syscall.NewCallback(wndProc) })
		classOnce.Do(func() {
			// The same system brush the paint uses (COLOR_WINDOW+1): never a
			// blank window before the first paint (W52's look-alike).
			const colorWindowBrush = 5 + 1
			wc := wndClassEx{wndProc: wndProcPtr, className: className, background: windows.Handle(colorWindowBrush)}
			wc.size = uint32(unsafe.Sizeof(wc))
			if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
				classErr = err
			}
		})
		if classErr != nil {
			result <- classErr
			return
		}
		challengeCur = state
		defer func() { challengeCur = nil }()
		title := windows.StringToUTF16Ptr("Killer Chicken 9000: unlock challenge")
		// A fixed frame with a close button only (closing cancels): no
		// minimise or maximise, so it never passes for an empty app window.
		// Made hidden at its 100 % size, then sized for its screen's scale
		// and shown.
		const wsCaption, wsSysMenu, wsExTopmost = 0x00C00000, 0x00080000, 0x8
		const cwUseDefault = 0x80000000
		hwnd, _, err := procCreateWindowExW.Call(wsExTopmost, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
			wsCaption|wsSysMenu, cwUseDefault, cwUseDefault, challengeWidth, challengeHeight, 0, 0, 0, 0)
		if hwnd == 0 {
			log.Printf("agent: challenge window: CreateWindowEx: %v", err)
			result <- err
			return
		}
		state.hwnd = hwnd
		state.setDPI(windowDPI(hwnd))
		log.Printf("agent: challenge window %#x open (%d pages, %d DPI)", hwnd, len(a.Pages), state.dpi)
		challengeHwnd.Store(hwnd)
		defer challengeHwnd.Store(0)
		defer func() {
			if state.font != 0 {
				procDeleteObject.Call(state.font)
			}
		}()
		const swpNoMove, swpNoZOrder, swpNoActivate = 0x2, 0x4, 0x10
		procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(scale(challengeWidth, state.dpi)), uintptr(scale(challengeHeight, state.dpi)), swpNoMove|swpNoZOrder|swpNoActivate)
		// Every key also as Raw Input, to tell typed keys from injected.
		const ridevInputSink = 0x100
		dev := rawInputDevice{usagePage: 1, usage: 6, flags: ridevInputSink, target: hwnd}
		if r, _, err := procRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&dev)), 1, unsafe.Sizeof(dev)); r == 0 {
			procDestroyWindow.Call(hwnd)
			result <- fmt.Errorf("raw input: %w", err)
			return
		}
		procShowWindow.Call(hwnd, 5)
		procUpdateWindow.Call(hwnd)
		procSetForegroundWindow.Call(hwnd)
		type msg struct {
			hwnd    uintptr
			message uint32
			wParam  uintptr
			lParam  uintptr
			time    uint32
			pt      [2]int32
			private uint32
		}
		for {
			var m msg
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		procDestroyWindow.Call(hwnd)
		log.Printf("agent: challenge window %#x closed", hwnd)
		result <- nil
	}()
	if err := <-result; err != nil {
		return "", err
	}
	if !state.done {
		state.say("c")
	}
	return strings.Join(state.events, "\n") + "\n", nil
}
