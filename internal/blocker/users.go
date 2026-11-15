// users.go — local accounts, for blocks that apply to some users only
// (§3.2 affected users).
package blocker

import (
	"os"
	"os/user"
	"strconv"
	"strings"
)

// RegularUsers lists the computer's people accounts from /etc/passwd:
// regular user IDs with a login shell.
func RegularUsers() []string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil
	}
	return parsePasswdUsers(string(data))
}

func parsePasswdUsers(passwd string) []string {
	var out []string
	for _, line := range strings.Split(passwd, "\n") {
		f := strings.Split(line, ":")
		if len(f) < 7 {
			continue
		}
		uid, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil || !regularUser(uid) {
			continue
		}
		if shell := f[6]; strings.HasSuffix(shell, "/nologin") || strings.HasSuffix(shell, "/false") {
			continue
		}
		out = append(out, f[0])
	}
	return out
}

// UserIDs maps account names to user IDs, leaving out unknown ones.
func UserIDs(names []string) []uint64 {
	var out []uint64
	for _, n := range names {
		if u, err := user.Lookup(n); err == nil {
			if id, err := strconv.ParseUint(u.Uid, 10, 32); err == nil {
				out = append(out, id)
			}
		}
	}
	return out
}

// ownedByAny reports whether pid runs as one of uids (all regular users
// when uids is nil).
func ownedByAny(pid int, uids []uint64) bool {
	if uids == nil {
		return ownedByUser(pid)
	}
	for _, uid := range uids {
		if ownedBy(pid, uid) {
			return true
		}
	}
	return false
}
