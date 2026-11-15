package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// ApplyNoexec stops programs in user-writable folders from starting, except
// in the exempt folders (§3.8, strict mode). Running programs carry on, and
// interpreted scripts still run through their interpreter.
func ApplyNoexec(exempt ...string) error {
	mounts, err := readMounts()
	if err != nil {
		return err
	}
	var firstErr error
	for _, args := range noexecPlan(mounts, ValidExemptFolders(exempt)) {
		if output, err := exec.Command("mount", args...).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("mount %v: %s (error: %w)", args, strings.TrimSpace(string(output)), err)
		}
	}
	return firstErr
}

// noexecPlan lists the mount commands that make the noexec folders noexec
// and each exempt folder exec again, skipping what is already so.
func noexecPlan(mounts map[string]string, exempt []string) [][]string {
	var plan [][]string
	for _, dir := range noexecDirs {
		if !hasOption(mounts[dir], "noexec") {
			plan = append(plan, []string{"--bind", dir, dir}, []string{"-o", "remount,bind,noexec,nosuid,nodev", dir})
		}
	}
	for _, dir := range exempt {
		if opts, ok := mounts[dir]; !ok || hasOption(opts, "noexec") {
			plan = append(plan, []string{"--bind", dir, dir}, []string{"-o", "remount,bind,exec,nosuid,nodev", dir})
		}
	}
	return plan
}

// ValidExemptFolders keeps the folders strict mode may exempt: absolute,
// clean, real directories (no symlink on the way) inside one of the noexec
// folders, not one of those itself.
func ValidExemptFolders(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if ExemptFolderError(d) == nil {
			out = append(out, d)
		}
	}
	return out
}

// ExemptFolderError says why a folder can't be exempted, or nil.
func ExemptFolderError(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return fmt.Errorf("%q isn't a clean absolute path", dir)
	}
	inside := false
	for _, base := range noexecDirs {
		inside = inside || strings.HasPrefix(dir, base+"/")
	}
	if !inside {
		return fmt.Errorf("%s isn't inside %s", dir, strings.Join(noexecDirs, ", "))
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	if real != dir {
		return fmt.Errorf("%s goes through a symbolic link", dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s isn't a folder", dir)
	}
	return nil
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

// UnexemptFolder takes away an exempt folder's exec mount, if it has one,
// so the noexec folder beneath holds again.
func UnexemptFolder(dir string) error {
	mounts, err := readMounts()
	if err != nil {
		return err
	}
	if opts, ok := mounts[dir]; !ok || hasOption(opts, "noexec") {
		return nil
	}
	if output, err := exec.Command("umount", "-l", dir).CombinedOutput(); err != nil {
		return fmt.Errorf("umount %s: %s (error: %w)", dir, strings.TrimSpace(string(output)), err)
	}
	return nil
}
