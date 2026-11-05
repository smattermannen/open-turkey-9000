package db

import "testing"

func TestAdvanceClock(t *testing.T) {
	prev := &clockState{BootID: "a", Boottime: 100, Trusted: 1000}
	cases := []struct {
		name     string
		wall     int64
		bootID   string
		boottime int64
		lock     bool
		want     int64
	}{
		{"wall jumps forward during lock", 999999, "a", 110, true, 1010},
		{"wall jumps backward during lock", 5, "a", 110, true, 1010},
		{"reboot during lock counts no downtime", 999999, "b", 3, true, 1000},
		{"no lock follows wall", 5000, "a", 110, false, 5000},
	}
	for _, c := range cases {
		if got := advanceClock(prev, c.wall, c.bootID, c.boottime, c.lock).Trusted; got != c.want {
			t.Errorf("%s: trusted = %d; want %d", c.name, got, c.want)
		}
	}
	if got := advanceClock(nil, 42, "a", 1, true).Trusted; got != 42 {
		t.Errorf("first use: trusted = %d; want 42", got)
	}
}
