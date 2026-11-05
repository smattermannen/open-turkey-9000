"use strict";

// Rules come from the Killer Chicken 9000 daemon. The last rules are kept, so blocking
// survives a daemon restart.
const RULES_URL = "http://127.0.0.1:47613/rules";
const POLL_MS = 3000;

let blocked = [];
let exceptions = [];
let images = false;
let webOnly = false;

function escapeRegex(text) {
  return text.replace(/[.+?^${}()|[\]\\]/g, "\\$&");
}

// Entries work as in Cold Turkey: reddit.com, reddit.com/r/funny, *.xxx, *.*,
// and keywords such as *casino*.
function compile(entry) {
  const value = entry.replace(/^[a-z*]+:\/\//i, "").replace(/^www\./, "").toLowerCase();
  if (value === "*" || value === "*.*") {
    return /.*/;
  }
  const pattern = escapeRegex(value).replace(/\*/g, ".*");
  if (value.startsWith("*.")) {
    return new RegExp("^[^/]*\\." + pattern.slice(4) + (value.includes("/") ? "" : "(/|$)"));
  }
  if (value.startsWith("*")) {
    return new RegExp("^" + pattern);
  }
  return new RegExp("^([^/]*\\.)?" + pattern + (value.includes("/") ? "" : "(/|$)"));
}

function target(url) {
  return url.replace(/^[a-z-]+:\/\//i, "").replace(/^www\./i, "").toLowerCase();
}

function isBlocked(url) {
  if (!/^(https?|ftp):/i.test(url)) {
    return false;
  }
  const text = target(url);
  return blocked.some((rule) => rule.test(text)) && !exceptions.some((rule) => rule.test(text));
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

function blockedPage(url) {
  return { redirectUrl: browser.runtime.getURL("blocked.html") + "?url=" + encodeURIComponent(url) };
}

browser.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.url.startsWith(RULES_URL)) {
      return {};
    }
    if (details.type === "image" || details.type === "imageset" || details.type === "media") {
      return images ? { cancel: true } : {};
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

function apply(rules) {
  blocked = (rules.block || []).map(compile);
  exceptions = (rules.exceptions || []).map(compile);
  images = Boolean(rules.images);
  webOnly = Boolean(rules.web_only);
}

async function closeNewlyBlockedTabs() {
  const tabs = await browser.tabs.query({});
  for (const tab of tabs) {
    if (tab.url && isBlocked(tab.url)) {
      browser.tabs.update(tab.id, blockedPage(tab.url).redirectUrl ? { url: blockedPage(tab.url).redirectUrl } : {});
    }
  }
}

async function poll() {
  try {
    const response = await fetch(RULES_URL, { cache: "no-store" });
    if (response.ok) {
      const rules = await response.json();
      apply(rules);
      await browser.storage.local.set({ rules, page: { images: rules.images, greyscale: rules.greyscale } });
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
