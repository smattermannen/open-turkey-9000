// Package platform answers the few questions whose answer differs between
// Linux and Windows for the whole program: where KC9K keeps its data, who
// runs a command and for whom (notes/windows.md).
package platform

import "path/filepath"

// DBPath is the database's path.
func DBPath() string { return filepath.Join(DataDir, "kc9k.db") }
