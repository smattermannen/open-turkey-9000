//go:build linux

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// readAsCallerUpTo is readAsCaller for files of up to limit bytes.
func readAsCallerUpTo(path string, limit int64) ([]byte, error) {
	uidStr, gidStr := os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID")
	if uidStr == "" || gidStr == "" || os.Geteuid() != 0 {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return readLimited(f, limit)
	}
	uid, err := strconv.ParseUint(uidStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid SUDO_UID: %w", err)
	}
	gid, err := strconv.ParseUint(gidStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid SUDO_GID: %w", err)
	}
	c := exec.Command("/bin/cat", "--", path)
	c.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("cannot read file as user %d: %w", uid, err)
	}
	data, readErr := readLimited(out, limit)
	_ = c.Process.Kill() // stops cat if the file was too large
	waitErr := c.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil && len(data) == 0 {
		return nil, fmt.Errorf("cannot read file as user %d: %w", uid, waitErr)
	}
	return data, nil
}
