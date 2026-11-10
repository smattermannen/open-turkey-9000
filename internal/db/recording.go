// recording.go — link recording: while it is on, the sites opened in the
// browser (reported by the extension, see enforce/recording.go) are kept
// here until 'kc9k record stop' adds them to the target: a block's list or
// a cluster. Nothing changes a block while recording; stopping does, through
// the same gate as any edit.
package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// SettingRecording holds the recording in progress, as JSON ("" when off).
const SettingRecording = "recording"

// MaxRecordedSites bounds one recording.
const MaxRecordedSites = 2000

const recordingSchema = `CREATE TABLE IF NOT EXISTS recorded_sites (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    site TEXT NOT NULL UNIQUE
);`

// Recording is a recording in progress and where its sites go: a cluster,
// or the list ListID of Block.
type Recording struct {
	Cluster string `json:"cluster,omitempty"`
	Block   string `json:"block,omitempty"`
	ListID  int    `json:"list,omitempty"`
	Started int64  `json:"started"`
}

// GetRecording returns the recording in progress, or nil.
func (d *DB) GetRecording() (*Recording, error) {
	value, err := d.GetSetting(SettingRecording)
	if err != nil || value == "" {
		return nil, err
	}
	var r Recording
	if err := json.Unmarshal([]byte(value), &r); err != nil {
		return nil, fmt.Errorf("bad recording setting: %w", err)
	}
	return &r, nil
}

// StartRecording begins a recording with nothing recorded yet.
func (d *DB) StartRecording(r Recording) error {
	r.Started = time.Now().Unix()
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM recorded_sites"); err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}
	return d.SetSetting(SettingRecording, string(data))
}

// EndRecording ends the recording and forgets its sites.
func (d *DB) EndRecording() error {
	if err := d.SetSetting(SettingRecording, ""); err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM recorded_sites"); err != nil {
		return fmt.Errorf("failed to clear the recording: %w", err)
	}
	return nil
}

// AddRecordedSite notes a site, once, up to MaxRecordedSites.
func (d *DB) AddRecordedSite(site string) error {
	_, err := d.conn.Exec(`INSERT OR IGNORE INTO recorded_sites (site)
		SELECT ? WHERE (SELECT COUNT(*) FROM recorded_sites) < ?`, site, MaxRecordedSites)
	if err != nil {
		return fmt.Errorf("failed to record %q: %w", site, err)
	}
	return nil
}

// RecordedSites returns the sites recorded so far, in the order opened.
func (d *DB) RecordedSites() ([]string, error) {
	rows, err := d.conn.Query("SELECT site FROM recorded_sites ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to read the recording: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
