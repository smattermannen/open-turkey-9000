//go:build linux

package blocker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// noexecDirs are where portable programs land: AppImages, downloaded browsers
// and anything a user unpacks. A bind mount lets each be noexec even when it
// shares a filesystem with the rest of the system. The bind is recursive, so
// what is mounted inside (a guest session's home, a separate home partition)
// stays in view; those mounts are then made noexec in place (see
// noexecSubmounts), as are mounts made inside later.
var noexecDirs = []string{"/home", "/tmp", "/var/tmp", "/dev/shm"}

// runtimeDirs holds each signed-in user's runtime folder ($XDG_RUNTIME_DIR,
// /run/user/<uid>), a tmpfs of its own that the user can write to and,
// without this, run programs from (B3). Its mounts are made noexec in place
// and made exec again when the lockdown lifts.
const runtimeDirs = "/run/user"

// noexecStatePath lists the runtime folder mounts ApplyNoexec made noexec,
// so RemoveNoexec gives back exactly those (in /run: gone at a restart, as
// the mounts are).
var noexecStatePath = "/run/kc9k-noexec-mounts"

// mountOptions returns the options of the mount in view at each mount
// point. Mounts can be stacked on one point, and a mount made inside a
// shared folder is copied under each of its peers, some hidden beneath
// later mounts (a guest's home under /tmp appears both beneath and inside
// the noexec bind of /tmp); only the one a path reaches counts. Mount
// points with escaped characters keep their escapes.
func mountOptions(mountinfo string) map[string]string {
	out := map[string]string{}
	for point, m := range visibleMounts(mountinfo) {
		out[point] = m.opts
	}
	return out
}

// mount is a line of /proc/self/mountinfo: its ID, the ID of the mount it
// sits on and that mount's point, the folder of its filesystem it shows
// (root), its mount point and options.
type mount struct{ id, parent, parentPoint, parentRoot, root, point, opts string }

// visibleMounts returns the mount in view at each mount point (see
// mountOptions).
func visibleMounts(mountinfo string) map[string]mount {
	var all []mount
	ids := map[string]bool{}
	points, roots := map[string]string{}, map[string]string{}
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 5 {
			all = append(all, mount{id: fields[0], parent: fields[1], root: fields[3], point: fields[4], opts: fields[5]})
			ids[fields[0]] = true
			points[fields[0]], roots[fields[0]] = fields[4], fields[3]
		}
	}
	for i := range all {
		all[i].parentPoint, all[i].parentRoot = points[all[i].parent], roots[all[i].parent]
	}
	// on lists the mounts at a point, by the mount they sit on.
	on := map[string]map[string]mount{}
	for _, m := range all {
		if on[m.point] == nil {
			on[m.point] = map[string]mount{}
		}
		on[m.point][m.parent] = m
	}
	// top follows mounts stacked on m at its own point.
	top := func(m mount) mount {
		for i := 0; i < len(all); i++ {
			next, ok := on[m.point][m.id]
			if !ok {
				break
			}
			m = next
		}
		return m
	}
	visible := map[string]mount{}
	var reach func(point string) (mount, bool)
	reach = func(point string) (mount, bool) {
		if m, ok := visible[point]; ok {
			return m, true
		}
		var found mount
		ok := false
		if point == "/" {
			for _, m := range all {
				if m.point == "/" && !ids[m.parent] {
					found, ok = m, true
					break
				}
			}
		} else {
			// It sits on the nearest mount in view above it.
			for parent := filepath.Dir(point); ; parent = filepath.Dir(parent) {
				if _, has := on[parent]; has {
					if pm, has := reach(parent); has {
						found, ok = on[point][pm.id]
						break
					}
				}
				if parent == "/" {
					break
				}
			}
		}
		if !ok {
			return mount{}, false
		}
		found = top(found)
		visible[point] = found
		return found, true
	}
	out := map[string]mount{}
	for point := range on {
		if m, ok := reach(point); ok {
			out[point] = m
		}
	}
	return out
}

