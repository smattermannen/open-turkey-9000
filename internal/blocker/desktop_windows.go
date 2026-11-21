package blocker

// desktop_windows.go — the desktop on Windows (ROADMAP W3): who is signed
// in (Windows sessions, WTS), and what the session agents do there
// (winagent): window titles and polite close, the app in front, idle
// time, notifications, questions, locking; logout and power off by the
// service itself; Frozen Chicken's logout freeze; network links for monk
// mode; hotkeys.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winagent"
	"golang.org/x/sys/windows"
)

var (
	wtsapi32                        = windows.NewLazySystemDLL("wtsapi32.dll")
	procWTSQuerySessionInformationW = wtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSLogoffSession            = wtsapi32.NewProc("WTSLogoffSession")
	procWTSFreeMemory               = wtsapi32.NewProc("WTSFreeMemory")
)

// deskSession is a Windows session with a signed-in person.
type deskSession struct {
	ID     uint32
	UID    int64
	User   string
	Active bool // at the screen (not disconnected)
	Locked bool
}

// deskSessions are the sessions with a signed-in user; replaced in tests.
var deskSessions = func() []deskSession {
	var info *windows.WTS_SESSION_INFO
	var n uint32
	if err := windows.WTSEnumerateSessions(0, 0, 1, &info, &n); err != nil {
		return nil
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(info)))
	var out []deskSession
	for _, s := range unsafe.Slice(info, n) {
		if s.SessionID == 0 {
			continue
		}
		var token windows.Token
		if windows.WTSQueryUserToken(s.SessionID, &token) != nil {
			continue
		}
		u, err := token.GetTokenUser()
		if err != nil {
			token.Close()
			continue
		}
		d := deskSession{ID: s.SessionID, UID: platform.RID(u.User.Sid), User: platform.AccountName(u.User.Sid),
			Active: s.State == windows.WTSActive}
		token.Close()
		d.Locked = sessionLocked(s.SessionID)
		out = append(out, d)
	}
	return out
}

// sessionLocked asks Windows whether session's screen is locked
// (WTSSessionInfoEx; never the session's own programs).
func sessionLocked(session uint32) bool {
	const wtsSessionInfoEx = 25
	var buf *byte
	var size uint32
	r, _, _ := procWTSQuerySessionInformationW.Call(0, uintptr(session), wtsSessionInfoEx,
		uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&size)))
	if r == 0 || buf == nil {
		return false
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(buf)))
	if size < 16 {
		return false
	}
	data := unsafe.Slice(buf, size)
	flags := int32(uint32(data[12]) | uint32(data[13])<<8 | uint32(data[14])<<16 | uint32(data[15])<<24)
	const wtsSessionStateLock = 0
	return flags == wtsSessionStateLock
}

// GraphicalSessions are the sessions people are signed in to, by ID.
var GraphicalSessions = func() map[string]bool {
	out := map[string]bool{}
	for _, s := range deskSessions() {
		out[strconv.FormatUint(uint64(s.ID), 10)] = true
	}
	return out
}

// SessionUsers are the people signed in.
func SessionUsers() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range deskSessions() {
		if !seen[s.User] {
			seen[s.User] = true
			out = append(out, s.User)
		}
	}
	return out
}

// SignedInUserIDs are the IDs of the people signed in.
func SignedInUserIDs() []string {
	var out []string
	for _, s := range deskSessions() {
		out = append(out, strconv.FormatInt(s.UID, 10))
	}
	return out
}

// RegularUsers are the computer's people (their profiles).
func RegularUsers() []string {
	var out []string
	for _, u := range librewolfUsers() {
		out = append(out, u.Username)
	}
	return out
}

// UserIDs are the account IDs (relative IDs) of names.
func UserIDs(names []string) []uint64 {
	var out []uint64
	for _, n := range names {
		sid, _, _, err := windows.LookupSID("", n)
		if err == nil {
			if rid := platform.RID(sid); rid >= 0 {
				out = append(out, uint64(rid))
			}
		}
	}
	return out
}

