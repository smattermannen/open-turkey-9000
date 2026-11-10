"use strict";

// Social media settings (the daemon's db/social.go): what each one does.
// A setting belongs to a site (hosts), sends some of its addresses elsewhere
// (redirects: the first whose path matches, unless keep matches), and hides
// parts of its pages (hide: CSS selectors, on the paths given by on, all
// when absent). Sites change their pages now and then; the selectors here
// are the part to update when a setting stops hiding something.
const KC9K_YOUTUBE = /(^|\.)youtube\.com$/;
const KC9K_INSTAGRAM = /(^|\.)instagram\.com$/;
const KC9K_FACEBOOK = /(^|\.)facebook\.com$/;
const KC9K_X = /(^|\.)(x|twitter)\.com$/;
const KC9K_REDDIT = /(^|\.)reddit\.com$/;
const KC9K_TIKTOK = /(^|\.)tiktok\.com$/;
const KC9K_LINKEDIN = /(^|\.)linkedin\.com$/;
// Pages a "messages only" setting must leave alone: signing in and the like.
const KC9K_ACCOUNT = /^\/(accounts?|login|logout|signup|challenge|checkpoint|two_factor|recover|privacy|legal|help|oauth|i\/flow|settings)(\/|$)/;

const KC9K_SOCIAL = {
  "youtube:no-shorts": {
    hosts: KC9K_YOUTUBE,
    redirects: [{ path: /^\/shorts(\/|$)/, to: "/" }],
    hide: [{ css: "ytd-reel-shelf-renderer, ytd-rich-shelf-renderer[is-shorts], ytd-rich-section-renderer:has([is-shorts]), " +
      "a[title=\"Shorts\"], ytd-guide-entry-renderer:has(a[title=\"Shorts\"]), ytd-mini-guide-entry-renderer[aria-label=\"Shorts\"], " +
      "ytd-video-renderer:has(a[href^=\"/shorts/\"]), grid-shelf-view-model:has(a[href^=\"/shorts/\"]), " +
      "yt-chip-cloud-chip-renderer:has([title=\"Shorts\"])" }],
  },
  "youtube:no-home": {
    hosts: KC9K_YOUTUBE,
    redirects: [{ path: /^\/$/, to: "/feed/subscriptions" }],
    hide: [{ css: "ytd-browse[page-subtype=\"home\"]" }],
  },
  "youtube:no-recommendations": {
    hosts: KC9K_YOUTUBE,
    hide: [{ css: "#related, ytd-watch-next-secondary-results-renderer, .ytp-endscreen-content, .ytp-ce-element, " +
      ".ytp-suggestion-set, .ytp-autonav-endscreen-upnext-container, .ytp-fullscreen-grid" }],
  },
  "youtube:no-comments": {
    hosts: KC9K_YOUTUBE,
    hide: [{ css: "#comments, ytd-comments, ytd-engagement-panel-section-list-renderer[target-id*=\"comments\"]" }],
  },
  "instagram:messages-only": {
    hosts: KC9K_INSTAGRAM,
    redirects: [{ path: /^\//, keep: /^\/direct(\/|$)/, to: "/direct/inbox/" }],
  },
  "instagram:no-reels": {
    hosts: KC9K_INSTAGRAM,
    redirects: [{ path: /^\/reels?(\/|$)/, to: "/" }],
    hide: [{ css: "a[href=\"/reels/\"], a[href^=\"/reels/\"]" }],
  },
  "instagram:no-explore": {
    hosts: KC9K_INSTAGRAM,
    redirects: [{ path: /^\/explore(\/|$)/, to: "/" }],
    hide: [{ css: "a[href=\"/explore/\"]" }],
  },
  "facebook:no-feed": {
    hosts: KC9K_FACEBOOK,
    hide: [{ on: /^\/(home\.php)?$/, css: "[role=\"feed\"], [role=\"main\"] [data-pagelet^=\"FeedUnit\"], [aria-label=\"Stories\"]" }],
  },
  "facebook:no-reels": {
    hosts: KC9K_FACEBOOK,
    redirects: [{ path: /^\/(reel|reels|watch)(\/|$)/, to: "/" }],
    hide: [{ css: "a[href*=\"/reel/\"], a[href*=\"/watch/\"], [aria-label=\"Reels\"]" }],
  },
  "facebook:messages-only": {
    hosts: KC9K_FACEBOOK,
    redirects: [{ path: /^\//, keep: /^\/messages(\/|$)/, to: "/messages/" }],
  },
  "x:no-explore": {
    hosts: KC9K_X,
    redirects: [{ path: /^\/(explore|i\/trends)(\/|$)/, to: "/home" }],
    hide: [{ css: "a[href=\"/explore\"], [data-testid=\"sidebarColumn\"] section, [aria-label=\"Timeline: Trending now\"], " +
      "[data-testid=\"trend\"]" }],
  },
  "x:messages-only": {
    hosts: KC9K_X,
    redirects: [{ path: /^\//, keep: /^\/messages(\/|$)/, to: "/messages" }],
  },
  "reddit:no-popular": {
    hosts: KC9K_REDDIT,
    redirects: [{ path: /^\/r\/(popular|all)(\/|$)/, to: "/" }],
    hide: [{ css: "a[href=\"/r/popular/\"], a[href=\"/r/all/\"]" }],
  },
  "reddit:no-home": {
    hosts: KC9K_REDDIT,
    hide: [{ on: /^\/(best|hot|new|top|rising)?\/?$/, css: "shreddit-feed, #siteTable, [data-testid=\"post-container\"]" }],
  },
  "tiktok:following-only": {
    hosts: KC9K_TIKTOK,
    redirects: [{ path: /^\/((en|foryou)\/?)?$/, to: "/following" }],
    hide: [{ css: "a[href=\"/foryou\"], a[href=\"/\"][data-e2e=\"nav-foryou\"]" }],
  },
  "tiktok:no-explore": {
    hosts: KC9K_TIKTOK,
    redirects: [{ path: /^\/explore(\/|$)/, to: "/following" }],
    hide: [{ css: "a[href=\"/explore\"]" }],
  },
  "linkedin:no-feed": {
    hosts: KC9K_LINKEDIN,
    hide: [{ on: /^\/feed(\/|$)/, css: "main .scaffold-finite-scroll, main [data-finite-scroll-hotkey-context]" }],
  },
  "linkedin:no-news": {
    hosts: KC9K_LINKEDIN,
    hide: [{ css: "#feed-news-module, aside.scaffold-layout__aside, [data-view-name=\"news-module\"]" }],
  },
};

const KC9K_PLAYLIST = "youtube:playlist=";

function kc9kSocialUrl(url) {
  try {
    const u = new URL(url);
    return /^https?:$/.test(u.protocol) ? u : null;
  } catch (error) {
    return null;
  }
}

// kc9kPlaylists returns the playlist IDs among the settings.
function kc9kPlaylists(presets) {
  return (presets || []).filter((p) => p.startsWith(KC9K_PLAYLIST)).map((p) => p.slice(KC9K_PLAYLIST.length));
}

// kc9kPlaylistBlocks reports whether a YouTube video page is outside the
// playlists let through: with any playlist set, a video page must carry
// list=<one of them>.
function kc9kPlaylistBlocks(presets, url) {
  const ids = kc9kPlaylists(presets);
  const u = kc9kSocialUrl(url);
  if (ids.length === 0 || !u || !KC9K_YOUTUBE.test(u.hostname.toLowerCase())) {
    return false;
  }
  if (!/^\/(watch|shorts|live|embed|v)(\/|$)/.test(u.pathname)) {
    return false;
  }
  return !ids.includes(u.searchParams.get("list") || "");
}

// kc9kSocialRedirect returns where the settings send url, or "".
function kc9kSocialRedirect(presets, url) {
  const u = kc9kSocialUrl(url);
  if (!u) {
    return "";
  }
  const host = u.hostname.toLowerCase();
  for (const name of presets || []) {
    const preset = KC9K_SOCIAL[name];
    if (!preset || !preset.hosts.test(host)) {
      continue;
    }
    for (const rule of preset.redirects || []) {
      if (!rule.path.test(u.pathname) || (rule.keep && (rule.keep.test(u.pathname) || KC9K_ACCOUNT.test(u.pathname)))) {
        continue;
      }
      const to = u.protocol + "//" + u.host + rule.to;
      if (to !== u.protocol + "//" + u.host + u.pathname) {
        return to;
      }
    }
  }
  return "";
}

// kc9kSocialCss returns the CSS that hides what the settings take away on url.
function kc9kSocialCss(presets, url) {
  const u = kc9kSocialUrl(url);
  if (!u) {
    return "";
  }
  const host = u.hostname.toLowerCase();
  let css = "";
  for (const name of presets || []) {
    const preset = KC9K_SOCIAL[name];
    if (!preset || !preset.hosts.test(host)) {
      continue;
    }
    for (const rule of preset.hide || []) {
      if (!rule.on || rule.on.test(u.pathname)) {
        css += rule.css + " { display: none !important; }\n";
      }
    }
  }
  return css;
}
