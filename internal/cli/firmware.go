// firmware.go — kc9k firmware-lock: factory reset protection's firmware lock
// (§3.8, §4.10 Device protection).
//
//	kc9k firmware-lock [--json]       what the firmware lets KC9K see
//	kc9k firmware-lock on [--yes]     USB/network boot and the boot menu off,
//	                                  boot order locked, then a firmware password
//	kc9k firmware-lock off            puts the settings back, clears the password
//
// The password comes from standard input (on a terminal it is asked twice):
// the trusted person who keeps it types it. KC9K never stores it. On is a
// lock (R-LOCK-12: confirmed); off needs the settings password and the
// firmware password, and is refused while anything is locked.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

// Settings the firmware lock keeps: whether KC9K set it, and what the
// settings it changed were before (JSON, driver/name → value).
const (
	SettingFirmwareLock   = "device.firmware-lock"
	SettingFirmwareBefore = "device.firmware-before"
)

// realFirmwareRoot is where the firmware really is; a test database
// (KC9K_DB) never reaches it.
var realFirmwareRoot = blocker.FirmwareRoot

// firmwareDeviceless reports whether firmware must not be touched: a test's
// database with the real firmware folder.
func firmwareDeviceless() bool {
	return os.Getenv("KC9K_DB") != "" && blocker.FirmwareRoot == realFirmwareRoot
}

// firmwareStatus reads the firmware, or nothing for a test's database.
func firmwareStatus() (blocker.FirmwareStatus, error) {
	if firmwareDeviceless() {
		return blocker.FirmwareStatus{}, nil
	}
	return blocker.FirmwareLockStatus()
}

