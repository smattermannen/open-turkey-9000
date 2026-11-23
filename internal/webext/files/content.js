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
const LOOKS = ["greyscale", "no-images", "mute", "no-video", "dark"];
// What dark turns back over the turned-over page so it shows as it is:
// pictures (not SVG ones), videos, frames.
const DARK_MEDIA = "img:not([src*=\".svg\" i]):not([src^=\"data:image/svg\" i]), video, canvas, iframe, " +
  "svg image, [style*=\"background-image\"]";
// Whether the page at lightFor has a light background (dark only acts on those).
let lightFor = null;
let light = true;
let noVideo = false;
let noImages = false; // see scanPseudoImages
const PSEUDO_ATTR = "data-kc9k-noimg";
// no-images: in a shadow root (where the page's own sheets don't reach) as
// is, in the document with :root before the marks so they beat the page's.
const NO_IMAGES_CSS = "img, picture, video, canvas, svg image, svg use, object, embed, input[type=\"image\"] " +
  "{ visibility: hidden !important; }\n" +
  "*, *::before, *::after { background-image: none !important; list-style-image: none !important; " +
  "border-image-source: none !important; }\n" +
  "[data-kc9k-noimg~=\"before\"]::before, [data-kc9k-noimg~=\"after\"]::after { content: \"\" !important; }\n" +
  "[data-kc9k-noimg~=\"self\"] { content: normal !important; }\n";
