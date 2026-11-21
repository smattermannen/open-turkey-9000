"use strict";

// Rules come from the Killer Chicken 9000 daemon. The last rules are kept, so blocking
// survives a daemon restart.
const RULES_URL = "http://127.0.0.1:47613/rules";
const POLL_MS = 3000;

// The blacklists of the active blocks, compiled (see match.js). Here they are
// judged by address and tab title; the content script reads the page too,
// handles greyscale, and reports what the title and text decide.
let lists = [];
let webOnly = false;
let safeSearch = false;
let safeSearchEngines = null; // null: every engine (rules without the list)
// Tamper protection (§4.10): "block inactive tabs" and "block embedded
// content" are on unless the rules say they are off.
let inactiveTabsOff = false;
let embeddedOff = false;

// Firefox for Android (IronFox) has no windows or idle API: there the tab in
// front is the active one, and the phone counts as in use.
async function frontTab() {
  if (browser.windows) {
    const win = await browser.windows.getLastFocused();
    if (!win || !win.focused) {
      return undefined;
    }
    const [tab] = await browser.tabs.query({ active: true, windowId: win.id });
    return tab;
  }
  const [tab] = await browser.tabs.query({ active: true });
  return tab;
}

// Use (SPEC §3.7): input within the last 3 minutes.
const USE_IDLE_SECONDS = 180;

async function userActive() {
  return !browser.idle || (await browser.idle.queryState(USE_IDLE_SECONDS)) === "active";
}

function addressOutcome(url) {
  return kc9kOutcome(kc9kListMatches(lists, { url }));
}

function isBlocked(url) {
  return kc9kTakesAway(addressOutcome(url));
}

// The page each tab is loading or showing, from its main_frame requests.
// Chrome doesn't say which page a request belongs to (Firefox's
// frameAncestors and documentUrl), so pageUrl falls back to this.
const tabPages = new Map();

// pageUrl is the address of the tab a request belongs to, so images are
// judged by the page showing them, not by the server they come from.
function pageUrl(details) {
  if (details.type === "main_frame") {
    return details.url;
  }
  const ancestors = details.frameAncestors || [];
  if (ancestors.length > 0) {
    return ancestors[ancestors.length - 1].url;
  }
  return details.documentUrl || tabPages.get(details.tabId) || details.originUrl || details.initiator || "";
}

function isGoogleSearch(url) {
  return url.hostname.startsWith("www.google.") && url.pathname === "/search";
}

function webOnlyAction(rawUrl) {
  const url = new URL(rawUrl);
  const host = url.hostname;
  if (/^(images|shopping|gemini|news|video)\.google\./.test(host) || host === "gemini.google.com") {
    return { block: true };
  }
  if (/\.google\./.test("." + host) && /^\/(ai|aimode|imghp|shopping)(\/|$)/.test(url.pathname)) {
    return { block: true };
  }
  if (isGoogleSearch(url) && (url.searchParams.get("udm") !== "14" || url.searchParams.has("tbm"))) {
    url.searchParams.set("udm", "14");
    url.searchParams.delete("tbm");
    url.searchParams.delete("tbs");
    return { redirect: url.href };
  }
  return null;
}

function blockedPage(url, reason, image) {
  let page = browser.runtime.getURL("blocked.html") + "?url=" + encodeURIComponent(url);
  if (reason) {
    page += "&reason=" + reason;
  }
  if (image) {
    page += "&image=" + encodeURIComponent(image);
  }
  return { redirectUrl: page };
}

// replacement is where a page goes instead, for an outcome without device
// or close-tab actions: the block page (with the list's image, if any), or
// the redirect's address unless that is blocked too.
function replacement(url, outcome, reason) {
  if (outcome.page === "redirect" && /^https?:\/\//i.test(outcome.param) && !isBlocked(outcome.param)) {
    return { redirectUrl: outcome.param };
  }
  return blockedPage(url, reason, outcome.page === "image" ? outcome.param : "");
}

// Reports to the daemon: what took a page away (it counts these for bypass
// prevention, and carries out logout, shutdown and monk mode). R-EVENT-1:
// at most one event per tab and blocked site per minute; pages loaded
// without a tab (preloads) and the first load of tabs restored when the
// browser started aren't reported.
const HIT_URL = "http://127.0.0.1:47613/hit";
const reported = new Map();
const restoredTabs = new Set();
if (browser.tabs && browser.tabs.query) {
  // A restored tab shows its old address from the start; a new window's
  // first tab shows about:newtab or about:blank.
  browser.tabs.query({}).then((tabs) => tabs.filter((tab) => /^(https?|ftp):/i.test(tab.url || ""))
    .forEach((tab) => restoredTabs.add(tab.id)), () => {});
  setTimeout(() => restoredTabs.clear(), 30000); // only the session the browser brought back
}

// hitSite is the site an event is counted for: the host without www.
function hitSite(url) {
  try {
    return new URL(url).hostname.toLowerCase().replace(/^www\./, "");
  } catch (error) {
    return String(url);
  }
}

// Where each tab's page was opened from: the page whose link led to it, as
// the request for it said. The daemon counts a blocked page opened from a
// search engine's results as looked for.
const cameFrom = new Map();

function noteOrigin(details) {
  if (details.tabId >= 0) {
    // Chrome gives only the origin (initiator), which is enough to tell a
    // search engine.
    cameFrom.set(details.tabId, { url: details.url, from: details.originUrl || details.initiator || "" });
    tabPages.set(details.tabId, details.url);
  }
}

// What each tab shows and what it showed before that (for a tab opened from
// another, what that one showed). The browser's own policy stops many blocked
// pages before any request, so there is nothing in cameFrom for them: then
// the page the tab was on just before stands in.
const trail = new Map();

function noteTabUrl(tabId, url) {
  const seen = trail.get(tabId) || { url: "", prev: "" };
  if (url !== seen.url) {
    trail.set(tabId, { url, prev: seen.url || seen.prev });
  }
}

browser.tabs.onCreated.addListener((tab) => {
  const opener = tab.openerTabId === undefined ? undefined : trail.get(tab.openerTabId);
  if (opener) {
    trail.set(tab.id, { url: "", prev: opener.url });
  }
});

