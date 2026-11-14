// monk.go — monk mode (R-MONK-1, R-MONK-2). It is a mode, not a block: all
// that is stored is when it ends (db.SettingMonkUntil, trusted time) and
// what the network looked like before (db.SettingMonkNet). While it runs:
//
//   - every website is blocked: monkBlock, a fixed set of lists that exists
//     only here, is enforced beside the active blocks (withMonk);
//   - the computer's connection is cut three ways: Wi-Fi radio off, every
//     Ethernet interface down, the firewall dropping all but loopback. The
//     daemon puts back whatever is switched on again, every cycle.
//
// It ends only when its time does. The network is then put back as it was,
// also after a restart or a crash (the saved state is in the database).
package enforce

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
)

// MonkName stands where a block's name would, in the extension's rules and
// on the block page. No block has it: it isn't looked up anywhere.
const MonkName = "Monk mode"

// MonkListID is the id of monk mode's blacklist in the extension's rules.
// Lists in the database count up from 1 and phrase sets are negative.
const MonkListID = 1 << 30

// What monk mode does to the network. Tests replace these, so they never
// cut the connection of the machine running them.
var (
	wifiRadio      = blocker.WifiRadio
	setWifiRadio   = blocker.SetWifiRadio
	ethernetLinks  = blocker.EthernetLinks
	setLink        = blocker.SetLink
	firewallIsCut  = blocker.IsFirewallCut
	cutFirewall    = func() error { return blocker.ApplyFirewallCut(nil, true) }
	monkDeviceless = func() bool { return os.Getenv("KC9K_DB") != "" }
)

// monkMu keeps the cut and the restore from running into each other (the
// daemon cycle and the rules server both start monk mode).
var monkMu sync.Mutex

// monkBlock is what monk mode blocks, in the shape of an active block so
// every layer treats it like one: all websites, local files spared. It is
// built here each time and never stored, so nothing can change it.
func monkBlock() db.ActiveBlockDetail {
	return db.ActiveBlockDetail{BlockName: MonkName, Lists: db.Lists{
		{ID: MonkListID, Kind: db.ListBlacklist, Name: MonkName, Where: db.Where{URL: true},
			Entries: []string{"*.*"}, Actions: []db.ListAction{{Name: db.ActionBlock, Distinct: 1, Hits: 1}}},
		{ID: MonkListID + 1, Kind: db.ListExceptions, Name: db.DefaultExceptionsList, Where: db.Where{URL: true},
			Entries: []string{db.FileException}},
	}}
}

// MonkEnd returns the trusted time monk mode ends, 0 when it is off. With
// trusted 0 (the clock couldn't be read) a monk mode that was started counts
// as on.
func MonkEnd(d *db.DB, trusted int64) int64 {
	until, err := d.MonkUntil()
	if err != nil {
		log.Printf("monk mode: %v", err)
		return 0
	}
	if until > trusted {
		return until
	}
	return 0
}

// MonkOn reports whether monk mode is running.
func MonkOn(d *db.DB, trusted int64) bool { return MonkEnd(d, trusted) > 0 }

// withMonk adds what monk mode blocks to the blocks being enforced.
func withMonk(d *db.DB, active []db.ActiveBlockDetail, trusted int64) []db.ActiveBlockDetail {
	if !MonkOn(d, trusted) {
		return active
	}
	return append(append([]db.ActiveBlockDetail{}, active...), monkBlock())
}

// StartMonk turns monk mode on for a while, or extends it; a shorter time
// than it has left changes nothing. The connection is cut at once.
func StartMonk(d *db.DB, length time.Duration) error {
	if length <= 0 || length > time.Duration(db.MaxMonkMinutes)*time.Minute {
		return fmt.Errorf("monk mode lasts from 1 minute to %d days", db.MaxMonkMinutes/(24*60))
	}
	trusted, err := d.TrustedNow(time.Now())
	if err != nil {
		return err
	}
	if err := d.ExtendMonk(trusted + int64(length/time.Second)); err != nil {
		return err
	}
	// A database used through KC9K_DB is a test's: it never reaches the
	// real network.
	if !monkDeviceless() {
		cutNetwork(d)
	}
	return nil
}

