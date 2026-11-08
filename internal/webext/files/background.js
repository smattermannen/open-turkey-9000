"use strict";

// Rules come from the Killer Chicken 9000 daemon. The last rules are kept, so blocking
// survives a daemon restart.
const RULES_URL = "http://127.0.0.1:47613/rules";
const POLL_MS = 3000;

// Compiled filters (see match.js). blocking holds the blocked sites and their
// exceptions; noImages the sites of blocks with the no-images action. The
// content script handles greyscale on its own.
const EMPTY = { match: [], except: [] };
let blocking = kc9kFilter(EMPTY);
let noImages = kc9kFilter(EMPTY);
let webOnly = false;

function isBlocked(url) {
  return kc9kMatches(blocking, url);
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

function blockedPage(url, reason) {
  const page = browser.runtime.getURL("blocked.html") + "?url=" + encodeURIComponent(url);
  return { redirectUrl: reason ? page + "&reason=" + reason : page };
}

// Window-title keywords from the active blocks: a tab whose title contains
// one is sent to the block page, as the daemon closes other apps' windows.
let titleWords = [];

function titleBlocked(title) {
  const text = (title || "").toLowerCase();
  return titleWords.some((word) => text.includes(word));
}

function blockTitledTab(tab) {
  if (tab.url && /^https?:/i.test(tab.url) && titleBlocked(tab.title)) {
    browser.tabs.update(tab.id, { url: blockedPage(tab.url, "title").redirectUrl });
  }
}

browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.title !== undefined) {
    blockTitledTab(tab);
  }
});

browser.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.url.startsWith(RULES_URL)) {
      return {};
    }
    if (details.type === "image" || details.type === "imageset" || details.type === "media") {
      return kc9kMatches(noImages, pageUrl(details)) ? { cancel: true } : {};
    }
    if (isBlocked(details.url)) {
      return blockedPage(details.url);
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

browser.runtime.onMessage.addListener((message) => {
  if (message && message.type === "still-blocked" && typeof message.url === "string") {
    return Promise.resolve(stillBlocked(message.url));
  }
  return undefined;
});

function apply(rules) {
  blocking = kc9kFilter({ match: rules.block, except: rules.exceptions });
  noImages = kc9kFilter(rules.no_image_sites);
  webOnly = Boolean(rules.web_only);
  titleWords = (rules.titles || []).map((word) => String(word).toLowerCase()).filter(Boolean);
}

async function closeNewlyBlockedTabs() {
  const tabs = await browser.tabs.query({});
  for (const tab of tabs) {
    if (tab.url && isBlocked(tab.url)) {
      browser.tabs.update(tab.id, { url: blockedPage(tab.url).redirectUrl });
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
      await browser.storage.local.set({
        rules,
        page: { greyscale: rules.greyscale_sites || EMPTY, no_images: rules.no_image_sites || EMPTY },
      });
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
