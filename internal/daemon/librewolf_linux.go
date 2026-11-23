//go:build linux

package daemon

import (
	"log"
	"os"
	"path/filepath"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// installLibreWolf installs LibreWolf in the background if it is missing,
// once (§4.11: the .deb's postinst can't run apt; see
// blocker.AutoInstallLibreWolf). A database used through KC9K_DB is a
// test's: it never installs anything.
func installLibreWolf(stop <-chan struct{}) {
	if os.Getenv("KC9K_DB") != "" {
		return
	}
	dir := filepath.Dir(db.ResolveDBPath())
	go blocker.AutoInstallLibreWolf(dir, stop, log.Printf)
}
