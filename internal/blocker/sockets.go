// sockets.go — closing root-equivalent control sockets during strict mode.
//
// Strict mode takes sudo and pkexec away, but some groups are root in all but
// name: a member of docker can start a container with the host's disk
// mounted, and members of lxd, incus or libvirt can do the same with their
// containers and virtual machines. Their power comes from a control socket
// the group may open. While strict mode holds, those sockets are closed to
// everyone but root, and their modes are restored when it lifts.
package blocker

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// privilegedSockets are the control sockets whose users are effectively root.
var privilegedSockets = []string{
	"/run/docker.sock",
	"/run/libvirt/libvirt-sock",
	"/var/snap/lxd/common/lxd/unix.socket",
	"/var/lib/lxd/unix.socket",
	"/var/lib/incus/unix.socket",
}

// socketStatePath remembers the modes the sockets had, to restore them.
var socketStatePath = "/var/lib/kc9k/locked-sockets.json"

func readSocketState() (map[string]fs.FileMode, error) {
	state := map[string]fs.FileMode{}
	data, err := os.ReadFile(socketStatePath)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("reading %s: %w", socketStatePath, err)
	}
	return state, nil
}

func writeSocketState(state map[string]fs.FileMode) error {
	if len(state) == 0 {
		if err := os.Remove(socketStatePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicWriteFile(socketStatePath, data, 0600)
}

// LockPrivilegedSockets makes every root-equivalent control socket root-only,
// remembering its earlier mode. It is safe to repeat: a socket that was
// recreated (a restarted daemon) is closed again, and the first remembered
// mode is kept.
func LockPrivilegedSockets() error {
	state, err := readSocketState()
	if err != nil {
		return err
	}
	changed := false
	var firstErr error
	for _, path := range privilegedSockets {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&fs.ModeSocket == 0 {
			continue // not installed, or not a socket
		}
		perm := info.Mode().Perm()
		if perm&0o077 == 0 {
			continue // already root-only
		}
		if _, known := state[path]; !known {
			state[path] = perm
			changed = true
		}
		if err := os.Chmod(path, 0o600); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing %s: %w", path, err)
		}
	}
	if changed {
		if err := writeSocketState(state); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// UnlockPrivilegedSockets restores the modes LockPrivilegedSockets saved.
func UnlockPrivilegedSockets() error {
	state, err := readSocketState()
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(state))
	for path := range state {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var firstErr error
	for _, path := range paths {
		if filepath.IsAbs(path) {
			if err := os.Chmod(path, state[path]); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = fmt.Errorf("reopening %s: %w", path, err)
				continue
			}
		}
		delete(state, path)
	}
	if err := writeSocketState(state); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