// ourBind reports whether m, in view at a noexec folder, is the bind mount
// ApplyNoexec put there: the folder bound onto itself, so it sits on a mount
// at the same point (a /dev/shm tmpfs, a /home partition) or shows the same
// folder of the filesystem it sits on (/tmp on the root filesystem, "/@/tmp"
// on a btrfs subvolume "/@"). A folder that is a noexec mount of its own (a
// tmpfs /tmp mounted noexec by fstab) isn't, and is never unmounted.
func ourBind(m mount) bool {
	if m.parentPoint == "" {
		return false
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(m.point, m.parentPoint), "/")
	return m.parentPoint == m.point || m.root == filepath.Join(m.parentRoot, rel)
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
	exempt = ValidExemptFolders(exempt)
	var firstErr error
	plan := noexecPlan(mounts, exempt)
	for _, args := range plan {
		if output, err := exec.Command("mount", args...).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("mount %v: %s (error: %w)", args, strings.TrimSpace(string(output)), err)
		}
	}
	if len(plan) > 0 {
		if mounts, err = readMounts(); err != nil {
			return err
		}
	}
	subs := noexecSubmounts(mounts, exempt)
	if len(subs) == 0 {
		return firstErr
	}
	var runtime []string
	for _, dir := range subs {
		if err := remountExec(dir, mounts[dir], false); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if strings.HasPrefix(dir, runtimeDirs+"/") {
			runtime = append(runtime, dir)
		}
	}
	if err := recordNoexecMounts(runtime); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// noexecSubmounts lists the mounts inside the noexec folders and people's
// runtime folders that still allow running programs, leaving out exempt folders and
// what is mounted inside them. Paths with escaped characters are skipped.
func noexecSubmounts(mounts map[string]string, exempt []string) []string {
	var out []string
	for dir, opts := range mounts {
		if hasOption(opts, "noexec") || strings.Contains(dir, "\\") || insideAny(dir, exempt, true) {
			continue
		}
		if insideAny(dir, noexecDirs, false) || runtimeDirOfPerson(dir) {
			out = append(out, dir)
		}
	}
	sort.Strings(out)
	return out
}

// runtimeDirOfPerson reports whether dir is in a person's runtime folder
// (root's and the display manager's are left alone).
func runtimeDirOfPerson(dir string) bool {
	rest, ok := strings.CutPrefix(dir, runtimeDirs+"/")
	if !ok {
		return false
	}
	uid, err := strconv.ParseUint(strings.SplitN(rest, "/", 2)[0], 10, 32)
	return err == nil && personUser(uid)
}

// insideAny reports whether dir is strictly inside one of bases, or equal to
// one when orEqual is set.
func insideAny(dir string, bases []string, orEqual bool) bool {
	for _, base := range bases {
		if strings.HasPrefix(dir, base+"/") || (orEqual && dir == base) {
			return true
		}
	}
	return false
}

// remountExec changes a mount, in place, to noexec (or back to exec), always
// nosuid and nodev, read-only if it was.
func remountExec(dir, opts string, exec bool) error {
	flags := uintptr(unix.MS_REMOUNT | unix.MS_BIND | unix.MS_NOSUID | unix.MS_NODEV)
	if !exec {
		flags |= unix.MS_NOEXEC
	}
	if hasOption(opts, "ro") {
		flags |= unix.MS_RDONLY
	}
	if err := unix.Mount("", dir, "", flags, ""); err != nil {
		return fmt.Errorf("remount %s (exec %v): %w", dir, exec, err)
	}
	return nil
}

// recordNoexecMounts adds dirs to the runtime folder mounts made noexec.
func recordNoexecMounts(dirs []string) error {
	if len(dirs) == 0 {
		return nil
	}
	known := readNoexecMounts()
	before := len(known)
	for _, d := range dirs {
		if !slices.Contains(known, d) {
			known = append(known, d)
		}
	}
	if len(known) == before {
		return nil
	}
	return atomicWriteFile(noexecStatePath, []byte(strings.Join(known, "\n")+"\n"), 0o600)
}

func readNoexecMounts() []string {
	data, err := os.ReadFile(noexecStatePath)
	if err != nil {
		return nil
	}
	return strings.Fields(string(data))
}

