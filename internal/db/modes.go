// modes.go — schedules kept outside blocks (§3.6 Schedule) and modes (§3.2
// Mode, §3.8 Monk mode, R-MODE-1/2, R-MONK-1–3; owner 2026-11-17).
//
// A block's automation is stored where it always was: its windows in
// schedules (with source 0 for its own windows and a schedule's id for
// those that come from a schedule), its places in place_rules, its default
// and both/either in automation. What it is made of (AutomationSpec: own
// windows, the schedules it uses, places, settings) is written in one go
// by SetBlockAutomation, so every reader of a block's windows is
// unchanged: a schedule's windows are copies kept up to date. Where
// windows overlap they are stored clipped (ResolveWindows): a block's
// windows never overlap (R-SCHED-2) and the time they cover is the same.
//
// A mode keeps its own automation in mode_* tables; its blocks hold copies
// (SetBlockAutomation for each). Monk mode is the mode with monk set.
package db

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/schedule"
)

const modesSchema = `CREATE TABLE IF NOT EXISTS saved_schedules (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);
CREATE TABLE IF NOT EXISTS saved_schedule_windows (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    schedule_id INTEGER NOT NULL,
    days_mask   INTEGER NOT NULL,
    start_min   INTEGER NOT NULL,
    end_min     INTEGER NOT NULL,
    FOREIGN KEY (schedule_id) REFERENCES saved_schedules(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS block_schedules (
    block_id    INTEGER NOT NULL,
    schedule_id INTEGER NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (block_id, schedule_id),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE,
    FOREIGN KEY (schedule_id) REFERENCES saved_schedules(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS modes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE,
    monk       BOOLEAN NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT 0,
    default_on BOOLEAN NOT NULL DEFAULT 0,
    need_both  BOOLEAN NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS mode_windows (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    mode_id   INTEGER NOT NULL,
    days_mask INTEGER NOT NULL,
    start_min INTEGER NOT NULL,
    end_min   INTEGER NOT NULL,
    FOREIGN KEY (mode_id) REFERENCES modes(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS mode_schedules (
    mode_id     INTEGER NOT NULL,
    schedule_id INTEGER NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (mode_id, schedule_id),
    FOREIGN KEY (mode_id) REFERENCES modes(id) ON DELETE CASCADE,
    FOREIGN KEY (schedule_id) REFERENCES saved_schedules(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS mode_places (
    mode_id  INTEGER NOT NULL,
    place_id INTEGER NOT NULL,
    PRIMARY KEY (mode_id, place_id),
    FOREIGN KEY (mode_id) REFERENCES modes(id) ON DELETE CASCADE,
    FOREIGN KEY (place_id) REFERENCES places(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS mode_blocks (
    block_id INTEGER PRIMARY KEY,
    mode_id  INTEGER NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE,
    FOREIGN KEY (mode_id) REFERENCES modes(id) ON DELETE CASCADE
);`

// MonkModeName is monk mode's name among the modes (R-MONK-1).
const MonkModeName = "Monk mode"

// AutomationSpec is what a block's or a mode's automation is made of: its
// own windows, the schedules it uses (in the order applied), its places
// and its settings. Window.Allow is ignored: the default decides.
type AutomationSpec struct {
	Own       []schedule.Window `json:"own"`
	Schedules []int             `json:"schedules"`
	Places    []int             `json:"places"`
	AutomationSettings
}

// Clone returns a copy that shares nothing with s.
func (s AutomationSpec) Clone() AutomationSpec {
	s.Own, s.Schedules, s.Places = slices.Clone(s.Own), slices.Clone(s.Schedules), slices.Clone(s.Places)
	return s
}

// Equal reports whether s and o are the same automation, as stored.
func (s AutomationSpec) Equal(o AutomationSpec) bool {
	return slices.Equal(s.Own, o.Own) && slices.Equal(s.Schedules, o.Schedules) &&
		slices.Equal(s.Places, o.Places) && s.AutomationSettings == o.AutomationSettings
}

// SavedSchedule is a schedule kept outside blocks (§3.6 Schedule): named
// windows that blocks and modes use.
type SavedSchedule struct {
	ID      int
	Name    string
	Windows []schedule.Window
}

