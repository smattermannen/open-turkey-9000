package cli

// windows_windows.go — the commands only Windows has (notes/windows.md):
// the service, the session agent, install, uninstall; and the forwarding
// of every other command to the service, as the sudo wrapper does on
// Linux.

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/daemon"
	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winagent"
	"github.com/smattermannen/kc9k/internal/winsvc"
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
)

// localCommands run in the process that was started, never forwarded.
var localCommands = map[string]bool{
	"service": true, "agent": true, "install": true, "finish-uninstall": true, "watchdog": true,
	"help": true, "--help": true, "-h": true, "completion": true,
}

// forwardIfNeeded hands the command to the service unless this process is
// the service's (LocalSystem), the command runs locally, or KC9K_DB names
// a test database. It reports whether it forwarded, and the exit code.
func forwardIfNeeded() (int, bool) {
	args := os.Args[1:]
	if platform.Privileged() || os.Getenv("KC9K_DB") != "" {
		return 0, false
	}
	if len(args) == 0 || localCommands[args[0]] {
		return 0, false
	}
	code, err := winsvc.Forward(args)
	if err != nil {
		if errors.Is(err, winsvc.ErrNoService) {
			fmt.Fprintln(os.Stderr, "Error: the Killer Chicken 9000 service is not running (install it with 'kc9k install' from an administrator's console)")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1, true
	}
	return code, true
}

var serviceCmd = &cobra.Command{
	Use:    "service",
	Short:  "Run as the Windows service (started by Windows)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if ok, err := svc.IsWindowsService(); err != nil || !ok {
			return errors.New("the service is started by Windows ('kc9k install' sets it up)")
		}
		logToFile()
		return svc.Run(winsvc.ServiceName, kc9kService{})
	},
}

// logToFile sends the service's log to ProgramData\kc9k\kc9k.log; a log
// grown past 10 MB starts again, the old one kept as kc9k.log.1.
func logToFile() {
	path := filepath.Join(platform.DataDir, "kc9k.log")
	if info, err := os.Stat(path); err == nil && info.Size() > 10<<20 {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

// kc9kService is the service: the daemon's cycle, the pipe, the agents.
type kc9kService struct{}

func (kc9kService) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	stop := make(chan struct{})
	winagent.Default = winagent.NewManager()
	go winagent.Default.Run(stop)
	go func() {
		if err := winsvc.Serve(stop); err != nil {
			log.Printf("service: commands can't reach the service: %v", err)
		}
	}()
	done := make(chan error, 1)
	go func() { done <- daemon.RunUntil(stop) }()
	program := programStamp()
	check := time.NewTicker(5 * time.Second)
	defer check.Stop()

	const accepts = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPowerEvent
	status <- svc.Status{State: svc.Running, Accepts: accepts}
	log.Println("service: running")
	for {
		select {
		case c := <-requests:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.PowerEvent:
				const pbtAPMSuspend = 4
				if c.EventType == pbtAPMSuspend && blocker.SleepHookOn() {
					// The sleep hook (R-MODE-5): the wallpaper default
					// before sleep; Windows waits a little for it.
					if self, err := os.Executable(); err == nil {
						out, err := exec.Command(self, "wallpaper", "before-sleep").CombinedOutput()
						if err != nil {
							log.Printf("service: before sleep: %v: %s", err, out)
						}
					}
				}
			case svc.Stop, svc.Shutdown:
				log.Printf("service: asked to stop (%v)", c.Cmd)
				status <- svc.Status{State: svc.StopPending}
				close(stop)
				select {
				case <-done:
				case <-time.After(10 * time.Second):
				}
				return false, 0
			}
		case err := <-done:
			log.Printf("service: the cycle ended: %v", err)
			return false, 1 // a failure: Windows starts the service again
		case <-check.C:
			if now := programStamp(); now != program {
				// An upgrade replaced kc9k.exe: exit as a failure so
				// Windows starts the new one.
				log.Printf("service: kc9k.exe was replaced; restarting")
				close(stop)
				return false, 1
			}
		}
	}
}

// programStamp identifies the installed kc9k.exe's contents (size and
// time), "" if it can't be read.
func programStamp() string {
	info, err := os.Stat(winsvc.ExePath())
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano())
}

var agentCmd = &cobra.Command{
	Use:    "agent",
	Short:  "Run as the session agent (started by the service)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(io.Discard)
		return winagent.RunAgent()
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Killer Chicken 9000 and its service (administrator's console)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return winsvc.Install(os.Stdout)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Killer Chicken 9000 (refused while anything is locked); your data stays",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !platform.Privileged() {
			return errors.New("uninstall runs through the service")
		}
		if err := prepareUninstall(); err != nil {
			return err
		}
		if err := blocker.RemoveWatchdogTask(); err != nil {
			return fmt.Errorf("the layers are removed, but the watchdog task could not be: %w", err)
		}
		blocker.RemoveSafeBoot()
		if err := winsvc.StartFinishUninstall(); err != nil {
			return fmt.Errorf("the layers are removed, but the service and program could not be: %w", err)
		}
		fmt.Println("The service stops and the program is removed in a few seconds. Your data stays in " + platform.DataDir + ".")
		return nil
	},
}

var finishUninstallCmd = &cobra.Command{
	Use:    "finish-uninstall",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !platform.Privileged() {
			return errors.New("only the service's uninstall runs this")
		}
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		return winsvc.FinishUninstall(pid)
	},
}

var watchdogCmd = &cobra.Command{
	Use:    "watchdog",
	Short:  "Start the service again if it was stopped (the SYSTEM task runs it every minute)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !platform.Privileged() {
			return errors.New("the watchdog runs as SYSTEM, from its scheduled task")
		}
		return winsvc.InstallService(winsvc.ExePath())
	},
}

func init() {
	// What strict mode and no-escape do on Windows (notes/windows.md W5).
	strictCmd.Short = "Deny administrators what would stop KC9K while a block is locked"
	strictCmd.Long = strings.Replace(strictCmd.Long, strictCmd.Long[:strings.Index(strictCmd.Long, "\n\nStrict mode is off")],
		`With strict mode on, while a block or setting is locked or Frozen Chicken
runs, administrators are denied what would stop Killer Chicken 9000: its
service's registry key, its program folder, the hosts file, the browsers'
policies, Safe Mode's settings, the boot configuration and the recovery
environment's; programs run from your own folders (your profile, USB
drives) are closed. Only kc9k itself keeps working. Everything comes back
by itself when the lock ends.`, 1)
	if i := strings.Index(strictCmd.Long, "\n\nIf something goes wrong"); i >= 0 {
		strictCmd.Long = strictCmd.Long[:i]
	}
	strictCmd.Long = strings.Replace(strictCmd.Long, "sudo is taken away only while", "administrators are denied only while", 1)
	noEscapeCmd.Long = `With no-escape on, the recovery environment is off (Reset this PC, Startup
Repair and its command prompt), automatic repair doesn't start, and the boot
menu is hidden. KC9K's service runs in Safe Mode too. It can't be turned off
during a lock.

Booting a USB stick still works unless the firmware is locked with a password.
Test it in a virtual machine first.`

	rootCmd.AddCommand(serviceCmd, agentCmd, installCmd, uninstallCmd, finishUninstallCmd, watchdogCmd)
}
