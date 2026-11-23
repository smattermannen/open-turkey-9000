package db

// partner.go — the accountability partner's outbox (SPEC §4.15, ROADMAP
// 2.13): alerts and reports wait here until they reach KC9K's cloud
// (`kc9k partner send`, and the daemon every minute). Nothing is queued
// while the user has no active partner (SettingPartnerActive).

import (
	"fmt"
	"strings"
	"time"
)

const partnerSchema = `CREATE TABLE IF NOT EXISTS partner_outbox (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	at      INTEGER NOT NULL,
	kind    TEXT NOT NULL,
	subject TEXT NOT NULL,
	body    TEXT NOT NULL
);`

// SettingPartnerActive is "on" while the user has at least one active
// accountability partner (as KC9K's cloud last said).
const SettingPartnerActive = "partner.active"

// PartnerMessage is one queued report or alert.
type PartnerMessage struct {
	ID      int64
	At      int64
	Kind    string // "report" or "alert"
	Subject string
	Body    string
}

// maxOutbox keeps an outbox that can't reach the cloud from growing
// without end: the oldest go first.
const maxOutbox = 500

// QueuePartnerMessage queues a report or alert for the user's partners;
// nothing while there is no active partner.
func (d *DB) QueuePartnerMessage(kind, subject, body string) error {
	if v, _ := d.GetSetting(SettingPartnerActive); v != "on" {
		return nil
	}
	if len(body) > 20000 {
		body = body[:20000]
	}
	if len(subject) > 300 {
		subject = subject[:300]
	}
	if _, err := d.conn.Exec("INSERT INTO partner_outbox (at, kind, subject, body) VALUES (?, ?, ?, ?)",
		time.Now().Unix(), kind, subject, body); err != nil {
		return fmt.Errorf("failed to queue a message for your partner: %w", err)
	}
	_, err := d.conn.Exec(`DELETE FROM partner_outbox WHERE id NOT IN
		(SELECT id FROM partner_outbox ORDER BY id DESC LIMIT ?)`, maxOutbox)
	return err
}

// PartnerAlert queues an alert (QueuePartnerMessage); errors are only
// logged by the caller's way: alerts never stop what caused them.
func (d *DB) PartnerAlert(subject, body string) {
	if v, _ := d.GetSetting(SettingPartnerActive); v != "on" {
		return
	}
	// The same alert again within 10 minutes (a task manager closed every
	// cycle) is one alert.
	key, now := subject+"\x1f"+body, time.Now().Unix()
	if last, _ := d.GetSetting("partner.alert.last"); last != "" {
		var at int64
		if text, stamp, ok := strings.Cut(last, "\x1e"); ok && text == key {
			fmt.Sscan(stamp, &at)
			if now-at < 600 {
				return
			}
		}
	}
	_ = d.SetSetting("partner.alert.last", key+"\x1e"+fmt.Sprint(now))
	_ = d.QueuePartnerMessage("alert", subject, body)
}

// PartnerOutbox returns the queued messages, oldest first.
func (d *DB) PartnerOutbox() ([]PartnerMessage, error) {
	rows, err := d.conn.Query("SELECT id, at, kind, subject, body FROM partner_outbox ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PartnerMessage
	for rows.Next() {
		var m PartnerMessage
		if err := rows.Scan(&m.ID, &m.At, &m.Kind, &m.Subject, &m.Body); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DropPartnerMessages removes sent messages (up to and including id).
func (d *DB) DropPartnerMessages(upTo int64) error {
	_, err := d.conn.Exec("DELETE FROM partner_outbox WHERE id <= ?", upTo)
	return err
}
