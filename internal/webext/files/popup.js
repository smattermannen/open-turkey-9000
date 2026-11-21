"use strict";

// The toolbar button's panel: what Killer Chicken 9000 does with the site in
// this tab, how long until it is blocked, and a way to add it to a block.

function line(parent, text, cls) {
  const p = document.createElement("p");
  p.textContent = text;
  if (cls) {
    p.className = cls;
  }
  parent.appendChild(p);
}

// popupTab is the tab in front of the window this panel opened in. The
// panel asks for it itself: in Chrome the window stops being the focused
// one while its panel is open, so the background script's front tab would
// be none (B12, 2026-11-21). Not a web page (opened as a tab of its own,
// Firefox for Android): the background script picks.
async function popupTab() {
  try {
    const [tab] = await browser.tabs.query({ active: true, currentWindow: true });
    return tab && /^https?:/i.test(tab.url || "") ? tab : undefined;
  } catch (e) {
    return undefined;
  }
}

async function show() {
  const tab = await popupTab();
  const state = await browser.runtime.sendMessage(tab
    ? { type: "popup", url: tab.url, title: tab.title || "" }
    : { type: "popup" });
  const status = document.getElementById("status");
  status.textContent = "";
  document.getElementById("site").textContent = state.site || "Killer Chicken 9000";
  if (!state.up) {
    line(status, "Killer Chicken 9000 isn't answering. The last rules still apply.", "warn");
  }
  if (!state.url) {
    line(status, "Nothing to show for this page.", "dim");
    return;
  }
  for (const left of state.left) {
    line(status, left.text + ".", "left");
  }
  for (const acting of state.acting) {
    line(status, "“" + acting.list + "” of the block “" + acting.block + "” acts on this page: " +
      acting.actions.join(", ") + ".");
  }
  if (state.left.length === 0 && state.acting.length === 0) {
    line(status, "No block covers this site right now.", "dim");
  }
  showDark(state);
  if (state.blocks.length > 0) {
    const select = document.getElementById("block");
    select.textContent = "";
    for (const name of state.blocks) {
      const option = document.createElement("option");
      option.value = option.textContent = name;
      select.appendChild(option);
    }
    document.getElementById("add").hidden = false;
    document.getElementById("go").onclick = async () => {
      const result = document.getElementById("result");
      const answer = await browser.runtime.sendMessage({ type: "add-site", block: select.value, url: state.url });
      result.textContent = answer && answer.ok
        ? answer.site + " added to " + select.value + "."
        : "It couldn't be added.";
    };
  }
}

// Dark pages (§4.10; owner, 2026-11-16): leave this site as it is, or show
// it dark again.
function showDark(state) {
  const box = document.getElementById("dark");
  box.hidden = !state.dark;
  if (!state.dark) {
    return;
  }
  const exempt = state.dark.exempt;
  document.getElementById("dark-text").textContent = exempt
    ? "Dark pages leave this site as it is."
    : "Dark pages show this site dark.";
  const button = document.getElementById("dark-go");
  button.textContent = exempt ? "Show it dark again" : "Exempt from dark pages";
  button.onclick = async () => {
    button.disabled = true;
    const answer = await browser.runtime.sendMessage({ type: "dark-exempt", url: state.url, exempt: !exempt });
    button.disabled = false;
    document.getElementById("dark-result").textContent = answer && answer.ok
      ? (answer.exempt ? answer.site + " is left as it is." : answer.site + " is shown dark again.")
      : "It couldn't be changed (an access password may guard the settings).";
    if (answer && answer.ok) {
      state.dark.exempt = answer.exempt;
      showDark(state);
    }
  };
}

show();
