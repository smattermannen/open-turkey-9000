// version.go — `kc9k version`: which build this is (W43, §4.11), so the
// Windows GUI's About and a person can tell what is installed. Version is
// set at build time:
//
//	go build -ldflags "-X github.com/smattermannen/kc9k/internal/cli.Version=1.2.3" ./cmd/kc9k
package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is this build's version; "dev" for a build without one.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show which version of Killer Chicken 9000 this is",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Printf("kc9k %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	accessExempt = append(accessExempt, "version")
}
