package cli

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winsvc"
	"golang.org/x/sys/windows"
)

func isConsole(fd int) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(fd), &mode) == nil
}

// isTerminalFd reports whether fd is a console, or the input the service
// relays from one.
func isTerminalFd(fd int) bool {
	if isConsole(fd) {
		return true
	}
	_, _, _, fwd := platform.ForwardedTerminal()
	return fwd
}

// echoOff stops the console at fd echoing what is typed; restore puts it
// back.
func echoOff(fd int) (restore func(), ok bool) {
	if _, _, nonce, fwd := platform.ForwardedTerminal(); fwd && !isConsole(fd) {
		fmt.Print(winsvc.Control(nonce, winsvc.CtlEchoOff))
		return func() { fmt.Print(winsvc.Control(nonce, winsvc.CtlEchoOn)) }, true
	}
	h := windows.Handle(fd)
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return nil, false
	}
	if windows.SetConsoleMode(h, mode&^windows.ENABLE_ECHO_INPUT) != nil {
		return nil, false
	}
	return func() { _ = windows.SetConsoleMode(h, mode) }, true
}
