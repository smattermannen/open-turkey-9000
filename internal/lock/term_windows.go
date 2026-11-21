package lock

// term_windows.go — the terminal the challenge is typed in, on a Windows
// console: raw input by console mode (no echo, no line editing, Ctrl-C
// read as a key), VT sequences on, so bracketed paste and the screen
// clearing work as on Linux. A command the service runs for a console
// (platform.ForwardedTerminal) has a pipe, not the console: it asks the
// person's kc9k.exe to switch the console (winsvc.Control).

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winsvc"
	"golang.org/x/sys/windows"
)

func consoleSize(fd int) (cols, rows int, ok bool) {
	if c, r, _, fwd := platform.ForwardedTerminal(); fwd && !isConsole(fd) {
		return c, r, c > 0 && r > 0
	}
	var info windows.ConsoleScreenBufferInfo
	out, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return 0, 0, false
	}
	if err := windows.GetConsoleScreenBufferInfo(out, &info); err != nil {
		return 0, 0, false
	}
	return int(info.Window.Right-info.Window.Left) + 1, int(info.Window.Bottom-info.Window.Top) + 1, true
}

// linesPerPageFor picks how many challenge lines fit the console window.
func linesPerPageFor(fd int) int {
	_, rows, ok := consoleSize(fd)
	if !ok {
		return defaultPageLines
	}
	return pageLinesForRows(rows)
}

// lineWidthFor picks the challenge line width for the console window: at
// most maxLineWidth, and narrow enough that a full line never wraps.
func lineWidthFor(fd int) int {
	cols, _, ok := consoleSize(fd)
	if !ok || cols == 0 {
		return maxLineWidth
	}
	width := cols - 1
	if width > maxLineWidth {
		return maxLineWidth
	}
	if width < 20 {
		return 20
	}
	return width
}

func isConsole(fd int) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(fd), &mode) == nil
}

// isTerminal reports whether fd is a console input handle, or the input
// the service relays from one.
func isTerminal(fd int) bool {
	if isConsole(fd) {
		return true
	}
	_, _, _, fwd := platform.ForwardedTerminal()
	return fwd
}

// makeRaw switches the console to raw VT input and VT output and returns a
// function that restores both modes.
func makeRaw(fd int) (func(), error) {
	if _, _, nonce, fwd := platform.ForwardedTerminal(); fwd && !isConsole(fd) {
		fmt.Print(winsvc.Control(nonce, winsvc.CtlRaw))
		return func() { fmt.Print(winsvc.Control(nonce, winsvc.CtlCooked)) }, nil
	}
	in := windows.Handle(fd)
	var inMode uint32
	if err := windows.GetConsoleMode(in, &inMode); err != nil {
		return nil, err
	}
	raw := inMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT)
	raw |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if err := windows.SetConsoleMode(in, raw); err != nil {
		return nil, err
	}
	out, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	var outMode uint32
	outOK := windows.GetConsoleMode(out, &outMode) == nil
	if outOK {
		_ = windows.SetConsoleMode(out, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	return func() {
		_ = windows.SetConsoleMode(in, inMode)
		if outOK {
			_ = windows.SetConsoleMode(out, outMode)
		}
	}, nil
}
