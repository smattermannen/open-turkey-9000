// adnetworks.go — the ad trigger (§3.9): a list that looks at "ads"
// matches a page that loads one of its ad networks. Its entries are ad
// network domains, or industry names standing for the networks shipped
// here (AdIndustries).
package db

import (
	"slices"
	"strings"

	"github.com/smattermannen/kc9k/internal/blocker"
)

// AdIndustries are ad networks that serve mostly one industry's ads: the
// domains their ad requests go to (a page's requests to a subdomain count).
// Checked 2026-11-21: each is named as an ad host by EasyList, AdGuard DNS,
// Peter Lowe's or StevenBlack's list, or was seen in a real page's requests
// (coinzilla.com); those seen live are in TestAdIndustriesSeenLive. Left out:
// affiliate platforms whose tracking runs on the operator's own domains
// (Income Access, Cellxpert, Referon, RavenTrack, CrakRevenue), a game maker
// (Playtech), and domains no list names and no page loaded.
var AdIndustries = map[string][]string{
	"adult": {"exoclick.com", "exosrv.com", "trafficjunky.net", "trafficjunky.com", "juicyads.com",
		"ero-advertising.com", "adxpansion.com", "plugrush.com", "trafficstars.com", "tsyndicate.com",
		"trafficfactory.biz", "adspyglass.com", "realsrv.com", "magsrv.com", "pemsrv.com", "orbsrv.com",
		"twinrdsrv.com", "twinrdengine.com", "adsco.re", "clickadu.com", "hilltopads.net", "tubecorporate.com",
		"traffichaus.com", "trafficshop.com", "clickadilla.com", "sexad.net", "adspector.io"},
	"gambling": {"netrefer.com", "myaffiliates.com", "gambling-affiliation.com"},
	"crypto": {"coinzilla.com", "coinzilla.io", "coinzillatag.com", "czilladx.com", "bitmedia.io", "a-ads.com",
		"cointraffic.io", "adshares.net", "mellowads.com", "blockchain-ads.com", "hypelab.com"},
	"dating": {"adultfriendfinder.com", "sexsearchcom.com"},
}

// AdIndustryNames lists the industries, sorted.
func AdIndustryNames() []string {
	out := make([]string, 0, len(AdIndustries))
	for name := range AdIndustries {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// AdNetworks returns the ad network domains a list stands for when it
// looks at ads: its domain entries, and its industry names' networks.
func (l WebList) AdNetworks() []string {
	if !l.Where.Ads {
		return nil
	}
	var out []string
	for _, e := range l.Entries {
		if networks, ok := AdIndustries[strings.ToLower(e)]; ok {
			out = append(out, networks...)
		} else if blocker.IsValidDomain(blocker.NormalizeDomain(e)) {
			out = append(out, blocker.NormalizeDomain(e))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
