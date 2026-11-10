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
let keywordHits = { block: false, greyscale: false, noImages: false };
// The list actions already reported for the page at keywordsFor ("id action").
let reportedHits = new Set();
let keywordsFor = null;
let textChanged = true;
// Only so much of a page's text is read, so a huge page can't slow it down.
const MAX_TEXT = 2000000;

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

function render() {
  if (!lists) {
    return;
  }
  const url = topUrl();
  if (keywordsFor !== url) {
    keywordHits = { block: false, greyscale: false, noImages: false };
    keywordsFor = url;
  }
  const byAddress = kc9kListActions(lists, { url });
  const greyscale = Boolean(byAddress.greyscale) || keywordHits.greyscale;
  const noImages = Boolean(byAddress["no-images"]) || keywordHits.noImages;
  shownFor = url;
  let css = "";
  if (greyscale) {
    css += "html { filter: grayscale(1) !important; }\n";
  }
  if (noImages) {
    css += "img, picture, video, canvas, svg image { visibility: hidden !important; }\n" +
      "* { background-image: none !important; }\n";
  }
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

function load(page) {
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
    keywordHits = { block: false, greyscale: false, noImages: false };
    reportedHits = new Set();
    keywordsFor = url;
  }
  // Lists that only look at addresses need no reading: this still catches a
  // page that changed address without loading (the background script only
  // sees real loads).
  const testing = lists.some((list) => list.test);
  const page = kc9kReadsPage(lists) || testing
    ? { url, title: document.title || "", text: (body.innerText || body.textContent || "").slice(0, MAX_TEXT) }
    : { url };
  if (testing) {
    showTestPanel(kc9kTestReport(lists, page));
  } else {
    hideTestPanel();
  }
  const matches = kc9kListMatches(lists, page);
  const fresh = [];
  for (const match of matches) {
    const names = match.actions.map((action) => action.action);
    keywordHits.greyscale = keywordHits.greyscale || names.includes("greyscale");
    keywordHits.noImages = keywordHits.noImages || names.includes("no-images");
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
// it changes, at most every second and a half.
new MutationObserver(() => {
  textChanged = true;
}).observe(document.documentElement || document, { childList: true, subtree: true, characterData: true });
setInterval(scanKeywords, 1500);
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
