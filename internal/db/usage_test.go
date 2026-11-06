package db

import (
	"path/filepath"
	"testing"
)

func TestUsage(t *testing.T) {
	d, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, r := range []UsageRow{
		{"2026-11-05", UsageSite, "old.example", 50},
		{"2026-11-06", UsageSite, "example.com", 30},
		{"2026-11-06", UsageSite, "example.com", 15},
		{"2026-11-06", UsageApp, "firefox", 60},
		{"2026-11-06", UsageApp, "ignored", 0},
	} {
		if err := d.AddUsage(r.Day, r.Kind, r.Name, r.Seconds); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := d.UsageSince("2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	want := []UsageRow{
		{"2026-11-06", UsageApp, "firefox", 60},
		{"2026-11-06", UsageSite, "example.com", 45},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %v, want %v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d: got %v, want %v", i, rows[i], want[i])
		}
	}

	if err := d.PruneUsage("2026-11-06"); err != nil {
		t.Fatal(err)
	}
	if rows, _ := d.UsageSince("2000-01-01"); len(rows) != 2 {
		t.Errorf("prune kept %v", rows)
	}
	if err := d.ClearUsage(); err != nil {
		t.Fatal(err)
	}
	if rows, _ := d.UsageSince("2000-01-01"); len(rows) != 0 {
		t.Errorf("clear kept %v", rows)
	}
}
