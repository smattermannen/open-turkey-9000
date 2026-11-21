//go:build linux

package blocker

import (
	"slices"
	"testing"
	"time"
)

// §3.2 affected users: a user's windows are judged by every account's
// targets and the user's own, not another user's.
func TestTitlesByUserFor(t *testing.T) {
	targets := TitlesByUser{
		All:   TitleTargets{Close: []string{"poker"}, Folders: []string{"/srv/games/"}},
		Users: map[uint64]TitleTargets{1001: {Close: []string{"youtube"}, Watch: []string{"news"}}},
	}
	anna := targets.For(1001)
	if !slices.Equal(anna.Close, []string{"poker", "youtube"}) || !slices.Equal(anna.Watch, []string{"news"}) ||
		!slices.Equal(anna.Folders, []string{"/srv/games/"}) {
		t.Errorf("anna's targets: %+v", anna)
	}
	ben := targets.For(1002)
	if !slices.Equal(ben.Close, []string{"poker"}) || len(ben.Watch) != 0 {
		t.Errorf("ben's targets: %+v", ben)
	}
	if targets.Empty() || !(TitlesByUser{Users: map[uint64]TitleTargets{1001: {}}}).Empty() {
		t.Error("Empty")
	}
	if (TitlesByUser{Users: map[uint64]TitleTargets{1001: {Folders: []string{"/x/"}}}}).Empty() {
		t.Error("a user's folder entry alone isn't nothing")
	}
}

// §3.2: a window title entry of a block for another account leaves this
// user's window open; one for this account asks it to close and names the
// user. On a virtual X server as the test's own user.
func TestTitleGuardPerUser(t *testing.T) {
	display, xauth := startX(t)
	uid := fileOwner(t, xauth)
	if !regularUser(uid) {
		t.Skip("run as a regular user: the guard leaves system accounts' windows alone")
	}
	startClient(t, display, xauth, "xterm", "-T", "Watching YouTube - Player", "-e", "sleep", "60")
	if _, ok := findWindow(t, display, xauth, "YouTube"); !ok {
		t.Fatal("window not found")
	}
	g := NewTitleGuard()
	g.sessions = func() []titleSession { return []titleSession{{display: display, xauth: xauth, uid: uid}} }
	now := time.Now()

	other := TitlesByUser{Users: map[uint64]TitleTargets{uid + 1: {Close: []string{"youtube"}}}}
	if _, hits := g.EnforceUsers(other, now); len(hits) != 0 {
		t.Errorf("another account's entry judged this user's window: %+v", hits)
	}
	watch := TitlesByUser{Users: map[uint64]TitleTargets{uid: {Watch: []string{"youtube"}}}}
	if _, hits := g.EnforceUsers(watch, now); len(hits) != 1 || !hits[0].Watched || hits[0].UID != uid {
		t.Errorf("watched: %+v", hits)
	}
	mine := TitlesByUser{Users: map[uint64]TitleTargets{uid: {Close: []string{"youtube"}}}}
	if _, hits := g.EnforceUsers(mine, now); len(hits) != 1 || hits[0].Watched || hits[0].UID != uid || hits[0].Keyword != "youtube" {
		t.Errorf("own entry: %+v", hits)
	}
}