var firmwareLockCmd = &cobra.Command{
	Use:   "firmware-lock [on|off]",
	Short: "Factory reset protection: lock the firmware with a password a trusted person keeps",
	Long: `The firmware lock (factory reset protection) changes the computer's firmware
(BIOS/UEFI) settings from Linux, where the firmware allows it (Dell, Lenovo and
HP computers, /sys/class/firmware-attributes):

  - booting from USB and from the network off, and the one-time boot menu off;
  - the boot order locked;
  - then a firmware password, typed by a trusted person who keeps it.

Without that password nobody can boot other media or change the firmware
settings back, so reinstalling needs the trusted person. KC9K never stores
the password; if it is lost, the firmware's maker decides what happens.

Settings the firmware doesn't offer are done by hand in its own setup (press
F2, F1, F10 or Del while the computer starts). Changes take effect at the
next start.

  on    asks for the password (on a terminal twice; else one line on
        standard input) and asks to confirm the lock (--yes for scripts)
  off   needs the firmware password; puts back the settings it changed and
        clears the password. Refused while anything is locked.

With a firmware password already set, 'on' needs that password and keeps it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if len(args) == 0 {
			return showFirmwareLock(cmd, database)
		}
		switch args[0] {
		case "on":
			return firmwareLockOn(cmd, database)
		case "off":
			return firmwareLockOff(database)
		}
		return fmt.Errorf("use 'on' or 'off'")
	}),
}

func showFirmwareLock(cmd *cobra.Command, database *db.DB) error {
	status, err := firmwareStatus()
	if err != nil {
		return err
	}
	set, _ := database.GetSetting(SettingFirmwareLock)
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		out := struct {
			blocker.FirmwareStatus
			Available bool     `json:"available"`
			Locked    bool     `json:"locked"`
			ByKC9K    bool     `json:"set_by_kc9k"`
			Missing   []string `json:"missing"`
		}{status, status.Available(), status.Locked(), set == "on", status.Missing()}
		if out.Drivers == nil {
			out.Drivers = []string{}
		}
		if out.Settings == nil {
			out.Settings = []blocker.FirmwareSetting{}
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	if !status.Available() {
		fmt.Println("Firmware lock: this computer's firmware settings can't be changed from " + thisSystem + ".")
		fmt.Println(firmwareByHand)
		return nil
	}
	fmt.Printf("Firmware lock: %s (firmware: %s)\n", map[bool]string{true: "on", false: "off"}[status.Locked()], strings.Join(status.Drivers, ", "))
	fmt.Printf("  firmware password: %s\n", map[bool]string{true: "set", false: "not set"}[status.Password])
	for _, s := range status.Settings {
		state := s.Current
		switch {
		case s.Want == "":
			state += " (KC9K can't change it)"
		case s.Done:
			state += " (as the lock wants)"
		default:
			state += " (the lock sets " + s.Want + ")"
		}
		fmt.Printf("  %s (%s): %s\n", s.Purpose, s.Name, state)
	}
	for _, m := range status.Missing() {
		fmt.Printf("  %s: not offered by the firmware; set it in the firmware's own setup\n", m)
	}
	if status.PendingReboot {
		fmt.Println("  Changes take effect when the computer next starts.")
	}
	return nil
}

// firmwareByHand is the firmware lock done in the firmware's own setup.
const firmwareByHand = "Do it in the firmware's own setup (press F2, F1, F10 or Del while the computer starts): " +
	"turn off booting from USB and the network, lock the boot order, and set a firmware (setup) " +
	"password that a trusted person types and keeps."

// readFirmwarePassword reads the password; on a terminal twice, so a typing
// mistake doesn't lock the firmware with an unknown password.
func readFirmwarePassword(prompt string, twice bool) (string, error) {
	password, err := readPasswordStdin(prompt)
	if err != nil {
		return "", err
	}
	if twice && isTerminalInput() {
		again, err := readPasswordStdin("Type it again")
		if err != nil {
			return "", err
		}
		if again != password {
			return "", fmt.Errorf("the two passwords differ: nothing was changed")
		}
	}
	return password, nil
}

func firmwareLockOn(cmd *cobra.Command, database *db.DB) error {
	if err := requireLockPassword(database); err != nil {
		return err
	}
	status, err := firmwareStatus()
	if err != nil {
		return err
	}
	if !status.Available() {
		return fmt.Errorf("this computer's firmware settings can't be changed from %s. %s", thisSystem, firmwareByHand)
	}
	kind := "firmware lock (USB and network boot off, boot order locked, a firmware password a trusted person keeps)"
	if status.Password {
		kind = "firmware lock (USB and network boot off, boot order locked, under the firmware password already set)"
	}
	if err := confirmLocks(cmd, lockToSet{Target: "the computer's firmware", Kind: kind}); err != nil {
		return err
	}
	prompt := "Firmware password (the trusted person types it)"
	if status.Password {
		prompt = "The firmware password already set"
	}
	password, err := readFirmwarePassword(prompt, !status.Password)
	if err != nil {
		return err
	}
	before, applyErr := blocker.ApplyFirmwareLock(password)
	// What was changed is kept even when a later step failed, so 'off' can
	// put it back.
	if len(before) > 0 {
		old := map[string]string{}
		if v, _ := database.GetSetting(SettingFirmwareBefore); v != "" {
			_ = json.Unmarshal([]byte(v), &old)
		}
		for k, v := range before {
			if _, ok := old[k]; !ok {
				old[k] = v
			}
		}
		data, _ := json.Marshal(old)
		if err := database.SetSetting(SettingFirmwareBefore, string(data)); err != nil {
			return err
		}
	}
	if applyErr != nil {
		return applyErr
	}
	if err := database.SetSetting(SettingFirmwareLock, "on"); err != nil {
		return err
	}
	after, _ := firmwareStatus()
	fmt.Println("Firmware lock is on. Give the password to the trusted person; KC9K doesn't keep it.")
	for _, m := range after.Missing() {
		fmt.Printf("  %s: not offered by the firmware; set it in the firmware's own setup\n", m)
	}
	fmt.Println("Changes take effect when the computer next starts.")
	return nil
}

func firmwareLockOff(database *db.DB) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	now := time.Now()
	trusted, err := database.TrustedNow(now)
	if err != nil {
		return err
	}
	if locked, err := enforce.AnyLock(database, now, trusted); err != nil {
		return err
	} else if locked {
		return refused(database, "", fmt.Errorf("the firmware lock can't be removed while a block or setting is locked"))
	}
	if err := refuseSettingWeakening(database, "the firmware lock"); err != nil {
		return err
	}
	status, err := firmwareStatus()
	if err != nil {
		return err
	}
	if !status.Available() {
		return fmt.Errorf("this computer's firmware settings can't be changed from %s: remove the password in the firmware's own setup", thisSystem)
	}
	before := map[string]string{}
	if v, _ := database.GetSetting(SettingFirmwareBefore); v != "" {
		_ = json.Unmarshal([]byte(v), &before)
	}
	password := ""
	if status.Password {
		if password, err = readFirmwarePassword("Firmware password (the trusted person types it)", false); err != nil {
			return err
		}
	}
	if err := blocker.RemoveFirmwareLock(password, before); err != nil {
		return err
	}
	if err := database.SetSetting(SettingFirmwareBefore, ""); err != nil {
		return err
	}
	if err := database.SetSetting(SettingFirmwareLock, "off"); err != nil {
		return err
	}
	fmt.Println("Firmware lock is off: settings put back, password cleared. Changes take effect when the computer next starts.")
	return nil
}

func init() {
	addYesFlag(firmwareLockCmd)
	firmwareLockCmd.Flags().Bool("json", false, "Show the firmware's state as JSON")
	rootCmd.AddCommand(firmwareLockCmd)
}
