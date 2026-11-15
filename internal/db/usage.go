package db

import "fmt"

// usageSchema holds the statistics: seconds of foreground use per day for each
// website and app. Nothing else about the activity is stored.
const usageSchema = `CREATE TABLE IF NOT EXISTS usage (
    day TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    seconds INTEGER NOT NULL,
    PRIMARY KEY (day, kind, name)
);
CREATE TABLE IF NOT EXISTS usage_users (
    day TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    user TEXT NOT NULL,
    seconds INTEGER NOT NULL,
    PRIMARY KEY (day, kind, name, user)
);`

// The kinds of usage that statistics record.
const (
	UsageSite = "site"
	UsageApp  = "app"
	// UsageBlocked counts (not seconds) how often a site was blocked
	// (§4.9 times blocked); UsageBlockedBlock the same per block.
	UsageBlocked      = "blocked"
	UsageBlockedBlock = "blocked-block"
)

// UsageRow is the time spent on one site or app on one day (YYYY-MM-DD, local).
type UsageRow struct {
	Day     string `json:"day"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Seconds int64  `json:"seconds"`
}

// AddUsage adds seconds of use to a site or app on a day.
func (d *DB) AddUsage(day, kind, name string, seconds int64) error {
	if seconds <= 0 {
		return nil
	}
	_, err := d.conn.Exec(`INSERT INTO usage (day, kind, name, seconds) VALUES (?, ?, ?, ?)
		ON CONFLICT(day, kind, name) DO UPDATE SET seconds = seconds + excluded.seconds`,
		day, kind, name, seconds)
	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}
	return nil
}

// AddUserUsage adds seconds of a user's use (§4.9 statistics per user),
// besides the totals AddUsage keeps.
func (d *DB) AddUserUsage(day, kind, name, user string, seconds int64) error {
	if seconds <= 0 || user == "" {
		return nil
	}
	_, err := d.conn.Exec(`INSERT INTO usage_users (day, kind, name, user, seconds) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(day, kind, name, user) DO UPDATE SET seconds = seconds + excluded.seconds`,
		day, kind, name, user, seconds)
	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}
	return nil
}

// UserUsageSince is UsageSince for one user's time (sites and apps).
func (d *DB) UserUsageSince(day, user string) ([]UsageRow, error) {
	rows, err := d.conn.Query(`SELECT day, kind, name, seconds FROM usage_users
		WHERE day >= ? AND user = ? ORDER BY day, seconds DESC, name`, day, user)
	if err != nil {
		return nil, fmt.Errorf("failed to read usage: %w", err)
	}
	defer rows.Close()
	result := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Day, &r.Kind, &r.Name, &r.Seconds); err != nil {
			return nil, fmt.Errorf("failed to read usage: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// UsageUsers lists the users with statistics.
func (d *DB) UsageUsers() ([]string, error) {
	rows, err := d.conn.Query(`SELECT DISTINCT user FROM usage_users ORDER BY user`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UsageSince returns every row from day (inclusive) on, oldest day first and
// the most used first within a day.
func (d *DB) UsageSince(day string) ([]UsageRow, error) {
	rows, err := d.conn.Query(`SELECT day, kind, name, seconds FROM usage
		WHERE day >= ? ORDER BY day, seconds DESC, name`, day)
	if err != nil {
		return nil, fmt.Errorf("failed to read usage: %w", err)
	}
	defer rows.Close()
	result := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Day, &r.Kind, &r.Name, &r.Seconds); err != nil {
			return nil, fmt.Errorf("failed to read usage: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ClearUsage deletes all statistics.
func (d *DB) ClearUsage() error {
	if _, err := d.conn.Exec("DELETE FROM usage; DELETE FROM usage_users"); err != nil {
		return fmt.Errorf("failed to clear usage: %w", err)
	}
	return nil
}

// PruneUsage deletes statistics older than day.
func (d *DB) PruneUsage(day string) error {
	if _, err := d.conn.Exec("DELETE FROM usage WHERE day < ?1; DELETE FROM usage_users WHERE day < ?1", day); err != nil {
		return fmt.Errorf("failed to prune usage: %w", err)
	}
	return nil
}

// ImportUsage adds statistics rows from a backup; a day, kind and name
// already recorded here is kept as it is, so importing twice adds nothing.
func (d *DB) ImportUsage(rows []UsageRow) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, r := range rows {
		if r.Seconds <= 0 || (r.Kind != UsageSite && r.Kind != UsageApp && r.Kind != UsageBlocked && r.Kind != UsageBlockedBlock) {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO usage (day, kind, name, seconds) VALUES (?, ?, ?, ?)`,
			r.Day, r.Kind, r.Name, r.Seconds); err != nil {
			return fmt.Errorf("failed to import statistics: %w", err)
		}
	}
	return tx.Commit()
}
