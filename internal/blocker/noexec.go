package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// noexecDirs are where portable programs land: AppImages, downloaded browsers
// and anything a user unpacks. A bind mount lets each be noexec even when it
// shares a filesystem with the rest of the system.
var noexecDirs = []string{"/home", "/tmp", "/var/tmp", "/dev/shm"}

// mountOptions returns the mount options per mount point, last mount winning.
func mountOptions(mountinfo string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 5 {
			out[fields[4]] = fields[5]
		}
	}
	return out
}

func hasOption(options, want string) bool {
	for _, o := range strings.Split(options, ",") {
		if o == want {
			return true
		}
	}
	return false
}

func readMounts() (map[string]string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	return mountOptions(string(data)), nil
}

// ApplyNoexec stops programs in user-writable folders from starting. Running
// programs carry on, and interpreted scripts still run through their
// interpreter.
func ApplyNoexec() error {
	mounts, err := readMounts()
	if err != nil {
		return err
	}
	var firstErr error
	for _, dir := range noexecDirs {
		if hasOption(mounts[dir], "noexec") {
			continue
		}
		commands := [][]string{{"--bind", dir, dir}, {"-o", "remount,bind,noexec,nosuid,nodev", dir}}
		for _, args := range commands {
			if output, err := exec.Command("mount", args...).CombinedOutput(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("mount %v: %s (error: %w)", args, strings.TrimSpace(string(output)), err)
			}
		}
	}
	return firstErr
}

// RemoveNoexec lifts what ApplyNoexec added.
func RemoveNoexec() error {
	mounts, err := readMounts()
	if err != nil {
		return err
	}
	var firstErr error
	for _, dir := range noexecDirs {
		if !hasOption(mounts[dir], "noexec") {
			continue
		}
		if output, err := exec.Command("umount", "-l", dir).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("umount %s: %s (error: %w)", dir, strings.TrimSpace(string(output)), err)
		}
	}
	return firstErr
}
