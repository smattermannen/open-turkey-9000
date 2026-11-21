package platform

import (
	"os"
	"os/user"
	"strconv"
)

// DataDir holds the database and KC9K's state; root's only.
var DataDir = "/var/lib/kc9k"

// ShareDir holds files browsers read (the packed extension).
var ShareDir = "/usr/share/kc9k"

// Privileged reports whether this process may write the layers: root.
func Privileged() bool { return os.Geteuid() == 0 }

// CallerUID is the account a command runs for: the one sudo was given by,
// or the user running it; -1 for root itself (every account).
func CallerUID() int64 {
	if os.Geteuid() == 0 {
		if uid, err := strconv.ParseInt(os.Getenv("SUDO_UID"), 10, 64); err == nil && uid > 0 {
			return uid
		}
		return -1
	}
	return int64(os.Getuid())
}

// CallerName is the name of the account a command runs for ("" for root
// itself).
func CallerName() string {
	if os.Geteuid() == 0 {
		return os.Getenv("SUDO_USER")
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}
