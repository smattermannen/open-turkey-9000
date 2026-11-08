// titles.go — closing apps by their window title, like Cold Turkey's window
// title blocking: block "YouTube" and any app showing a window with YouTube
// in its title is closed.
//
// A matching window is first asked to close (as its close button would), so
// the app can save or ask. If it is still there on the next cycle, its
// program is killed, with everything it started. Browsers are left alone
// here: the extension closes just the tab whose title matches. The same
// rules as app blocking apply: only regular users' programs, and never ones
// the desktop needs.
//
// Window titles come from X11 (Linux Mint's Cinnamon). Under Wayland only
// apps running through XWayland are seen.
package blocker

import (
	"log"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TitleCloseGrace is how long a window asked to close may stay before its
// program is killed.
const TitleCloseGrace = 4 * time.Second

// titleSession is one desktop to watch: its X display and Xauthority file.
type titleSession struct {
	display string
	xauth   string
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
	return titleSession{display: ":0", xauth: filepath.Join(u.HomeDir, ".Xauthority")}
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

// Enforce runs one cycle at now with the active blocks' keywords and
// returns how many programs it killed.
func (g *TitleGuard) Enforce(keywords []string, now time.Time) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(keywords) == 0 {
		g.asked = map[uint32]time.Time{}
		return 0
	}
	seen := map[uint32]bool{}
	var kill []int
	for _, s := range g.sessions() {
		c, err := openUserDisplay(s.display, s.xauth)
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
			if keyword == "" || w.PID <= 1 || !ownedByUser(w.PID) {
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
			case now.Sub(first) >= TitleCloseGrace:
				log.Printf("titles: closing %s (pid %d): %q is still open", exeName(w.PID), w.PID, w.Title)
				kill = append(kill, w.PID)
			}
		}
		c.Close()
	}
	for id := range g.asked {
		if !seen[id] {
			delete(g.asked, id)
		}
	}
	return killTrees(kill)
}

// killTrees kills processes and everything they started, under the same
// rules as app blocking: regular users' programs only, never init or this
// process and its ancestors.
func killTrees(roots []int) int {
	if len(roots) == 0 {
		return 0
	}
	parent, children := processTree()
	protected := map[int]bool{1: true}
	for pid := syscall.Getpid(); pid > 0 && !protected[pid]; pid = parent[pid] {
		protected[pid] = true
	}
	killed := 0
	for _, pid := range append(roots, descendants(roots, children)...) {
		if protected[pid] || !ownedByUser(pid) {
			continue
		}
		protected[pid] = true
		if syscall.Kill(pid, syscall.SIGKILL) == nil {
			killed++
		}
	}
	return killed
}
