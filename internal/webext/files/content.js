"use strict";

// Greys out pages and hides images on the sites of blocks with those actions.
// The background script also cancels image requests from those pages; hiding
// them here covers images already cached or drawn by the page itself.

const STYLE_ID = "kc9k-page-style";
let filters = null;
let shownFor = null;

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
  if (!filters) {
    return;
  }
  const url = topUrl();
  const greyscale = kc9kMatches(filters.greyscale, url);
  const noImages = kc9kMatches(filters.noImages, url);
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
  filters = {
    greyscale: kc9kFilter(page && page.greyscale),
    noImages: kc9kFilter(page && page.no_images),
  };
  render();
}

browser.storage.local.get("page").then((stored) => load(stored.page));
browser.storage.onChanged.addListener((changes) => {
  if (changes.page) {
    load(changes.page.newValue);
  }
});

// Sites that change pages without reloading (YouTube, Reddit) keep this
// script, so follow their address.
setInterval(() => {
  if (filters && topUrl() !== shownFor) {
    render();
  }
}, 1000);
