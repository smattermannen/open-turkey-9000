package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// §4.3 (owner, 2026-11-16): KC9K tells the layouts of a file apart:
// phrases, rendered terms, sites, hosts files and adblock rules.
func TestClassifyEntryLines(t *testing.T) {
	f := classifyEntryLines(strings.Split(`# a mixed file
[Adblock Plus 2.0]
! an adblock comment
[eat, nice] [hello, grandpa]
[eat, NICE]   [hello, grandpa]
[only one set]
0.0.0.0 tracker.example.com other.example.net # two names
127.0.0.1 localhost
::1 ip6-localhost
||ads.example.org^
||cdn.example.org/banners^$third-party
|https://example.edu/promo|
@@||allowed.example.com^
example.com##.banner
Reddit.com
https://www.youtube.com/shorts/
*casino*
naughty words
naughty words
`, "\n"))
	want := entryFile{
		Phrases: []string{"[eat, nice] [hello, grandpa]"},
		Entries: []string{"tracker.example.com", "other.example.net", "ads.example.org", "cdn.example.org/banners",
			"https://example.edu/promo", "reddit.com", "https://www.youtube.com/shorts/", "*casino*", "naughty words"},
		Sites: []string{"tracker.example.com", "other.example.net", "ads.example.org", "cdn.example.org/banners",
			"https://example.edu/promo", "reddit.com", "https://www.youtube.com/shorts/", "*casino*"},
		Terms:   []string{"naughty words"},
		Skipped: []string{"[only one set]", "@@||allowed.example.com^", "example.com##.banner"},
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("got  %+v\nwant %+v", f, want)
	}
	if got := describeEntryFile(f); got != "1 phrase, 8 sites, 1 rendered term, 3 lines KC9K can't use" {
		t.Errorf("describe: %q", got)
	}
}

// A hosts file, phrases and a mixed file go where they belong; what doesn't
// fit is left out and named, never stored as a rendered term with brackets
// or an IP address in it (they were).
func TestImportLayouts(t *testing.T) {
	d := testDB(t)
	notTerminal(t)
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	mustRun(t, "block", "create", "b")
	hosts := write("hosts", "0.0.0.0 a.example.com\n0.0.0.0 b.example.com\n")
	mixed := write("mixed", "[eat, nice] [hello, grandpa]\nc.example.com\nplain term\n")
	phrases := write("phrases", "[eat, nice] [hello, grandpa]\n[big-, strong] [-gorilla]\n")

	mustRun(t, "list", "import", "b", "Blocked", hosts)
	mustRun(t, "list", "import", "b", "Blocked", mixed)
	l := list(t, d, "b", "Blocked")
	if got := strings.Join(l.Entries, " | "); strings.Contains(got, "[") || strings.Contains(got, "0.0.0.0") ||
		!strings.Contains(got, "a.example.com") || !strings.Contains(got, "c.example.com") || !strings.Contains(got, "plain term") {
		t.Errorf("Blocked = %s", got)
	}
	if err := runCLI(t, "list", "import", "b", "Blocked", phrases); err == nil {
		t.Error("a file of phrases was imported into a list")
	}

	mustRun(t, "phrase-set", "create", "b", "Words")
	mustRun(t, "phrase-set", "import", "b", "Words", mixed)
	sets, _ := d.PhraseSets("b")
	if len(sets) != 1 || len(sets[0].Phrases) != 1 {
		t.Fatalf("phrase sets = %+v", sets)
	}
	if err := runCLI(t, "phrase-set", "import", "b", "Words", hosts); err == nil {
		t.Error("a hosts file was imported into a phrase set")
	}

	// A file of phrases makes a phrase cluster without being told.
	mustRun(t, "cluster", "import", "found", phrases)
	if c, err := d.GetCluster("found"); err != nil || !c.Phrases || len(c.Entries) != 2 {
		t.Errorf("list preset = %+v, %v", c, err)
	}
}