// SourcedWindow is a window with where it comes from: 0 for its owner's
// own, else the id of the schedule it is from.
type SourcedWindow struct {
	schedule.Window
	Source int
}

// ResolveWindows returns the windows own and the schedules give, in order:
// own windows as they are, then each schedule's windows without the time
// already covered (by own windows or an earlier schedule), so none overlap
// and together they cover what all of them cover.
func ResolveWindows(own []schedule.Window, schedules []SavedSchedule) []SourcedWindow {
	var covered [7][1441]bool
	mark := func(w schedule.Window) {
		for day := 0; day < 7; day++ {
			if w.Days&(1<<uint(day)) == 0 {
				continue
			}
			for m := max(w.Start, 0); m < min(w.End, 1440); m++ {
				covered[day][m] = true
			}
		}
	}
	var out []SourcedWindow
	for _, w := range own {
		w.Allow = false
		out = append(out, SourcedWindow{Window: w})
		mark(w)
	}
	for _, s := range schedules {
		for _, w := range s.Windows {
			pieces := map[[2]int]int{} // start, end -> days
			var order [][2]int
			for day := 0; day < 7; day++ {
				if w.Days&(1<<uint(day)) == 0 {
					continue
				}
				for m := max(w.Start, 0); m < min(w.End, 1440); {
					if covered[day][m] {
						m++
						continue
					}
					start := m
					for m < min(w.End, 1440) && !covered[day][m] {
						m++
					}
					key := [2]int{start, m}
					if _, seen := pieces[key]; !seen {
						order = append(order, key)
					}
					pieces[key] |= 1 << uint(day)
				}
			}
			for _, key := range order {
				piece := schedule.Window{Days: pieces[key], Start: key[0], End: key[1]}
				out = append(out, SourcedWindow{Window: piece, Source: s.ID})
				mark(piece)
			}
		}
	}
	return out
}

// Automation returns the schedule.Automation spec gives, with schedules
// the schedules it may use.
func (s AutomationSpec) Automation(schedules []SavedSchedule) (schedule.Automation, []SourcedWindow) {
	var used []SavedSchedule
	for _, id := range s.Schedules {
		for _, sc := range schedules {
			if sc.ID == id {
				used = append(used, sc)
			}
		}
	}
	rows := ResolveWindows(s.Own, used)
	a := schedule.Automation{Places: slices.Clone(s.Places), DefaultOn: s.DefaultOn, NeedBoth: s.NeedBoth}
	for _, r := range rows {
		w := r.Window
		w.Allow = s.DefaultOn
		a.Windows = append(a.Windows, w)
	}
	return a, rows
}

// --------------------------------------------------------------------------
// Schedules kept outside blocks
// --------------------------------------------------------------------------

