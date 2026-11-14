package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// runCLI runs a kc9k command line against the test database.
func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	resetFlags(rootCmd)
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// resetFlags puts every flag back to its default: cobra keeps them between
// runs in one process.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if slice, ok := f.Value.(pflag.SliceValue); ok {
			_ = slice.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}

func testDB(t *testing.T) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	t.Setenv("KC9K_DB", path)
	d, err := db.OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func mustRun(t *testing.T, args ...string) {
	t.Helper()
	if err := runCLI(t, args...); err != nil {
		t.Fatalf("%s: %v", strings.Join(args, " "), err)
	}
}

func list(t *testing.T, d *db.DB, block, name string) db.WebList {
	t.Helper()
	b, err := d.GetBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	l, ok := b.Lists.Find(name)
	if !ok {
		t.Fatalf("block %s has no list %s", block, name)
	}
	return l
}

func TestListEntries(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	// Address lists take domains, paths and patterns, and keep domains
	// normalized; a lone word doesn't match addresses.
	mustRun(t, "list", "add", "b", "Blocked", "https://Reddit.com", "reddit.com/r/funny", "*casino*")
	if err := runCLI(t, "list", "add", "b", "Blocked", "nsfw"); err == nil {
		t.Error("a lone word was accepted in an address list")
	}
	if got := list(t, d, "b", "Blocked").Entries; !reflect.DeepEqual(got, []string{"reddit.com", "reddit.com/r/funny", "*casino*"}) {
		t.Errorf("Blocked = %q", got)
	}
	// §3.2: a list is an address list or a term list; a term list may look
	// at the url too (R-PHRASE-6), an address list only at the url.
	mustRun(t, "list", "create", "b", "Both", "--url", "--text")
	if l := list(t, d, "b", "Both"); l.Type != db.ListTerm || l.Where != (db.Where{URL: true, Text: true}) {
		t.Errorf("Both = %+v; want a term list looking at the url and the text", l)
	}
	if err := runCLI(t, "list", "scope", "b", "Blocked", "title", "on"); err == nil {
		t.Error("an address list was made to look at the title")
	}
	mustRun(t, "list", "create", "b", "Mixed", "--text")
	mustRun(t, "list", "add", "b", "Mixed", "nsfw", "18+", "example.com")
	if err := runCLI(t, "list", "add", "b", "Mixed", "***"); err == nil {
		t.Error("an entry of only wildcards was accepted")
	}
	if l := list(t, d, "b", "Mixed"); l.Where != (db.Where{Text: true}) || len(l.Entries) != 3 {
		t.Errorf("Mixed = %+v", l)
	}
	mustRun(t, "list", "scope", "b", "Mixed", "url", "on")
	if l := list(t, d, "b", "Mixed"); l.Type != db.ListTerm || len(l.Entries) != 3 {
		t.Errorf("Mixed after url on = %+v", l)
	}
	// Removing a domain finds it however it is written.
	mustRun(t, "list", "remove", "b", "Blocked", "https://Reddit.com/")
	if got := list(t, d, "b", "Blocked").Entries; len(got) != 2 {
		t.Errorf("after removing reddit.com: %q", got)
	}
	mustRun(t, "list", "where", "b", "Mixed", "text", "off")
	if err := runCLI(t, "list", "where", "b", "Mixed", "url", "off"); err == nil {
		t.Error("a list was left looking nowhere")
	}
	if err := runCLI(t, "list", "action", "b", "Mixed", "block", "off"); err == nil {
		t.Error("a list was left without actions")
	}
	if err := runCLI(t, "list", "action", "b", "Exceptions", "greyscale", "on"); err == nil {
		t.Error("an exception list took an action")
	}

	// The old commands reach the default lists; add-keyword makes Page words.
	mustRun(t, "block", "add-exception", "b", "reddit.com/r/golang")
	mustRun(t, "block", "add-keyword", "b", "nsfw", "18+", "*porn*")
	if l := list(t, d, "b", db.DefaultWordsList); l.Where != (db.Where{Text: true}) || len(l.Entries) != 3 {
		t.Errorf("Page words = %+v", l)
	}
	if got := list(t, d, "b", "Exceptions").Entries; !reflect.DeepEqual(got, []string{db.FileException, "reddit.com/r/golang"}) {
		t.Errorf("Exceptions = %q", got)
	}
}

