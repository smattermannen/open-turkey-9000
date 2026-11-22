package blocker

import "strings"

// fetchTools are programs that download or play what an address points to.
// One of them given a blocked site's address is a way around the browser.
var fetchTools = map[string]bool{
	"yt-dlp": true, "youtube-dl": true, "yt-dlp_linux": true, "gallery-dl": true, "streamlink": true,
	"mpv": true, "vlc": true, "celluloid": true, "smplayer": true, "mplayer": true, "ffplay": true,
	"ffmpeg": true, "totem": true, "haruna": true, "curl": true, "wget": true, "wget2": true,
	"aria2c": true, "axel": true, "http": true, "https": true, "xh": true, "lynx": true, "w3m": true,
	"links": true, "links2": true, "elinks": true, "browsh": true, "freetube": true,
	// Windows' own names (W12).
	"yt-dlp_x86": true, "yt-dlp_win": true, "yt-dlp_min": true, "mpc-hc": true, "mpc-hc64": true,
	"mpc-be": true, "mpc-be64": true, "potplayer": true, "potplayermini": true, "potplayermini64": true,
	"wmplayer": true, "bitsadmin": true, "certutil": true, "streamlink-twitch-gui": true,
}

// namesDomain reports whether an argument names a domain (or a subdomain of
// it) as a host: at the start, after "//", "@", "." or "=", and followed by
// the end, "/", ":", "?" or "#". The domain inside a longer word doesn't count.
func namesDomain(arg, domain string) bool {
	arg = strings.ToLower(arg)
	for from := 0; ; {
		i := strings.Index(arg[from:], domain)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(domain)
		before := i == 0 || strings.ContainsRune("/@.=", rune(arg[i-1]))
		after := end == len(arg) || strings.ContainsRune("/:?#", rune(arg[end]))
		if before && after {
			return true
		}
		from = i + 1
	}
}
