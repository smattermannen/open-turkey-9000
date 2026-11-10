// events.go — a log of what KC9K stopped: blocked pages, killed apps, closed
// windows and browsers, refused attempts to loosen a held block, and signs of
// tampering (the daemon restarted within a boot, the clock moved). Bypass
// prevention (nepsis.go) counts recent events; they are kept for a day.
package db

import (
	"fmt"
	"strings"
	"time"
)

// Event kinds.
const (
	EventPage    = "page"    // the extension took a page away
	EventApp     = "app"     // a blocked app was closed
	EventTitle   = "title"   // a window was closed for its title
	EventBrowser = "browser" // a browser without the extension was closed
	EventRefused = "refused" // a loosening command was refused
	EventTamper  = "tamper"  // the daemon restarted within a boot, or the clock jumped
)

// EventsKept is how long events are kept.
const EventsKept = 24 * time.Hour

const eventsSchema = `CREATE TABLE IF NOT EXISTS events (
    id     INTEGER PRIMARY KEY AUTOINCREMENT,
    at     INTEGER NOT NULL,
    kind   TEXT NOT NULL,
    block  TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS events_at ON events (at);`

// Event is one logged event; At is unix seconds (wall clock).
type Event struct {
	At     int64
	Kind   string
	Block  string // "" when no single block is to blame
	Detail string
}

// RecordEvent logs an event now.
func (d *DB) RecordEvent(kind, block, detail string) error {
	if len(detail) > 300 {
		detail = detail[:300]
	}
	_, err := d.conn.Exec("INSERT INTO events (at, kind, block, detail) VALUES (?, ?, ?, ?)",
		time.Now().Unix(), kind, block, detail)
	if err != nil {
		return fmt.Errorf("failed to log an event: %w", err)
	}
	return nil
}

// CountEvents counts the events since a time (unix seconds), all of them for
// block "", or one block's.
func (d *DB) CountEvents(block string, since int64) (int, error) {
	return d.CountEventsOf(block, since, nil)
}

// CountEventsOf is CountEvents for the given kinds only (none: every kind).
func (d *DB) CountEventsOf(block string, since int64, kinds []string) (int, error) {
	query, args := "SELECT COUNT(*) FROM events WHERE at >= ?", []any{since}
	if block != "" {
		query, args = query+" AND block = ?", append(args, block)
	}
	if len(kinds) > 0 {
		query += " AND kind IN (?" + strings.Repeat(", ?", len(kinds)-1) + ")"
		for _, k := range kinds {
			args = append(args, k)
		}
	}
	var n int
	if err := d.conn.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}
	return n, nil
}

// RecentEvents returns the events since a time, newest first.
func (d *DB) RecentEvents(since int64, limit int) ([]Event, error) {
	rows, err := d.conn.Query("SELECT at, kind, block, detail FROM events WHERE at >= ? ORDER BY at DESC, id DESC LIMIT ?", since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.At, &e.Kind, &e.Block, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PruneEvents drops events older than EventsKept.
func (d *DB) PruneEvents(now time.Time) error {
	_, err := d.conn.Exec("DELETE FROM events WHERE at < ?", now.Add(-EventsKept).Unix())
	return err
}