// What strengthens a block is always allowed; what loosens it is refused
// while it is locked (before anything is applied).
func TestListGating(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "block", "add-keyword", "b", "nsfw", "18+")
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	mustRun(t, "list", "threshold", "b", "Page words", "block", "3")
	mustRun(t, "list", "hits", "b", "Page words", "block", "4")
	if err := runCLI(t, "list", "threshold", "b", "Page words", "block", "0"); err == nil {
		t.Error("threshold 0 accepted")
	}
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	refused := [][]string{
		{"list", "threshold", "b", "Page words", "block", "5"},
		{"list", "remove", "b", "Page words", "nsfw"},
		{"list", "remove", "b", "Blocked", "reddit.com"},
		{"list", "add", "b", "Exceptions", "reddit.com/r/golang"},
		{"list", "where", "b", "Exceptions", "text", "on"},
		{"list", "delete", "b", "Page words"},
		{"block", "remove-keyword", "b", "nsfw"},
		{"block", "threshold", "b", "block", "9"},
		{"list", "hits", "b", "Page words", "block", "9"},
	}
	for _, args := range refused {
		if err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "locked") {
			t.Errorf("%s on a locked block: %v; want refused as held", strings.Join(args, " "), err)
		}
	}
	if l := list(t, d, "b", "Page words"); len(l.Entries) != 2 || l.Actions[0].Distinct != 3 || l.Actions[0].Hits != 4 {
		t.Errorf("after refused edits: %+v", l)
	}
}

func TestBlockExportImport(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	mustRun(t, "list", "create", "b", "Lectures", "--exceptions", "--title")
	mustRun(t, "list", "add", "b", "Lectures", "lecture")
	mustRun(t, "list", "create", "b", "Words", "--text")
	mustRun(t, "list", "add", "b", "Words", "nsfw")
	mustRun(t, "list", "action", "b", "Words", "greyscale", "on")
	mustRun(t, "list", "threshold", "b", "Words", "greyscale", "2")

	before, _ := d.GetBlock("b")
	file := filepath.Join(t.TempDir(), "b.json")
	out, err := captureStdout(func() error { return runCLI(t, "block", "export", "b") })
	if err != nil {
		t.Fatal(err)
	}
	exported := strings.Replace(out, `"name": "b"`, `"name": "copy"`, 1)
	if err := os.WriteFile(file, []byte(exported), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "block", "import", file)
	after, _ := d.GetBlock("copy")
	for i := range before.Lists {
		before.Lists[i].ID, after.Lists[i].ID = 0, 0
	}
	if !reflect.DeepEqual(before.Lists, after.Lists) {
		t.Errorf("imported lists:\n got %+v\nwant %+v", after.Lists, before.Lists)
	}

	// Exports from before lists still import.
	legacy := `{"name": "old", "mode": "allowlist", "actions": ["block", "greyscale"],
		"sites": ["work.com"], "apps": ["steam"], "patterns": ["*ads*"], "page_keywords": ["nsfw"],
		"keyword_thresholds": {"block": 2, "greyscale": 1, "no-images": 1}}`
	if err := os.WriteFile(file, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "block", "import", file)
	if l := list(t, d, "old", "Blocked"); !reflect.DeepEqual(l.Entries, []string{"*.*", "*ads*"}) || len(l.Actions) != 2 {
		t.Errorf("old Blocked = %+v", l)
	}
	if l := list(t, d, "old", "Exceptions"); !reflect.DeepEqual(l.Entries, []string{"work.com"}) {
		t.Errorf("old Exceptions = %+v", l)
	}
	if l := list(t, d, "old", db.DefaultWordsList); l.Actions[0] != (db.ListAction{Name: "block", Distinct: 2, Hits: 1}) {
		t.Errorf("old Page words = %+v", l)
	}
}