function sameAddress(a, b) {
  return a.split("#")[0] === b.split("#")[0];
}

// originOf is where a tab's page at url was opened from, or "". A redirect
// keeps the origin of the request that started it.
function originOf(tabId, url) {
  const seen = cameFrom.get(tabId);
  if (seen && seen.from && sameAddress(seen.url, url)) {
    return seen.from;
  }
  // A page stopped by policy may never be announced as the tab's address:
  // then the last address heard of is the page it was opened from.
  const before = trail.get(tabId);
  const prev = !before ? "" : sameAddress(before.url, url) ? before.prev : before.url;
  return /^https?:/i.test(prev) ? prev : "";
}

// Word recording (§3.12): a page's words go to the daemon, as the rules
// say ("record" or "baseline"); the baseline never takes private windows.
const WORDS_URL = "http://127.0.0.1:47613/words";
function pageWords(message, tab) {
  const mode = message.mode;
  if ((mode !== "record" && mode !== "baseline") || (mode === "baseline" && tab.incognito) ||
    !message.words || typeof message.words !== "object") {
    return;
  }
  fetch(WORDS_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-KC9K-Extension": "1" },
    body: JSON.stringify({ mode, words: message.words }),
  }).catch(() => {});
}

// Link recording ('kc9k record'): while the rules say so, the address of
// every page and frame opened goes to the daemon, once per site.
const VISIT_URL = "http://127.0.0.1:47613/visit";
let recording = false;
const visited = new Set();

function noteVisit(url) {
  if (!recording || !/^https?:/i.test(url)) {
    return;
  }
  let host;
  try {
    host = new URL(url).hostname;
  } catch (error) {
    return;
  }
  if (!host || visited.has(host)) {
    return;
  }
  visited.add(host);
  fetch(VISIT_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-KC9K-Extension": "1" },
    body: JSON.stringify({ url }),
  }).catch(() => visited.delete(host));
}

// Which block and list last took a page away in each tab, for the block
// page to show (see the "why" message).
const lastHit = new Map();

function noteHit(tabId, outcome) {
  const first = outcome.lists[0];
  const list = first && lists.find((l) => l.id === first.id);
  if (tabId >= 0 && list) {
    lastHit.set(tabId, { block: list.block, list: list.name });
  }
}

// viaProxy reports whether url is blocked only for the site it shows (a
// proxy, translator or archive address), not for its own address.
function viaProxy(url) {
  return isBlocked(url) && !kc9kDirectly(() => isBlocked(url));
}

function report(tabId, url, outcome, via) {
  noteHit(tabId, outcome);
  via = via || (viaProxy(url) ? "proxy" : "");
  if (tabId < 0) {
    return Promise.resolve(); // a preload, not a page anyone opened
  }
  if (restoredTabs.delete(tabId) && !outcome.device) {
    return Promise.resolve(); // brought back by the browser at its start
  }
  const key = tabId + " " + hitSite(url);
  const now = Date.now();
  if (now - (reported.get(key) || 0) < 60000) {
    return Promise.resolve();
  }
  reported.set(key, now);
  const from = originOf(tabId, url);
  for (const [k, at] of reported) {
    if (now - at > 60000) {
      reported.delete(k);
    }
  }
  return Promise.all(outcome.lists.map((hit) => fetch(HIT_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-KC9K-Extension": "1" },
    body: JSON.stringify({ list: hit.id, actions: hit.actions, url, from, via }),
  }).then((response) => response.json()).then((answer) => {
    // A bypass detection step that fired on this page closes its tab.
    if (answer && answer.close_tab && tabId >= 0) {
      browser.tabs.remove(tabId).catch(() => {});
    }
  }).catch(() => {})));
}

// closeEverything tells the daemon (first: closing the last tab quits the
// browser), then closes the tab, so a restored session doesn't bring the page
// back, then every window: the browser quits cleanly.
async function closeEverything(tabId, url, outcome, via) {
  await Promise.race([report(tabId, url, outcome, via), new Promise((done) => setTimeout(done, 2000))]);
  try {
    await browser.tabs.remove(tabId);
  } catch (error) {
    // Already gone.
  }
  if (!browser.windows) {
    // Android: closing every tab is closing the browser.
    for (const tab of await browser.tabs.query({})) {
      browser.tabs.remove(tab.id).catch(() => {});
    }
    return;
  }
  const windows = await browser.windows.getAll();
  for (const win of windows) {
    browser.windows.remove(win.id).catch(() => {});
  }
}

// act carries out an outcome on a tab showing url (not from a request:
// see onBeforeRequest for those).
function act(tabId, url, outcome, reason) {
  const via = reason === "mirror" ? "mirror" : "";
  if (outcome.device) {
    closeEverything(tabId, url, outcome, via);
    return;
  }
  report(tabId, url, outcome, via);
  if (outcome.page === "close-tab") {
    browser.tabs.remove(tabId).catch(() => {});
    return;
  }
  browser.tabs.update(tabId, { url: replacement(url, outcome, reason).redirectUrl });
}

// A tab's title is judged by the title lists only: window title entries
// never apply to browsers (R-TITLE-1).
function blockTitledTab(tab) {
  if (!tab.url || !kc9kJudgedAddress(lists, tab.url)) {
    return;
  }
  const outcome = kc9kOutcome(kc9kListMatches(lists, { url: tab.url, title: tab.title || "" }));
  if (kc9kTakesAway(outcome)) {
    // A local file never reaches onBeforeRequest, so its address is judged
    // here too: the block page names it when the address alone did it.
    const byAddress = kc9kTakesAway(kc9kOutcome(kc9kListMatches(lists, { url: tab.url, title: "" })));
    act(tab.id, tab.url, outcome, byAddress ? "" : "title");
    return;
  }
  if (outcome.custom) {
    report(tab.id, tab.url, outcome, ""); // §3.9 Custom action: the daemon runs it
  }
}

// Tabs whose page a list's title or text stripped of images, with the
// address it happened at: their images are cancelled like those of a
// no-images site, frames included, until the tab goes somewhere else.
// keywordNoVideo and keywordMute are the same for those actions.
const keywordNoImages = new Map();
const keywordNoVideo = new Map();
const keywordMute = new Map();
const keywordGreyscale = new Map();

