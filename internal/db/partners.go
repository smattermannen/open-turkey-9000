package db

// partners.go — alerts waiting for the accountability partners (SPEC §4.15):
// whatever happens on this device (a block switched off, unlocked or
// deleted, bypass prevention, tamper events, KC9K stopped) is queued here
// at once, with no network, and the daemon sends the queue to KC9K's cloud
// while the account has partners (cli/partner.go). Without an account or
// partners the queue is emptied unsent: nothing leaves the device
// (principle 6).

import (
	"fmt"
	"time"
)

const partnerAlertsSchema = `CREATE TABLE IF NOT EXISTS partner_alerts (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    at      INTEGER NOT NULL,
    subject TEXT NOT NULL,
    body    TEXT NOT NULL DEFAULT ''
);`

// PartnerAlert is one queued alert.
type PartnerAlert struct {
	ID      int64
	At      time.Time
	Subject string
	Body    string
}

// partnerAlertsKept caps the queue: a device long offline keeps the newest.
const partnerAlertsKept = 500

// QueuePartnerAlert queues an alert for the partners. It never fails the
// caller's work: an error is only returned for logging.
func (d *DB) QueuePartnerAlert(subject, body string) error {
	if _, err := d.conn.Exec("INSERT INTO partner_alerts (at, subject, body) VALUES (?, ?, ?)",
		time.Now().Unix(), subject, body); err != nil {
		return fmt.Errorf("failed to queue a partner alert: %w", err)
	}
	_, err := d.conn.Exec(`DELETE FROM partner_alerts WHERE id NOT IN
		(SELECT id FROM partner_alerts ORDER BY id DESC LIMIT ?)`, partnerAlertsKept)
	return err
}

// PartnerAlerts returns the queued alerts, oldest first.
func (d *DB) PartnerAlerts() ([]PartnerAlert, error) {
	rows, err := d.conn.Query("SELECT id, at, subject, body FROM partner_alerts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PartnerAlert
	for rows.Next() {
		var a PartnerAlert
		var at int64
		if err := rows.Scan(&a.ID, &at, &a.Subject, &a.Body); err != nil {
			return nil, err
		}
		a.At = time.Unix(at, 0)
		out = append(out, a)
	}
	return out, rows.Err()
}

// DropPartnerAlerts removes alerts up to and including id (sent, or
// dropped for want of partners).
func (d *DB) DropPartnerAlerts(upTo int64) error {
	_, err := d.conn.Exec("DELETE FROM partner_alerts WHERE id <= ?", upTo)
	return err
}
