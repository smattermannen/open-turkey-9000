// breaks.go — breaks a block allows, set before it is locked.
//
// An allowance earns break time while the block is on: Allowance seconds
// for every Per seconds of blocking, kept up to one Allowance (Balance, in
// milliseconds so small steps add up). A break is taken when the user
// chooses and lifts the block as a pause (pauses.go).
//
// Relock makes unlocking a break too: passing the block's challenge or
// giving its password lifts it for Relock seconds, after which it is back
// with its locks. Unlocking again during that break ends the block.
package db

import (
	"database/sql"
	"fmt"
)

const breaksSchema = `CREATE TABLE IF NOT EXISTS block_breaks (
    block_id   INTEGER PRIMARY KEY,
    allowance  INTEGER NOT NULL DEFAULT 0,
    per        INTEGER NOT NULL DEFAULT 0,
    balance_ms INTEGER NOT NULL DEFAULT 0,
    relock     INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// Limits on breaks.
const (
	MaxBreakSeconds = 4 * 3600
	MaxBreakPer     = 24 * 3600
)

// Breaks is a block's break settings.
type Breaks struct {
	Allowance int64 // seconds earned per Per seconds of blocking; 0: none
	Per       int64
	BalanceMs int64 // earned and not yet taken
	Relock    int64 // seconds an unlock lifts the block for; 0: unlocking ends it
}

// Balance returns the break time available, in whole seconds.
func (b Breaks) Balance() int64 { return b.BalanceMs / 1000 }

// Earn adds what seconds of blocking earn, up to one allowance.
func (b Breaks) Earn(seconds int64) Breaks {
	if b.Allowance <= 0 || b.Per <= 0 || seconds <= 0 {
		return b
	}
	b.BalanceMs = min(b.BalanceMs+seconds*1000*b.Allowance/b.Per, b.Allowance*1000)
	return b
}

// LooserThan reports whether b allows more breaks than old.
func (b Breaks) LooserThan(old Breaks) bool {
	if b.Relock > old.Relock {
		return true
	}
	if b.Allowance == 0 {
		return false
	}
	// More time per hour of blocking, or more at once.
	return old.Allowance == 0 || b.Allowance > old.Allowance || b.Allowance*old.Per > old.Allowance*b.Per
}

// GetBreaks returns a block's break settings (zero when it has none).
func (d *DB) GetBreaks(name string) (Breaks, error) {
	var b Breaks
	err := d.conn.QueryRow(`SELECT k.allowance, k.per, k.balance_ms, k.relock FROM block_breaks k
		JOIN blocks b ON b.id = k.block_id WHERE b.name = ?`, name).Scan(&b.Allowance, &b.Per, &b.BalanceMs, &b.Relock)
	if err == sql.ErrNoRows {
		return Breaks{}, nil
	}
	if err != nil {
		return Breaks{}, fmt.Errorf("failed to read the breaks of '%s': %w", name, err)
	}
	return b, nil
}

// SetBreaks saves a block's break settings.
func (d *DB) SetBreaks(name string, b Breaks) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if b.Allowance == 0 {
		b.Per, b.BalanceMs = 0, 0
	}
	if b.Allowance == 0 && b.Relock == 0 {
		_, err = d.conn.Exec("DELETE FROM block_breaks WHERE block_id = ?", blockID)
	} else {
		_, err = d.conn.Exec(`INSERT INTO block_breaks (block_id, allowance, per, balance_ms, relock) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(block_id) DO UPDATE SET allowance = excluded.allowance, per = excluded.per,
			balance_ms = excluded.balance_ms, relock = excluded.relock`, blockID, b.Allowance, b.Per, b.BalanceMs, b.Relock)
	}
	if err != nil {
		return fmt.Errorf("failed to save the breaks of '%s': %w", name, err)
	}
	return nil
}