// enforceMonk is monk mode's part of the daemon cycle, and reports whether
// it is on. On: the three cuts are held. Over: it is switched off and the
// network put back. Off with a saved network state (a crash, or a restart
// that outlasted it): the network is put back.
func enforceMonk(d *db.DB, trusted int64) bool {
	until, err := d.MonkUntil()
	if err != nil {
		log.Printf("monk mode: %v", err)
		return false
	}
	// trusted 0: the clock couldn't be read, so nothing counts as over.
	if until > 0 && (trusted == 0 || trusted < until) {
		cutNetwork(d)
		return true
	}
	if until > 0 {
		ended, err := d.EndMonkIfOver(trusted)
		if err != nil {
			log.Printf("monk mode: %v", err)
			return true
		}
		if !ended { // extended this very moment
			return true
		}
		log.Println("monk mode: its time is over")
	}
	restoreNetwork(d)
	return false
}

// cutNetwork cuts the connection the three ways, saving how it was first.
// Anything already cut is left alone, so calling it every cycle only acts
// on what was switched back on.
func cutNetwork(d *db.DB) {
	monkMu.Lock()
	defer monkMu.Unlock()
	wifi, wifiKnown := wifiRadio()
	links := ethernetLinks()
	first, err := d.SaveMonkNetIfNone(db.MonkNet{Wifi: wifi && wifiKnown, Ethernet: links})
	if err != nil {
		// Without the state there is no way back: only the firewall, which
		// has nothing to restore, is cut.
		log.Printf("monk mode: the network state couldn't be saved, leaving Wi-Fi and Ethernet as they are: %v", err)
	} else {
		what := "was switched back on; off again"
		if first {
			what = "off"
		}
		if wifi && wifiKnown {
			log.Printf("monk mode: Wi-Fi radio %s", what)
			if err := setWifiRadio(false); err != nil {
				log.Printf("monk mode: %v", err)
			}
		}
		what = "was brought back up; down again"
		if first {
			what = "down"
		}
		for _, name := range sortedLinks(links) {
			if !links[name] {
				continue
			}
			log.Printf("monk mode: Ethernet interface %s %s", name, what)
			if err := setLink(name, false); err != nil {
				log.Printf("monk mode: %v", err)
			}
		}
	}
	if !firewallIsCut() {
		log.Println("monk mode: firewall drops everything except loopback")
		if err := cutFirewall(); err != nil {
			log.Printf("monk mode: %v", err)
		}
	}
}

// restoreNetwork puts Wi-Fi and Ethernet back as they were before the cut,
// if a state is saved: what was on comes back on, what was off stays off.
// An interface that wasn't there then (plugged in since) is brought up, as
// it would have been. The firewall is the daemon cycle's to reopen. The
// state is forgotten only when everything worked; otherwise the next cycle
// tries again.
func restoreNetwork(d *db.DB) {
	monkMu.Lock()
	defer monkMu.Unlock()
	saved, err := d.MonkNet()
	if err != nil {
		// Unreadable: nothing to go by, so everything comes back on.
		log.Printf("monk mode: %v; switching Wi-Fi and Ethernet on", err)
		saved = &db.MonkNet{Wifi: true, Ethernet: map[string]bool{}}
	}
	if saved == nil {
		return
	}
	ok := true
	if on, known := wifiRadio(); saved.Wifi && known && !on {
		log.Println("monk mode: Wi-Fi radio back on")
		if err := setWifiRadio(true); err != nil {
			log.Printf("monk mode: %v", err)
			ok = false
		}
	}
	links := ethernetLinks()
	for _, name := range sortedLinks(links) {
		was, known := saved.Ethernet[name]
		if links[name] || (known && !was) {
			continue
		}
		log.Printf("monk mode: Ethernet interface %s back up", name)
		if err := setLink(name, true); err != nil {
			log.Printf("monk mode: %v", err)
			ok = false
		}
	}
	if !ok {
		return
	}
	if forgotten, err := d.ForgetMonkNetIfOff(); err != nil {
		log.Printf("monk mode: %v", err)
	} else if forgotten {
		log.Println("monk mode: the network is as it was before")
		firewallRefresh = time.Now().Add(firewallRefreshAfter)
	}
}

// firewallRefreshAfter is how long after the network is restored the
// firewall looks up the blocked sites' addresses again: right after, there
// is no connection yet to ask a DNS server over.
const firewallRefreshAfter = 45 * time.Second

// firewallRefresh is when that is due; zero when it isn't.
var firewallRefresh time.Time

func sortedLinks(links map[string]bool) []string {
	names := make([]string, 0, len(links))
	for name := range links {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