// SavedSchedules returns every schedule, by name.
func (d *DB) SavedSchedules() ([]SavedSchedule, error) {
	rows, err := d.conn.Query("SELECT id, name FROM saved_schedules ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, fmt.Errorf("failed to read the schedules: %w", err)
	}
	var out []SavedSchedule
	for rows.Next() {
		var s SavedSchedule
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		ws, err := d.windowsOf("saved_schedule_windows", "schedule_id", out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Windows = ws
	}
	return out, nil
}

// windowsOf reads the windows in table whose column is id, in order.
func (d *DB) windowsOf(table, column string, id int) ([]schedule.Window, error) {
	rows, err := d.conn.Query(fmt.Sprintf("SELECT days_mask, start_min, end_min FROM %s WHERE %s = ? ORDER BY id", table, column), id)
	if err != nil {
		return nil, fmt.Errorf("failed to read windows: %w", err)
	}
	defer rows.Close()
	var out []schedule.Window
	for rows.Next() {
		var w schedule.Window
		if err := rows.Scan(&w.Days, &w.Start, &w.End); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetSavedSchedule returns the schedule with that name (case ignored).
func (d *DB) GetSavedSchedule(name string) (SavedSchedule, error) {
	all, err := d.SavedSchedules()
	if err != nil {
		return SavedSchedule{}, err
	}
	for _, s := range all {
		if strings.EqualFold(s.Name, name) {
			return s, nil
		}
	}
	return SavedSchedule{}, fmt.Errorf("no schedule named '%s'", name)
}

// Kinds of names: blocks, modes and schedules each have their own names
// (§4.2 Names, owner 2026-11-17): a block and a mode may share one.
const (
	KindBlock    = "block"
	KindMode     = "mode"
	KindSchedule = "schedule"
)

// NameTaken reports whether a block, mode or schedule (kind) has name, case
// ignored.
func (d *DB) NameTaken(kind, name string) (bool, error) {
	table := map[string]string{KindBlock: "blocks", KindMode: "modes", KindSchedule: "saved_schedules"}[kind]
	if table == "" {
		return false, fmt.Errorf("no kind of name %q", kind)
	}
	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE name = ? COLLATE NOCASE", name).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// checkNewName refuses a name for a new mode or schedule (kind) that
// another of its kind has.
func (d *DB) checkNewName(kind, name string) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	taken, err := d.NameTaken(kind, name)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("a %s named '%s' already exists", kind, name)
	}
	return nil
}

// CreateSavedSchedule makes an empty schedule.
func (d *DB) CreateSavedSchedule(name string) (int, error) {
	if err := d.checkNewName(KindSchedule, name); err != nil {
		return 0, err
	}
	res, err := d.conn.Exec("INSERT INTO saved_schedules (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("failed to make the schedule: %w", err)
	}
	id, err := res.LastInsertId()
	return int(id), err
}

// RenameSavedSchedule renames a schedule.
func (d *DB) RenameSavedSchedule(id int, name string) error {
	var old string
	if err := d.conn.QueryRow("SELECT name FROM saved_schedules WHERE id = ?", id).Scan(&old); err != nil {
		return fmt.Errorf("no such schedule")
	}
	if !strings.EqualFold(old, name) {
		if err := d.checkNewName(KindSchedule, name); err != nil {
			return err
		}
	}
	_, err := d.conn.Exec("UPDATE saved_schedules SET name = ? WHERE id = ?", name, id)
	return err
}

// SetSavedScheduleWindows replaces a schedule's windows. Its users' copies
// are the caller's to bring up to date (SetBlockAutomation), after the
// gate (R-MODE-2).
func (d *DB) SetSavedScheduleWindows(id int, windows []schedule.Window) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM saved_schedule_windows WHERE schedule_id = ?", id); err != nil {
		return err
	}
	for _, w := range windows {
		if _, err := tx.Exec("INSERT INTO saved_schedule_windows (schedule_id, days_mask, start_min, end_min) VALUES (?, ?, ?, ?)",
			id, w.Days, w.Start, w.End); err != nil {
			return fmt.Errorf("failed to save the schedule: %w", err)
		}
	}
	return tx.Commit()
}

// ScheduleUsers returns the blocks (by name) and modes (by id) that use a
// schedule. Blocks in a mode use it through the mode, and are listed too.
func (d *DB) ScheduleUsers(id int) (blocks []string, modes []int, err error) {
	rows, err := d.conn.Query(`SELECT b.name FROM block_schedules s JOIN blocks b ON b.id = s.block_id
		WHERE s.schedule_id = ? ORDER BY b.name`, id)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		blocks = append(blocks, name)
	}
	rows.Close()
	rows, err = d.conn.Query("SELECT mode_id FROM mode_schedules WHERE schedule_id = ? ORDER BY mode_id", id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m int
		if err := rows.Scan(&m); err != nil {
			return nil, nil, err
		}
		modes = append(modes, m)
	}
	return blocks, modes, rows.Err()
}

// DeleteSavedSchedule forgets a schedule. Everything that used it keeps
// its windows as its own, so nothing changes in what is enforced.
func (d *DB) DeleteSavedSchedule(id int) error {
	all, err := d.SavedSchedules()
	if err != nil {
		return err
	}
	// Modes: the clipped windows the schedule gave become the mode's own.
	_, modes, err := d.ScheduleUsers(id)
	if err != nil {
		return err
	}
	for _, m := range modes {
		mode, err := d.modeByID(m)
		if err != nil {
			return err
		}
		_, rows := mode.Spec.Automation(all)
		spec := mode.Spec.Clone()
		for _, r := range rows {
			if r.Source == id {
				spec.Own = append(spec.Own, r.Window)
			}
		}
		spec.Schedules = slices.DeleteFunc(spec.Schedules, func(s int) bool { return s == id })
		if err := d.SetModeAutomation(m, spec); err != nil {
			return err
		}
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Blocks: their copies become their own windows, as they are.
	if _, err := tx.Exec("UPDATE schedules SET source = 0 WHERE source = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM saved_schedules WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to remove the schedule: %w", err)
	}
	return tx.Commit()
}

