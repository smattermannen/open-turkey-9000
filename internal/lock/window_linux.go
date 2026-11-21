package lock

// window_linux.go — the challenge window on a Linux desktop: the embedded
// GTK script, run as the signed-in user on their X11 display.

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

//go:embed challenge_window.py
var windowScript string

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
