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

function report(tabId, url, outcome) {
  const key = tabId + " " + url;
  const now = Date.now();
  if (now - (reported.get(key) || 0) < 10000) {
    return Promise.resolve();
  }
  reported.set(key, now);
  for (const [k, at] of reported) {
    if (now - at > 60000) {
      reported.delete(k);
    }
  }
  return Promise.all(outcome.lists.map((hit) => fetch(HIT_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-KC9K-Extension": "1" },
    body: JSON.stringify({ list: hit.id, actions: hit.actions, url }),
  }).catch(() => {})));
}

// closeEverything closes the tab first, so a restored session doesn't bring
// the page back, then tells the daemon, then closes every window: the
// browser quits cleanly.
async function closeEverything(tabId, url, outcome) {
  try {
    await browser.tabs.remove(tabId);
  } catch (error) {
    // Already gone.
  }
  await report(tabId, url, outcome);
  const windows = await browser.windows.getAll();
  for (const win of windows) {
    browser.windows.remove(win.id).catch(() => {});
  }
}

// act carries out an outcome on a tab showing url (not from a request:
// see onBeforeRequest for those).
function act(tabId, url, outcome, reason) {
  if (outcome.device) {
    closeEverything(tabId, url, outcome);
    return;
  }
  report(tabId, url, outcome);
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
const keywordNoImages = new Map();

browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.url !== undefined && keywordNoImages.get(tabId) !== changeInfo.url) {
    keywordNoImages.delete(tabId);
  }
  if (changeInfo.title !== undefined) {
    blockTitledTab(tab);
  }
});
browser.tabs.onRemoved.addListener((tabId) => keywordNoImages.delete(tabId));

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
  if (matches.some((m) => m.actions.some((action) => action.action === "no-images"))) {
    keywordNoImages.set(tab.id, message.url);
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
    if (details.url.startsWith(RULES_URL) || details.url.startsWith(HIT_URL)) {
      return {};
    }
    if (details.type === "image" || details.type === "imageset" || details.type === "media") {
      if (keywordNoImages.has(details.tabId)) {
        return { cancel: true };
      }
      return kc9kListActions(lists, { url: pageUrl(details) })["no-images"] ? { cancel: true } : {};
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
  ["blocking"]
);

// stillBlocked tells the block page whether its site is still blocked, so it
// can go back to the site once the block ends.
function stillBlocked(url) {
  if (isBlocked(url)) {
    return true;
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
  return undefined;
});

function apply(rules) {
  lists = kc9kCompileLists(rules.lists);
  webOnly = Boolean(rules.web_only);
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
    } else {
      blockTitledTab(tab);
    }
  }
}

async function poll() {
  try {
    // The header marks this as the extension checking in (see the daemon's
    // ExtensionHeader); web pages can't send it.
    const response = await fetch(RULES_URL, { cache: "no-store", headers: { "X-KC9K-Extension": "1" } });
    if (response.ok) {
      const rules = await response.json();
      apply(rules);
      await browser.storage.local.set({ rules, page: { lists: rules.lists || [] } });
      await closeNewlyBlockedTabs();
    }
  } catch (error) {
    // The daemon is down: keep enforcing the last rules.
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
    if ((await browser.idle.queryState(60)) !== "active") {
      return;
    }
    const win = await browser.windows.getLastFocused();
    if (!win || !win.focused) {
      return;
    }
    const [tab] = await browser.tabs.query({ active: true, windowId: win.id });
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