// mute: the tab's sound is switched off while its page is one a list mutes,
// and switched on again when it leaves (only if this is what muted it).
const mutedHere = new Set();

function applyMute(tab) {
  if (!tab || tab.id === undefined || !tab.url) {
    return;
  }
  const want = /^https?:/i.test(tab.url) && (keywordMute.get(tab.id) === tab.url ||
    Boolean(kc9kListActions(lists, { url: tab.url, title: tab.title || "" }).mute));
  const muted = Boolean(tab.mutedInfo && tab.mutedInfo.muted);
  if (want) {
    mutedHere.add(tab.id);
    if (!muted) {
      browser.tabs.update(tab.id, { muted: true }).catch(() => {});
    }
  } else if (mutedHere.has(tab.id)) {
    mutedHere.delete(tab.id);
    if (muted) {
      browser.tabs.update(tab.id, { muted: false }).catch(() => {});
    }
  }
}

// FRAME_LOOKS are the look actions every frame of a tab must apply itself:
// a frame from another site can't read the tab's title, text or (often)
// address, and hiding it from the page around it doesn't stop its sound.
// Mute needs no help: it silences the whole tab; dark is drawn over the
// frames by the tab's own page. Greyscale is among them so it is carried to
// the next page like them and drawn by the user sheet from the first paint.
const FRAME_LOOKS = ["no-video", "no-images", "greyscale"];
// What each tab's frames were last told ("no-video no-images").
const sentLooks = new Map();

// tabLooks is the tab's current outcome for FRAME_LOOKS: what its title or
// text earned it (from the top frame's report), plus what its address and
// title earn it now. The tab's address comes from the browser, so it holds
// even for a page whose requests the extension never saw.
function tabLooks(tab) {
  if (!tab || tab.id === undefined || !tab.url || !/^https?:/i.test(tab.url)) {
    return [];
  }
  const byPage = kc9kListActions(lists, { url: tab.url, title: tab.title || "" });
  const kept = { "no-video": keywordNoVideo, "no-images": keywordNoImages, greyscale: keywordGreyscale };
  return FRAME_LOOKS.filter((name) => byPage[name] || kept[name].has(tab.id) || carriedLook(tab.id, name));
}

// A tab whose page earned no-images or no-video from its title or text
// keeps them when it goes to another address (a click inside a site that
// changes pages without loading, back and forward, a link) until the new
// page has been read (the top frame's "page-judged"), at most CARRY_MS: the
// new page's pictures never show while it is still being read.
const CARRY_MS = 15000;
const carried = new Map(); // tab id → { looks: Set, until }

function carriedLook(tabId, name) {
  const carry = carried.get(tabId);
  return Boolean(carry && carry.until > Date.now() && carry.looks.has(name));
}

function carryLooks(tabId, names) {
  const carry = carried.get(tabId);
  const looks = new Set(carry && carry.until > Date.now() ? carry.looks : []);
  names.forEach((name) => looks.add(name));
  if (looks.size === 0) {
    return;
  }
  carried.set(tabId, { looks, until: Date.now() + CARRY_MS });
  setTimeout(() => {
    browser.tabs.get(tabId).then(tellFrames, () => {});
  }, CARRY_MS + 100);
}

// The looks are also drawn by a user style sheet in every frame of the tab:
// a user sheet's !important beats anything the page sets (its own
// !important and inline styles included), reaches into shadow roots, can't
// be removed by the page, and comes with the frame's first paint.
// Pictures behind ::before/::after, list markers and borders go too.
const LOOK_CSS = {
  // Pictures a sheet draws as content are marked by the content script
  // (data-kc9k-noimg): their content is emptied.
  "no-images": "img, picture, video, canvas, svg image, svg use, image, object, embed, input[type=\"image\"] " +
    "{ visibility: hidden !important; }\n" +
    "*, *::before, *::after, *::marker { background-image: none !important; list-style-image: none !important; " +
    "border-image-source: none !important; }\n" +
    "[data-kc9k-noimg~=\"before\"]::before, [data-kc9k-noimg~=\"after\"]::after { content: \"\" !important; }\n" +
    "[data-kc9k-noimg~=\"self\"] { content: normal !important; }\n",
  // A page in dark mode draws greyscale itself, with its inversion (both
  // are filters on the same element).
  greyscale: ":root:not([data-kc9k-dark]) { filter: grayscale(1) !important; }\n",
  "no-video": "video, audio, object, embed, iframe[src*=\"youtube\"], iframe[src*=\"vimeo\"], " +
    "iframe[src*=\"player\"], iframe[src*=\"embed\"] { display: none !important; }\n",
};
const lookCss = new Map(); // tab id → the CSS last inserted in all its frames

function cssFor(looks) {
  return looks.map((name) => LOOK_CSS[name] || "").join("");
}

function insertCss(tabId, frameId, css) {
  if (!css) {
    return;
  }
  if (browser.scripting && browser.scripting.insertCSS) {
    const target = frameId === undefined ? { tabId, allFrames: true } : { tabId, frameIds: [frameId] };
    browser.scripting.insertCSS({ target, css, origin: "USER" }).catch(() => {});
    return;
  }
  const details = { code: css, cssOrigin: "user", runAt: "document_start", matchAboutBlank: true };
  if (frameId === undefined) {
    details.allFrames = true;
  } else {
    details.frameId = frameId;
  }
  browser.tabs.insertCSS(tabId, details).catch(() => {});
}

function removeCss(tabId, css) {
  if (!css) {
    return;
  }
  if (browser.scripting && browser.scripting.removeCSS) {
    browser.scripting.removeCSS({ target: { tabId, allFrames: true }, css, origin: "USER" }).catch(() => {});
    return;
  }
  browser.tabs.removeCSS(tabId, { code: css, cssOrigin: "user", allFrames: true, matchAboutBlank: true }).catch(() => {});
}

function syncLookCss(tabId, looks) {
  const css = cssFor(looks);
  const old = lookCss.get(tabId) || "";
  if (css === old) {
    insertCss(tabId, undefined, css); // documents loaded since may lack it
    return;
  }
  removeCss(tabId, old);
  insertCss(tabId, undefined, css);
  lookCss.set(tabId, css);
}

