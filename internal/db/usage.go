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
);`

// The kinds of usage that statistics record.
const (
	UsageSite = "site"
	UsageApp  = "app"
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
	if _, err := d.conn.Exec("DELETE FROM usage"); err != nil {
		return fmt.Errorf("failed to clear usage: %w", err)
	}
	return nil
}

// PruneUsage deletes statistics older than day.
func (d *DB) PruneUsage(day string) error {
	if _, err := d.conn.Exec("DELETE FROM usage WHERE day < ?", day); err != nil {
		return fmt.Errorf("failed to prune usage: %w", err)
	}
	return nil
}