// noexecPlan lists the mount commands that make the noexec folders noexec
// and each exempt folder exec again, skipping what is already so.
func noexecPlan(mounts map[string]string, exempt []string) [][]string {
	var plan [][]string
	for _, dir := range noexecDirs {
		if !hasOption(mounts[dir], "noexec") {
			plan = append(plan, []string{"--rbind", dir, dir}, []string{"-o", "remount,bind,noexec,nosuid,nodev", dir})
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

// RemoveNoexec lifts what ApplyNoexec added: the runtime folder mounts it
// made noexec run programs again, and the bind mounts go (removeNoexecPlan).
// exempt names the exempt folders, whose exec binds go too.
func RemoveNoexec(exempt ...string) error {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	mounts := visibleMounts(string(data))
	var firstErr error
	if known := readNoexecMounts(); len(known) > 0 {
		for _, dir := range known {
			if m, ok := mounts[dir]; ok && hasOption(m.opts, "noexec") {
				if err := remountExec(dir, m.opts, true); err != nil && firstErr == nil {
					firstErr = err
				}
			}
		}
		if err := os.Remove(noexecStatePath); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	for _, args := range removeNoexecPlan(mounts, exempt) {
		if output, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%v: %s (error: %w)", args, strings.TrimSpace(string(output)), err)
		}
	}
	return firstErr
}

// removeNoexecPlan lists the commands that take away ApplyNoexec's bind
// mounts (ourBind; a folder noexec of its own is left alone). The exempt
// folders' exec binds inside one go first: made while the bind was shared,
// each has a copy beneath it, which goes with it. Then the bind is made
// private before it goes, so that the copies inside it of mounts made
// meanwhile (a guest's home, mounted at login) go without the mounts beneath
// them, which come back into view.
func removeNoexecPlan(mounts map[string]mount, exempt []string) [][]string {
	var plan [][]string
	for _, dir := range noexecDirs {
		m, ok := mounts[dir]
		if !ok || !hasOption(m.opts, "noexec") || !ourBind(m) {
			continue
		}
		for _, e := range exempt {
			if em, ok := mounts[e]; ok && strings.HasPrefix(e, dir+"/") && !hasOption(em.opts, "noexec") && ourBind(em) {
				plan = append(plan, []string{"umount", "-l", e})
			}
		}
		plan = append(plan, []string{"mount", "--make-rprivate", dir}, []string{"umount", "-l", dir})
	}
	return plan
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

// CloseNoexecPrograms closes people's programs that strict mode forbids but
// noexec folders alone don't stop (B3), and returns how many it closed:
//   - a Flatpak app from a user installation (flatpak --user, in
//     ~/.local/share/flatpak) or on a user-installed runtime, in a noexec
//     folder and not exempt: one started before the lockdown keeps its own
//     view of its files and could go on starting programs from them;
//   - a program whose executable file a person owns (copied there by them)
//     in a noexec folder or a runtime folder, outside the exempt folders:
//     one started before the lockdown;
//   - one whose executable a person owns below a folder anyone can write to
//     on a mount that allows running programs (/var/crash on Ubuntu,
//     /var/lib/BrlAPI, a Timeshift snapshot's tmp).
//
// A person's file elsewhere (a tarball unpacked into /opt keeping its
// owner) is left alone. Only processes in the daemon's own mount namespace
// are judged by path; containers (Steam's, snaps) see other paths.
func CloseNoexecPrograms(exempt []string) int {
	exempt = ValidExemptFolders(exempt)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	ownNS, _ := os.Readlink("/proc/self/ns/mnt")
	closed := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() || !ownedByUser(pid) {
			continue
		}
		dir := "/proc/" + e.Name()
		shut := false
		if info, err := os.ReadFile(dir + "/root/.flatpak-info"); err == nil {
			shut = flatpakFromNoexec(string(info), exempt)
		} else if ns, err := os.Readlink(dir + "/ns/mnt"); err == nil && ns == ownNS {
			shut = personsProgram(dir+"/exe", exempt)
		}
		if shut && syscall.Kill(pid, syscall.SIGKILL) == nil {
			closed++
		}
	}
	return closed
}

// personsProgram reports whether the executable at exe (a /proc/<pid>/exe
// link) is a person's file strict mode doesn't let run (CloseNoexecPrograms).
func personsProgram(exe string, exempt []string) bool {
	path, err := os.Readlink(exe)
	if err != nil {
		return false
	}
	var st syscall.Stat_t
	if err := syscall.Stat(exe, &st); err != nil || !personUser(uint64(st.Uid)) {
		return false
	}
	path = strings.TrimSuffix(path, " (deleted)")
	return forbiddenPlace(path, exempt, worldWritableAbove(path))
}

// forbiddenPlace reports whether a person's executable at path may not run
// in strict mode: in a noexec folder or a person's runtime folder and not
// exempt, or below a folder anyone can write to (shared).
func forbiddenPlace(path string, exempt []string, shared bool) bool {
	if insideAny(path, exempt, false) {
		return false
	}
	return shared || insideAny(path, noexecDirs, false) || runtimeDirOfPerson(path)
}

// worldWritableAbove reports whether a folder above path lets anyone write.
func worldWritableAbove(path string) bool {
	for dir := filepath.Dir(path); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && info.Mode().Perm()&0o002 != 0 {
			return true
		}
	}
	return false
}

// flatpakFromNoexec reports whether a sandbox's /.flatpak-info names an app
// or runtime in a noexec folder outside the exempt folders.
func flatpakFromNoexec(info string, exempt []string) bool {
	inInstance := false
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inInstance = line == "[Instance]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !inInstance || !ok || (key != "app-path" && key != "runtime-path") {
			continue
		}
		path := filepath.Clean(value)
		if insideAny(path, noexecDirs, false) && !insideAny(path, exempt, true) {
			return true
		}
	}
	return false
}