// tellFrames sends every frame of the tab its looks when they changed.
// Frames that load later ask for them themselves ("tab-looks").
function tellFrames(tab) {
  if (!tab || tab.id === undefined) {
    return;
  }
  const looks = tabLooks(tab);
  const key = looks.join(" ");
  if ((sentLooks.get(tab.id) || "") === key) {
    return;
  }
  sentLooks.set(tab.id, key);
  syncLookCss(tab.id, looks);
  browser.tabs.sendMessage(tab.id, { type: "tab-looks", looks }).catch(() => {});
}

browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.url !== undefined) {
    const dropped = [];
    for (const [name, kept] of [["no-images", keywordNoImages], ["no-video", keywordNoVideo],
      ["greyscale", keywordGreyscale], ["mute", keywordMute]]) {
      if (kept.has(tabId) && kept.get(tabId) !== changeInfo.url) {
        kept.delete(tabId);
        dropped.push(name);
      }
    }
    carryLooks(tabId, dropped.filter((name) => FRAME_LOOKS.includes(name)));
  }
  if (changeInfo.status === "loading" || changeInfo.status === "complete") {
    // A new document: the user sheet went with the old one.
    insertCss(tabId, undefined, lookCss.get(tabId) || "");
  }
  if (changeInfo.url !== undefined || changeInfo.title !== undefined || changeInfo.mutedInfo !== undefined) {
    applyMute(tab);
  }
  if (changeInfo.url !== undefined || changeInfo.title !== undefined) {
    tellFrames(tab);
  }
  if (changeInfo.url !== undefined) {
    noteTabUrl(tabId, changeInfo.url);
    noteVisit(changeInfo.url);
  }
  if (changeInfo.title !== undefined) {
    blockTitledTab(tab);
  }
});
browser.tabs.onRemoved.addListener((tabId) => {
  keywordNoImages.delete(tabId);
  keywordNoVideo.delete(tabId);
  keywordMute.delete(tabId);
  keywordGreyscale.delete(tabId);
  carried.delete(tabId);
  lookCss.delete(tabId);
  sentLooks.delete(tabId);
  mutedHere.delete(tabId);
  cameFrom.delete(tabId);
  trail.delete(tabId);
  tabPages.delete(tabId);
  lastHit.delete(tabId);
});

// mirrorHit acts on a content script's report that its page shows another
// site: a mirror or an alternative front-end (see content.js). The page is
// treated as the first of those sites that is blocked.
function mirrorHit(message, sender) {
  const tab = sender && sender.tab;
  if (!tab || sender.frameId !== 0 || typeof message.url !== "string" || !/^https?:/i.test(message.url)) {
    return;
  }
  for (const as of Array.isArray(message.as) ? message.as.slice(0, 8) : []) {
    if (typeof as !== "string" || !/^https?:\/\//i.test(as)) {
      continue;
    }
    const outcome = addressOutcome(as);
    if (kc9kTakesAway(outcome)) {
      act(tab.id, message.url, outcome, "mirror");
      return;
    }
  }
}

// frameHit answers a frame's report of the lists its own text matched
// (R-LIST-4): when they would take a page away, the frame gets the block
// page; the tab and the rest of the page stay. The lists and actions are
// taken from this script's own rules, never from the report.
function frameHit(message, sender) {
  const tab = sender && sender.tab;
  if (embeddedOff || !tab || !sender.frameId || typeof message.url !== "string" || !kc9kJudgedAddress(lists, message.url)) {
    return null;
  }
  const matches = [];
  for (const hit of Array.isArray(message.matches) ? message.matches : []) {
    const list = lists.find((l) => l.id === hit.id);
    const names = Array.isArray(hit.actions) ? hit.actions : [];
    if (list && !list.test) {
      const actions = list.actions.filter((action) => names.includes(action.action));
      if (actions.length > 0) {
        matches.push({ list, actions });
      }
    }
  }
  return kc9kTakesAway(kc9kOutcome(matches)) ? { blocked: blockedPage(message.url).redirectUrl } : null;
}

// why tells the block page what stopped its page, when the rules allow it.
function why(sender) {
  const tab = sender && sender.tab;
  const hit = tab && lastHit.get(tab.id);
  if (!showWhy || !hit) {
    return null;
  }
  return { block: hit.block, list: hit.list, ends: ends[hit.block] || 0 };
}

// pageHit acts on a content script's report of the lists that matched its
// page (by address, title and text). Only the tab's own page counts, and
// only web pages; the lists and actions are taken from this script's own
// rules, never from the report.
function pageHit(message, sender) {
  const tab = sender && sender.tab;
  if (!tab || sender.frameId !== 0 || typeof message.url !== "string" || !kc9kJudgedAddress(lists, message.url)) {
    return;
  }
  const matches = [];
  for (const hit of Array.isArray(message.matches) ? message.matches : []) {
    const list = lists.find((l) => l.id === hit.id);
    const names = Array.isArray(hit.actions) ? hit.actions : [];
    if (list) {
      const actions = list.actions.filter((action) => names.includes(action.action));
      if (actions.length > 0) {
        matches.push({ list, actions });
      }
    }
  }
  const takes = (name) => matches.some((m) => m.actions.some((action) => action.action === name));
  if (takes("no-images")) {
    keywordNoImages.set(tab.id, message.url);
  }
  if (takes("no-video")) {
    keywordNoVideo.set(tab.id, message.url);
  }
  if (takes("greyscale")) {
    keywordGreyscale.set(tab.id, message.url);
  }
  if (takes("mute")) {
    keywordMute.set(tab.id, message.url);
    applyMute(tab);
  }
  if (FRAME_LOOKS.some(takes)) {
    tellFrames(tab);
  }
  const outcome = kc9kOutcome(matches);
  if (kc9kTakesAway(outcome)) {
    // A page taken away by its address alone (one that changed address
    // without loading, or whose block has exceptions by title or text) goes
    // back when the block ends; one taken away for its title or text stays.
    act(tab.id, message.url, outcome, isBlocked(message.url) ? "" : "keyword");
  } else if (outcome.custom) {
    report(tab.id, message.url, outcome, ""); // §3.9 Custom action: the daemon runs it
  }
}

