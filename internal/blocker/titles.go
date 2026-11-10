// titles.go — closing apps by their window title, like Cold Turkey's window
// title blocking: block "YouTube" and any app showing a window with YouTube
// in its title is closed.
//
// A matching window is first asked to close (as its close button would), so
// the app can save or ask. If it is still there on the next cycle, its
// program is killed, with everything it started. Browsers are left alone
// here: the extension closes just the tab whose title matches. The same
// rules as app blocking apply: only regular users' programs, and never ones
// the desktop needs. A window names its own process (_NET_WM_PID), so the
// name is believed only for a process of the desktop's own user: no one can
// title a window to get someone else's program killed.
//
// Window titles come from X11 (Linux Mint's Cinnamon). Under Wayland only
// apps running through XWayland are seen.
package blocker

import (
	"log"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TitleCloseGrace is how long a window asked to close may stay before its
// program is killed.
const TitleCloseGrace = 4 * time.Second

// titleSession is one desktop to watch: its X display, and the Xauthority
// file and user ID of the user it belongs to.
type titleSession struct {
	display string
	xauth   string
	uid     uint64
}

// TitleGuard closes apps whose window titles contain blocked keywords.
type TitleGuard struct {
	mu       sync.Mutex
	asked    map[uint32]time.Time // window -> when it was asked to close
	browsers *browserCache
	sessions func() []titleSession
}

// NewTitleGuard returns a guard watching every desktop session on display :0.
func NewTitleGuard() *TitleGuard {
	return &TitleGuard{
		asked:    map[uint32]time.Time{},
		browsers: newBrowserCache(),
		sessions: func() []titleSession {
			var out []titleSession
			for _, u := range desktopUsers() {
				out = append(out, sessionFor(u))
			}
			return out
		},
	}
}

func sessionFor(u *user.User) titleSession {
	uid, _ := strconv.ParseUint(u.Uid, 10, 32)
	return titleSession{display: ":0", xauth: filepath.Join(u.HomeDir, ".Xauthority"), uid: uid}
}

// matchingKeyword returns the keyword title contains (case-insensitive), or "".
func matchingKeyword(title string, keywords []string) string {
	lower := strings.ToLower(title)
	for _, k := range keywords {
		if k != "" && strings.Contains(lower, strings.ToLower(k)) {
			return k
		}
	}
	return ""
}

// Enforce runs one cycle at now with the active blocks' keywords. It returns
// how many programs it killed, and the keyword of each window it newly asked
// to close (so the caller can tell which block stopped something).
func (g *TitleGuard) Enforce(keywords []string, now time.Time) (int, []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(keywords) == 0 {
		g.asked = map[uint32]time.Time{}
		return 0, nil
	}
	var newlyAsked []string
	seen := map[uint32]bool{}
	kill := map[int]uint64{} // process -> the user it must still belong to
	for _, s := range g.sessions() {
		c, err := openUserDisplay(s.display, s.xauth, s.uid)
		if err != nil {
			continue // no X session there (a text console, or Wayland)
		}
		windows, err := c.windows()
		if err != nil {
			c.Close()
			continue
		}
		for _, w := range windows {
			keyword := matchingKeyword(w.Title, keywords)
			if keyword == "" || w.PID <= 1 || !ownedBy(w.PID, s.uid) {
				continue
			}
			if isEssentialApp(exeName(w.PID)) || g.browsers.guarded(w.PID) {
				continue // the desktop needs it, or the extension handles tabs
			}
			seen[w.ID] = true
			first, asked := g.asked[w.ID]
			switch {
			case !asked:
				log.Printf("titles: asking %q (pid %d) to close: its title contains %q", w.Title, w.PID, keyword)
				_ = c.closeWindow(w.ID)
				g.asked[w.ID] = now
				newlyAsked = append(newlyAsked, keyword)
			case now.Sub(first) >= TitleCloseGrace:
				log.Printf("titles: closing %s (pid %d): %q is still open", exeName(w.PID), w.PID, w.Title)
				kill[w.PID] = s.uid
			}
		}
		c.Close()
	}
	for id := range g.asked {
		if !seen[id] {
			delete(g.asked, id)
		}
	}
	return killTrees(kill), newlyAsked
}

// killTrees kills each process in roots and everything it started, as long
// as they run as the user given for it (so a program another user started
// under it, through su, is spared). Init, this process and its ancestors are
// never killed.
func killTrees(roots map[int]uint64) int {
	if len(roots) == 0 {
		return 0
	}
	parent, children := processTree()
	protected := map[int]bool{1: true}
	for pid := syscall.Getpid(); pid > 0 && !protected[pid]; pid = parent[pid] {
		protected[pid] = true
	}
	killed := 0
	for root, uid := range roots {
		for _, pid := range append([]int{root}, descendants([]int{root}, children)...) {
			if protected[pid] || !ownedBy(pid, uid) {
				continue
			}
			protected[pid] = true
			if syscall.Kill(pid, syscall.SIGKILL) == nil {
				killed++
			}
		}
	}
	return killed
}
