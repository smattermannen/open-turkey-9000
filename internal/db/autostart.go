// autostart.go — autostart (§3.6): rules that switch a block on. Kinds:
// once (at a time), at (a time of day on set days), sign-in (a desktop
// sign-in or waking from sleep) and next-window (the next time one of its
// block windows opens; then gone). Autostart never sets a lock (R-LOCK-11).
package db

import (
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/schedule"
)

const autostartSchema = `CREATE TABLE IF NOT EXISTS autostarts (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id  INTEGER NOT NULL,
    kind      TEXT NOT NULL,
    at_unix   INTEGER NOT NULL DEFAULT 0,
    days      INTEGER NOT NULL DEFAULT 0,
    minute    INTEGER NOT NULL DEFAULT 0,
    last_day  TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// Autostart kinds.
const (
	AutostartOnce   = "once"
	AutostartAt     = "at"
	AutostartSignIn = "sign-in"
	AutostartWindow = "next-window"
)

// Autostart is one rule. Num is its 1-based position for the block.
type Autostart struct {
	ID      int    `json:"id"`
	Num     int    `json:"num"`
	Block   string `json:"block"`
	Kind    string `json:"kind"`
	At      int64  `json:"at,omitempty"`       // once: unix seconds
	Days    int    `json:"days,omitempty"`     // at: schedule day bits
	Minute  int    `json:"minute,omitempty"`   // at: minute of the day
	LastDay string `json:"last_day,omitempty"` // at: the day it last fired
}

// String reads like the command that made it.
func (a Autostart) String() string {
	switch a.Kind {
	case AutostartOnce:
		return "once " + time.Unix(a.At, 0).Format("2006-01-02 15:04")
	case AutostartAt:
		return fmt.Sprintf("at %s %s", schedule.FormatMinutes(a.Minute), schedule.FormatDays(a.Days))
	}
	return a.Kind
}

// AddAutostart stores a rule for a block.
func (d *DB) AddAutostart(a Autostart) error {
	id, err := d.getBlockID(a.Block)
	if err != nil {
		return err
	}
	switch a.Kind {
	case AutostartOnce, AutostartAt, AutostartSignIn, AutostartWindow:
	default:
		return fmt.Errorf("unknown autostart %q", a.Kind)
	}
	_, err = d.conn.Exec("INSERT INTO autostarts (block_id, kind, at_unix, days, minute) VALUES (?, ?, ?, ?, ?)",
		id, a.Kind, a.At, a.Days, a.Minute)
	if err != nil {
		return fmt.Errorf("failed to add the autostart: %w", err)
	}
	return nil
}

// Autostarts lists the rules of one block ("" for every block).
func (d *DB) Autostarts(block string) ([]Autostart, error) {
	rows, err := d.conn.Query(`SELECT a.id, b.name, a.kind, a.at_unix, a.days, a.minute, a.last_day
		FROM autostarts a JOIN blocks b ON b.id = a.block_id
		WHERE ? = '' OR b.name = ? ORDER BY b.name, a.id`, block, block)
	if err != nil {
		return nil, fmt.Errorf("failed to list autostarts: %w", err)
	}
	defer rows.Close()
	out := []Autostart{}
	nums := map[string]int{}
	for rows.Next() {
		var a Autostart
		if err := rows.Scan(&a.ID, &a.Block, &a.Kind, &a.At, &a.Days, &a.Minute, &a.LastDay); err != nil {
			return nil, err
		}
		nums[a.Block]++
		a.Num = nums[a.Block]
		out = append(out, a)
	}
	return out, rows.Err()
}

// RemoveAutostart deletes a rule by its ID.
func (d *DB) RemoveAutostart(id int) error {
	_, err := d.conn.Exec("DELETE FROM autostarts WHERE id = ?", id)
	return err
}

// SetAutostartFired records the day an "at" rule last fired.
func (d *DB) SetAutostartFired(id int, day string) error {
	_, err := d.conn.Exec("UPDATE autostarts SET last_day = ? WHERE id = ?", day, id)
	return err
}

// AutostartDue reports whether a rule switches its block on at now, given
// whether a sign-in or wake was just seen and whether one of the block's
// windows just opened. today is now's day, "2006-01-02".
func AutostartDue(a Autostart, now time.Time, signedIn, windowOpened bool) bool {
	switch a.Kind {
	case AutostartOnce:
		return now.Unix() >= a.At
	case AutostartAt:
		minute := now.Hour()*60 + now.Minute()
		day := now.Format("2006-01-02")
		// Within ten minutes after its time, once a day: a computer that
		// was off at the time doesn't start the block hours later.
		return a.Days&(1<<uint(now.Weekday())) != 0 && minute >= a.Minute && minute < a.Minute+10 && a.LastDay != day
	case AutostartSignIn:
		return signedIn
	case AutostartWindow:
		return windowOpened
	}
	return false
}