// Ad trigger (§3.9): a page that loads one of a list's ad networks (a
// third-party request from its tab) matches that list, once per page.
const adSeen = new Map(); // tab id → "page url" + list ids already acted on
function noteAdRequest(details) {
  if (details.tabId < 0 || details.type === "main_frame" || !lists.some((l) => l.ads && l.ads.length > 0)) {
    return;
  }
  const pageUrl = details.documentUrl || details.originUrl || details.initiator || "";
  let host = "";
  let pageHost = "";
  try {
    host = new URL(details.url).hostname;
    pageHost = new URL(pageUrl).hostname;
  } catch (error) {
    return;
  }
  if (!/^https?:/i.test(pageUrl) || host === pageHost || host.endsWith("." + pageHost)) {
    return; // first-party: not an ad network's
  }
  const found = kc9kAdMatches(lists, host, pageUrl);
  if (found.length === 0) {
    return;
  }
  let seen = adSeen.get(details.tabId);
  if (!seen || seen.page !== pageUrl) {
    seen = { page: pageUrl, ids: new Set() };
    adSeen.set(details.tabId, seen);
  }
  const fresh = found.filter((m) => !seen.ids.has(m.list.id));
  if (fresh.length === 0) {
    return;
  }
  fresh.forEach((m) => seen.ids.add(m.list.id));
  browser.tabs.get(details.tabId).then((tab) => {
    pageHit({ url: tab.url, matches: fresh.map((m) => ({ id: m.list.id, actions: m.actions.map((a) => a.action) })) },
      { tab, frameId: 0 });
  }, () => {});
}
browser.tabs.onRemoved.addListener((tabId) => adSeen.delete(tabId));

browser.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.url.startsWith("http://127.0.0.1:47613/")) {
      return {};
    }
    noteAdRequest(details);
    if (details.type === "main_frame" || details.type === "sub_frame") {
      noteVisit(details.url);
    }
    if (details.type === "image" || details.type === "imageset" || details.type === "media") {
      if (keywordNoImages.has(details.tabId) || carriedLook(details.tabId, "no-images") ||
        (details.type === "media" && (keywordNoVideo.has(details.tabId) || carriedLook(details.tabId, "no-video")))) {
        return { cancel: true };
      }
      const looks = kc9kListActions(lists, { url: pageUrl(details) });
      return looks["no-images"] || (details.type === "media" && looks["no-video"]) ? { cancel: true } : {};
    }
    if (details.type === "main_frame") {
      noteOrigin(details);
    }
    const outcome = addressOutcome(details.url);
    // Block embedded content off (§4.10): frames are left alone.
    if (kc9kTakesAway(outcome) && !(embeddedOff && details.type !== "main_frame")) {
      // A frame only ever gets the block page: something embedded in a page
      // mustn't close the tab or the browser.
      if (details.type !== "main_frame") {
        return blockedPage(details.url);
      }
      if (outcome.device || outcome.page === "close-tab") {
        if (details.tabId >= 0) {
          setTimeout(() => act(details.tabId, details.url, outcome, ""), 0);
        }
        return { cancel: true };
      }
      report(details.tabId, details.url, outcome);
      return replacement(details.url, outcome, "");
    }
    if (outcome.custom && details.type === "main_frame") {
      report(details.tabId, details.url, outcome); // §3.9 Custom action: the daemon runs it
    }
    if (details.type === "main_frame") {
      // Social media settings: a video outside the playlists let through,
      // and the parts of sites that are switched off.
      if (kc9kPlaylistBlocks(social, details.url)) {
        return blockedPage(details.url, "playlist");
      }
      const elsewhere = kc9kSocialRedirect(social, details.url);
      if (elsewhere) {
        return { redirectUrl: elsewhere };
      }
    }
    if (safeSearch && (details.type === "main_frame" || details.type === "sub_frame")) {
      const form = details.method === "POST" && details.requestBody ? details.requestBody.formData : null;
      const action = kc9kSafeSearch(details.url, form, safeSearchEngines);
      if (action && action.block) {
        return blockedPage(details.url, "safesearch");
      }
      if (action && action.redirect) {
        const also = webOnly ? webOnlyAction(action.redirect) : null;
        if (also && also.block) {
          return blockedPage(details.url);
        }
        return { redirectUrl: also && also.redirect ? also.redirect : action.redirect };
      }
    }
    if (webOnly && /^https?:/.test(details.url)) {
      const action = webOnlyAction(details.url);
      if (action && action.block) {
        return blockedPage(details.url);
      }
      if (action && action.redirect) {
        return { redirectUrl: action.redirect };
      }
    }
    return {};
  },
  { urls: ["<all_urls>"], types: requestTypes() },
  ["blocking", "requestBody"]
);

// SafeSearch on YouTube: the header Google honours for Restricted Mode.
browser.webRequest.onBeforeSendHeaders.addListener(
  (details) => {
    if (!safeSearch || !kc9kSafeSearchEngine(safeSearchEngines, "youtube")) {
      return {};
    }
    const headers = (details.requestHeaders || []).filter((h) => h.name.toLowerCase() !== "youtube-restrict");
    headers.push({ name: "YouTube-Restrict", value: "Strict" });
    return { requestHeaders: headers };
  },
  { urls: ["*://youtube.com/*", "*://*.youtube.com/*", "*://*.youtube-nocookie.com/*", "*://youtubei.googleapis.com/*"] },
  ["blocking", "requestHeaders"]
);

// requestTypes are the kinds of request judged: Chrome has no imageset (and
// refuses a filter that names it).
function requestTypes() {
  const types = ["main_frame", "sub_frame", "image", "media"];
  const known = browser.webRequest.ResourceType;
  if (known && Object.values(known).includes("imageset")) {
    types.push("imageset");
  }
  return types;
}

// stillBlocked tells the block page whether its site is still blocked, so it
// can go back to the site once the block ends.
function stillBlocked(url) {
  if (isBlocked(url) || kc9kPlaylistBlocks(social, url)) {
    return true;
  }
  if (safeSearch) {
    const action = kc9kSafeSearch(url, null, safeSearchEngines);
    if (action && action.block) {
      return true;
    }
  }
  if (webOnly && /^https?:/i.test(url)) {
    const action = webOnlyAction(url);
    return Boolean(action && action.block);
  }
  return false;
}

