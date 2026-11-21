//go:build linux

package blocker

import "os"

// resolvConfPaths are read, in order, for the network's DNS servers: the
// upstream list of systemd-resolved first, then the system's own.
var resolvConfPaths = []string{"/run/systemd/resolve/resolv.conf", "/etc/resolv.conf"}

// networkDNS reads the network's DNS servers from the first resolv.conf that
// names any besides this computer's own (a local stub reads /etc/hosts).
func networkDNS() []string {
	for _, path := range resolvConfPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if servers := parseResolvConf(string(data)); len(servers) > 0 {
			return servers
		}
	}
	return nil
}
