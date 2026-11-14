"use strict";

// Greys out pages and hides images for the blacklists with those actions,
// and reads the page's title and text for the lists that look at them. The
// background script also cancels image requests from those pages; hiding
// them here covers images already cached or drawn by the page itself.

const STYLE_ID = "kc9k-page-style";
let lists = null;
let shownFor = null;
// What the lists decided from the title and text of the page at keywordsFor.
// Once a list applies to a page it stays until the address changes, so a
// page can't shake it off by changing its text.
let keywordLooks = new Set();
// The actions that change how a page looks or sounds rather than take it away.
const LOOKS = ["greyscale", "no-images", "mute", "no-video", "declutter", "dark"];
// Whether the page at lightFor has a light background (dark only acts on those).
let lightFor = null;
let light = true;
let noVideo = false;

function pageIsLight() {
  if (lightFor === location.href && document.body) {
    return light;
  }
  const parse = (color) => {
    const m = /rgba?\(([^)]+)\)/.exec(color || "");
    if (!m) {
      return null;
    }
    const [r, g, b, a] = m[1].split(",").map((v) => parseFloat(v));
    return a === 0 ? null : (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
  };
  let value = null;
  for (const el of [document.body, document.documentElement]) {
    if (el && value === null) {
      value = parse(getComputedStyle(el).backgroundColor);
    }
  }
  light = value === null ? true : value > 0.5;
  if (document.body) {
    lightFor = location.href;
  }
  return light;
}
// The list actions already reported for the page at keywordsFor ("id action").
let reportedHits = new Set();
let keywordsFor = null;
let textChanged = true;
// Only so much of a page's text is read, so a huge page can't slow it down.
const MAX_TEXT = 2000000;
// Principle 9: the page is read whole once per address, then only what is
// added to it (pageText grows), in idle time; reading stops once the page
// is taken away or every list has done all it can (readingDone).
let pageText = "";
let readWhole = false;
let addedNodes = new Set();
let readingDone = false;

// topUrl is the address of the tab, so a frame follows the page around it.
function topUrl() {
  if (window === window.top) {
    return location.href;
  }
  try {
    return window.top.location.href;
  } catch (error) {
    // A frame from another site can't read the page's address; the referrer
    // usually names it.
    return document.referrer || location.href;
  }
}

// The circadian rhythm's dark pages: on now (or following the system's dark
// mode), except on exempt sites.
let darkPages = null;
function darkPage(url) {
  if (!darkPages || !(darkPages.on ||
      (darkPages.follow_system && matchMedia("(prefers-color-scheme: dark)").matches))) {
    return false;
  }
  let host = "";
  try {
    host = new URL(url).hostname.replace(/^www\./, "");
  } catch (error) {
    return false;
  }
  return !(darkPages.exempt || []).some((site) => host === site || host.endsWith("." + site));
}

