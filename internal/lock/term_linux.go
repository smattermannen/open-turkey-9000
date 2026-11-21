package lock

// term_linux.go — the terminal the challenge is typed in, by ioctl.

import "golang.org/x/sys/unix"

// linesPerPageFor picks how many challenge lines fit the terminal at fd,
// with room for the header, the instructions and the same number of typed
// lines below them.
func linesPerPageFor(fd int) int {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return defaultPageLines
	}
	return pageLinesForRows(int(ws.Row))
}

// lineWidthFor picks the challenge line width for the terminal at fd: at most
// maxLineWidth, and narrow enough that a full line never auto-wraps.
func lineWidthFor(fd int) int {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return maxLineWidth
	}
	width := int(ws.Col) - 1
	if width > maxLineWidth {
		return maxLineWidth
	}
	if width < 20 {
		return 20
	}
	return width
}

// isTerminal reports whether fd refers to a terminal.
func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

// makeRaw puts the terminal into raw mode (no echo, no line buffering, no
// signal generation) and returns a function that restores the previous state.
// We handle Ctrl-C ourselves (as a cancel) since ISIG is disabled.
func makeRaw(fd int) (func(), error) {
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	saved := *old

	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, &saved)
	}, nil
}
