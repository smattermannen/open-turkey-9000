package blocker

import (
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// networkDNS returns the DNS servers of the network adapters that are up,
// leaving out loopback addresses (GetAdaptersAddresses).
func networkDNS() []string {
	size := uint32(15000)
	for range 3 {
		buf := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST, 0, first, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil
		}
		var servers []string
		seen := map[string]bool{}
		for a := first; a != nil; a = a.Next {
			if a.OperStatus != windows.IfOperStatusUp {
				continue
			}
			for d := a.FirstDnsServerAddress; d != nil; d = d.Next {
				ip := d.Address.IP()
				if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || seen[ip.String()] {
					continue
				}
				// Site-local IPv6 defaults (fec0:0:0:ffff::1-3) are
				// placeholders Windows shows without a real server.
				if ip.To4() == nil && ip.Equal(net.ParseIP("fec0:0:0:ffff::1")) {
					continue
				}
				seen[ip.String()] = true
				servers = append(servers, ip.String())
			}
		}
		return servers
	}
	return nil
}