const NO_IMAGES_PAGE_CSS = NO_IMAGES_CSS.replace(/\[data-kc9k-noimg/g, ":root [data-kc9k-noimg");
// The looks the background script says this tab has now (no-video,
// no-images, greyscale): a frame can't see the tab's title or text, so it learns them
// from there, when it starts and whenever they change.
let tabLooks = new Set();

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
const adHosts = new Set(); // other sites this page loaded from (see noteResource)
// Only so much of a page's text is read, so a huge page can't slow it down.
const MAX_TEXT = 2000000;
// Principle 9: the page is read whole once per address, then only what is
// added to it (pageText grows), in idle time; reading stops once the page
// is taken away or every list has done all it can (readingDone).
let pageText = "";
let readWhole = false;
let addedNodes = new Set();
let readingDone = false;

// R-PAGE-1: no flashing. While a list or phrase set could still act on the
// top page by its title or text (and no address exception spares it), the
// page is kept invisible until it is judged, so a page that is blocked or
// given a look never shows without it first. Invisible means opacity 0 on
// what the root holds, not on the root: its text is still laid out and read
// (visibility: hidden would hide it from innerText), and the page's own
// background stays (the root at opacity 0 takes the canvas with it, so every
// page flashed white while judged, dark ones too). Judged means the first full read once the page has settled:
// loaded, or SETTLE_MS after its document is ready, or SETTLE_MS after an
// address change without loading (its new content comes in by then). A page
// that is taken away stays hidden until the block page replaces it; nothing
// stays hidden longer than HIDE_CAP_MS.
const HIDE_ID = "kc9k-hide";
const HIDE_CAP_MS = 3000;
const SETTLE_MS = 500;
let hidden = false;
let hideCap = null;
let settleAt = 0; // when the page may be shown, once judged (0: not yet known)
let takenAwayFor = null;

// Reading lays the page out, and a page laid out before its stylesheets
// apply gets painted at once: unstyled, on a white canvas, dark sites too
// (Firefox; without reading it waits for them). So nothing is read until
// the body is there and the page's stylesheets have loaded or failed, or
// the page is complete, or SHEETS_WAIT_MS have passed since it started.
const SHEETS_WAIT_MS = 2000;
const startedAt = Date.now();
const sheetsIn = new WeakSet();
function sheetIn(event) {
  if (event.target && event.target.localName === "link") {
    sheetsIn.add(event.target);
    textChanged = true;
    queueScan();
  }
}
document.addEventListener("load", sheetIn, true);
document.addEventListener("error", sheetIn, true);

function styled() {
  if (document.readyState === "complete" || Date.now() - startedAt > SHEETS_WAIT_MS) {
    return true;
  }
  if (!document.body) {
    return false;
  }
  for (const link of document.querySelectorAll("link[rel~='stylesheet' i]")) {
    if (!sheetsIn.has(link) && !link.disabled && !/\balternate\b/i.test(link.rel) &&
      (!link.media || matchMedia(link.media).matches)) {
      return false;
    }
  }
  return true;
}

// judgedHere reports whether this document's address is one the lists
// judge: web pages, and local files while the rules say so (§3.1).
function judgedHere() {
  return /^https?:$/.test(location.protocol) || (location.protocol === "file:" && kc9kJudgedAddress(lists, location.href));
}

function hidePage() {
  if (window !== window.top || !judgedHere()) {
    return;
  }
  if (!hidden) {
    hidden = true;
    clearTimeout(hideCap);
    hideCap = setTimeout(showPage, HIDE_CAP_MS);
  }
  ensureHidden();
}

// ensureHidden puts the hiding style back if the page threw it out.
function ensureHidden() {
  const root = document.documentElement;
  if (!hidden || !root || document.getElementById(HIDE_ID)) {
    return;
  }
  const style = document.createElement("style");
  style.id = HIDE_ID;
  style.textContent = "html > :not(head) { opacity: 0 !important; pointer-events: none !important; }";
  root.appendChild(style);
}

function showPage() {
  hidden = false;
  clearTimeout(hideCap);
  const style = document.getElementById(HIDE_ID);
  if (style) {
    style.remove();
  }
}

// mayActByText reports whether a list could still act on the page at url by
// what it shows: one that reads the title or text (or whose exceptions do),
// not in test mode, and not spared by an address exception.
function mayActByText(url) {
  const file = /^file:/i.test(url || "");
  return Boolean(lists) && lists.some((list) => !list.test && kc9kReadsPage([list]) &&
    (!file || (list.filePages && !kc9kFileMatch(list.except.files, url))) &&
    !(list.except.url.match.length > 0 && kc9kMatches(list.except.url, url)) &&
    !kc9kUrlTermsMatch(list.except.urlTerms, url));
}

// showWhenJudged shows a hidden page once it is judged and settled, or reads
// it again when it settles.
function showWhenJudged(url) {
  if (!hidden || takenAwayFor === url) {
    return;
  }
  const wait = settleAt ? settleAt - Date.now() : SETTLE_MS;
  if (wait <= 0) {
    showPage();
    return;
  }
  setTimeout(() => {
    textChanged = true;
    queueScan();
  }, wait + 10);
}

// The page itself starts hidden until the lists are known (a few ms); load
// shows it at once when no list reads pages.
if (window === window.top) {
  hidePage();
  if (document.readyState !== "loading") {
    settleAt = Date.now();
  }
  document.addEventListener("DOMContentLoaded", () => {
    if (!settleAt) {
      settleAt = Date.now() + SETTLE_MS;
    }
  });
  // Loaded isn't settled: scripts often write the text in just after
  // (ci/flash_test.py "late"), so the page waits SETTLE_MS after its
  // document is ready either way.
  window.addEventListener("load", () => {
    textChanged = true;
    queueScan();
  });
}

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
  enterAddress(url);
  const byAddress = kc9kListActions(lists, { url });
  const on = (name) => Boolean(byAddress[name]) || keywordLooks.has(name) || tabLooks.has(name);
  shownFor = url;
  const wasNoVideo = noVideo;
  noVideo = on("no-video");
  if (noVideo && !wasNoVideo) {
    stopAllMedia();
  }
  let css = "";
  const filters = [];
  const dark = (on("dark") || darkPage(url)) && pageIsLight();
  // The background script's user sheet greys pages, except those marked
  // dark: they get both filters here.
  if (document.documentElement) {
    document.documentElement.toggleAttribute("data-kc9k-dark", dark);
  }
  if (dark) {
    // Turn the page over, then turn pictures and videos back, once: one
    // inside another that is turned back already (a <picture>'s img, a
    // picture over a background image) would be turned over again. SVG
    // pictures are mostly logos and drawings, dark lines on nothing: they
    // turn with the page, or a black wordmark vanishes on the dark page.
    filters.push("invert(1) hue-rotate(180deg)");
    css += DARK_MEDIA + " { filter: invert(1) hue-rotate(180deg) !important; }\n" +
      "[style*=\"background-image\"] :is(" + DARK_MEDIA + ") { filter: none !important; }\n" +
      "html { background: #fff !important; }\n";
  }
  if (on("greyscale")) {
    filters.push("grayscale(1)");
  }
  if (filters.length > 0) {
    css += "html { filter: " + filters.join(" ") + " !important; }\n";
  }
  // The background script also draws no-images and no-video with a user
  // style sheet no page style can beat; this is the same in case that one
  // isn't in yet.
  const wasNoImages = noImages;
  noImages = on("no-images");
  if (noImages) {
    css += NO_IMAGES_PAGE_CSS;
    if (!wasNoImages) {
      pseudoRescanAfter = 1000;
      scanPseudoImages(document);
    }
  } else if (wasNoImages) {
    uncoverShadows();
  }
  if (noVideo) {
    css += "video, audio, object, embed, iframe[src*=\"youtube\"], iframe[src*=\"vimeo\"], " +
      "iframe[src*=\"player\"], iframe[src*=\"embed\"] { display: none !important; }\n";
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
    const parent = document.head || document.documentElement;
    if (!parent) {
      // Nothing to put it in yet: once the document has its elements.
      document.addEventListener("DOMContentLoaded", render, { once: true });
      return;
    }
    style = document.createElement("style");
    style.id = STYLE_ID;
    parent.appendChild(style);
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

// Search results leading to an address the lists take away are taken off
// the page (R-PAGE-2, search.js): judged when the lists are known, before
// the page shows, and again each time the page adds links (before it
// paints them).
const searchEngine = window === window.top ? kc9kSearchEngine(location.href) : null;
let searchResults = null;

function judgeResults() {
  if (!searchEngine || !lists) {
    return;
  }
  if (!searchResults) {
    searchResults = kc9kSearchResults(document, searchEngine, location.href,
      (url) => kc9kTakesAway(kc9kOutcome(kc9kListMatches(lists, { url }))));
  }
  searchResults.judge();
}

if (searchEngine) {
  const addsLinks = (node) => node.nodeType === 1 && (node.matches("a[href]") || node.querySelector("a[href]") !== null);
  new MutationObserver((records) => {
    if (lists && records.some((r) => [...r.addedNodes].some(addsLinks))) {
      judgeResults();
    }
  }).observe(document, { childList: true, subtree: true });
}

function load(page) {
  wordsMode = (page && page.words) || "";
  social = (page && page.social) || [];
  darkPages = (page && page.dark) || null;
  lists = kc9kCompileLists(page && page.lists);
  quotaOpen = Boolean(page && page.quota);
  textChanged = true;
  if (searchResults) {
    searchResults.forget();
  }
  judgeResults();
  if (window === window.top) {
    const url = topUrl();
    if (!mayActByText(url)) {
      showPage();
    } else if (judgedFor !== url) {
      hidePage();
    }
  }
  render();
  scanKeywords();
}

// scanKeywords reads the page's title and text (in the top frame only: a
// frame's text is part of what the page shows) and applies the lists,
// reporting each list action that newly applies to the background script.
// Blocking goes through the background script, which sends the tab to the
// block page and also hides images in the page's frames.
// A page is judged in pieces too (principle 9): kc9kListMatchesSteps runs
// READ_SLICE_MS at a time in idle time, the page staying hidden meanwhile
// where a list could act on it (R-PAGE-1).
let judging = null; // { url, lists, steps } while a page is judged

function scanKeywords() {
  if (window !== window.top) {
    scanFrame();
    return;
  }
  if (!lists) {
    return;
  }
  if (judging) {
    judgeOn();
    return;
  }
  if (!textChanged || !styled()) {
    return; // not styled yet: textChanged stays, read again soon
  }
  const body = document.body || document.documentElement;
  if (!body) {
    return;
  }
  textChanged = false;
  const url = location.href;
  enterAddress(url);
  const testing = lists.some((list) => list.test);
  if (readingDone && !testing) {
    showWhenJudged(url);
    return;
  }
  // Lists that only look at addresses need no reading: this still catches a
  // page that changed address without loading (the background script only
  // sees real loads).
  let page = { url, adHosts: [...adHosts] };
  if (kc9kReadsPage(lists) || testing) {
    const text = readPageText(body);
    if (text === null) {
      // The first read isn't finished: go on in the next idle time.
      textChanged = true;
      queueScan();
      return;
    }
    page = { url, title: document.title || "", text, adHosts: page.adHosts };
  }
  if (testing) {
    showTestPanel(kc9kTestReport(lists, page));
  } else {
    hideTestPanel();
  }
  judging = { url, lists, steps: kc9kListMatchesSteps(lists, page) };
  judgeOn();
}

// judgeOn judges the page for a piece of idle time, and acts once it is
// done; a page that changed address or rules meanwhile is read again.
function judgeOn() {
  const j = judging;
  if (j.url !== keywordsFor || j.url !== location.href || j.lists !== lists) {
    judging = null;
    textChanged = true;
    queueScan();
    return;
  }
  const until = performance.now() + READ_SLICE_MS;
  let r;
  do {
    r = j.steps.next();
  } while (!r.done && performance.now() < until);
  if (!r.done) {
    queueScan(); // on in the next idle time
    return;
  }
  judging = null;
  judged(j.url, r.value);
}

// judged acts on what judging a page's address, title and text found.
function judged(url, matches) {
  // Nothing more to learn once the page is taken away, or once every list
  // that reads it has reported all its actions.
  const takenAway = kc9kTakesAway(kc9kOutcome(matches));
  if (takenAway) {
    takenAwayFor = url; // stays hidden until the block page replaces it
  }
  if (takenAway || lists.every((list) =>
    list.test || list.actions.every((a) => reportedHits.has(list.id + " " + a.action) ||
      matches.some((m) => m.list.id === list.id && m.actions.some((x) => x.action === a.action))))) {
    readingDone = true;
  }
  const fresh = [];
  const looksBefore = keywordLooks.size;
  for (const match of matches) {
    const names = match.actions.map((action) => action.action);
    names.filter((name) => LOOKS.includes(name)).forEach((name) => keywordLooks.add(name));
    const unseen = names.filter((name) => !reportedHits.has(match.list.id + " " + name));
    if (unseen.length > 0) {
      unseen.forEach((name) => reportedHits.add(match.list.id + " " + name));
      fresh.push({ id: match.list.id, actions: unseen });
    }
  }
  if (keywordLooks.size !== looksBefore) {
    render();
  }
  if (fresh.length > 0) {
    browser.runtime.sendMessage({ type: "page-hits", url, matches: fresh }).catch(() => {});
  }
  if (judgedFor !== url) {
    // The first full read of this address is done: the looks the last
    // address earned (kept meanwhile) can go.
    judgedFor = url;
    browser.runtime.sendMessage({ type: "page-judged", url }).catch(() => {});
  }
  showWhenJudged(url);
}

// R-LIST-4: each frame is judged on its own text (the top page's text holds
// none of its frames'). A frame's matches never act on the tab: what would
// take a page away gives the frame the block page (the background script
// decides, from its own rules), its looks show in the frame. No events:
// embedded frames don't count (R-EVENT-1).
let frameJudging = null; // { url, lists, steps } while a frame is judged
let frameBlocked = false;

function scanFrame() {
  if (!lists || frameBlocked || !judgedHere() || !document.body || !kc9kReadsPage(lists) || !styled()) {
    return;
  }
  enterAddress(topUrl()); // as render() keeps it: a frame's looks follow the tab's address
  if (!frameJudging) {
    if (!textChanged) {
      return;
    }
    const text = readPageText(document.body);
    if (text === null) {
      queueScan(); // the first read isn't finished
      return;
    }
    textChanged = false;
    frameJudging = { url: location.href, lists,
      steps: kc9kListMatchesSteps(lists, { url: location.href, title: document.title || "", text }) };
  }
  const j = frameJudging;
  if (j.url !== location.href || j.lists !== lists) {
    frameJudging = null;
    textChanged = true;
    queueScan();
    return;
  }
  const until = performance.now() + READ_SLICE_MS;
  let r;
  do {
    r = j.steps.next();
  } while (!r.done && performance.now() < until);
  if (!r.done) {
    queueScan();
    return;
  }
  frameJudging = null;
  const matches = r.value;
  const looksBefore = keywordLooks.size;
  for (const match of matches) {
    match.actions.map((a) => a.action).filter((name) => LOOKS.includes(name)).forEach((name) => keywordLooks.add(name));
  }
  if (keywordLooks.size !== looksBefore) {
    render();
  }
  if (kc9kTakesAway(kc9kOutcome(matches))) {
    browser.runtime.sendMessage({ type: "frame-hits", url: j.url,
      matches: matches.map((m) => ({ id: m.list.id, actions: m.actions.map((a) => a.action) })) })
      .then((answer) => {
        if (answer && typeof answer.blocked === "string" && location.href === j.url) {
          frameBlocked = true;
          location.replace(answer.blocked);
        }
      }).catch(() => {});
  }
}

// enterAddress starts over for a new address of the top page: the one place
// that forgets what the last address earned, so rendering and reading can't
// disagree about which page they are on. (Rendering used to forget the
// looks alone, and reading then thought the page was done: a page reached
// by a click inside a site never got its looks back.)
let judgedFor = null;
function enterAddress(url) {
  if (keywordsFor === url) {
    return;
  }
  if (keywordsFor !== null && window === window.top && mayActByText(url)) {
    // An address changed without loading (a click inside YouTube, Reddit):
    // hidden until the new content is judged.
    settleAt = Date.now() + SETTLE_MS;
    hidePage();
  }
  keywordsFor = url;
  keywordLooks = new Set();
  reportedHits = new Set();
  readWhole = false;
  readingDone = false;
  wholeRead = null;
  textChanged = true;
}

// readPageText returns the page's rendered text: all of it the first time
// for an address, then the text so far plus the visible text of what was
// added since, so a feed that keeps loading isn't read again and again.
// Text changed in place (not added) is caught by reading the whole page
// again, at most this often.
const REREAD_MS = 30000;
let readWholeAt = 0;

// A whole read goes in pieces of at most READ_SLICE_MS (principle 9: no
// single piece of work in a page over 16 ms): readPageText returns null
// until it is done. A container whose children are all elements is opened
// up; anything with text of its own is read whole, so sentences stay whole.
const READ_SLICE_MS = 8;
let wholeRead = null; // { queue: [nodes], parts: [strings] } while reading

function readPageText(body) {
  if (!wholeRead && (!readWhole || (Date.now() - readWholeAt > REREAD_MS && addedNodes.size === 0))) {
    readWhole = true; // the observer collects additions from now on
    readWholeAt = Date.now();
    addedNodes = new Set();
    wholeRead = { queue: Array.from(body.childNodes), parts: [], length: 0 };
  }
  if (wholeRead) {
    const until = performance.now() + READ_SLICE_MS;
    const w = wholeRead;
    while (w.queue.length > 0 && w.length < MAX_TEXT) {
      if (performance.now() > until) {
        return null;
      }
      const node = w.queue.shift();
      if (node.nodeType === Node.TEXT_NODE) {
        const parent = node.parentElement;
        if (node.textContent.trim() && parent && parent.getClientRects().length > 0) {
          w.parts.push(node.textContent);
          w.length += node.textContent.length;
        }
        continue;
      }
      if (node.nodeType !== Node.ELEMENT_NODE || node.id === PANEL_ID ||
        ["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE"].includes(node.tagName)) {
        continue;
      }
      const ownText = Array.from(node.childNodes).some((c) => c.nodeType === Node.TEXT_NODE && c.textContent.trim());
      if (!ownText && node.childElementCount > 1) {
        w.queue.unshift(...node.childNodes); // open it up, in order
        continue;
      }
      // Only what is shown (R-LIST-4): innerText of an element that isn't
      // rendered would be all of its text.
      if (node.getClientRects().length === 0 && getComputedStyle(node).display !== "contents") {
        continue;
      }
      const text = node.innerText;
      if (text && text.trim()) {
        w.parts.push(text);
        w.length += text.length;
      }
    }
    pageText = w.parts.join("\n").slice(0, MAX_TEXT);
    wholeRead = null;
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
  add(box, "h1", "Test mode");
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
  if (window === window.top && lists) {
    ensureHidden();
    if (location.href !== keywordsFor && keywordsFor !== null) {
      // The page moved to another address without loading: hide before the
      // new content is painted (this runs before the next frame).
      render();
    }
  }
  if (readWhole && !readingDone && addedNodes.size < 5000) {
    for (const record of records) {
      for (const node of record.addedNodes) {
        addedNodes.add(node);
      }
    }
  }
  if (noImages) {
    for (const record of records) {
      for (const node of record.addedNodes) {
        if (node.nodeType === 1 && pseudoWalkers.length < 500) {
          scanPseudoImages(node);
        }
      }
    }
  }
}).observe(document.documentElement || document, { childList: true, subtree: true, characterData: true });
// §3.9 ad trigger for lists whose block has exceptions by title or text
// (kc9kAdsByPage): the background can't know those when an ad loads, so the
// page's own judging counts the other sites it loaded from (resource
// timing, adHosts), with every exception. A host one of them lists reads
// the page again.
function noteResource(name) {
  let host;
  try {
    host = new URL(name).hostname.toLowerCase();
  } catch (error) {
    return;
  }
  const own = location.hostname.toLowerCase();
  if (!host || host === own || host.endsWith("." + own) || adHosts.has(host) || adHosts.size >= 2000) {
    return;
  }
  adHosts.add(host);
  if (lists && lists.some((list) => kc9kAdsByPage(list) && kc9kAdHost(list, host))) {
    textChanged = true;
    queueScan();
  }
}
if (window === window.top && typeof PerformanceObserver === "function") {
  try {
    new PerformanceObserver((entries) => entries.getEntries().forEach((e) => noteResource(e.name)))
      .observe({ type: "resource", buffered: true });
  } catch (error) {
    // no resource timing here: the background's ad trigger alone
  }
}

let scanQueued = false;
function queueScan() {
  if (scanQueued) {
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
    setTimeout(run, 0);
  }
}
setInterval(() => {
  if (textChanged) {
    queueScan();
  }
}, 250);
// The first read waits for idle time (principle 9): read at once, its
// first visibility check made the browser lay out a whole big page inside
// this event, holding the page up; in idle time that layout is done. The
// page stays hidden meanwhile (R-PAGE-1).
document.addEventListener("DOMContentLoaded", () => {
  textChanged = true;
  queueScan();
});

browser.storage.local.get("page").then((stored) => load(stored.page));
function setTabLooks(looks) {
  tabLooks = new Set(Array.isArray(looks) ? looks.filter((name) => LOOKS.includes(name)) : []);
  render();
}
browser.runtime.sendMessage({ type: "tab-looks" }).then(setTabLooks, () => {});
browser.runtime.onMessage.addListener((message) => {
  if (message && message.type === "tab-looks") {
    setTabLooks(message.looks);
  }
});
browser.storage.onChanged.addListener((changes) => {
  if (changes.page) {
    load(changes.page.newValue);
  }
});

// Word recording (§3.12): while the rules say so, the top frame reports
// its page's rendered words once per address, a few seconds after it
// shows (so what loads late is in). Shown text only: innerText leaves out
// what isn't rendered.
let wordsMode = "";
let wordsSentFor = "";
let wordsSince = { url: "", at: 0 };
setInterval(() => {
  if (!wordsMode || window !== window.top || !document.body) {
    return;
  }
  const url = location.href;
  if (wordsSince.url !== url) {
    wordsSince = { url, at: Date.now() };
    return;
  }
  if (wordsSentFor === url || Date.now() - wordsSince.at < 4000) {
    return;
  }
  wordsSentFor = url;
  const words = kc9kPageWords((document.title || "") + "\n" + document.body.innerText.slice(0, 500000), 400);
  if (Object.keys(words).length > 0) {
    browser.runtime.sendMessage({ type: "page-words", mode: wordsMode, words }).catch(() => {});
  }
}, 1000);

// Sites that change pages without reloading (YouTube, Reddit) keep this
// script, so follow their address.
setInterval(() => {
  if (lists && topUrl() !== shownFor) {
    textChanged = true;
    render();
    scanKeywords();
  }
}, 1000);

// no-images covers every image (§3.9), also one a style sheet draws as
// content: ::before/::after { content: url(x.gif) }, or an element's own
// content. No selector picks only those, so on a no-images page every
// element is looked at in idle time (READ_SLICE_MS a piece, shadow roots
// too; again for added content, and after 1, 2, 4 s, then every 10 s, for
// shadow roots attached later and style changes) and marked; the style sheets (here and the background script's user sheet)
// then empty that content. Marks stay: without no-images they do nothing.
let pseudoWalkers = []; // [walker, root element still to look at, or null]
let pseudoRunning = false;
let pseudoScannedAt = 0;
let pseudoRescanAfter = 1000;

function drawsImage(content) {
  return typeof content === "string" && /(url|image-set|cross-fade|gradient|image)\(/i.test(content);
}

function markPseudoImage(el) {
  let marks = "";
  try {
    for (const [pseudo, mark] of [["::before", "before"], ["::after", "after"], [null, "self"]]) {
      if (drawsImage(getComputedStyle(el, pseudo).content)) {
        marks += " " + mark;
      }
    }
  } catch (error) {
    return;
  }
  const had = el.getAttribute(PSEUDO_ATTR) || "";
  for (const mark of had.split(" ")) {
    // Emptied by the sheet, a marked part reads "" now: keep its mark.
    if (mark && !marks.includes(mark)) {
      marks += " " + mark;
    }
  }
  marks = marks.trim();
  if (marks && marks !== had) {
    el.setAttribute(PSEUDO_ATTR, marks);
  }
}

function scanPseudoImages(root) {
  pseudoWalkers.push([document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT), root.nodeType === 1 ? root : null]);
  if (root === document) {
    pseudoScannedAt = Date.now();
  }
  if (!pseudoRunning) {
    pseudoRunning = true;
    whenIdle(pseudoSlice);
  }
}

function whenIdle(fn) {
  if (typeof requestIdleCallback === "function") {
    requestIdleCallback(fn, { timeout: 500 });
  } else {
    setTimeout(fn, 0);
  }
}

function pseudoSlice() {
  const until = performance.now() + READ_SLICE_MS;
  let n = 0;
  while (noImages && pseudoWalkers.length > 0) {
    const top = pseudoWalkers[pseudoWalkers.length - 1];
    let el = top[1];
    if (el) {
      top[1] = null;
    } else if (!(el = top[0].nextNode())) {
      pseudoWalkers.pop();
      continue;
    }
    markPseudoImage(el);
    const shadow = shadowOf(el);
    if (shadow) {
      coverShadow(shadow);
      pseudoWalkers.push([document.createTreeWalker(shadow, NodeFilter.SHOW_ELEMENT), null]);
    }
    if ((++n & 15) === 0 && performance.now() >= until) {
      break;
    }
  }
  if (noImages && pseudoWalkers.length > 0) {
    whenIdle(pseudoSlice);
    return;
  }
  pseudoRunning = false;
  if (!noImages) {
    pseudoWalkers = [];
  }
}

// A shadow root gets the no-images rules as its own sheet while no-images
// is on (the background script's user sheet reaches it too, where it can).
const shadowSheets = new Map(); // shadow root → the sheet given to it
function coverShadow(shadow) {
  if (shadowSheets.has(shadow) || typeof CSSStyleSheet !== "function") {
    return;
  }
  try {
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(NO_IMAGES_CSS);
    shadow.adoptedStyleSheets = [...shadow.adoptedStyleSheets, sheet];
    shadowSheets.set(shadow, sheet);
  } catch (error) {
    // Not allowed here (an older browser): the user sheet covers it.
  }
}
function uncoverShadows() {
  for (const [shadow, sheet] of shadowSheets) {
    try {
      shadow.adoptedStyleSheets = shadow.adoptedStyleSheets.filter((s) => s !== sheet);
    } catch (error) {
      // gone with its page part
    }
  }
  shadowSheets.clear();
}

setInterval(() => {
  if (noImages && !pseudoRunning && Date.now() - pseudoScannedAt >= pseudoRescanAfter) {
    pseudoRescanAfter = Math.min(2 * pseudoRescanAfter, 10000);
    scanPseudoImages(document);
  }
}, 500);

// no-video: nothing plays. A video that starts is stopped and muted at
// once (also inside shadow roots, which the page's events don't leave), and
// every second whatever plays is stopped.
function shadowOf(el) {
  try {
    if (typeof chrome !== "undefined" && chrome.dom && chrome.dom.openOrClosedShadowRoot) {
      return chrome.dom.openOrClosedShadowRoot(el);
    }
    return el.openOrClosedShadowRoot || el.shadowRoot || null;
  } catch (error) {
    return null;
  }
}
const watchedRoots = new WeakSet();
function stopMedia(event) {
  const m = event.target;
  if (noVideo && m && typeof m.pause === "function" && /^(VIDEO|AUDIO)$/.test(m.tagName || "")) {
    m.muted = true;
    m.pause();
  }
}
function watchRoot(root) {
  if (!watchedRoots.has(root)) {
    watchedRoots.add(root);
    for (const type of ["play", "playing", "loadeddata"]) {
      root.addEventListener(type, stopMedia, true);
    }
  }
}
watchRoot(document);
function allMedia(root, found) {
  for (const el of root.querySelectorAll("*")) {
    if (el.tagName === "VIDEO" || el.tagName === "AUDIO") {
      found.push(el);
    }
    const shadow = shadowOf(el);
    if (shadow) {
      watchRoot(shadow);
      allMedia(shadow, found);
    }
  }
  return found;
}
function stopAllMedia() {
  for (const m of allMedia(document, [])) {
    if (!m.paused || !m.muted) {
      m.muted = true;
      m.pause();
    }
  }
}
setInterval(() => {
  if (noVideo) {
    stopAllMedia();
  }
}, 1000);

// A page shown again from the back/forward cache keeps this script's state,
// but the background script forgot what it earned: read it again.
window.addEventListener("pageshow", (event) => {
  if (event.persisted && window === window.top) {
    // Shown before it was cached, so it isn't hidden again: just re-read.
    keywordsFor = null;
    judgedFor = null;
    takenAwayFor = null;
    settleAt = Date.now();
    render();
    scanKeywords();
  }
});

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
