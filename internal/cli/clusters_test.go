package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
)

func TestClusters(t *testing.T) {
	d := testDB(t)
	mustRun(t, "cluster", "create", "academia")
	if err := runCLI(t, "cluster", "create", "academia"); err == nil {
		t.Error("two clusters got the same name")
	}
	// Entries follow the same rules as a list's, and domains are normalized.
	mustRun(t, "cluster", "add", "academia", "https://ArXiv.org/", "jstor.org", "scholar.google.com", "arxiv.org")
	if err := runCLI(t, "cluster", "add", "academia", "homework"); err == nil {
		t.Error("a lone word was accepted in an address cluster")
	}
	c, err := d.GetCluster("academia")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Entries, []string{"arxiv.org", "jstor.org", "scholar.google.com"}) {
		t.Errorf("entries = %q", c.Entries)
	}
	mustRun(t, "cluster", "remove", "academia", "https://jstor.org/")
	if err := runCLI(t, "cluster", "remove", "academia", "nothere.org"); err == nil {
		t.Error("removing a missing entry succeeded")
	}
	if err := runCLI(t, "cluster", "where", "academia", "url", "off"); err == nil {
		t.Error("a cluster was left looking nowhere")
	}
	mustRun(t, "cluster", "where", "academia", "title", "on")
	mustRun(t, "cluster", "rename", "academia", "school")
	if c, _ = d.GetCluster("school"); c.Where != (db.Where{URL: true, Title: true}) || len(c.Entries) != 2 {
		t.Errorf("after edits: %+v", c)
	}

	// As a new list: a blacklist that blocks, named after the cluster.
	mustRun(t, "block", "create", "b")
	mustRun(t, "cluster", "copy", "school", "b")
	l := list(t, d, "b", "school")
	if l.IsExceptions() || l.Where != c.Where || !reflect.DeepEqual(l.Entries, c.Entries) || !l.HasAction(db.ActionBlock) {
		t.Errorf("copied list = %+v", l)
	}
	if err := runCLI(t, "cluster", "copy", "school", "b"); err == nil {
		t.Error("a second copy under the same name succeeded")
	}
	// As an exception list under another name, and into a list.
	mustRun(t, "cluster", "copy", "school", "b", "--as", "Spared", "--exceptions")
	if l := list(t, d, "b", "Spared"); !l.IsExceptions() || len(l.Entries) != 2 {
		t.Errorf("Spared = %+v", l)
	}
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	mustRun(t, "cluster", "copy", "school", "b", "--into", "Blocked")
	if got := list(t, d, "b", "Blocked").Entries; !reflect.DeepEqual(got, []string{"reddit.com", "arxiv.org", "scholar.google.com"}) {
		t.Errorf("Blocked = %q", got)
	}
	// The copy is a copy: the cluster's later changes stay in the cluster.
	mustRun(t, "cluster", "add", "school", "nature.com")
	if got := list(t, d, "b", "school").Entries; len(got) != 2 {
		t.Errorf("a cluster edit reached the block: %q", got)
	}
	mustRun(t, "cluster", "delete", "school")
	if got := list(t, d, "b", "school").Entries; len(got) != 2 {
		t.Errorf("deleting the cluster changed the block: %q", got)
	}

	// A block's list saved as a cluster.
	mustRun(t, "cluster", "save", "b", "Blocked", "mine")
	if c, _ := d.GetCluster("mine"); len(c.Entries) != 3 || c.Where != (db.Where{URL: true}) {
		t.Errorf("saved cluster = %+v", c)
	}
}

// Copying exceptions into a locked block is refused and leaves no list
// behind. (Copying into a blacklist goes through 'list add', always allowed.)
func TestClusterCopyGating(t *testing.T) {
	d := testDB(t)
	mustRun(t, "cluster", "create", "anime")
	mustRun(t, "cluster", "add", "anime", "crunchyroll.com", "myanimelist.net")
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "cluster", "copy", "anime", "b", "--exceptions"); err == nil || !strings.Contains(err.Error(), "held") {
		t.Errorf("copying exceptions into a locked block: %v; want refused as held", err)
	}
	if b, _ := d.GetBlock("b"); len(b.Lists) != 2 {
		t.Errorf("a refused copy left a list behind: %d lists", len(b.Lists))
	}
	if err := runCLI(t, "cluster", "copy", "anime", "b", "--into", "Exceptions"); err == nil {
		t.Error("copying into the exception list of a locked block succeeded")
	}
	if got := list(t, d, "b", "Exceptions").Entries; len(got) != 1 { // file://*
		t.Errorf("Exceptions = %q", got)
	}
}