// agentStatus asks session's agent for its idle time.
func agentStatus(session uint32) (winagent.Status, bool) {
	var st winagent.Status
	if err := winagent.Call(session, winagent.OpStatus, nil, &st); err != nil {
		return st, false
	}
	return st, true
}

// atScreen reports whether someone uses session now: unlocked, at the
// screen, and with input within idleLimit seconds.
func atScreen(s deskSession, idleLimit uint64) bool {
	if s.Locked || !s.Active {
		return false
	}
	st, ok := agentStatus(s.ID)
	return ok && uint64(st.IdleMillis)/1000 <= idleLimit
}

// SomeoneAtDesktop reports whether a person is at the screen.
func SomeoneAtDesktop(idleLimit uint64) bool {
	for _, s := range deskSessions() {
		if atScreen(s, idleLimit) {
			return true
		}
	}
	return false
}

// ForegroundAppsByUser is the app in front for each person at the screen.
func ForegroundAppsByUser(idleLimit uint64) []UserApp {
	var out []UserApp
	procs := map[uint32]*winProc(nil)
	for _, s := range deskSessions() {
		if !atScreen(s, idleLimit) {
			continue
		}
		var fg winagent.Foreground
		if winagent.Call(s.ID, winagent.OpForeground, nil, &fg) != nil || fg.PID == 0 {
			continue
		}
		if procs == nil {
			procs = processSnapshot()
		}
		// A window names its own process: believed only for a process of
		// the session's own user.
		if p := procs[fg.PID]; p != nil && p.UID == s.UID && p.Session == s.ID {
			out = append(out, UserApp{User: s.User, App: p.Name})
		}
	}
	return out
}

// NotifyDesktop shows a notification to every person signed in.
func NotifyDesktop(title, body string) {
	for _, s := range deskSessions() {
		if err := winagent.Call(s.ID, winagent.OpNotify, winagent.NotifyArgs{Title: title, Body: body}, nil); err != nil {
			log.Printf("notify: session %d: %v", s.ID, err)
		}
	}
}

// confirmSession is the session to ask in: the caller's, else the one at
// the screen.
func confirmSession() (uint32, bool) {
	if s := platform.CallerSession(); s > 0 {
		return uint32(s), true
	}
	for _, s := range deskSessions() {
		if s.Active {
			return s.ID, true
		}
	}
	return 0, false
}

// ConfirmDesktop asks a yes/no question on the caller's desktop.
func ConfirmDesktop(title, text, okLabel string) bool {
	session, ok := confirmSession()
	if !ok {
		return false
	}
	var yes bool
	err := winagent.Call(session, winagent.OpConfirm, winagent.ConfirmArgs{Title: title, Text: text + "\n\n(Yes: " + okLabel + ")"}, &yes)
	return err == nil && yes
}

// DesktopWarning says why desktop features can't work for the caller, or
// "".
func DesktopWarning() string {
	session, ok := confirmSession()
	if !ok {
		return ""
	}
	for _, s := range winagent.Sessions() {
		if s == session {
			return ""
		}
	}
	return "the Killer Chicken 9000 agent isn't running in your session yet: window titles, notifications and hotkeys start working when it is"
}

// LockSessions locks every person's screen.
func LockSessions() error {
	var firstErr error
	for _, s := range deskSessions() {
		if err := winagent.Call(s.ID, winagent.OpLock, nil, nil); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("lock session %d: %w", s.ID, err)
		}
	}
	return firstErr
}

// RelockUnlocked locks every screen that is unlocked (a lock freeze).
func RelockUnlocked() {
	for _, s := range deskSessions() {
		if !s.Locked {
			_ = winagent.Call(s.ID, winagent.OpLock, nil, nil)
		}
	}
}

// LogoutUsers signs every person out (consequences, logout freeze).
func LogoutUsers() error {
	var firstErr error
	for _, s := range deskSessions() {
		if r, _, err := procWTSLogoffSession.Call(0, uintptr(s.ID), 0); r == 0 && firstErr == nil {
			firstErr = fmt.Errorf("sign out session %d: %w", s.ID, err)
		}
	}
	return firstErr
}