function render() {
  if (!lists) {
    return;
  }
  const url = topUrl();
  if (keywordsFor !== url) {
    keywordLooks = new Set();
    keywordsFor = url;
  }
  const byAddress = kc9kListActions(lists, { url });
  const on = (name) => Boolean(byAddress[name]) || keywordLooks.has(name);
  shownFor = url;
  noVideo = on("no-video");
  let css = "";
  const filters = [];
  if ((on("dark") || darkPage(url)) && pageIsLight()) {
    // Turn the page over, then turn pictures and videos back.
    filters.push("invert(1) hue-rotate(180deg)");
    css += "img, picture, video, canvas, iframe, svg image, [style*=\"background-image\"] " +
      "{ filter: invert(1) hue-rotate(180deg) !important; }\n" +
      "html { background: #fff !important; }\n";
  }
  if (on("greyscale")) {
    filters.push("grayscale(1)");
  }
  if (filters.length > 0) {
    css += "html { filter: " + filters.join(" ") + " !important; }\n";
  }
  if (on("no-images")) {
    css += "img, picture, video, canvas, svg image { visibility: hidden !important; }\n" +
      "* { background-image: none !important; }\n";
  }
  if (noVideo) {
    css += "video, audio, object, embed, iframe[src*=\"youtube\"], iframe[src*=\"vimeo\"], " +
      "iframe[src*=\"player\"], iframe[src*=\"embed\"] { display: none !important; }\n";
  }
  if (on("declutter")) {
    // The page without what surrounds its text: side columns, bars,
    // banners, pop-ups and suggestions.
    css += "aside, nav, footer, [role=\"complementary\"], [role=\"banner\"], [role=\"navigation\"], " +
      "[role=\"contentinfo\"], [class*=\"sidebar\" i], [id*=\"sidebar\" i], [class*=\"recommend\" i], " +
      "[class*=\"related\" i], [id*=\"related\" i], [class*=\"promo\" i], [class*=\"banner\" i], " +
      "[class*=\"popup\" i], [class*=\"modal\" i], [class*=\"cookie\" i], [id*=\"cookie\" i], " +
      "[class*=\"newsletter\" i], [class*=\"share\" i], [class*=\"social\" i], [class*=\"comments\" i], " +
      "[id*=\"comments\" i], [class*=\"trending\" i], [class*=\"advert\" i], [id*=\"advert\" i], " +
      "ins.adsbygoogle, iframe { display: none !important; }\n" +
      "* { animation: none !important; transition: none !important; position: static !important; " +
      "float: none !important; background-image: none !important; box-shadow: none !important; }\n" +
      "body { max-width: 46em !important; margin: 0 auto !important; padding: 1em !important; " +
      "font: 18px/1.6 serif !important; }\n" +
      "img, video { max-width: 100% !important; height: auto !important; }\n";
  }
  css += kc9kSocialCss(social, url);
  let style = document.getElementById(STYLE_ID);
  if (!css) {
    if (style) {
      style.remove();
    }
    return;
  }
  if (!style) {
    style = document.createElement("style");
    style.id = STYLE_ID;
    (document.head || document.documentElement).appendChild(style);
  }
  style.textContent = css;
}

let quotaOpen = false;
let social = [];

// Social media settings on pages that change address without loading: a
// part of the site that is switched off goes where the setting sends it,
// and a video outside the playlists let through is reported.
let playlistSince = 0;
let playlistFor = "";

function enforceSocial() {
  if (window !== window.top || social.length === 0) {
    return;
  }
  const url = location.href;
  if (kc9kPlaylistBlocks(social, url)) {
    browser.runtime.sendMessage({ type: "playlist-violation", url }).catch(() => {});
    return;
  }
  const elsewhere = kc9kSocialRedirect(social, url);
  if (elsewhere) {
    location.replace(elsewhere);
    return;
  }
  checkPlaylistVideo(url);
}

// A video page that names a playlist let through must show a video of that
// playlist: YouTube plays any video with any list= added to its address.
// The playlist panel beside the video lists the playlist's videos; a video
// that isn't among them, or a page that shows no panel for 20 seconds,
// is outside the playlist.
function checkPlaylistVideo(url) {
  const ids = kc9kPlaylists(social);
  let u;
  try {
    u = new URL(url);
  } catch (error) {
    return;
  }
  if (ids.length === 0 || !KC9K_YOUTUBE.test(u.hostname) || u.pathname !== "/watch") {
    return;
  }
  const video = u.searchParams.get("v") || "";
  const key = video + " " + (u.searchParams.get("list") || "");
  if (playlistFor !== key) {
    playlistFor = key;
    playlistSince = Date.now();
  }
  const items = document.querySelectorAll("ytd-playlist-panel-video-renderer a[href*=\"v=\"]");
  if (items.length === 0) {
    if (Date.now() - playlistSince > 20000) {
      browser.runtime.sendMessage({ type: "playlist-violation", url }).catch(() => {});
    }
    return;
  }
  const inList = Array.from(items).some((a) => {
    try {
      return new URL(a.href, location.href).searchParams.get("v") === video;
    } catch (error) {
      return false;
    }
  });
  if (!inList && Date.now() - playlistSince > 4000) {
    browser.runtime.sendMessage({ type: "playlist-violation", url }).catch(() => {});
  }
}

setInterval(enforceSocial, 1000);