// Chrome ignores a promise returned by an onMessage listener, so answers go
// through sendResponse, which Firefox takes too.
browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
  const answer = onMessage(message, sender);
  if (answer === undefined) {
    return false;
  }
  Promise.resolve(answer).then(sendResponse, () => sendResponse(undefined));
  return true;
});

function onMessage(message, sender) {
  if (message && message.type === "still-blocked" && typeof message.url === "string") {
    return Promise.resolve(stillBlocked(message.url));
  }
  if (message && message.type === "page-hits") {
    pageHit(message, sender);
    if (Array.isArray(message.matches)) {
      noteDownloadPage(message.url, message.matches.flatMap((m) => (Array.isArray(m.actions) ? m.actions : [])));
    }
  }
  if (message && message.type === "mirror") {
    mirrorHit(message, sender);
  }
  if (message && message.type === "frame-hits") {
    return Promise.resolve(frameHit(message, sender));
  }
  if (message && message.type === "playlist-violation" && sender && sender.tab && sender.frameId === 0 &&
    typeof message.url === "string" && kc9kPlaylists(social).length > 0) {
    browser.tabs.update(sender.tab.id, { url: blockedPage(message.url, "playlist").redirectUrl });
  }
  if (message && message.type === "tab-looks" && sender && sender.tab) {
    // Every new document and frame asks as it starts: it gets the user
    // sheet at once too.
    const looks = tabLooks(sender.tab);
    insertCss(sender.tab.id, sender.frameId, cssFor(looks));
    return Promise.resolve(looks);
  }
  if (message && message.type === "page-judged" && sender && sender.tab && sender.frameId === 0 &&
    message.url === sender.tab.url) {
    // The top frame has read its page: what it earned was reported in
    // page-hits; what the last page earned no longer holds.
    if (carried.delete(sender.tab.id)) {
      tellFrames(sender.tab);
    }
  }
  if (message && message.type === "why") {
    return Promise.resolve(why(sender));
  }
  if (message && message.type === "popup") {
    return popupState(message);
  }
  if (message && message.type === "add-site" && typeof message.block === "string" && typeof message.url === "string") {
    return addSite(message.block, message.url);
  }
  if (message && message.type === "dark-exempt" && typeof message.url === "string") {
    return darkExempt(message.url, Boolean(message.exempt));
  }
  if (message && message.type === "page-words" && sender && sender.tab) {
    pageWords(message, sender.tab);
  }
  if (message && message.type === "playing" && sender && sender.tab) {
    playing.set(sender.tab.id, Date.now());
  }
  return undefined;
}

// From the rules: whether the block page may say what stopped a page, when
// each active block ends, the blocks that will take pages away soon (for
// the time left on the toolbar button), and every block's name.
let showWhy = false;
let ends = {};
let soon = [];
let blockNames = [];
let social = [];
let daemonUp = true;
let darkRule = null; // dark pages (§4.10): on, follow_system, exempt sites

function apply(rules) {
  showWhy = Boolean(rules.details);
  ends = rules.ends || {};
  soon = (rules.soon || []).map((s) => ({
    block: s.block, kind: s.kind, seconds: s.seconds, lists: kc9kCompileLists(s.lists),
  }));
  blockNames = rules.blocks || [];
  social = rules.social || [];
  darkRule = rules.dark || null;
  lists = kc9kCompileLists(rules.lists);
  quotaLists = kc9kCompileLists(rules.quota);
  webOnly = Boolean(rules.web_only);
  safeSearch = Boolean(rules.safesearch);
  safeSearchEngines = Array.isArray(rules.safesearch_engines) ? rules.safesearch_engines : null;
  strictUse = Boolean(rules.strict_use);
  statsSkipPrivate = Boolean(rules.stats_skip_private);
  inactiveTabsOff = Boolean(rules.inactive_tabs_off);
  embeddedOff = Boolean(rules.embedded_off);
  if (recording !== Boolean(rules.recording)) {
    recording = Boolean(rules.recording);
    visited.clear();
  }
}

async function closeNewlyBlockedTabs() {
  const tabs = await browser.tabs.query({});
  for (const tab of tabs) {
    // Block inactive tabs off (§4.10): a background tab waits until it is
    // switched to (judgeTab from onActivated).
    if (inactiveTabsOff && !tab.active) {
      continue;
    }
    judgeTab(tab);
  }
}

// judgeTab acts on what a tab already shows, by the rules now.
function judgeTab(tab) {
  if (!tab || !tab.url || !kc9kJudgedAddress(lists, tab.url)) {
    return;
  }
  const outcome = addressOutcome(tab.url);
  if (kc9kTakesAway(outcome)) {
    act(tab.id, tab.url, outcome, "");
  } else if (kc9kPlaylistBlocks(social, tab.url)) {
    browser.tabs.update(tab.id, { url: blockedPage(tab.url, "playlist").redirectUrl });
  } else if (kc9kSocialRedirect(social, tab.url)) {
    browser.tabs.update(tab.id, { url: kc9kSocialRedirect(social, tab.url) });
  } else {
    blockTitledTab(tab);
    applyMute(tab);
    tellFrames(tab);
  }
}

browser.tabs.onActivated.addListener((info) => {
  if (inactiveTabsOff) {
    browser.tabs.get(info.tabId).then(judgeTab, () => {});
  }
});

async function poll() {
  try {
    // The header marks this as the extension checking in (see the daemon's
    // ExtensionHeader); web pages can't send it.
    // Version and private-tab access tell KC9K on Android that this copy is
    // current and also sees private tabs.
    let privateTabs = "0";
    try {
      privateTabs = (await browser.extension.isAllowedIncognitoAccess()) ? "1" : "0";
    } catch (error) {
      // Not known here.
    }
    const response = await fetch(RULES_URL, {
      cache: "no-store",
      headers: {
        "X-KC9K-Extension": "1",
        "X-KC9K-Version": browser.runtime.getManifest().version,
        "X-KC9K-Private": privateTabs,
      },
    });
    if (response.ok) {
      const rules = await response.json();
      apply(rules);
      checkForUpdate(rules.extension_version);
      await browser.storage.local.set({
        rules,
        page: { lists: rules.lists || [], quota: (rules.quota || []).length > 0, social: rules.social || [], dark: rules.dark || null,
          words: rules.words || "" },
      });
      await closeNewlyBlockedTabs();
      daemonUp = true;
    } else {
      daemonUp = false;
    }
  } catch (error) {
    // The daemon is down: keep enforcing the last rules.
    daemonUp = false;
  }
  updateBadges();
}

