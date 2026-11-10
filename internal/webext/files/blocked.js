"use strict";

// Goes back to the blocked site once its block ends, so a tab doesn't stay on
// this page after the block is stopped.
const params = new URLSearchParams(location.search);
const target = params.get("url") || "";
// A page blocked for its title or its text would come straight back here,
// so it stays.
const reason = params.get("reason");
const byTitle = reason === "title" || reason === "keyword";
const notes = {
  title: "Its title contains a word one of your blocks closes.",
  keyword: "Its title or text contains words one of your blocks looks for.",
};
if (notes[reason]) {
  const note = document.getElementById("note");
  if (note) {
    note.textContent = notes[reason];
  }
}

// A list can show a picture instead of the words. Only the daemon's stored
// images (named by their SHA-256) are loaded.
const image = params.get("image") || "";
if (/^[0-9a-f]{64}\.(png|jpg|gif|webp)$/.test(image)) {
  const picture = document.getElementById("picture");
  picture.src = "http://127.0.0.1:47613/image/" + image;
  picture.hidden = false;
  document.getElementById("words").hidden = true;
}

async function check() {
  // Only web addresses: anything else in the query could run as script.
  if (byTitle || !/^https?:\/\//i.test(target)) {
    return;
  }
  try {
    const blocked = await browser.runtime.sendMessage({ type: "still-blocked", url: target });
    if (blocked === false) {
      location.replace(target);
    }
  } catch (error) {
    // The extension is reloading: try again on the next round.
  }
}

check();
setInterval(check, 2000);
