package enforce

import (
	"os/user"
	"slices"
	"testing"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// §3.2: window titles, folder entries, fetch tools and the extension guard
// follow a block's affected users.
func TestAffectedUsersTitlesAndGuard(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	old := regularUsers
	regularUsers = func() []string { return []string{me.Username, "root"} }
	defer func() { regularUsers = old }()
	var uid uint64 // the account's ID as KC9K knows it (a relative ID on Windows)
	if ids := blocker.UserIDs([]string{me.Username}); len(ids) == 1 {
		uid = ids[0]
	}
	d := testDB(t)

	required, uids := extensionUsers(d, nil)
	if required || uids != nil {
		t.Errorf("nothing active: %v %v", required, uids)
	}
	newUserBlock(t, d, "mine", "youtube.com", "/srv/games/", me.Username)
	if err := d.AddTitles("mine", []string{"YouTube"}); err != nil {
		t.Fatal(err)
	}
	active, _ := d.GetActiveBlocks()
	required, uids = extensionUsers(d, active)
	if !required || !slices.Equal(uids, []uint64{uid}) {
		t.Errorf("a block for one account: required %v in %v", required, uids)
	}
	// The GUI's "isn't running" dialog asks for its own account only.
	for who, want := range map[int64]bool{int64(uid): true, int64(uid) + 1: false, -1: true} {
		if got, err := ExtensionRequiredNowFor(d, who); err != nil || got != want {
			t.Errorf("ExtensionRequiredNowFor(%d) = %v, %v; want %v", who, got, err, want)
		}
	}
	if got := blockDomains(active[0]); !slices.Contains(got, "youtube.com") {
		t.Errorf("fetch tools: a block for some users still names its sites: %v", got)
	}
	targets, _ := titleTargets(d, active)
	if len(targets.All.Close)+len(targets.All.Folders) != 0 ||
		!slices.Equal(targets.Users[uid].Close, []string{"YouTube"}) || !slices.Equal(targets.Users[uid].Folders, []string{"/srv/games/"}) {
		t.Errorf("title targets: %+v", targets)
	}
	hit := blocker.TitleHit{Keyword: "YouTube", UID: uid}
	if got := titleHitBlocks(active, hit); !slices.Equal(got, []string{"mine"}) {
		t.Errorf("hit in my window: %v", got)
	}
	hit.UID = uid + 1
	if got := titleHitBlocks(active, hit); len(got) != 0 {
		t.Errorf("hit in another account's window blamed on %v", got)
	}

	newUserBlock(t, d, "everyone", "reddit.com", "steam")
	if err := d.AddTitles("everyone", []string{"Reddit"}); err != nil {
		t.Fatal(err)
	}
	active, _ = d.GetActiveBlocks()
	if required, uids = extensionUsers(d, active); !required || uids != nil {
		t.Errorf("a block for everyone: required %v in %v", required, uids)
	}
	targets, _ = titleTargets(d, active)
	if !slices.Equal(targets.All.Close, []string{"Reddit"}) || !slices.Equal(targets.For(uid).Close, []string{"Reddit", "YouTube"}) {
		t.Errorf("title targets with a block for everyone: %+v", targets)
	}
}

// §3.6 places: nmcli's terse lines, with "\:" for a colon inside a field;
// a line without a whole BSSID is skipped.
func TestParseNmcli(t *testing.T) {
	out := "aa\\:bb\\:cc\\:dd\\:ee\\:01:Home\n" +
		"AA\\:BB\\:CC\\:DD\\:EE\\:02:Café\\: upstairs\n" +
		"broken line\n" +
		"aa\\:bb:Short\n" +
		"AA\\:BB\\:CC\\:DD\\:EE\\:03:\n\n"
	nets := parseNmcli(out)
	if len(nets) != 3 || nets[0].BSSID != "AA:BB:CC:DD:EE:01" || nets[0].SSID != "Home" ||
		nets[1].SSID != "Café: upstairs" || nets[2].BSSID != "AA:BB:CC:DD:EE:03" || nets[2].SSID != "" {
		t.Errorf("parsed %+v", nets)
	}
	if len(parseNmcli("")) != 0 {
		t.Error("nothing from nothing")
	}
}