func TestClusterExportImport(t *testing.T) {
	d := testDB(t)
	mustRun(t, "cluster", "create", "words", "--title", "--text")
	mustRun(t, "cluster", "add", "words", "anime", "*manga*", "light novel")
	c, _ := d.GetCluster("words")
	file := filepath.Join(t.TempDir(), "words.txt")
	text := clusterHeader + strings.Join(c.Where.List(), ", ") + "\n" + strings.Join(c.Entries, "\n") + "\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	// A new cluster takes where it looks from the file.
	mustRun(t, "cluster", "import", "again", file)
	got, err := d.GetCluster("again")
	if err != nil {
		t.Fatal(err)
	}
	if got.Where != c.Where || !reflect.DeepEqual(got.Entries, c.Entries) {
		t.Errorf("imported = %+v; want %+v", got, c)
	}
	// Into an existing one, only new entries are added.
	mustRun(t, "cluster", "import", "again", file)
	if got, _ = d.GetCluster("again"); len(got.Entries) != 3 {
		t.Errorf("a second import duplicated entries: %q", got.Entries)
	}
	// Words don't fit an address cluster: nothing is made.
	if err := runCLI(t, "cluster", "import", "bad", file, "--url"); err == nil {
		t.Error("words were imported into an address cluster")
	}
	if exists, _ := d.ClusterExists("bad"); exists {
		t.Error("a failed import left a cluster behind")
	}
}

func TestRecording(t *testing.T) {
	d := testDB(t)
	mustRun(t, "block", "create", "b")
	mustRun(t, "list", "create", "b", "Words", "--text")
	if err := runCLI(t, "record", "start", "b", "Words"); err == nil {
		t.Error("recording into a list that doesn't look at addresses started")
	}
	if err := runCLI(t, "record", "start"); err == nil {
		t.Error("recording without a target started")
	}
	// Into a cluster, made on the way.
	mustRun(t, "record", "start", "--cluster", "school")
	if err := runCLI(t, "record", "start", "b", "Exceptions"); err == nil {
		t.Error("a second recording started")
	}
	for _, site := range []string{"login.school.example", "school.example", "login.school.example"} {
		if err := d.AddRecordedSite(site); err != nil {
			t.Fatal(err)
		}
	}
	mustRun(t, "record")
	mustRun(t, "record", "stop")
	if c, _ := d.GetCluster("school"); !reflect.DeepEqual(c.Entries, []string{"login.school.example", "school.example"}) {
		t.Errorf("cluster after recording = %q", c.Entries)
	}
	if r, _ := d.GetRecording(); r != nil {
		t.Error("still recording after stop")
	}

	// Into a list; a rename in between doesn't lose it.
	mustRun(t, "record", "start", "b", "Exceptions")
	if sites, _ := d.RecordedSites(); len(sites) != 0 {
		t.Errorf("a new recording started with %q", sites)
	}
	mustRun(t, "list", "rename", "b", "Exceptions", "Spared")
	if err := d.AddRecordedSite("arxiv.org"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "record", "stop")
	if got := list(t, d, "b", "Spared").Entries; !reflect.DeepEqual(got, []string{db.FileException, "arxiv.org"}) {
		t.Errorf("Spared = %q", got)
	}

	// Into the exceptions of a locked block: refused, and the recording stays.
	mustRun(t, "list", "add", "b", "Blocked", "reddit.com")
	mustRun(t, "record", "start", "b", "Spared")
	if err := d.AddRecordedSite("jstor.org"); err != nil {
		t.Fatal(err)
	}
	if err := d.ActivateBlock("b", true, 300, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "record", "stop"); err == nil || !strings.Contains(err.Error(), "held") {
		t.Errorf("stop into a locked block's exceptions: %v; want refused as held", err)
	}
	if sites, _ := d.RecordedSites(); len(sites) != 1 {
		t.Errorf("a refused stop lost the recording: %q", sites)
	}
	mustRun(t, "record", "stop", "--discard")
	if got := list(t, d, "b", "Spared").Entries; len(got) != 2 {
		t.Errorf("Spared after a discarded recording = %q", got)
	}
}