function load(page) {
  social = (page && page.social) || [];
  darkPages = (page && page.dark) || null;
  lists = kc9kCompileLists(page && page.lists);
  quotaOpen = Boolean(page && page.quota);
  textChanged = true;
  render();
  scanKeywords();
}

// scanKeywords reads the page's title and text (in the top frame only: a
// frame's text is part of what the page shows) and applies the lists,
// reporting each list action that newly applies to the background script.
// Blocking goes through the background script, which sends the tab to the
// block page and also hides images in the page's frames.
function scanKeywords() {
  if (window !== window.top || !lists || !textChanged) {
    return;
  }
  const body = document.body || document.documentElement;
  if (!body) {
    return;
  }
  textChanged = false;
  const url = location.href;
  if (keywordsFor !== url) {
    keywordLooks = new Set();
    reportedHits = new Set();
    keywordsFor = url;
    readWhole = false;
    readingDone = false;
  }
  const testing = lists.some((list) => list.test);
  if (readingDone && !testing) {
    return;
  }
  // Lists that only look at addresses need no reading: this still catches a
  // page that changed address without loading (the background script only
  // sees real loads).
  const page = kc9kReadsPage(lists) || testing
    ? { url, title: document.title || "", text: readPageText(body) }
    : { url };
  if (testing) {
    showTestPanel(kc9kTestReport(lists, page));
  } else {
    hideTestPanel();
  }
  const matches = kc9kListMatches(lists, page);
  // Nothing more to learn once the page is taken away, or once every list
  // that reads it has reported all its actions.
  if (kc9kTakesAway(kc9kOutcome(matches)) || lists.every((list) =>
    list.test || list.actions.every((a) => reportedHits.has(list.id + " " + a.action) ||
      matches.some((m) => m.list.id === list.id && m.actions.some((x) => x.action === a.action))))) {
    readingDone = true;
  }
  const fresh = [];
  for (const match of matches) {
    const names = match.actions.map((action) => action.action);
    names.filter((name) => LOOKS.includes(name)).forEach((name) => keywordLooks.add(name));
    const unseen = names.filter((name) => !reportedHits.has(match.list.id + " " + name));
    if (unseen.length > 0) {
      unseen.forEach((name) => reportedHits.add(match.list.id + " " + name));
      fresh.push({ id: match.list.id, actions: unseen });
    }
  }
  if (fresh.length === 0) {
    return;
  }
  render();
  browser.runtime.sendMessage({ type: "page-hits", url, matches: fresh }).catch(() => {});
}

// readPageText returns the page's rendered text: all of it the first time
// for an address, then the text so far plus the visible text of what was
// added since, so a feed that keeps loading isn't read again and again.
// Text changed in place (not added) is caught by reading the whole page
// again, at most this often.
const REREAD_MS = 30000;
let readWholeAt = 0;

function readPageText(body) {
  if (!readWhole || (Date.now() - readWholeAt > REREAD_MS && addedNodes.size === 0)) {
    readWhole = true;
    readWholeAt = Date.now();
    addedNodes = new Set();
    pageText = (body.innerText || body.textContent || "").slice(0, MAX_TEXT);
    return pageText;
  }
  const nodes = addedNodes;
  addedNodes = new Set();
  const inside = (node) => {
    for (let p = node.parentNode; p; p = p.parentNode) {
      if (nodes.has(p)) {
        return true;
      }
    }
    return false;
  };
  for (const node of nodes) {
    if (pageText.length >= MAX_TEXT) {
      break;
    }
    const el = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
    // Only what is shown (R-LIST-4): an element that isn't rendered has no
    // boxes, and its innerText would be all of its text. A node inside
    // another added one is read with it.
    if (!el || !el.isConnected || el.closest("#" + PANEL_ID) || el.getClientRects().length === 0 || inside(node)) {
      continue;
    }
    const text = node.nodeType === Node.ELEMENT_NODE ? el.innerText : node.textContent;
    if (text && text.trim()) {
      pageText = (pageText + "\n" + text).slice(0, MAX_TEXT);
    }
  }
  return pageText;
}

