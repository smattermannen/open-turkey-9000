//go:build linux

package blocker

import (
	"os/user"
	"slices"
	"testing"
)

// A wallpaper or dark mode change reaches everyone signed in but those
// already given it, and with SignedOut the people who aren't signed in
// (their desktop then starts with it, owner 2026-11-17).
func TestReachTargets(t *testing.T) {
	in, out := desktopPrefsUsers, signedOutPrefsUsers
	t.Cleanup(func() { desktopPrefsUsers, signedOutPrefsUsers = in, out })
	anna, ben, cleo := &user.User{Uid: "1000", Username: "anna"}, &user.User{Uid: "1001", Username: "ben"},
		&user.User{Uid: "1002", Username: "cleo"}
	desktopPrefsUsers = func() []*user.User { return []*user.User{anna, ben} }
	signedOutPrefsUsers = func() []*user.User { return []*user.User{cleo} }
	ids := func(r Reach) (got []string) {
		for _, t := range r.targets() {
			got = append(got, t.Uid+map[bool]string{true: " out", false: ""}[t.signedOut])
		}
		return
	}
	if got := ids(Reach{}); !slices.Equal(got, []string{"1000", "1001"}) {
		t.Errorf("signed in only: %v", got)
	}
	if got := ids(Reach{SignedOut: true}); !slices.Equal(got, []string{"1000", "1001", "1002 out"}) {
		t.Errorf("everyone: %v", got)
	}
	if got := ids(Reach{Skip: []string{"1000"}}); !slices.Equal(got, []string{"1001"}) {
		t.Errorf("skipping anna: %v", got)
	}
	if got := SignedInUserIDs(); !slices.Equal(got, []string{"1000", "1001"}) {
		t.Errorf("signed in: %v", got)
	}
}