// Chrome only looks for a new version of the extension every few hours, so
// when the daemon has a newer one, the extension asks for it at once, at most
// every ten minutes, and restarts on it. (Firefox gets each version through
// the policy, which names its file.)
let updateAsked = 0;

function checkForUpdate(version) {
  if (!version || version === browser.runtime.getManifest().version || !browser.runtime.requestUpdateCheck ||
    Date.now() - updateAsked < 10 * 60 * 1000) {
    return;
  }
  updateAsked = Date.now();
  browser.runtime.requestUpdateCheck().catch(() => {});
}

if (browser.runtime.onUpdateAvailable) {
  browser.runtime.onUpdateAvailable.addListener(() => browser.runtime.reload());
}

// Chrome runs this as a service worker, which it stops when the worker does
// nothing for 30 seconds; the blocking request listener would then start
// without rules. Any extension call keeps it running, and an alarm starts it
// again should it stop anyway.
if (browser.runtime.getManifest().manifest_version === 3) {
  setInterval(() => browser.runtime.getPlatformInfo().catch(() => {}), 20000);
  browser.alarms.create("kc9k-keepalive", { periodInMinutes: 0.5 });
  browser.alarms.onAlarm.addListener(() => {});
}

// Time left. timeLeftFor returns the blocks that will take the page away
// soon, soonest first: [{ block, kind, seconds }].
function timeLeftFor(tab) {
  if (!tab || !tab.url || !/^https?:/i.test(tab.url)) {
    return [];
  }
  const page = { url: tab.url, title: tab.title || "" };
  return soon
    .filter((s) => kc9kListMatches(s.lists, page).length > 0)
    .map((s) => ({ block: s.block, kind: s.kind, seconds: s.seconds }))
    .sort((a, b) => a.seconds - b.seconds);
}

// The toolbar button shows, on the tab in front, the time until its site
// is blocked; "!" when the daemon doesn't answer.
// (Chrome's toolbar button is browser.action.)
async function updateBadges() {
  const button = browser.action || browser.browserAction;
  if (!button) {
    return;
  }
  try {
    const tabs = await browser.tabs.query({ active: true });
    for (const tab of tabs) {
      const left = timeLeftFor(tab)[0];
      let text = "";
      let color = "#3b6ea8";
      let title = "Killer Chicken 9000";
      if (!daemonUp) {
        text = "!";
        color = "#c0392b";
        title = "Killer Chicken 9000 isn't answering. The last rules still apply.";
      } else if (left) {
        text = kc9kTimeLeft(left.seconds);
        color = left.seconds <= 300 ? "#c0392b" : "#3b6ea8";
        title = leftSentence(left);
      }
      button.setBadgeText({ text, tabId: tab.id });
      button.setBadgeBackgroundColor({ color, tabId: tab.id });
      button.setTitle({ title, tabId: tab.id });
    }
  } catch (error) {
    // A tab closed meanwhile.
  }
}

function leftSentence(left) {
  const time = kc9kTimeLeft(left.seconds);
  if (left.kind === "quota") {
    return time + " of the allowance left for '" + left.block + "'";
  }
  if (left.kind === "break") {
    return time + " of break left; then '" + left.block + "' blocks this again";
  }
  return "'" + left.block + "' blocks this in " + time;
}

browser.tabs.onActivated.addListener(updateBadges);

// popupState describes the tab in front (or, for the tests, the page given
// in the message).
async function popupState(message) {
  let tab;
  if (message && typeof message.url === "string") {
    tab = { url: message.url, title: typeof message.title === "string" ? message.title : "" };
  } else {
    tab = await frontTab();
  }
  const url = tab && tab.url && /^https?:/i.test(tab.url) ? tab.url : "";
  let site = "";
  if (url) {
    site = new URL(url).hostname.replace(/^www\./, "");
  }
  const page = { url, title: (tab && tab.title) || "" };
  const acting = url ? kc9kListMatches(lists, page).map((m) => ({
    block: m.list.block, list: m.list.name, actions: m.actions.map((a) => a.action),
  })) : [];
  // Dark pages (§4.10): the site can be left as it is, or shown dark again.
  let dark = null;
  if (url && darkRule && (darkRule.on || darkRule.follow_system)) {
    const exempt = (darkRule.exempt || []).some((s) => site === s || site.endsWith("." + s));
    dark = { exempt };
  }
  return {
    up: daemonUp, url, site, acting,
    left: timeLeftFor(tab).map((l) => ({ block: l.block, kind: l.kind, seconds: l.seconds, text: leftSentence(l) })),
    blocks: blockNames,
    dark,
  };
}

// darkExempt asks the daemon to leave a site out of dark pages, or to show
// it dark again (owner, 2026-11-16: "Exempt from dark pages").
async function darkExempt(url, exempt) {
  try {
    const response = await fetch("http://127.0.0.1:47613/dark-exempt", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-KC9K-Extension": "1" },
      body: JSON.stringify({ url, exempt }),
    });
    if (!response.ok) {
      return { ok: false };
    }
    const answer = await response.json();
    await poll();
    return { ok: true, site: answer.site, exempt: answer.exempt };
  } catch (error) {
    return { ok: false };
  }
}

async function addSite(block, url) {
  try {
    const response = await fetch("http://127.0.0.1:47613/add", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-KC9K-Extension": "1" },
      body: JSON.stringify({ block, url }),
    });
    if (!response.ok) {
      return { ok: false };
    }
    const answer = await response.json();
    poll();
    return { ok: true, site: answer.site };
  } catch (error) {
    return { ok: false };
  }
}

browser.storage.local.get("rules").then((stored) => {
  if (stored.rules) {
    apply(stored.rules);
  }
  poll();
  setInterval(poll, POLL_MS);
});

