"use strict";

// Goes back to the blocked site once its block ends, so a tab doesn't stay on
// this page after the block is stopped.
const target = new URLSearchParams(location.search).get("url") || "";

async function check() {
  // Only web addresses: anything else in the query could run as script.
  if (!/^https?:\/\//i.test(target)) {
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
