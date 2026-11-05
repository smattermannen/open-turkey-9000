"use strict";

const STYLE_ID = "open-turkey-page-style";

function render(page) {
  let css = "";
  if (page && page.greyscale) {
    css += "html { filter: grayscale(1) !important; }\n";
  }
  if (page && page.images) {
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

browser.storage.local.get("page").then((stored) => render(stored.page));
browser.storage.onChanged.addListener((changes) => {
  if (changes.page) {
    render(changes.page.newValue);
  }
});
