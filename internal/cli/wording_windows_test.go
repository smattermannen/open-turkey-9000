package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// W16: no help text names Linux's tools or paths on Windows.
func TestWindowsWording(t *testing.T) {
	rewordForWindows(rootCmd)
	bad := []string{"sudo", "GRUB", "/etc/", "systemd", "journalctl", "kc9k.nofreeze", "/home/", "pkexec",
		"LightDM", "Cinnamon", "htop", "NetworkManager", "daemon", "kc9k-bin", "~/", " root ", "root,", "nmcli",
		"X11", ".deb", "apt ", "/var/", "/usr/"}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Name() == "completion" {
			return // cobra's own, for Unix shells
		}
		for _, b := range bad {
			for _, text := range []string{c.Short, c.Long, c.Example} {
				if i := strings.Index(text, b); i >= 0 {
					t.Errorf("%s: %q in …%s…", c.CommandPath(), b, text[max(0, i-40):min(len(text), i+40)])
				}
			}
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(rootCmd)
}