// PowerOff shuts the computer down now.
func PowerOff() error {
	if err := enablePrivilege("SeShutdownPrivilege"); err != nil {
		return err
	}
	const ewxPowerOff, ewxForce = 0x8, 0x4
	const reasonPlanned = 0x80000000
	return windows.ExitWindowsEx(ewxPowerOff|ewxForce, reasonPlanned)
}

func enablePrivilege(name string) error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
		return err
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	return windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil)
}

// holdPath records a logout freeze's end: the service signs out every
// session that starts before it (HoldLogins).
var holdPath = filepath.Join(platform.DataDir, "hold-logins")

// HoldLogins refuses sign-ins until until: the service signs out every
// new session at once (Windows has no /run/nologin).
func HoldLogins(until time.Time) error {
	return os.WriteFile(holdPath, []byte(strconv.FormatInt(until.Unix(), 10)), 0o600)
}

// ReleaseLogins allows sign-ins again.
func ReleaseLogins() error {
	if err := os.Remove(holdPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// loginsHeld reports whether sign-ins are refused now.
func loginsHeld(now time.Time) bool {
	data, err := os.ReadFile(holdPath)
	if err != nil {
		return false
	}
	until, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return err == nil && now.Unix() < until
}

func init() {
	// In the service: no agent for a session while logins are held; the
	// session is signed out instead.
	winagent.SessionFilter = func(sessions []uint32) []uint32 {
		if !loginsHeld(time.Now()) {
			return sessions
		}
		for _, s := range sessions {
			procWTSLogoffSession.Call(0, uintptr(s), 0)
		}
		return nil
	}
	winagent.OnHotkey = runHotkey
}

// RunCustom carries out a custom action in the first person's session:
// opens (files, folders, web addresses) and runs (command lines), as that
// person.
func RunCustom(opens, runs []string) bool {
	sessions := deskSessions()
	if len(sessions) == 0 {
		return false
	}
	s := sessions[0]
	for _, target := range opens {
		_ = startInSession(s.ID, []string{filepath.Join(systemRoot(), "explorer.exe"), target})
	}
	for _, command := range runs {
		_ = startInSession(s.ID, []string{filepath.Join(systemRoot(), "System32", "cmd.exe"), "/c", command})
	}
	return true
}

// startInSession starts a program as session's user, on their desktop.
func startInSession(session uint32, argv []string) error {
	var token windows.Token
	if err := windows.WTSQueryUserToken(session, &token); err != nil {
		return err
	}
	defer token.Close()
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, token, false); err != nil {
		return err
	}
	defer windows.DestroyEnvironmentBlock(env)
	var quoted []string
	for _, a := range argv {
		quoted = append(quoted, windows.EscapeArg(a))
	}
	cmdline, _ := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	desktop, _ := windows.UTF16PtrFromString(`winsta0\default`)
	si := &windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})), Desktop: desktop}
	var pi windows.ProcessInformation
	if err := windows.CreateProcessAsUser(token, nil, cmdline, nil, nil, false, windows.CREATE_UNICODE_ENVIRONMENT, env, nil, si, &pi); err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}

// TitleGuard closes apps whose window titles contain blocked words (see
// the Linux one): windows are asked to close (WM_CLOSE through the agent),
// and a window still there after TitleCloseGrace has its program killed.
// Browsers are left to the extension, Windows' own programs alone.
type TitleGuard struct {
	mu      sync.Mutex
	asked   map[uintptr]time.Time
	watched map[uintptr]bool
}

// NewTitleGuard makes a guard.
func NewTitleGuard() *TitleGuard {
	return &TitleGuard{asked: map[uintptr]time.Time{}, watched: map[uintptr]bool{}}
}

// fileManagersWindows show folders in their titles.
var fileManagersWindows = map[string]bool{"explorer": true}

