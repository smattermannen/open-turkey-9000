package winsvc

// install_windows.go — `kc9k install` and the end of `kc9k uninstall`.
//
// Install (an administrator's console, not through the service): kc9k.exe
// into Program Files\KC9K (SYSTEM owns it; administrators and users may
// only read and run it), the data folder ProgramData\kc9k (SYSTEM only),
// the service (automatic start, started again 0, 5 and 10 s after any
// failure, its own exit included), Program Files\KC9K on the system PATH,
// and the service started. Run again with a newer kc9k.exe it upgrades:
// the running service notices its program changed and exits, and Windows
// starts it again with the new one.
//
// Uninstall runs in the service (`kc9k uninstall`, after the same checks
// as prepare-uninstall, which refuses while anything is locked); its last
// step, FinishUninstall, runs from a copy outside Program Files once the
// command has answered: stop and delete the service, remove the program
// folder and the PATH entry. The data folder stays, as on Linux.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winapi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Security descriptors: the data folder is SYSTEM's alone; the program
// folder is SYSTEM's, readable and runnable by administrators and users.
const (
	DataSDDL    = "O:SYG:SYD:P(A;OICI;FA;;;SY)"
	ProgramSDDL = "O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;0x1200a9;;;BA)(A;OICI;0x1200a9;;;BU)"
	// installingSDDL lets administrators write while install copies.
	installingSDDL = "O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1200a9;;;BU)"
)

// ExePath is the installed kc9k.exe.
func ExePath() string { return filepath.Join(platform.InstallDir, "kc9k.exe") }

// Elevated reports whether this process runs with an administrator's
// full token.
func Elevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

// SetSecurity puts sddl's owner and protected DACL on path; children
// inherit it.
func SetSecurity(path, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner, nil, dacl, nil)
}

// Install installs or upgrades KC9K from the running kc9k.exe.
func Install(out io.Writer) error {
	if !Elevated() {
		return errors.New("install from an administrator's console (Run as administrator)")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	for _, dir := range []string{platform.InstallDir, platform.ShareDir, platform.DataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// Ownership and DACLs before anything is written into the folders.
	// Making SYSTEM the owner takes SeRestorePrivilege.
	_ = winapi.EnablePrivilege("SeRestorePrivilege")
	_ = winapi.EnablePrivilege("SeTakeOwnershipPrivilege")
	if err := SetSecurity(platform.DataDir, DataSDDL); err != nil {
		return fmt.Errorf("secure %s: %w", platform.DataDir, err)
	}
	// Administrators may write the program folder only while install
	// copies the program; then it is SYSTEM's again.
	if err := SetSecurity(platform.InstallDir, installingSDDL); err != nil {
		return fmt.Errorf("open %s: %w", platform.InstallDir, err)
	}
	target := ExePath()
	if !strings.EqualFold(filepath.Clean(self), filepath.Clean(target)) {
		if err := copyExe(self, target); err != nil {
			_ = SetSecurity(platform.InstallDir, ProgramSDDL)
			return err
		}
		fmt.Fprintf(out, "Installed %s\n", target)
	}
	if err := SetSecurity(platform.InstallDir, ProgramSDDL); err != nil {
		return fmt.Errorf("secure %s: %w", platform.InstallDir, err)
	}
	if err := addToPath(platform.InstallDir); err != nil {
		fmt.Fprintf(out, "warning: kc9k is not on the PATH: %v\n", err)
	}
	if err := InstallService(target); err != nil {
		return err
	}
	fmt.Fprintln(out, "Killer Chicken 9000 is installed and its service runs. Open a new console and type: kc9k --help")
	return nil
}

// copyExe puts src at dst. A running dst (an upgrade) is renamed aside
// first: Windows lets a running program be renamed, not overwritten.
func copyExe(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		old := dst + ".old"
		_ = os.Remove(old)
		if err := os.Rename(dst, old); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("replace %s: %w", dst, err)
		}
		// Removed at the next start if still running now.
		_ = windows.MoveFileEx(windows.StringToUTF16Ptr(old), nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
	}
	return os.Rename(tmp, dst)
}

// InstallService creates the service and starts it (or starts the one
// there). The watchdog uses it too.
func InstallService(exe string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	cfg := mgr.Config{
		DisplayName:  "Killer Chicken 9000",
		Description:  "Keeps Killer Chicken 9000's blocks in force.",
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}
	s, err := m.OpenService(ServiceName)
	if err == nil {
		// Installed already (an upgrade): the service keeps its own
		// settings (blocker.GuardService) and restarts itself with the new
		// program; administrators may not change it anyway.
		defer s.Close()
		if status, err := s.Query(); err == nil && status.State == svc.Stopped {
			return s.Start()
		}
		return nil
	}
	s, err = m.CreateService(ServiceName, exe, cfg, "service")
	if err != nil {
		return fmt.Errorf("create the service: %w", err)
	}
	defer s.Close()
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 0},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
	}, 60); err != nil {
		return fmt.Errorf("restart on failure: %w", err)
	}
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("restart on failure: %w", err)
	}
	status, err := s.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		if err := s.Start(); err != nil {
			return fmt.Errorf("start the service: %w", err)
		}
	}
	return nil
}

const envKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`

// addToPath puts dir on the system PATH once.
func addToPath(dir string) error {
	return editPath(func(parts []string) []string {
		for _, p := range parts {
			if strings.EqualFold(strings.TrimRight(p, `\`), dir) {
				return parts
			}
		}
		return append(parts, dir)
	})
}

// RemoveFromPath takes dir off the system PATH.
func RemoveFromPath(dir string) error {
	return editPath(func(parts []string) []string {
		var out []string
		for _, p := range parts {
			if !strings.EqualFold(strings.TrimRight(p, `\`), dir) {
				out = append(out, p)
			}
		}
		return out
	})
}

func editPath(edit func([]string) []string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, envKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	cur, kind, err := k.GetStringValue("Path")
	if err != nil {
		return err
	}
	var parts []string
	for _, p := range strings.Split(cur, ";") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	next := strings.Join(edit(parts), ";")
	if next == cur {
		return nil
	}
	if kind == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", next)
	} else {
		err = k.SetStringValue("Path", next)
	}
	if err != nil {
		return err
	}
	broadcastEnvironment()
	return nil
}

// broadcastEnvironment tells open programs (Explorer) the environment
// changed, so new consoles see the PATH.
func broadcastEnvironment() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	send := user32.NewProc("SendMessageTimeoutW")
	env, _ := windows.UTF16PtrFromString("Environment")
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	var result uintptr
	send.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&result)))
}

// StartFinishUninstall starts FinishUninstall from a copy of kc9k.exe
// outside Program Files, so the program folder can go. It runs after the
// command that started it has answered.
func StartFinishUninstall() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "kc9k-uninstall-")
	if err != nil {
		return err
	}
	copyPath := filepath.Join(tmpDir, "kc9k-uninstall.exe")
	if err := os.WriteFile(copyPath, data, 0o755); err != nil {
		return err
	}
	cmd := exec.Command(copyPath, "finish-uninstall", strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	return cmd.Start()
}

// FinishUninstall waits for the uninstall command (process waitPID) to
// end, then stops and deletes the service and removes the program folder
// and its PATH entry. It runs as SYSTEM from a copy of kc9k.exe.
func FinishUninstall(waitPID int) error {
	if h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(waitPID)); err == nil {
		windows.WaitForSingleObject(h, 30000)
		windows.CloseHandle(h)
	}
	time.Sleep(time.Second) // let the service send the command's last words
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(ServiceName); err == nil {
		_ = s.ResetRecoveryActions()
		_ = s.Delete()
		_, _ = s.Control(svc.Stop)
		for i := 0; i < 60; i++ {
			if st, err := s.Query(); err != nil || st.State == svc.Stopped {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		s.Close()
	}
	// The service may have put these back before it stopped.
	root := os.Getenv("SystemRoot")
	_ = exec.Command(filepath.Join(root, "System32", "schtasks.exe"), "/Delete", "/TN", `\KC9K\Watchdog`, "/F").Run()
	for _, mode := range []string{"Minimal", "Network"} {
		_ = registry.DeleteKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SafeBoot\`+mode+`\`+ServiceName)
	}
	_ = RemoveFromPath(platform.InstallDir)
	var firstErr error
	for i := 0; i < 10; i++ {
		if firstErr = os.RemoveAll(platform.InstallDir); firstErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	// This copy goes at the next start.
	if self, err := os.Executable(); err == nil {
		_ = windows.MoveFileEx(windows.StringToUTF16Ptr(self), nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
		_ = windows.MoveFileEx(windows.StringToUTF16Ptr(filepath.Dir(self)), nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
	}
	return firstErr
}
