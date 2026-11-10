// templates.go — ready-made site lists for quick block setup.
//
// "kc9k block create focus --template social-media" seeds a new block
// with a curated list instead of making the user type every domain. Templates
// are intentionally small and mainstream; users extend them with add-site.
package cli

import (
	"fmt"
	"sort"
	"strings"
)

// templates maps a template name to its seed domains.
var templates = map[string][]string{
	"social-media": {
		"facebook.com", "instagram.com", "x.com", "twitter.com", "tiktok.com",
		"reddit.com", "snapchat.com", "linkedin.com", "pinterest.com",
		"tumblr.com", "threads.net", "mastodon.social",
	},
	"news": {
		"cnn.com", "bbc.com", "nytimes.com", "theguardian.com", "foxnews.com",
		"reuters.com", "apnews.com", "washingtonpost.com", "news.google.com",
	},
	"video": {
		"youtube.com", "netflix.com", "twitch.tv", "hulu.com", "disneyplus.com",
		"primevideo.com", "vimeo.com", "dailymotion.com",
	},
	"shopping": {
		"amazon.com", "ebay.com", "etsy.com", "aliexpress.com", "walmart.com",
		"target.com", "temu.com", "shein.com",
	},
	"games": {
		"store.steampowered.com", "steamcommunity.com", "epicgames.com", "roblox.com", "twitch.tv",
		"itch.io", "gog.com", "battle.net", "ea.com", "minecraft.net", "chess.com", "lichess.org",
		"poki.com", "crazygames.com", "miniclip.com", "ign.com", "gamespot.com",
	},
	"gambling": {
		"bet365.com", "betfair.com", "pokerstars.com", "draftkings.com", "fanduel.com",
		"williamhill.com", "888casino.com", "unibet.com", "bwin.com", "stake.com",
		"betway.com", "leovegas.com", "polymarket.com",
	},
	"forums": {
		"reddit.com", "quora.com", "news.ycombinator.com", "4chan.org", "9gag.com",
		"discord.com", "stackexchange.com", "lemmy.world", "tumblr.com",
	},
	"streaming": {
		"netflix.com", "disneyplus.com", "primevideo.com", "max.com", "hulu.com",
		"twitch.tv", "kick.com", "crunchyroll.com", "viaplay.com", "svtplay.se",
		"paramountplus.com", "tv.apple.com",
	},
	"music": {
		"spotify.com", "music.youtube.com", "soundcloud.com", "music.apple.com",
		"deezer.com", "tidal.com", "bandcamp.com", "pandora.com",
	},
	"dating": {
		"tinder.com", "bumble.com", "hinge.co", "okcupid.com", "match.com",
		"badoo.com", "grindr.com", "pof.com",
	},
	"messaging": {
		"web.whatsapp.com", "web.telegram.org", "messenger.com", "discord.com",
		"slack.com", "signal.org", "teams.microsoft.com",
	},
	"ai-chat": {
		"chatgpt.com", "chat.openai.com", "claude.ai", "gemini.google.com", "copilot.microsoft.com",
		"perplexity.ai", "character.ai", "poe.com", "grok.com",
	},
	"sports": {
		"espn.com", "skysports.com", "bleacherreport.com", "nba.com", "nfl.com",
		"uefa.com", "fifa.com", "goal.com", "flashscore.com", "livescore.com",
	},
	"finance": {
		"tradingview.com", "coinmarketcap.com", "coingecko.com", "binance.com", "coinbase.com",
		"robinhood.com", "finance.yahoo.com", "investing.com", "avanza.se", "nordnet.se",
	},
}

// templateDomains returns the seed domains for a template name, or an error
// listing the available templates.
func templateDomains(name string) ([]string, error) {
	domains, ok := templates[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("unknown template %q (available: %s)", name, availableTemplates())
	}
	// Return a copy so callers can't mutate the shared slice.
	out := make([]string, len(domains))
	copy(out, domains)
	return out, nil
}

func availableTemplates() string {
	names := make([]string, 0, len(templates))
	for n := range templates {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
