// The toolbar button's popup (SPEC §4.7: break status in the toolbar button
// and popup; notes/extension.md) shows the site of the tab it was opened
// over. Fake browser APIs that behave as Chrome's: while the panel is open
// its window is no longer the focused one. Found in Google Chrome on the
// desktop, 2026-11-21 (B12): a click on the button showed "Nothing to show
// for this page." on example.com.
//   node ci/popup_background_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const dir = path.join(__dirname, "../internal/webext/files");
const rules = {
  lists: [{
    id: 1, block: "pb", name: "Words", url: ["kcpopup.example"], title: [], text: [],
    actions: [{ action: "block" }], except: { url: [], title: [], text: [] },
  }],
  quota: [], titles: [], social: [], soon: [], blocks: ["pb", "other"],
};

const page = { id: 7, windowId: 1, active: true, url: "https://www.kcpopup.example/a", title: "A page" };

// The background script, with Chrome's focus while a panel is open.
function background() {
  const listeners = {};
  const event = (name) => ({ addListener: (fn) => { listeners[name] = fn; } });
  const browser = {
    runtime: {
      getURL: (p) => "chrome-extension://id/" + p, getManifest: () => ({ version: "1", manifest_version: 2 }),
      onMessage: event("message"),
    },
    extension: { isAllowedIncognitoAccess: async () => false },
    windows: { getLastFocused: async () => ({ id: 1, focused: false }) },
    tabs: {
      onCreated: event("tabCreated"), onUpdated: event("tabUpdated"), onRemoved: event("tabRemoved"),
      onActivated: event("tabActivated"), update: async () => {}, remove: async () => {},
      query: async () => [page], get: async () => page, sendMessage: async () => {},
    },
    scripting: { insertCSS: async () => {}, removeCSS: async () => {} },
    storage: {
      local: { get: async () => ({}), set: async () => {} },
      session: { get: async () => ({}), set: async () => {} },
    },
    webRequest: { onBeforeRequest: event("request"), onBeforeSendHeaders: event("headers") },
    downloads: { onCreated: event("download"), cancel: async () => {}, removeFile: async () => {}, erase: async () => {} },
  };
  const context = vm.createContext({
    browser, console, URL, URLSearchParams, Promise, Date, Math, JSON, Map, Set, Array, Object, String, RegExp,
    setTimeout: () => 0, clearTimeout: () => {}, setInterval: () => 0,
    fetch: async (url) => ({ ok: true, json: async () => (url.endsWith("/rules") ? rules : {}) }),
  });
  for (const f of ["match.js", "social.js", "background.js"]) {
    vm.runInContext(fs.readFileSync(path.join(dir, f), "utf8"), context, { filename: f });
  }
  return listeners;
}

// The popup page, with a small fake document; its messages go to the
// background script as Chrome sends them (answer through sendResponse).
function popup(listeners, tabs) {
  const elements = {};
  const element = (id) => {
    if (!elements[id]) {
      elements[id] = {
        id, hidden: id === "dark" || id === "add", textContent: "", className: "", children: [],
        appendChild(c) { this.children.push(c); },
      };
    }
    return elements[id];
  };
  const document = {
    getElementById: element,
    createElement: () => ({ textContent: "", className: "", value: "", children: [], appendChild(c) { this.children.push(c); } }),
  };
  const browser = {
    tabs: { query: async (q) => tabs(q) },
    runtime: {
      sendMessage: (message) => new Promise((done) => {
        if (!listeners.message(message, { id: "id", url: "chrome-extension://id/popup.html" }, done)) {
          done(undefined);
        }
      }),
    },
  };
  vm.runInContext(fs.readFileSync(path.join(dir, "popup.js"), "utf8"), vm.createContext({ browser, document, console }),
    { filename: "popup.js" });
  return elements;
}

const settled = async () => {
  for (let i = 0; i < 30; i++) {
    await new Promise((done) => setImmediate(done));
  }
};

let failed = 0;
function check(what, got, want) {
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    console.error("FAIL", what, "got", JSON.stringify(got), "want", JSON.stringify(want));
    failed++;
  } else {
    console.log("PASS", what);
  }
}

(async () => {
  const listeners = background();
  await settled();

  // Opened by a click on the button over the page: its own window's tab.
  let shown = popup(listeners, (q) => (q.active && q.currentWindow ? [page] : []));
  await settled();
  const lines = (e) => e.status.children.map((c) => c.textContent);
  check("the popup names the site of the tab it opened over", shown.site.textContent, "kcpopup.example");
  check("and the block acting on it",
    lines(shown).some((l) => l.includes("“Words” of the block “pb”") && l.includes("block")), true);
  check("and offers to add it to a block", shown.add ? shown.add.hidden : true, false);

  // Opened as a page of its own (its tab is the extension's page).
  shown = popup(listeners, () => [{ id: 9, windowId: 1, active: true, url: "chrome-extension://id/popup.html", title: "" }]);
  await settled();
  check("as a tab of its own it has nothing to show", lines(shown), ["Nothing to show for this page."]);

  if (failed) {
    process.exit(1);
  }
  console.log("all passed");
})();