// --------------------------------------------------------------------------
// A block's automation, as a whole
// --------------------------------------------------------------------------

// BlockAutomationSpec returns what a block's automation is made of.
func (d *DB) BlockAutomationSpec(block string) (AutomationSpec, error) {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return AutomationSpec{}, err
	}
	settings, err := d.Automation(block)
	if err != nil {
		return AutomationSpec{}, err
	}
	spec := AutomationSpec{AutomationSettings: settings}
	rows, err := d.ListSchedules(block)
	if err != nil {
		return AutomationSpec{}, err
	}
	for i, r := range rows {
		if i == 0 {
			spec.DefaultOn = r.Allow // its windows follow the default; theirs decides
		}
		if r.Source == 0 {
			w := r.Window()
			w.Allow = false
			spec.Own = append(spec.Own, w)
		}
	}
	if spec.Schedules, err = d.intColumn("SELECT schedule_id FROM block_schedules WHERE block_id = ? ORDER BY position, schedule_id", blockID); err != nil {
		return AutomationSpec{}, err
	}
	if spec.Places, err = d.intColumn("SELECT place_id FROM place_rules WHERE block_id = ? ORDER BY place_id", blockID); err != nil {
		return AutomationSpec{}, err
	}
	return spec, nil
}

func (d *DB) intColumn(query string, args ...any) ([]int, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetBlockAutomation stores a block's automation as a whole: its windows
// (its own, and the clipped copies of its schedules' windows), its places
// and its settings. Windows that stay as they were keep their row, so
// their number's break and its taken occurrence stay with them. The gate
// is the caller's (R-LOCK-1, R-MODE-1/2).
func (d *DB) SetBlockAutomation(block string, spec AutomationSpec) error {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return err
	}
	all, err := d.SavedSchedules()
	if err != nil {
		return err
	}
	_, want := spec.Automation(all)
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	type row struct {
		id     int
		w      schedule.Window
		source int
		allow  bool
	}
	q, err := tx.Query("SELECT id, days_mask, start_min, end_min, source, allow FROM schedules WHERE block_id = ? ORDER BY id", blockID)
	if err != nil {
		return err
	}
	var have []row
	for q.Next() {
		var r row
		if err := q.Scan(&r.id, &r.w.Days, &r.w.Start, &r.w.End, &r.source, &r.allow); err != nil {
			q.Close()
			return err
		}
		have = append(have, r)
	}
	q.Close()
	kept := map[int]bool{}
	for _, w := range want {
		found := -1
		for i, r := range have {
			if !kept[r.id] && r.source == w.Source && r.w.Days == w.Days && r.w.Start == w.Start && r.w.End == w.End {
				found = i
				break
			}
		}
		if found >= 0 {
			kept[have[found].id] = true
			if have[found].allow != spec.DefaultOn {
				if _, err := tx.Exec("UPDATE schedules SET allow = ? WHERE id = ?", spec.DefaultOn, have[found].id); err != nil {
					return err
				}
			}
			continue
		}
		if _, err := tx.Exec("INSERT INTO schedules (block_id, days_mask, start_min, end_min, allow, source) VALUES (?, ?, ?, ?, ?, ?)",
			blockID, w.Days, w.Start, w.End, spec.DefaultOn, w.Source); err != nil {
			return fmt.Errorf("failed to save the windows of '%s': %w", block, err)
		}
	}
	for _, r := range have {
		if !kept[r.id] {
			if _, err := tx.Exec("DELETE FROM schedules WHERE id = ?", r.id); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec("DELETE FROM block_schedules WHERE block_id = ?", blockID); err != nil {
		return err
	}
	for i, id := range spec.Schedules {
		if _, err := tx.Exec("INSERT INTO block_schedules (block_id, schedule_id, position) VALUES (?, ?, ?)", blockID, id, i); err != nil {
			return fmt.Errorf("failed to save the schedules of '%s': %w", block, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM place_rules WHERE block_id = ?", blockID); err != nil {
		return err
	}
	for _, id := range spec.Places {
		if _, err := tx.Exec("INSERT OR IGNORE INTO place_rules (block_id, place_id) VALUES (?, ?)", blockID, id); err != nil {
			return fmt.Errorf("failed to save the places of '%s': %w", block, err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO automation (block_id, default_on, need_both) VALUES (?, ?, ?)
		ON CONFLICT(block_id) DO UPDATE SET default_on = excluded.default_on, need_both = excluded.need_both`,
		blockID, spec.DefaultOn, spec.NeedBoth); err != nil {
		return fmt.Errorf("failed to save the automation of '%s': %w", block, err)
	}
	return tx.Commit()
}

// --------------------------------------------------------------------------
// Modes
// --------------------------------------------------------------------------

// Mode is a group of blocks with one automation for all of them, or monk
// mode (Monk). Active is monk mode's switch: its automation runs it.
type Mode struct {
	ID     int
	Name   string
	Monk   bool
	Active bool
	Spec   AutomationSpec
	Blocks []string
}

// Modes returns every mode: monk mode first, then by name.
func (d *DB) Modes() ([]Mode, error) {
	rows, err := d.conn.Query("SELECT id, name, monk, active, default_on, need_both FROM modes ORDER BY monk DESC, name COLLATE NOCASE")
	if err != nil {
		return nil, fmt.Errorf("failed to read the modes: %w", err)
	}
	var out []Mode
	for rows.Next() {
		var m Mode
		if err := rows.Scan(&m.ID, &m.Name, &m.Monk, &m.Active, &m.Spec.DefaultOn, &m.Spec.NeedBoth); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := d.fillMode(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (d *DB) fillMode(m *Mode) error {
	var err error
	if m.Spec.Own, err = d.windowsOf("mode_windows", "mode_id", m.ID); err != nil {
		return err
	}
	if m.Spec.Schedules, err = d.intColumn("SELECT schedule_id FROM mode_schedules WHERE mode_id = ? ORDER BY position, schedule_id", m.ID); err != nil {
		return err
	}
	if m.Spec.Places, err = d.intColumn("SELECT place_id FROM mode_places WHERE mode_id = ? ORDER BY place_id", m.ID); err != nil {
		return err
	}
	rows, err := d.conn.Query(`SELECT b.name FROM mode_blocks m JOIN blocks b ON b.id = m.block_id
		WHERE m.mode_id = ? ORDER BY b.name COLLATE NOCASE`, m.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	m.Blocks = nil
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		m.Blocks = append(m.Blocks, name)
	}
	return rows.Err()
}

// GetMode returns the mode with that name (case ignored).
func (d *DB) GetMode(name string) (Mode, error) {
	all, err := d.Modes()
	if err != nil {
		return Mode{}, err
	}
	for _, m := range all {
		if strings.EqualFold(m.Name, name) {
			return m, nil
		}
	}
	return Mode{}, fmt.Errorf("no mode named '%s'", name)
}

func (d *DB) modeByID(id int) (Mode, error) {
	all, err := d.Modes()
	if err != nil {
		return Mode{}, err
	}
	for _, m := range all {
		if m.ID == id {
			return m, nil
		}
	}
	return Mode{}, fmt.Errorf("no such mode")
}

// MonkMode returns monk mode as a mode.
func (d *DB) MonkMode() (Mode, error) {
	all, err := d.Modes()
	if err != nil {
		return Mode{}, err
	}
	for _, m := range all {
		if m.Monk {
			return m, nil
		}
	}
	return Mode{}, fmt.Errorf("monk mode is missing from the modes")
}

// CreateMode makes an empty mode (a group of blocks).
func (d *DB) CreateMode(name string) error {
	if err := d.checkNewName(KindMode, name); err != nil {
		return err
	}
	if _, err := d.conn.Exec("INSERT INTO modes (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("failed to make the mode: %w", err)
	}
	return nil
}

// RenameMode renames a mode (not monk mode).
func (d *DB) RenameMode(id int, name string) error {
	m, err := d.modeByID(id)
	if err != nil {
		return err
	}
	if m.Monk {
		return fmt.Errorf("monk mode keeps its name")
	}
	if !strings.EqualFold(m.Name, name) {
		if err := d.checkNewName(KindMode, name); err != nil {
			return err
		}
	}
	_, err = d.conn.Exec("UPDATE modes SET name = ? WHERE id = ?", name, id)
	return err
}

// DeleteMode removes a mode (not monk mode). Its blocks stay, with the
// automation they have.
func (d *DB) DeleteMode(id int) error {
	m, err := d.modeByID(id)
	if err != nil {
		return err
	}
	if m.Monk {
		return fmt.Errorf("monk mode can't be deleted")
	}
	_, err = d.conn.Exec("DELETE FROM modes WHERE id = ?", id)
	return err
}

// SetModeAutomation stores a mode's own automation. Its blocks' copies are
// the caller's to bring up to date, after the gate (R-MODE-2).
func (d *DB) SetModeAutomation(id int, spec AutomationSpec) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE modes SET default_on = ?, need_both = ? WHERE id = ?", spec.DefaultOn, spec.NeedBoth, id); err != nil {
		return err
	}
	for _, table := range []string{"mode_windows", "mode_schedules", "mode_places"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE mode_id = ?", id); err != nil {
			return err
		}
	}
	for _, w := range spec.Own {
		if _, err := tx.Exec("INSERT INTO mode_windows (mode_id, days_mask, start_min, end_min) VALUES (?, ?, ?, ?)",
			id, w.Days, w.Start, w.End); err != nil {
			return fmt.Errorf("failed to save the mode's windows: %w", err)
		}
	}
	for i, s := range spec.Schedules {
		if _, err := tx.Exec("INSERT INTO mode_schedules (mode_id, schedule_id, position) VALUES (?, ?, ?)", id, s, i); err != nil {
			return fmt.Errorf("failed to save the mode's schedules: %w", err)
		}
	}
	for _, p := range spec.Places {
		if _, err := tx.Exec("INSERT OR IGNORE INTO mode_places (mode_id, place_id) VALUES (?, ?)", id, p); err != nil {
			return fmt.Errorf("failed to save the mode's places: %w", err)
		}
	}
	return tx.Commit()
}

// SetModeActive switches monk mode's automation on or off.
func (d *DB) SetModeActive(id int, on bool) error {
	_, err := d.conn.Exec("UPDATE modes SET active = ? WHERE id = ?", on, id)
	return err
}

// ModeOf returns the mode a block is in; ok is false when it is in none.
func (d *DB) ModeOf(block string) (Mode, bool, error) {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return Mode{}, false, err
	}
	var id int
	err = d.conn.QueryRow("SELECT mode_id FROM mode_blocks WHERE block_id = ?", blockID).Scan(&id)
	if err == sql.ErrNoRows {
		return Mode{}, false, nil
	}
	if err != nil {
		return Mode{}, false, err
	}
	m, err := d.modeByID(id)
	return m, err == nil, err
}

// SetBlockMode puts a block into a mode (a block is in at most one), or
// with id 0 takes it out. Its automation is the caller's (R-MODE-1).
func (d *DB) SetBlockMode(block string, id int) error {
	blockID, err := d.getBlockID(block)
	if err != nil {
		return err
	}
	if id == 0 {
		_, err = d.conn.Exec("DELETE FROM mode_blocks WHERE block_id = ?", blockID)
		return err
	}
	_, err = d.conn.Exec(`INSERT INTO mode_blocks (block_id, mode_id) VALUES (?, ?)
		ON CONFLICT(block_id) DO UPDATE SET mode_id = excluded.mode_id`, blockID, id)
	return err
}

// migrateModes makes monk mode's row among the modes, once.
func migrateModes(conn *sql.DB) error {
	_, err := conn.Exec(`INSERT INTO modes (name, monk)
		SELECT ?, 1 WHERE NOT EXISTS (SELECT 1 FROM modes WHERE monk = 1)
		AND NOT EXISTS (SELECT 1 FROM modes WHERE name = ? COLLATE NOCASE)`, MonkModeName, MonkModeName)
	return err
}
