package db

import (
	"reflect"
	"testing"
)

// Owner 2026-11-17 (§3.2 Exception list): a playlist let through was a
// setting "youtube:playlist=<id>" beside two exceptions, its page and the
// video pages. Opening keeps only its page in an exception list (added
// where missing) and drops the video pages of blocks that had a playlist;
// another block's own video pages exception stays.
func TestPlaylistExceptionsMigrated(t *testing.T) {
	d, path := openTest(t)
	for _, b := range []string{"old", "bare", "own"} {
		if err := d.CreateBlock(b); err != nil {
			t.Fatal(err)
		}
	}
	add := func(block string, entries ...string) {
		detail, _ := d.GetBlock(block)
		l, _ := detail.Lists.Find(DefaultExceptionsList)
		if err := d.AddListEntries(l.ID, entries); err != nil {
			t.Fatal(err)
		}
	}
	add("old", "youtube.com/playlist?list=PLabcdefghij12", VideoPages)
	add("own", VideoPages)
	for _, row := range [][2]string{{"old", "PLabcdefghij12"}, {"bare", "PLzyxwvutsrq98"}} {
		id, _ := d.getBlockID(row[0])
		if _, err := d.conn.Exec("INSERT INTO block_social (block_id, preset) VALUES (?, ?)", id, PlaylistPrefix+row[1]); err != nil {
			t.Fatal(err)
		}
	}
	d.Close()
	d, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for block, want := range map[string][]string{
		"old":  {FileException, "youtube.com/playlist?list=PLabcdefghij12"},
		"bare": {FileException, "youtube.com/playlist?list=PLzyxwvutsrq98"},
		"own":  {FileException, VideoPages},
	} {
		detail, _ := d.GetBlock(block)
		l, _ := detail.Lists.Find(DefaultExceptionsList)
		if !reflect.DeepEqual(l.Entries, want) || len(detail.Social) != 0 {
			t.Errorf("%s: exceptions %q, social %v", block, l.Entries, detail.Social)
		}
	}
}

func TestPlaylistInEntry(t *testing.T) {
	for entry, want := range map[string]string{
		"youtube.com/playlist?list=PLabcdefghij12":                    "PLabcdefghij12",
		"https://www.youtube.com/playlist?list=PLabcdefghij12&si=x":   "PLabcdefghij12",
		"https://m.youtube.com/playlist?si=x&list=PL_abc-defghij12#x": "PL_abc-defghij12",
		"youtube.com/watch?v=abc&list=PLabcdefghij12":                 "",
		"youtube.com/playlist?list=short":                             "",
		"evil.com/youtube.com/playlist?list=PLabcdefghij12":           "",
	} {
		got, ok := PlaylistInEntry(entry)
		if got != want || ok != (want != "") {
			t.Errorf("%s: %q %v, want %q", entry, got, ok, want)
		}
	}
}
