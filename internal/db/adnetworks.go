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

// AdIndustries are ad networks that serve mostly one industry's ads.
var AdIndustries = map[string][]string{
	"adult": {"exoclick.com", "exosrv.com", "trafficjunky.net", "trafficjunky.com", "juicyads.com",
		"ero-advertising.com", "adxpansion.com", "plugrush.com", "trafficstars.com", "tsyndicate.com",
		"trafficfactory.biz", "adspyglass.com", "crakrevenue.com", "realsrv.com", "magsrv.com",
		"twinrdsrv.com", "adsco.re", "clickadu.com", "hilltopads.net", "tubecorporate.com"},
	"gambling": {"income-access.com", "netrefer.com", "myaffiliates.com", "affiliatehub.com",
		"cellxpert.com", "referon.com", "raventrack.com", "quintessence.media", "gamblingads.com",
		"betpartners.com", "playtech.com", "gambling-affiliation.com"},
	"crypto": {"coinzilla.com", "coinzilla.io", "bitmedia.io", "a-ads.com", "cointraffic.io",
		"adshares.net", "adex.network"},
	"dating": {"datingpartners.com", "adultfriendfinder.com", "sexsearchcom.com", "datingfactory.net"},
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