// EnforceUsers runs one cycle; see the Linux one.
func (g *TitleGuard) EnforceUsers(targets TitlesByUser, now time.Time) (int, []TitleHit) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if targets.Empty() {
		g.asked, g.watched = map[uintptr]time.Time{}, map[uintptr]bool{}
		return 0, nil
	}
	var hits []TitleHit
	seen := map[uintptr]bool{}
	kill := map[uint32]int64{}
	var procs map[uint32]*winProc
	for _, s := range deskSessions() {
		mine := targets.For(uint64(s.UID))
		if mine.Empty() {
			continue
		}
		var wins []winagent.Window
		if winagent.Call(s.ID, winagent.OpWindows, nil, &wins) != nil {
			continue
		}
		if procs == nil {
			procs = processSnapshot()
		}
		var closeNow []uintptr
		for _, w := range wins {
			p := procs[w.PID]
			// A window names its own process: believed only for the
			// session's own user's processes.
			if p == nil || p.UID != s.UID || p.Session != s.ID || systemProgram(p) && !fileManagersWindows[p.Name] {
				continue
			}
			keyword := matchingKeyword(w.Title, mine.Close)
			if keyword == "" && fileManagersWindows[p.Name] {
				keyword = folderWindowWin(w.Title, mine.Folders)
			}
			watched := ""
			if keyword == "" {
				watched = matchingKeyword(w.Title, mine.Watch)
			}
			if (keyword == "" && watched == "") || isBrowserProc(p) {
				continue
			}
			seen[w.HWND] = true
			if keyword == "" {
				if !g.watched[w.HWND] {
					g.watched[w.HWND] = true
					hits = append(hits, TitleHit{Keyword: watched, UID: uint64(s.UID), Watched: true})
				}
				continue
			}
			first, asked := g.asked[w.HWND]
			switch {
			case !asked:
				log.Printf("titles: asking %q (pid %d) to close: its title contains %q", w.Title, w.PID, keyword)
				closeNow = append(closeNow, w.HWND)
				g.asked[w.HWND] = now
				hits = append(hits, TitleHit{Keyword: keyword, UID: uint64(s.UID)})
			case now.Sub(first) >= TitleCloseGrace && !fileManagersWindows[p.Name]:
				log.Printf("titles: closing %s (pid %d): %q is still open", p.Name, p.PID, w.Title)
				kill[p.PID] = s.UID
			}
		}
		if len(closeNow) > 0 {
			_ = winagent.Call(s.ID, winagent.OpClose, winagent.CloseArgs{HWNDs: closeNow}, nil)
		}
	}
	for h := range g.asked {
		if !seen[h] {
			delete(g.asked, h)
		}
	}
	for h := range g.watched {
		if !seen[h] {
			delete(g.watched, h)
		}
	}
	killed := 0
	if len(kill) > 0 {
		children := childrenOf(procs)
		protected := protectedPIDs(procs)
		for root, uid := range kill {
			tree := treeOf(root, children)
			for i := len(tree) - 1; i >= 0; i-- {
				p := procs[tree[i]]
				if p == nil || protected[p.PID] || p.UID != uid {
					continue
				}
				protected[p.PID] = true
				if killProcess(p.PID) == nil {
					killed++
				}
			}
		}
	}
	return killed, hits
}

