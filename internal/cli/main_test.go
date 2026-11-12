package cli

import (
	"fmt"
	"os"
	"testing"
)

// TestMain refuses to run as root. Root always uses the real database
// (db.ResolveDBPath ignores KC9K_DB for root), so these tests would create
// blocks and set access passwords in /var/lib/kc9k/kc9k.db.
func TestMain(m *testing.M) {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "internal/cli tests must run as an ordinary user: as root they would use the real database at /var/lib/kc9k/kc9k.db")
		os.Exit(1)
	}
	os.Exit(m.Run())
}
