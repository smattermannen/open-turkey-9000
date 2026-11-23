package db

// syncclear.go — Download all (SPEC §4.14, cli/sync.go) replaces the whole
// setup: ClearForSync removes, in one transaction, everything a project file
// brings back, and leaves what belongs to the device (places and their
// networks, statistics, the device's settings). Place rules, modes' places
// and wallpapers go with their blocks and modes; the caller saves them by
// name first and puts them back.

import "fmt"

// syncedSettingPrefixes and syncedSettings are the settings a project file
// holds (cli/project.go buildProject), cleared so the file's values are the
// only ones. tamper.clock isn't cleared (TrustedNow needs it set; the
// import writes the file's value).
var (
	syncedSettingPrefixes = []string{"browser.", "option.", AccessPasswordSetting, SettingDraftOf, "draft.", "mode.wallpapers."}
	syncedSettings        = []string{"tamper.file-urls", "tamper.task-managers", "tamper.private",
		"tamper.guest", "tamper.extension-stop", "tamper.installer", "tamper.inactive-tabs", "tamper.embedded",
		"tamper.grace", SettingUnsupportedBrowsers, SettingNepsis, "use.strict", "notifications",
		"settings.password", "settings.password.locks", "circadian"}
)

// ClearForSync removes every block (and with it its lists, locks, schedules,
// place rules), preset, list and phrase set preset, saved schedule, mode but
// monk mode (whose windows, schedules and places are emptied), scheduled
// freeze, hotkey, setting lock not named in keepLocks, and the synced
// settings.
func (d *DB) ClearForSync(keepLocks []string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		"DELETE FROM blocks", "DELETE FROM clusters", "DELETE FROM templates", "DELETE FROM presets",
		"DELETE FROM saved_schedules", "DELETE FROM modes WHERE monk = 0",
		"DELETE FROM mode_windows", "DELETE FROM mode_schedules", "DELETE FROM mode_places",
		"UPDATE modes SET default_on = 0, need_both = 0",
		"DELETE FROM freeze_schedules", "DELETE FROM hotkeys",
	} {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("failed to clear the setup (%s): %w", q, err)
		}
	}
	rows, err := tx.Query("SELECT name FROM setting_locks")
	if err != nil {
		return err
	}
	var drop []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		keep := false
		for _, k := range keepLocks {
			keep = keep || k == name
		}
		if !keep {
			drop = append(drop, name)
		}
	}
	rows.Close()
	for _, name := range drop {
		if _, err := tx.Exec("DELETE FROM setting_locks WHERE name = ?", name); err != nil {
			return err
		}
	}
	for _, p := range syncedSettingPrefixes {
		if _, err := tx.Exec("DELETE FROM settings WHERE substr(key, 1, ?) = ?", len(p), p); err != nil {
			return err
		}
	}
	for _, k := range syncedSettings {
		if _, err := tx.Exec("DELETE FROM settings WHERE key = ?", k); err != nil {
			return err
		}
	}
	return tx.Commit()
}
