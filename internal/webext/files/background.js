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

async function userActive() {
  return !browser.idle || (await browser.idle.queryState(60)) === "active";
}

function addressOutcome(url) {
  return kc9kOutcome(kc9kListMatches(lists, { url }));
}

function isBlocked(url) {
  return kc9kTakesAway(addressOutcome(url));
}

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
  return details.documentUrl || details.originUrl || "";
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
// prevention, and carries out logout, shutdown and monk mode). A tab and
// address already reported in the last few seconds isn't reported again.
const HIT_URL = "http://127.0.0.1:47613/hit";
const reported = new Map();

// Where each tab's page was opened from: the page whose link led to it, as
// the request for it said. The daemon counts a blocked page opened from a
// search engine's results as looked for.
const cameFrom = new Map();

function noteOrigin(details) {
  if (details.tabId >= 0) {
    cameFrom.set(details.tabId, { url: details.url, from: details.originUrl || "" });
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
  const key = tabId + " " + url;
  const now = Date.now();
  if (now - (reported.get(key) || 0) < 10000) {
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

// Window-title keywords from the active blocks: a tab whose title contains
// one is sent to the block page, as the daemon closes other apps' windows.
let titleWords = [];

function blockTitledTab(tab) {
  if (!tab.url || !/^https?:/i.test(tab.url)) {
    return;
  }
  const outcome = kc9kOutcome(kc9kListMatches(lists, { url: tab.url, title: tab.title || "" }));
  if (kc9kTakesAway(outcome)) {
    act(tab.id, tab.url, outcome, "title");
    return;
  }
  const text = (tab.title || "").toLowerCase();
  if (titleWords.some((word) => text.includes(word))) {
    browser.tabs.update(tab.id, { url: blockedPage(tab.url, "title").redirectUrl });
  }
}

// Tabs whose page a list's title or text stripped of images, with the
// address it happened at: their images are cancelled like those of a
// no-images site, frames included, until the tab goes somewhere else.
// keywordNoVideo and keywordMute are the same for those actions.
const keywordNoImages = new Map();
const keywordNoVideo = new Map();
const keywordMute = new Map();

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

browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.url !== undefined) {
    for (const kept of [keywordNoImages, keywordNoVideo, keywordMute]) {
      if (kept.get(tabId) !== changeInfo.url) {
        kept.delete(tabId);
      }
    }
  }
  if (changeInfo.url !== undefined || changeInfo.title !== undefined || changeInfo.mutedInfo !== undefined) {
    applyMute(tab);
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
  mutedHere.delete(tabId);
  cameFrom.delete(tabId);
  trail.delete(tabId);
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
  if (!tab || sender.frameId !== 0 || typeof message.url !== "string" || !/^https?:/i.test(message.url)) {
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
  if (takes("mute")) {
    keywordMute.set(tab.id, message.url);
    applyMute(tab);
  }
  const outcome = kc9kOutcome(matches);
  if (kc9kTakesAway(outcome)) {
    // A page taken away by its address alone (one that changed address
    // without loading, or whose block has exceptions by title or text) goes
    // back when the block ends; one taken away for its title or text stays.
    act(tab.id, message.url, outcome, isBlocked(message.url) ? "" : "keyword");
  }
}

browser.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.url.startsWith("http://127.0.0.1:47613/")) {
      return {};
    }
    if (details.type === "main_frame" || details.type === "sub_frame") {
      noteVisit(details.url);
    }
    if (details.type === "image" || details.type === "imageset" || details.type === "media") {
      if (keywordNoImages.has(details.tabId) || (details.type === "media" && keywordNoVideo.has(details.tabId))) {
        return { cancel: true };
      }
      const looks = kc9kListActions(lists, { url: pageUrl(details) });
      return looks["no-images"] || (details.type === "media" && looks["no-video"]) ? { cancel: true } : {};
    }
    if (details.type === "main_frame") {
      noteOrigin(details);
    }
    const outcome = addressOutcome(details.url);
    if (kc9kTakesAway(outcome)) {
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
      const action = kc9kSafeSearch(details.url, form);
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
  { urls: ["<all_urls>"], types: ["main_frame", "sub_frame", "image", "imageset", "media"] },
  ["blocking", "requestBody"]
);

// SafeSearch on YouTube: the header Google honours for Restricted Mode.
browser.webRequest.onBeforeSendHeaders.addListener(
  (details) => {
    if (!safeSearch) {
      return {};
    }
    const headers = (details.requestHeaders || []).filter((h) => h.name.toLowerCase() !== "youtube-restrict");
    headers.push({ name: "YouTube-Restrict", value: "Strict" });
    return { requestHeaders: headers };
  },
  { urls: ["*://youtube.com/*", "*://*.youtube.com/*", "*://*.youtube-nocookie.com/*", "*://youtubei.googleapis.com/*"] },
  ["blocking", "requestHeaders"]
);

// stillBlocked tells the block page whether its site is still blocked, so it
// can go back to the site once the block ends.
function stillBlocked(url) {
  if (isBlocked(url) || kc9kPlaylistBlocks(social, url)) {
    return true;
  }
  if (safeSearch) {
    const action = kc9kSafeSearch(url, null);
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

browser.runtime.onMessage.addListener((message, sender) => {
  if (message && message.type === "still-blocked" && typeof message.url === "string") {
    return Promise.resolve(stillBlocked(message.url));
  }
  if (message && message.type === "page-hits") {
    pageHit(message, sender);
  }
  if (message && message.type === "mirror") {
    mirrorHit(message, sender);
  }
  if (message && message.type === "playlist-violation" && sender && sender.tab && sender.frameId === 0 &&
    typeof message.url === "string" && kc9kPlaylists(social).length > 0) {
    browser.tabs.update(sender.tab.id, { url: blockedPage(message.url, "playlist").redirectUrl });
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
  if (message && message.type === "playing" && sender && sender.tab) {
    playing.set(sender.tab.id, Date.now());
  }
  return undefined;
});

// From the rules: whether the block page may say what stopped a page, when
// each active block ends, the blocks that will take pages away soon (for
// the time left on the toolbar button), and every block's name.
let showWhy = false;
let ends = {};
let soon = [];
let blockNames = [];
let social = [];
let daemonUp = true;

function apply(rules) {
  showWhy = Boolean(rules.details);
  ends = rules.ends || {};
  soon = (rules.soon || []).map((s) => ({
    block: s.block, kind: s.kind, seconds: s.seconds, lists: kc9kCompileLists(s.lists),
  }));
  blockNames = rules.blocks || [];
  social = rules.social || [];
  lists = kc9kCompileLists(rules.lists);
  quotaLists = kc9kCompileLists(rules.quota);
  webOnly = Boolean(rules.web_only);
  safeSearch = Boolean(rules.safesearch);
  if (recording !== Boolean(rules.recording)) {
    recording = Boolean(rules.recording);
    visited.clear();
  }
  titleWords = (rules.titles || []).map((word) => String(word).toLowerCase()).filter(Boolean);
}

async function closeNewlyBlockedTabs() {
  const tabs = await browser.tabs.query({});
  for (const tab of tabs) {
    if (!tab.url || !/^https?:/i.test(tab.url)) {
      continue;
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
    }
  }
}

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
      await browser.storage.local.set({
        rules,
        page: { lists: rules.lists || [], quota: (rules.quota || []).length > 0, social: rules.social || [] },
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
async function updateBadges() {
  if (!browser.browserAction) {
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
      browser.browserAction.setBadgeText({ text, tabId: tab.id });
      browser.browserAction.setBadgeBackgroundColor({ color, tabId: tab.id });
      browser.browserAction.setTitle({ title, tabId: tab.id });
    }
  } catch (error) {
    // A tab closed meanwhile.
  }
}

function leftSentence(left) {
  const time = kc9kTimeLeft(left.seconds);
  if (left.kind === "quota") {
    return time + " of today's quota left for '" + left.block + "'";
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
  return {
    up: daemonUp, url, site, acting,
    left: timeLeftFor(tab).map((l) => ({ block: l.block, kind: l.kind, seconds: l.seconds, text: leftSentence(l) })),
    blocks: blockNames,
  };
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
// window, unless the user has been idle for a minute, and send the totals to
// the daemon every 30 seconds. The daemon drops them if statistics are off.
const USAGE_URL = "http://127.0.0.1:47613/usage";
const SAMPLE_SECONDS = 5;
const REPORT_MS = 30000;
let usage = {};

async function sampleUsage() {
  try {
    if (!(await userActive())) {
      return;
    }
    const tab = await frontTab();
    if (!tab || !tab.url || !/^https?:/i.test(tab.url)) {
      return;
    }
    const site = new URL(tab.url).hostname.replace(/^www\./, "");
    usage[site] = (usage[site] || 0) + SAMPLE_SECONDS;
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
let quotaUse = {};
const playing = new Map(); // tab id -> when its content script last said it plays

async function sampleQuota() {
  if (quotaLists.length === 0) {
    return;
  }
  const tabs = new Map();
  try {
    if (await userActive()) {
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
