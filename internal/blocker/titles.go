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
	watched  map[uint32]bool      // windows with a watched keyword, already reported
	browsers *browserCache
	sessions func() []titleSession
}

// NewTitleGuard returns a guard watching every desktop session on display :0.
func NewTitleGuard() *TitleGuard {
	return &TitleGuard{
		asked:    map[uint32]time.Time{},
		watched:  map[uint32]bool{},
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

// TitleTargets are what the title guard looks for in one user's windows.
type TitleTargets struct {
	Close   []string // window title entries whose windows close
	Watch   []string // window title entries only reported (owner, 2026-11-16)
	Folders []string // folder entries (§3.1): file manager windows showing them close
}

func (t TitleTargets) Empty() bool {
	return len(t.Close) == 0 && len(t.Watch) == 0 && len(t.Folders) == 0
}

// TitlesByUser are the title guard's targets per user (§3.2 affected
// users): All for every account, Users for the accounts (by user ID) a
// block names.
type TitlesByUser struct {
	All   TitleTargets
	Users map[uint64]TitleTargets
}

// For returns the targets in the windows of the user uid: every account's
// and the user's own.
func (t TitlesByUser) For(uid uint64) TitleTargets {
	own, ok := t.Users[uid]
	if !ok {
		return t.All
	}
	return TitleTargets{
		Close:   append(append([]string{}, t.All.Close...), own.Close...),
		Watch:   append(append([]string{}, t.All.Watch...), own.Watch...),
		Folders: append(append([]string{}, t.All.Folders...), own.Folders...),
	}
}

// Empty reports whether nobody's windows are looked at.
func (t TitlesByUser) Empty() bool {
	if !t.All.Empty() {
		return false
	}
	for _, own := range t.Users {
		if !own.Empty() {
			return false
		}
	}
	return true
}

// TitleHit is a window the guard newly asked to close, or newly saw with a
// watched entry: the entry (or folder entry) and the user whose window it was.
type TitleHit struct {
	Keyword string
	UID     uint64
	Watched bool
}

// Enforce runs one cycle at now with the active blocks' keywords. It returns
// how many programs it killed, and the keyword of each window it newly asked
// to close (so the caller can tell which block stopped something).
func (g *TitleGuard) Enforce(keywords []string, now time.Time) (int, []string) {
	killed, asked, _ := g.EnforceWatch(keywords, nil, now)
	return killed, asked
}

// EnforceWatch is Enforce, and also reports the keyword of each window
// whose title contains a watched keyword, once per window, without closing
// it: a block whose window titles don't close only fire their other actions
// (owner, 2026-11-16). Every user's windows, no folder entries.
func (g *TitleGuard) EnforceWatch(keywords, watch []string, now time.Time) (int, []string, []string) {
	killed, hits := g.EnforceUsers(TitlesByUser{All: TitleTargets{Close: keywords, Watch: watch}}, now)
	var asked, seen []string
	for _, h := range hits {
		if h.Watched {
			seen = append(seen, h.Keyword)
		} else {
			asked = append(asked, h.Keyword)
		}
	}
	return killed, asked, seen
}

// EnforceUsers runs one cycle at now: each desktop user's windows are
// judged by that user's targets (targets.For). It returns how many programs
// it killed and each window newly asked to close or newly seen.
func (g *TitleGuard) EnforceUsers(targets TitlesByUser, now time.Time) (int, []TitleHit) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.watched == nil {
		g.watched = map[uint32]bool{}
	}
	if targets.Empty() {
		g.asked, g.watched = map[uint32]time.Time{}, map[uint32]bool{}
		return 0, nil
	}
	var hits []TitleHit
	seen := map[uint32]bool{}
	kill := map[int]uint64{} // process -> the user it must still belong to
	for _, s := range g.sessions() {
		mine := targets.For(s.uid)
		if mine.Empty() {
			continue // no block applies to this user's windows
		}
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
			keyword := matchingKeyword(w.Title, mine.Close)
			if keyword == "" && fileManagers[exeName(w.PID)] {
				keyword = folderWindow(w.Title, mine.Folders) // §3.1: a window showing a folder entry
			}
			watched := ""
			if keyword == "" {
				watched = matchingKeyword(w.Title, mine.Watch)
			}
			if (keyword == "" && watched == "") || w.PID <= 1 || !ownedBy(w.PID, s.uid) {
				continue
			}
			if essentialProcess(w.PID) || g.browsers.guarded(w.PID) {
				continue // the desktop needs it, or the extension handles tabs
			}
			if keyword == "" {
				seen[w.ID] = true
				if !g.watched[w.ID] {
					g.watched[w.ID] = true
					hits = append(hits, TitleHit{Keyword: watched, UID: s.uid, Watched: true})
				}
				continue
			}
			seen[w.ID] = true
			first, asked := g.asked[w.ID]
			switch {
			case !asked:
				log.Printf("titles: asking %q (pid %d) to close: its title contains %q", w.Title, w.PID, keyword)
				_ = c.closeWindow(w.ID)
				g.asked[w.ID] = now
				hits = append(hits, TitleHit{Keyword: keyword, UID: s.uid})
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
	for id := range g.watched {
		if !seen[id] {
			delete(g.watched, id)
		}
	}
	return killTrees(kill), hits
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

// fileManagers are the programs whose windows show folders.
var fileManagers = map[string]bool{"nemo": true, "nautilus": true, "caja": true, "thunar": true,
	"dolphin": true, "pcmanfm": true, "pcmanfm-qt": true, "nemo-desktop": false}

// folderWindow returns the folder a file manager window shows, by its
// title: the folder's name ("games", "games — Dolphin") or its whole path;
// "" for none.
func folderWindow(title string, folders []string) string {
	for _, f := range folders {
		path := strings.TrimSuffix(f, "/")
		base := path[strings.LastIndex(path, "/")+1:]
		if base == "" {
			continue
		}
		if title == base || strings.HasPrefix(title, base+" ") || strings.Contains(title, path) {
			return f
		}
	}
	return ""
}
