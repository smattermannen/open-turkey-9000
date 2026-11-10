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

function load(page) {
  lists = kc9kCompileLists(page && page.lists);
  textChanged = true;
  render();
  scanKeywords();
}

// scanKeywords reads the page's title and text (in the top frame only: a
// frame's text is part of what the page shows) and applies the lists.
// Blocking goes through the background script, which sends the tab to the
// block page and also hides images in the page's frames.
function scanKeywords() {
  if (window !== window.top || !lists || !kc9kReadsPage(lists) || !textChanged) {
    return;
  }
  const body = document.body || document.documentElement;
  if (!body) {
    return;
  }
  textChanged = false;
  const text = (body.innerText || body.textContent || "").slice(0, MAX_TEXT);
  const url = location.href;
  if (keywordsFor !== url) {
    keywordHits = { block: false, greyscale: false, noImages: false };
    keywordsFor = url;
  }
  const acts = kc9kListActions(lists, { url, title: document.title || "", text });
  const hits = { block: Boolean(acts.block), greyscale: Boolean(acts.greyscale), noImages: Boolean(acts["no-images"]) };
  const before = JSON.stringify(keywordHits);
  keywordHits = {
    block: keywordHits.block || hits.block,
    greyscale: keywordHits.greyscale || hits.greyscale,
    noImages: keywordHits.noImages || hits.noImages,
  };
  if (JSON.stringify(keywordHits) === before) {
    return;
  }
  render();
  browser.runtime.sendMessage({ type: "keyword-hits", url, hits: keywordHits }).catch(() => {});
}

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
