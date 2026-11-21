package blocker

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// hostsPath is Windows' hosts file (System32\drivers\etc\hosts); a
// variable for tests.
var hostsPath = filepath.Join(systemRoot(), "System32", "drivers", "etc", "hosts")

// systemRoot is the Windows folder (normally C:\Windows).
func systemRoot() string {
	if r := os.Getenv("SystemRoot"); r != "" {
		return r
	}
	return `C:\Windows`
}

var procDnsFlushResolverCache = windows.NewLazySystemDLL("dnsapi.dll").NewProc("DnsFlushResolverCache")

// hostsWritten empties the DNS Client's cache, which keeps answers it read
// from the hosts file (as ipconfig /flushdns does).
func hostsWritten() {
	if procDnsFlushResolverCache.Find() == nil {
		procDnsFlushResolverCache.Call()
	}
}
