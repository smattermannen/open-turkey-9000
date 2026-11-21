package lock

// window.go — the text lock's challenge without a terminal (SPEC §4.6,
// R-LOCK-5; §6 2026-11-14, 2026-11-20). The command opens a window itself,
// as the signed-in user on their desktop, and reads the keys typed there one
// by one; it never reads an answer piped to it. The window
// (challenge_window.py, embedded, so nothing on disk can stand in for it)
// only shows the pages and reports keys; scoring, paste and burst detection
// happen here.

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

//go:embed challenge_window.py
var windowScript string

// windowPageLines is how many challenge lines one page of the window holds.
const windowPageLines = 10

// burstMillis: more than pasteBurst printable keys within this many
// milliseconds can't be typed by hand (the window's backstop like the
// terminal's, for input that bypasses its paste checks).
const burstMillis = 5

// StartWindow opens the challenge window for pages and returns the key
// events it writes and a function that waits for it to close. Tests replace
// it.
var StartWindow = startWindow

// windowPython runs the window: python3 with GTK 3 (a package dependency).
// Never taken from the environment: whatever runs the window could type
// for the user.
var windowPython = "python3"

func startWindow(pages []string) (io.Reader, func() error, error) {
	python, err := exec.LookPath(windowPython)
	if err != nil {
		return nil, nil, fmt.Errorf("%s is not installed", windowPython)
	}
	input, err := json.Marshal(map[string]any{"pages": pages, "accuracy": MinAccuracy * 100, "width": maxLineWidth})
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command(python, "-I", "-c", windowScript)
	if err := asDesktopUser(cmd); err != nil {
		return nil, nil, err
	}
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	wait := func() error {
		_ = cmd.Process.Kill() // closed already, or the keys were read
		if err := cmd.Wait(); err != nil && stderr.Len() > 0 {
			return fmt.Errorf("%v: %s", err, lastLine(stderr.String()))
		}
		return nil
	}
	return out, wait, nil
}

// lastLine is the last non-empty line of text (a Python traceback's error).
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// asDesktopUser makes cmd run on the desktop of the user who ran kc9k: under
// sudo (the wrapper) as that user, on the display sudo kept (DISPLAY,
// XAUTHORITY), never as root; without root, as the user, in their own
// environment. X11 only (GDK_BACKEND), where a synthetic key is told apart.
func asDesktopUser(cmd *exec.Cmd) error {
	if os.Geteuid() != 0 {
		if os.Getenv("DISPLAY") == "" {
			return fmt.Errorf("no desktop to open it on (DISPLAY is not set)")
		}
		cmd.Env = append(os.Environ(), "GDK_BACKEND=x11")
		return nil
	}
	uid := os.Getenv("SUDO_UID")
	if uid == "" || uid == "0" {
		return fmt.Errorf("run by root without sudo: no user's desktop to open it on")
	}
	u, err := user.LookupId(uid)
	if err != nil {
		return err
	}
	display, xauth := os.Getenv("DISPLAY"), os.Getenv("XAUTHORITY")
	if display == "" {
		display = ":0"
	}
	if xauth == "" {
		xauth = u.HomeDir + "/.Xauthority"
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "C.UTF-8"
	}
	cmd.Env = []string{"DISPLAY=" + display, "XAUTHORITY=" + xauth, "HOME=" + u.HomeDir, "USER=" + u.Username,
		"LOGNAME=" + u.Username, "LANG=" + lang, "PATH=/usr/local/bin:/usr/bin:/bin", "GDK_BACKEND=x11",
		"XDG_RUNTIME_DIR=/run/user/" + u.Uid, "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + u.Uid + "/bus"}
	cmd.Dir = "/"
	userID, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return err
	}
	groupID, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(userID), Gid: uint32(groupID)}}
	return nil
}

// runChallengeWindow has the challenge typed in the window.
func runChallengeWindow(challenge string) (bool, error) {
	pageList := pages(challenge, maxLineWidth, windowPageLines)
	fmt.Println("No terminal: the challenge opens in its own window on your desktop. Type it there.")
	events, wait, err := StartWindow(pageList)
	if err != nil {
		return false, fmt.Errorf("no terminal, and the challenge window didn't open: %w (an answer piped in is never read)", err)
	}
	typed, outcome, readErr := readWindowKeys(events, pageList)
	waitErr := wait()
	if readErr != nil {
		return false, readErr
	}
	switch outcome {
	case outcomePaste:
		fmt.Println("Paste detected. Unlocking requires typing the text by hand.")
		return false, nil
	case outcomeSynthetic:
		fmt.Println("A key that wasn't typed on the keyboard. Unlocking requires typing the text by hand.")
		return false, nil
	case outcomeCancel:
		if waitErr != nil {
			return false, fmt.Errorf("the challenge window closed: %w", waitErr)
		}
		fmt.Println("Cancelled.")
		return false, nil
	}
	return reportAccuracy(score(typed, challenge)), nil
}

// readWindowKeys reads the window's key lines (challenge_window.py) and
// returns what was typed over every page, or how it ended otherwise.
func readWindowKeys(events io.Reader, pageList []string) (string, inputOutcome, error) {
	var total, page strings.Builder
	var times []int64 // the last printable keys' times, for burst detection
	n := 0
	scanner := bufio.NewScanner(events)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "p":
			return "", outcomePaste, nil
		case "s":
			return "", outcomeSynthetic, nil
		case "c":
			return "", outcomeCancel, nil
		case "b":
			if s := page.String(); s != "" {
				page.Reset()
				page.WriteString(s[:len(s)-1])
			}
		case "n":
			total.WriteString(page.String())
			page.Reset()
			if n++; n >= len(pageList) {
				return total.String(), outcomeDone, nil
			}
		case "k":
			if len(fields) < 3 {
				continue
			}
			code, err1 := strconv.Atoi(fields[1])
			at, err2 := strconv.ParseInt(fields[2], 10, 64)
			if err1 != nil || err2 != nil || code < 0x20 || code >= 0x7f {
				continue
			}
			times = append(times, at)
			if len(times) > pasteBurst+1 {
				times = times[1:]
			}
			if len(times) == pasteBurst+1 && times[len(times)-1]-times[0] <= burstMillis {
				return "", outcomePaste, nil
			}
			if page.Len() < len(pageList[n])+inputSlack {
				page.WriteByte(byte(code))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", outcomeDone, fmt.Errorf("failed to read the challenge window: %w", err)
	}
	return "", outcomeCancel, nil // closed before the last page was done
}
