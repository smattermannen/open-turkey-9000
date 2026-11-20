// Runs the extension's background script with stub browser APIs and checks
// the tamper protection settings it follows (SPEC §4.10): block inactive tabs
// (background tabs blocked at once; off: when switched to) and block embedded
// content (a blocked site in a frame gets the block page; off: frames are
// left alone).
//   node ci/tamper_tabs_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const listeners = {};
const event = (name) => ({
  addListener: (fn) => { (listeners[name] = listeners[name] || []).push(fn); },
});
const fire = (name, ...args) => (listeners[name] || []).map((fn) => fn(...args));
const rules = {
  lists: [{
    id: 1, block: "b", name: "Blocked", url: ["reddit.com"], title: [], text: [],
    actions: [{ action: "block", distinct: 1, hits: 1 }],
    except: { url: [], title: [], text: [] },
  }],
  quota: [], titles: [], social: [],
};
const tabs = [
  { id: 1, active: true, url: "https://www.reddit.com/r/a", title: "a" },
  { id: 2, active: false, url: "https://www.reddit.com/r/b", title: "b" },
];
const updated = [];
globalThis.browser = {
  runtime: { getURL: (p) => "moz-extension://id/" + p, getManifest: () => ({ version: "1.28.0" }), onMessage: event("message") },
  extension: { isAllowedIncognitoAccess: async () => true },
  tabs: {
    onCreated: event("tabCreated"), onUpdated: event("tabUpdated"), onRemoved: event("tabRemoved"),
    onActivated: event("tabActivated"),
    query: async (q) => (q && q.active ? tabs.filter((t) => t.active) : q && q.audible ? [] : tabs),
    get: async (id) => tabs.find((t) => t.id === id),
    update: async (id) => { updated.push(id); },
    remove: async () => {}, sendMessage: async () => {},
  },
  storage: { local: { get: async () => ({}), set: async () => {} } },
  webRequest: { onBeforeRequest: event("request"), onBeforeSendHeaders: event("headers") },
};
globalThis.fetch = async (url) => ({ ok: true, json: async () => (url.endsWith("/rules") ? rules : {}) });
const timers = [];
globalThis.setInterval = (fn) => { timers.push(fn); return timers.length; };

for (const f of ["match.js", "social.js", "background.js"]) {
  vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files", f), "utf8"), { filename: f });
}

let failed = 0;
function check(what, got, want) {
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    console.error("FAIL", what, "got", JSON.stringify(got), "want", JSON.stringify(want));
    failed++;
  }
}
const settle = () => new Promise((done) => setTimeout(done, 50));
const frame = (url) => listeners.request[0]({ url, type: "sub_frame", tabId: 1, frameId: 3, method: "GET" });
const blockedTo = (r) => Boolean(r && r.redirectUrl && r.redirectUrl.startsWith("moz-extension://id/blocked.html"));

(async () => {
  await settle(); // the first poll: both settings on (absent)
  check("both tabs blocked at once", updated.slice().sort(), [1, 2]);
  check("frame blocked", blockedTo(frame("https://www.reddit.com/embed")), true);
  check("frame hit answered", Boolean(frameHit({ url: "https://x.example/", matches: [{ id: 1, actions: ["block"] }] },
    { tab: tabs[0], frameId: 3 })), true);

  rules.inactive_tabs_off = true;
  rules.embedded_off = true;
  updated.length = 0;
  await poll();
  check("only the active tab", updated, [1]);
  updated.length = 0;
  fire("tabActivated", { tabId: 2, windowId: 1 });
  await settle();
  check("background tab blocked when switched to", updated, [2]);
  check("frame left alone", frame("https://www.reddit.com/embed"), {});
  check("frame hit ignored", frameHit({ url: "https://x.example/", matches: [{ id: 1, actions: ["block"] }] },
    { tab: tabs[0], frameId: 3 }), null);
  check("top page still blocked", blockedTo(listeners.request[0]({ url: "https://reddit.com/", type: "main_frame", tabId: 1, method: "GET" })), true);

  console.log(failed ? `tamper tabs: ${failed} failed` : "tamper tabs: all passed");
  process.exit(failed ? 1 : 0);
})();