// Keyword test mode: a panel in the corner shows what each list in test
// mode found on this page, against each action's thresholds. It lives in a
// closed shadow root, so the page's styles don't reach it and its words
// aren't read as page text.
const PANEL_ID = "kc9k-test-panel";
let panelRoot = null;
let panelShown = "";

function hideTestPanel() {
  const host = document.getElementById(PANEL_ID);
  if (host) {
    host.remove();
  }
  panelRoot = null;
  panelShown = "";
}

function showTestPanel(report) {
  const key = JSON.stringify(report);
  if (key === panelShown && document.getElementById(PANEL_ID)) {
    return;
  }
  panelShown = key;
  let host = document.getElementById(PANEL_ID);
  if (!host || !panelRoot) {
    if (host) {
      host.remove();
    }
    host = document.createElement("div");
    host.id = PANEL_ID;
    host.style.cssText = "all: initial; position: fixed; right: 12px; bottom: 12px; z-index: 2147483647;";
    panelRoot = host.attachShadow({ mode: "closed" });
    document.documentElement.appendChild(host);
  }
  panelRoot.textContent = "";
  const style = document.createElement("style");
  style.textContent = ".box { font: 13px/1.4 sans-serif; background: #1b1d21; color: #eceff4; border: 1px solid #4c566a; " +
    "border-radius: 8px; padding: 10px 12px; max-width: 340px; max-height: 60vh; overflow: auto; box-shadow: 0 4px 16px rgba(0,0,0,.4); }" +
    " h1 { font-size: 13px; margin: 0 0 6px; } h2 { font-size: 13px; margin: 10px 0 2px; }" +
    " .dim { color: #a7b0c0; } .met { color: #ff8a80; font-weight: bold; } .not { color: #a3d9a5; }" +
    " table { border-collapse: collapse; } td { padding: 0 10px 0 0; vertical-align: top; }";
  const box = document.createElement("div");
  box.className = "box";
  const add = (parent, tag, text, cls) => {
    const el = document.createElement(tag);
    el.textContent = text;
    if (cls) {
      el.className = cls;
    }
    parent.appendChild(el);
    return el;
  };
  add(box, "h1", "Keyword test mode");
  add(box, "div", "These lists only show what they find. Nothing is done to the page.", "dim");
  for (const list of report) {
    add(box, "h2", list.name + " (" + list.block + ")");
    if (list.spared) {
      add(box, "div", "An exception of this block spares the page.", "not");
    }
    if (list.address) {
      add(box, "div", "The address matches: every action applies.");
    }
    if (list.title.length > 0) {
      add(box, "div", "In the title: " + list.title.join(", ") + ". Every action applies.");
    }
    if (list.entries.length === 0) {
      add(box, "div", "Nothing found in the text.", "dim");
    } else {
      add(box, "div", list.distinct + " different, " + list.total + " in all:");
      const table = add(box, "table", "");
      for (const found of list.entries.slice(0, 40)) {
        const row = add(table, "tr", "");
        add(row, "td", found.entry);
        add(row, "td", "× " + found.count, "dim");
      }
    }
    for (const action of list.actions) {
      add(box, "div", action.action + ": needs " + action.distinct + " different, " + action.hits + " in all. " +
        (action.met ? "Would apply." : "Would not apply."), action.met ? "met" : "not");
    }
  }
  panelRoot.appendChild(style);
  panelRoot.appendChild(box);
}

// Mirrors and other front-ends. A page that says it is another site (its
// canonical address, or what it gives as its own address to share) or that
// is a known alternative front-end is reported to the background script,
// which treats it as that site if the site is blocked.
const FRONTENDS = [
  { names: /^(invidious|piped|yewtu\.be|poketube|viewtube|materialious)$/i, sites: ["https://www.youtube.com/"] },
  { names: /^(nitter|xcancel)$/i, sites: ["https://x.com/", "https://twitter.com/"] },
  { names: /^(redlib|libreddit|teddit)$/i, sites: ["https://www.reddit.com/"] },
  { names: /^(proxitok)$/i, sites: ["https://www.tiktok.com/"] },
  { names: /^(bibliogram|proxigram|imginn|picuki)$/i, sites: ["https://www.instagram.com/"] },
  { names: /^(scribe|libmedium)$/i, sites: ["https://medium.com/"] },
  { names: /^(rimgo)$/i, sites: ["https://imgur.com/"] },
  { names: /^(quetre)$/i, sites: ["https://www.quora.com/"] },
];
let mirrorFor = null;

