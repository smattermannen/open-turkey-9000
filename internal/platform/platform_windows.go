package platform

// On Windows the CLI forwards each command to the service (LocalSystem)
// over the named pipe; the service runs it as a child of its own and tells
// it who called in KC9K_CALLER_* (taken from the pipe client's token, as
// the socket's uid is on Linux). A user's account ID ("uid") is its
// relative ID, the last part of its SID: unique among the computer's
// local accounts, 1000 and up for people, like Linux uids (SPEC §6
// 2026-11-21, By Opus).

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

// DataDir holds the database and KC9K's state: SYSTEM's only.
var DataDir = filepath.Join(programData(), "kc9k")

// ShareDir holds files browsers read (the packed extension).
var ShareDir = filepath.Join(programFiles(), "KC9K", "share")

// InstallDir holds kc9k.exe.
var InstallDir = filepath.Join(programFiles(), "KC9K")

func programData() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return d
	}
	return `C:\ProgramData`
}

func programFiles() string {
	if d := os.Getenv("ProgramFiles"); d != "" {
		return d
	}
	return `C:\Program Files`
}

// Caller environment, set only by the service for the commands it runs.
const (
	EnvCallerSID     = "KC9K_CALLER_SID"
	EnvCallerUID     = "KC9K_CALLER_UID"
	EnvCallerUser    = "KC9K_CALLER_USER"
	EnvCallerSession = "KC9K_CALLER_SESSION"
	EnvCallerToken   = "KC9K_CALLER_TOKEN"
	EnvTerminal      = "KC9K_TERMINAL"
)

// Privileged reports whether this process may write the layers: it runs
// as LocalSystem (the service, or a command it runs).
func Privileged() bool { return IsSystem(windows.GetCurrentProcessToken()) }

// IsSystem reports whether token is LocalSystem's.
func IsSystem(token windows.Token) bool {
	u, err := token.GetTokenUser()
	if err != nil {
		return false
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	return err == nil && u.User.Sid.Equals(system)
}

// RID is the last sub-authority of sid: the account's ID on this computer.
func RID(sid *windows.SID) int64 {
	n := sid.SubAuthorityCount()
	if n == 0 {
		return -1
	}
	return int64(sid.SubAuthority(uint32(n - 1)))
}

// AccountName is sid's account name without its domain or computer.
func AccountName(sid *windows.SID) string {
	account, _, _, err := sid.LookupAccount("")
	if err != nil {
		return sid.String()
	}
	return account
}

// CallerUID is the account a command runs for: the caller the service
// named, or this process's own account; -1 for SYSTEM itself (every
// account).
func CallerUID() int64 {
	if Privileged() {
		if uid, err := strconv.ParseInt(os.Getenv(EnvCallerUID), 10, 64); err == nil && uid > 0 {
			return uid
		}
		return -1
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return -1
	}
	return RID(u.User.Sid)
}

// CallerName is the name of the account a command runs for ("" for
// SYSTEM itself).
func CallerName() string {
	if Privileged() {
		return os.Getenv(EnvCallerUser)
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ""
	}
	return AccountName(u.User.Sid)
}

// CallerSession is the Windows session the caller's console is in, or -1.
func CallerSession() int {
	if s, err := strconv.Atoi(os.Getenv(EnvCallerSession)); err == nil && Privileged() {
		return s
	}
	return -1
}

// ForwardedTerminal is the caller's console, when the service runs a
// command typed in one: its size and the nonce of its control sequences.
func ForwardedTerminal() (cols, rows int, nonce string, ok bool) {
	if !Privileged() {
		return 0, 0, "", false
	}
	parts := strings.Split(os.Getenv(EnvTerminal), ",")
	if len(parts) != 3 {
		return 0, 0, "", false
	}
	c, err1 := strconv.Atoi(parts[0])
	r, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || parts[2] == "" {
		return 0, 0, "", false
	}
	return c, r, parts[2], true
}