// List import and export, and the same for apps.
func TestListFiles(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "create", "b", "Words", "--text")
	file := filepath.Join(t.TempDir(), "words.txt")
	if err := os.WriteFile(file, []byte("# words\nnsfw\n\n18+\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "list", "import", "b", "Words", file)
	mustRun(t, "block", "import-list", "b", "apps", file)
	if got := list(t, d, "b", "Words").Entries; !reflect.DeepEqual(got, []string{"nsfw", "18+"}) {
		t.Errorf("Words = %q", got)
	}
	out, err := captureStdout(func() error { return runCLI(t, "list", "export", "b", "Words") })
	if err != nil || out != "nsfw\n18+\n" {
		t.Errorf("export = %q, %v", out, err)
	}
}

// captureStdout returns what fn prints.
func captureStdout(fn func() error) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	old := os.Stdout
	os.Stdout = w
	runErr := fn()
	os.Stdout = old
	w.Close()
	data, _ := io.ReadAll(r)
	return string(data), runErr
}

// Word combinations go in title and text lists, written one way; a
// malformed one is refused rather than kept as plain text.
func TestWordCombinations(t *testing.T) {
	good := map[string]string{
		"cars(red, big, blue, beautiful)":                  "cars (red, big, blue, beautiful)",
		"car* (red,big)":                                   "car* (red, big)",
		"(red, big) cars":                                  "(red, big) cars",
		" (car, lambo*, ferrari)(red, big beautiful) ":     "(car, lambo*, ferrari) (red, big beautiful)",
		"(car, lambo, sheep)(big, small, nice) : 2(sport)": "(car, lambo, sheep) (big, small, nice):2 (sport)",
		"(a, b):2":                 "(a, b):2",
		"(a):1 (b) (c):3 d":        "(a) (b) (c):3 d",
		"(very cheap) used (cars)": "(very cheap) used (cars)",
	}
	for k, want := range good {
		if err := checkPageKeywords([]string{k}); err != nil {
			t.Errorf("%q refused: %v", k, err)
		}
		if got := normalizeCombination(k); got != want {
			t.Errorf("normalizeCombination(%q) = %q, want %q", k, got, want)
		}
	}
	boxes, _ := pageCombination("(car, lambo*) (big, small, nice):2 sport")
	if want := []comboBox{{Terms: []string{"car", "lambo*"}, Min: 1}, {Terms: []string{"big", "small", "nice"}, Min: 2},
		{Terms: []string{"sport"}, Min: 1, Bare: true}}; !reflect.DeepEqual(boxes, want) {
		t.Errorf("boxes = %+v", boxes)
	}
	for _, k := range []string{"cars(red, big", "cars)red(", ")(a)(b)", "(a))(b)", "(a) ( (b)", "cars()", "cars(red, )", "(*)(car)", "a, b(c)", "(a)",
		"(a):2:3", "(a, b):0", "(a):10 (b)", "(a):x", "a:(b)", "(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)"} {
		if err := checkPageKeywords([]string{k}); err == nil {
			t.Errorf("%q was accepted", k)
		}
	}
	long := "cars(" + strings.Repeat("beautiful, ", 20) + "red)"
	if err := checkPageKeywords([]string{long}); err != nil {
		t.Errorf("a %d-character combination was refused: %v", len(long), err)
	}

	// Combinations are phrases now: typing one is refused, with the
	// phrases it would be.
	testDB(t)
	mustRun(t, "block", "create", "b")
	err := runCLI(t, "block", "add-keyword", "b", "(car,lambo*)(big, small, nice):2", "nsfw")
	if err == nil || !strings.Contains(err.Error(), `as "[car, lambo*] [big, small, nice]"`) {
		t.Errorf("a typed combination: %v", err)
	}
	mustRun(t, "cluster", "create", "words", "--text")
	if err := runCLI(t, "cluster", "add", "words", "cars(red, big)"); err == nil {
		t.Error("a cluster took a typed combination")
	}
	// An address with brackets is still an address.
	mustRun(t, "list", "add", "b", "Blocked", "en.wikipedia.org/wiki/Mercury_(planet)")
}