function metaContent(selector) {
  const el = document.querySelector(selector);
  return el ? (el.getAttribute("content") || el.getAttribute("title") || "").trim() : "";
}

function checkMirror() {
  if (window !== window.top || !lists || lists.length === 0 || mirrorFor === location.href || !document.head) {
    return;
  }
  mirrorFor = location.href;
  const as = [];
  const own = location.hostname.replace(/^www\./, "");
  for (const selector of ['link[rel="canonical"]', 'meta[property="og:url"]']) {
    const el = document.querySelector(selector);
    const address = el ? (el.getAttribute("href") || el.getAttribute("content") || "") : "";
    try {
      const u = new URL(address, location.href);
      if (/^https?:$/.test(u.protocol) && u.hostname.replace(/^www\./, "") !== own) {
        as.push(u.href);
      }
    } catch (error) {
      // Not an address.
    }
  }
  const claims = [
    metaContent('meta[property="og:site_name"]'), metaContent('meta[name="application-name"]'),
    metaContent('meta[name="generator"]'),
    metaContent('link[rel="search"][type="application/opensearchdescription+xml"]'),
    // Piped says what it is only in its title.
    document.title.trim() === "Piped" ? "Piped" : "",
  ];
  for (const frontend of FRONTENDS) {
    if (claims.some((claim) => frontend.names.test(claim.split(/[\s|–-]/)[0] || ""))) {
      as.push(...frontend.sites);
    }
  }
  if (as.length > 0) {
    browser.runtime.sendMessage({ type: "mirror", url: location.href, as }).catch(() => {});
  }
}

document.addEventListener("DOMContentLoaded", checkMirror);
setInterval(checkMirror, 3000);

// Pages that fill in their text later (most of them) are scanned again when
// it changes, within half a second (principle 9), in idle time so scrolling
// and typing come first. Only what was added is read again.
new MutationObserver((records) => {
  textChanged = true;
  if (readWhole && !readingDone && addedNodes.size < 5000) {
    for (const record of records) {
      for (const node of record.addedNodes) {
        addedNodes.add(node);
      }
    }
  }
}).observe(document.documentElement || document, { childList: true, subtree: true, characterData: true });
let scanQueued = false;
setInterval(() => {
  if (!textChanged || scanQueued) {
    return;
  }
  scanQueued = true;
  const run = () => {
    scanQueued = false;
    scanKeywords();
  };
  if (typeof requestIdleCallback === "function") {
    requestIdleCallback(run, { timeout: 500 });
  } else {
    run();
  }
}, 250);
document.addEventListener("DOMContentLoaded", () => {
  textChanged = true;
  scanKeywords();
});

browser.storage.local.get("page").then((stored) => load(stored.page));
browser.storage.onChanged.addListener((changes) => {
  if (changes.page) {
    load(changes.page.newValue);
  }
});

// Sites that change pages without reloading (YouTube, Reddit) keep this
// script, so follow their address.
setInterval(() => {
  if (lists && topUrl() !== shownFor) {
    textChanged = true;
    render();
    scanKeywords();
  }
}, 1000);

// no-video: whatever still plays is stopped.
setInterval(() => {
  if (!noVideo) {
    return;
  }
  for (const m of document.querySelectorAll("video, audio")) {
    if (!m.paused) {
      m.pause();
    }
  }
}, 1000);

// While a quota session is open, tell the background script when this page
// (or frame) plays sound or video, even muted: that time counts as use.
setInterval(() => {
  if (!quotaOpen) {
    return;
  }
  const media = document.querySelectorAll("video, audio");
  for (const m of media) {
    if (!m.paused && !m.ended && m.readyState > 2) {
      browser.runtime.sendMessage({ type: "playing" }).catch(() => {});
      return;
    }
  }
}, 5000);
