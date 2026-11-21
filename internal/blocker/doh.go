package blocker

// dohServersIPv4 and dohServersIPv6 hold the addresses of well-known
// DNS-over-HTTPS resolvers. We drop TCP port 443 to these so browsers can't
// use them to resolve names and bypass /etc/hosts. We do NOT touch port 53, so
// ordinary system DNS keeps working for sites that are not blocked.
var dohServersIPv4 = []string{
	"1.1.1.1", "1.0.0.1", // Cloudflare
	"8.8.8.8", "8.8.4.4", // Google
	"9.9.9.9", "149.112.112.112", // Quad9
	"208.67.222.222", "208.67.220.220", // OpenDNS
	"94.140.14.14", "94.140.15.15", // AdGuard
	"185.228.168.9", "185.228.169.9", // CleanBrowsing
	"76.76.2.0", "76.76.10.0", // Control D
	"194.242.2.2", // Mullvad
}

var dohServersIPv6 = []string{
	"2606:4700:4700::1111", "2606:4700:4700::1001", // Cloudflare
	"2001:4860:4860::8888", "2001:4860:4860::8844", // Google
	"2620:fe::fe", "2620:fe::9", // Quad9
	"2620:119:35::35", "2620:119:53::53", // OpenDNS
	"2a10:50c0::ad1:ff", "2a10:50c0::ad2:ff", // AdGuard
}
