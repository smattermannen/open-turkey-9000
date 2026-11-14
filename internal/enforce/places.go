// places.go — location by Wi-Fi: a block with a place rule is on while the
// computer is at that place.
//
// A place is known by the access points in range when it was saved (your
// own and the neighbours'). The computer is there when enough of them are
// in range again. When nothing at all can be seen (Wi-Fi switched off, no
// adapter), it counts as being at every place: switching Wi-Fi off must not
// be a way out. With place rules set, a switched-off Wi-Fi radio is
// switched back on.
package enforce

import (
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// ScanWifi returns the access points in range. rescan asks the adapter to
// look again first (slower). It is a variable so tests can stand in for it.
var ScanWifi = func(rescan bool) ([]db.Network, error) {
	mode := "no"
	if rescan {
		mode = "yes"
	}
	out, err := exec.Command("nmcli", "-t", "-f", "BSSID,SSID", "dev", "wifi", "list", "--rescan", mode).Output()
	if err != nil {
		return nil, err
	}
	return parseNmcli(string(out)), nil
}

// parseNmcli reads nmcli's terse output: fields split by ":", with the
// colons inside a field written "\:".
func parseNmcli(out string) []db.Network {
	var nets []db.Network
	for _, line := range strings.Split(out, "\n") {
		line = strings.ReplaceAll(line, `\:`, "\x00")
		bssid, ssid, ok := strings.Cut(line, ":")
		bssid = strings.ToUpper(strings.ReplaceAll(bssid, "\x00", ":"))
		if !ok || len(bssid) != 17 {
			continue
		}
		nets = append(nets, db.Network{BSSID: bssid, SSID: strings.ReplaceAll(ssid, "\x00", ":")})
	}
	return nets
}

// AtPlace reports whether the networks in range say the computer is at a
// place: two of its networks are seen (one, if it only knows one). Seeing
// nothing at all counts as being there.
func AtPlace(p db.Place, seen []db.Network) bool {
	if len(seen) == 0 {
		return true
	}
	have := map[string]bool{}
	for _, n := range seen {
		have[strings.ToUpper(n.BSSID)] = true
	}
	matches := 0
	for _, n := range p.Networks {
		if have[strings.ToUpper(n.BSSID)] {
			matches++
		}
	}
	return matches >= min(2, len(p.Networks)) && matches > 0
}

var (
	placeMu     sync.Mutex
	placeSeen   []db.Network
	placeAt     time.Time
	placeRescan time.Time
)

// seenNetworks returns the networks in range, read at most every 10
// seconds, asking for a fresh look every minute.
func seenNetworks(now time.Time) []db.Network {
	placeMu.Lock()
	defer placeMu.Unlock()
	if now.Sub(placeAt) < 10*time.Second && !placeAt.IsZero() {
		return placeSeen
	}
	if now.Sub(placeRescan) > time.Minute {
		placeRescan = now
		go func() { _ = exec.Command("nmcli", "dev", "wifi", "rescan").Run() }()
	}
	seen, err := ScanWifi(false)
	if err != nil {
		log.Printf("places: %v", err)
		seen = nil
	}
	placeSeen, placeAt = seen, now
	return seen
}

// PlaceBlocks returns the blocks that a place rule turns on right now.
func PlaceBlocks(d *db.DB, now time.Time) (map[string]bool, error) {
	places, err := d.Places()
	if err != nil {
		return nil, err
	}
	on := map[string]bool{}
	ruled := false
	for _, p := range places {
		ruled = ruled || len(p.Blocks) > 0
	}
	if !ruled {
		return on, nil
	}
	seen := seenNetworks(now)
	for _, p := range places {
		if len(p.Blocks) > 0 && AtPlace(p, seen) {
			for _, b := range p.Blocks {
				on[b] = true
			}
		}
	}
	return on, nil
}

// PlaceRuled returns the blocks that have a place rule at all.
func PlaceRuled(d *db.DB) (map[string]bool, error) {
	places, err := d.Places()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, p := range places {
		for _, b := range p.Blocks {
			out[b] = true
		}
	}
	return out, nil
}

// keepWifiOn switches the Wi-Fi radio back on when place rules exist.
func keepWifiOn(d *db.DB) {
	ruled, err := PlaceRuled(d)
	if err != nil || len(ruled) == 0 {
		return
	}
	// Monk mode switches the radio off (R-MONK-2) and puts it back itself.
	if until, err := d.MonkUntil(); err != nil || until > 0 {
		return
	}
	out, err := exec.Command("nmcli", "radio", "wifi").Output()
	if err == nil && strings.TrimSpace(string(out)) == "disabled" {
		log.Println("places: Wi-Fi was switched off; switching it on (a block depends on where the computer is)")
		_ = exec.Command("nmcli", "radio", "wifi", "on").Run()
	}
}