// Statistics: every 5 seconds, count the site in the active tab of the focused
// window, unless the user has been idle for 3 minutes, and every site playing
// sound, and send the totals to the daemon every 30 seconds. The daemon drops them if statistics are off.
const USAGE_URL = "http://127.0.0.1:47613/usage";
let statsSkipPrivate = false;
const SAMPLE_SECONDS = 5;
const REPORT_MS = 30000;
let usage = {};

async function sampleUsage() {
  try {
    // Use (§3.7): the tab in front while the user is active, and any tab
    // playing sound, in the background too. A site counts once a sample.
    const tabs = [];
    if (await userActive()) {
      const tab = await frontTab();
      if (tab) {
        tabs.push(tab);
      }
    }
    for (const tab of await browser.tabs.query({ audible: true })) {
      tabs.push(tab);
    }
    const sites = new Set();
    for (const tab of tabs) {
      if (tab.incognito && statsSkipPrivate) {
        continue; // §4.9: private windows left out when asked
      }
      if (tab.url && /^https?:/i.test(tab.url)) {
        sites.add(new URL(tab.url).hostname.replace(/^www\./, ""));
      }
    }
    for (const site of sites) {
      usage[site] = (usage[site] || 0) + SAMPLE_SECONDS;
    }
  } catch (error) {
    // A closed window or tab between the calls: skip this sample.
  }
}

async function reportUsage() {
  if (Object.keys(usage).length === 0) {
    return;
  }
  const sites = usage;
  usage = {};
  try {
    await fetch(USAGE_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-KC9K-Usage": "1" },
      body: JSON.stringify({ sites }),
    });
  } catch (error) {
    // The daemon is down: this report is lost, which only costs statistics.
  }
}

setInterval(sampleUsage, SAMPLE_SECONDS * 1000);
setInterval(reportUsage, REPORT_MS);

// Quota sessions: while a block's quota session is open, its lists come as
// rules.quota, only to count the time its websites are in use. Every 5
// seconds, a block counts once if one of its sites is in the focused tab
// (unless the user is idle), or plays sound, or plays a video even muted
// (content scripts report those). The daemon spends that time.
const QUOTA_REPORT_MS = 10000;
let quotaLists = [];
let strictUse = false;
let quotaUse = {};
const playing = new Map(); // tab id -> when its content script last said it plays

async function sampleQuota() {
  if (quotaLists.length === 0) {
    return;
  }
  const tabs = new Map();
  try {
    if (strictUse) {
      // Strict use counting (§3.7): every open tab counts.
      for (const tab of await browser.tabs.query({})) {
        tabs.set(tab.id, tab);
      }
    } else if (await userActive()) {
      const tab = await frontTab();
      if (tab) {
        tabs.set(tab.id, tab);
      }
    }
    for (const tab of await browser.tabs.query({ audible: true })) {
      tabs.set(tab.id, tab);
    }
    for (const [id, at] of playing) {
      if (Date.now() - at > 3 * SAMPLE_SECONDS * 1000) {
        playing.delete(id);
        continue;
      }
      try {
        tabs.set(id, await browser.tabs.get(id));
      } catch (error) {
        playing.delete(id);
      }
    }
  } catch (error) {
    // A window or tab closed between the calls: count what was found.
  }
  const blocks = new Set();
  for (const tab of tabs.values()) {
    if (tab.url && /^https?:/i.test(tab.url)) {
      for (const match of kc9kListMatches(quotaLists, { url: tab.url, title: tab.title || "", text: "" })) {
        blocks.add(match.list.block);
      }
    }
  }
  for (const block of blocks) {
    quotaUse[block] = (quotaUse[block] || 0) + SAMPLE_SECONDS;
  }
}

async function reportQuota() {
  if (Object.keys(quotaUse).length === 0) {
    return;
  }
  const quota = quotaUse;
  quotaUse = {};
  try {
    await fetch(USAGE_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-KC9K-Usage": "1" },
      body: JSON.stringify({ sites: {}, quota }),
    });
  } catch (error) {
    // The daemon is down: this time isn't spent.
  }
}

setInterval(sampleQuota, SAMPLE_SECONDS * 1000);
setInterval(reportQuota, QUOTA_REPORT_MS);

// No downloads (§3.9): a download is cancelled when the page it comes from
// (its referrer, by address or by the hits its text reported) or its own
// address has a list with the no-downloads action.
const downloadPages = new Map(); // page address -> actions its text matched
// Kept in session storage too: Chrome stops this service worker after a
// while without events, and a page reports its hits only once, so a
// download after that would otherwise go through.
const DOWNLOAD_PAGES_KEY = "kc9kDownloadPages";
const downloadStore = (browser.storage && browser.storage.session) || null;
const downloadPagesLoaded = downloadStore
  ? downloadStore.get(DOWNLOAD_PAGES_KEY).then((stored) => {
    for (const url of (stored && stored[DOWNLOAD_PAGES_KEY]) || []) {
      if (typeof url === "string") {
        downloadPages.set(url, true);
      }
    }
  }).catch(() => {})
  : Promise.resolve();
function noteDownloadPage(url, actions) {
  if (typeof url !== "string" || !Array.isArray(actions) || !actions.includes("no-downloads")) {
    return;
  }
  downloadPages.set(url, true);
  if (downloadPages.size > 200) {
    downloadPages.delete(downloadPages.keys().next().value);
  }
  if (downloadStore) {
    downloadStore.set({ [DOWNLOAD_PAGES_KEY]: Array.from(downloadPages.keys()) }).catch(() => {});
  }
}
function downloadBlocked(item) {
  for (const url of [item.referrer, item.url, item.finalUrl]) {
    if (typeof url !== "string" || !url) {
      continue;
    }
    if (downloadPages.has(url) || kc9kListActions(lists, { url })["no-downloads"]) {
      return true;
    }
  }
  return false;
}
if (browser.downloads && browser.downloads.onCreated) {
  browser.downloads.onCreated.addListener((item) => {
    downloadPagesLoaded.then(() => {
      if (!downloadBlocked(item)) {
        return;
      }
      // A small file may be done before it can be cancelled: then its file
      // goes instead.
      browser.downloads.cancel(item.id)
        .catch(() => browser.downloads.removeFile(item.id))
        .then(() => browser.downloads.erase({ id: item.id }))
        .catch(() => {});
    });
  });
}
