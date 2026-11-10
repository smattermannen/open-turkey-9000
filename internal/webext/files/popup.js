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

async function show() {
  const state = await browser.runtime.sendMessage({ type: "popup" });
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

show();
