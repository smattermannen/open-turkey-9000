"use strict";

// Goes back to the blocked site once its block ends, so a tab doesn't stay on
// this page after the block is stopped.
const params = new URLSearchParams(location.search);
const target = params.get("url") || "";
// A page blocked for its title or its text would come straight back here,
// so it stays.
const reason = params.get("reason");
const byTitle = reason === "title" || reason === "keyword" || reason === "mirror" || reason === "playlist";
const notes = {
  title: "Its title contains a word one of your blocks closes.",
  keyword: "Its title or text contains words one of your blocks looks for.",
  mirror: "It shows a site one of your blocks stops.",
  playlist: "Only the videos of the playlists you let through are open.",
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

// Which block and list stopped the page, and when the block ends, if the
// "Say why on the block page" option is on. Only this page shows it: what a
// list does (redirect, picture, closing the tab) is decided before and
// doesn't pass through here.
async function showWhy() {
  try {
    const why = await browser.runtime.sendMessage({ type: "why" });
    if (!why || !why.block) {
      return;
    }
    let text = "Stopped by the list \u201c" + why.list + "\u201d of the block \u201c" + why.block + "\u201d.";
    if (why.ends) {
      const at = new Date(why.ends * 1000);
      const sameDay = at.toDateString() === new Date().toDateString();
      text += " The block ends at " + at.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) +
        (sameDay ? "." : " on " + at.toLocaleDateString() + ".");
    }
    document.getElementById("why").textContent = text;
  } catch (error) {
    // The extension is reloading.
  }
}

showWhy();

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