// folderWindowWin is the folder entry an Explorer window shows, by its
// title (the folder's name, or its path with "full path in title bar").
func folderWindowWin(title string, folders []string) string {
	lt := strings.ToLower(title)
	for _, f := range folders {
		path := strings.TrimRight(strings.ReplaceAll(f, "/", `\`), `\`)
		base := strings.ToLower(filepath.Base(path))
		if base == "" || base == "." {
			continue
		}
		if lt == base || strings.HasPrefix(lt, base+" ") || strings.Contains(lt, strings.ToLower(path)) {
			return f
		}
	}
	return ""
}

// askToCloseWin asks the windows of pids to close, in their sessions.
func askToCloseWin(pids []int, procs map[uint32]*winProc) {
	bySession := map[uint32][]uint32{}
	for _, pid := range pids {
		if p := procs[uint32(pid)]; p != nil && p.Session != 0 {
			bySession[p.Session] = append(bySession[p.Session], p.PID)
		}
	}
	for s, list := range bySession {
		var n int
		if err := winagent.Call(s, winagent.OpClose, winagent.CloseArgs{PIDs: list}, &n); err != nil || n == 0 {
			ints := make([]int, len(list))
			for i, p := range list {
				ints[i] = int(p)
			}
			polite.expire(ints) // no window asked: nothing to wait for
		}
	}
}

// hotkeysPath keeps the hotkeys the agents register.
var hotkeysPath = filepath.Join(platform.DataDir, "hotkeys.json")

// SyncHotkeys sets the key combinations every agent registers; a press
// runs `kc9k hotkey run <id>` for the person who pressed it.
func SyncHotkeys(keys []DesktopHotkey) error {
	var list []winagent.Hotkey
	for _, k := range keys {
		mods, vk, err := accelToVK(k.Accel)
		if err != nil {
			return fmt.Errorf("hotkey %d: %w", k.ID, err)
		}
		list = append(list, winagent.Hotkey{ID: k.ID, Mods: mods, Key: vk})
	}
	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	if err := atomicWriteFile(hotkeysPath, data, 0o600); err != nil {
		return err
	}
	var firstErr error
	for _, s := range winagent.Sessions() {
		if err := winagent.Call(s, winagent.OpHotkeys, list, nil); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// savedHotkeys are the hotkeys to give an agent that just connected.
func savedHotkeys() []winagent.Hotkey {
	var list []winagent.Hotkey
	if data, err := os.ReadFile(hotkeysPath); err == nil {
		_ = json.Unmarshal(data, &list)
	}
	return list
}

func init() {
	winagent.OnConnect = func(session uint32) {
		if keys := savedHotkeys(); len(keys) > 0 {
			_ = winagent.Call(session, winagent.OpHotkeys, keys, nil)
		}
	}
}

// accelToVK turns a GTK accelerator (<Primary><Alt>s, <Super>F9) into
// RegisterHotKey's modifiers and virtual key.
func accelToVK(accel string) (mods, vk uint32, err error) {
	const modAlt, modControl, modShift, modWin = 0x1, 0x2, 0x4, 0x8
	rest := accel
	for strings.HasPrefix(rest, "<") {
		end := strings.IndexByte(rest, '>')
		if end < 0 {
			return 0, 0, errors.New("bad accelerator " + accel)
		}
		switch rest[1:end] {
		case "Primary", "Control", "Ctrl":
			mods |= modControl
		case "Alt":
			mods |= modAlt
		case "Shift":
			mods |= modShift
		case "Super":
			mods |= modWin
		}
		rest = rest[end+1:]
	}
	switch {
	case len(rest) == 1 && rest[0] >= 'a' && rest[0] <= 'z':
		vk = uint32(rest[0] - 'a' + 'A')
	case len(rest) == 1 && rest[0] >= 'A' && rest[0] <= 'Z', len(rest) == 1 && rest[0] >= '0' && rest[0] <= '9':
		vk = uint32(rest[0])
	case len(rest) >= 2 && rest[0] == 'F':
		n, err := strconv.Atoi(rest[1:])
		if err != nil || n < 1 || n > 24 {
			return 0, 0, errors.New("bad key " + rest)
		}
		vk = 0x70 + uint32(n-1)
	default:
		names := map[string]uint32{"space": 0x20, "Return": 0x0D, "Escape": 0x1B, "Tab": 0x09, "Home": 0x24,
			"End": 0x23, "Insert": 0x2D, "Delete": 0x2E, "Page_Up": 0x21, "Page_Down": 0x22}
		v, ok := names[rest]
		if !ok {
			return 0, 0, errors.New("key " + rest + " isn't available on Windows")
		}
		vk = v
	}
	return mods, vk, nil
}

// runHotkey runs `kc9k hotkey run <id>` for the person in session, as the
// service runs any command for a caller.
func runHotkey(session uint32, id int) {
	var who *deskSession
	for _, s := range deskSessions() {
		if s.ID == session {
			s := s
			who = &s
		}
	}
	if who == nil {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, "hotkey", "run", strconv.Itoa(id))
	cmd.Env = append(os.Environ(),
		platform.EnvCallerUID+"="+strconv.FormatInt(who.UID, 10),
		platform.EnvCallerUser+"="+who.User,
		platform.EnvCallerSession+"="+strconv.FormatUint(uint64(session), 10))
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("hotkey %d: %v: %s", id, err, strings.TrimSpace(string(out)))
	}
}
