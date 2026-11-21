package cli

import "golang.org/x/sys/unix"

// isTerminalFd reports whether fd is a terminal.
func isTerminalFd(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

// echoOff stops the terminal at fd echoing what is typed; restore puts it
// back.
func echoOff(fd int) (restore func(), ok bool) {
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, false
	}
	quiet := *old
	quiet.Lflag &^= unix.ECHO
	if unix.IoctlSetTermios(fd, unix.TCSETS, &quiet) != nil {
		return nil, false
	}
	return func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, old) }, true
}
