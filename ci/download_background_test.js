// No downloads (SPEC §3.9) in the extension's background script, with fake
// browser APIs: a download from a page whose text matched a no-downloads list
// is cancelled also when the browser names only the page's origin as
// referrer (Chrome, for a file from another host) or no referrer at all, and
// after Chrome restarted the service worker (the pages are kept in session
// storage). Found in Google Chrome on the desktop, 2026-11-20 (A3).
//   node ci/download_background_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const rules = {
  lists: [{
    id: 1, block: "dl", name: "Words", url: [], title: [], text: ["kcninedownload"],
    actions: [{ action: "no-downloads", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] },
  }],
  quota: [], titles: [], social: [],
};
const session = {}; // survives a worker restart, as storage.session does

// load runs the background script in a fresh context, as a (re)started
// service worker would, with these open tabs.
function load(tabs) {
  const listeners = {};
  const event = (name) => ({ addListener: (fn) => { listeners[name] = fn; } });
  const cancelled = [];
  const browser = {
    runtime: {
      getURL: (p) => "chrome-extension://id/" + p, getManifest: () => ({ version: "1", manifest_version: 2 }),
      onMessage: event("message"),
    },
    extension: { isAllowedIncognitoAccess: async () => false },
    tabs: {
      onCreated: event("tabCreated"), onUpdated: event("tabUpdated"), onRemoved: event("tabRemoved"),
      onActivated: event("tabActivated"), update: async () => {}, remove: async () => {},
      query: async (q) => tabs.filter((t) => !q.active || t.active),
    },
    storage: {
      local: { get: async () => ({}), set: async () => {} },
      session: {
        get: async (key) => (key in session ? { [key]: session[key] } : {}),
        set: async (items) => { Object.assign(session, JSON.parse(JSON.stringify(items))); },
      },
    },
    webRequest: { onBeforeRequest: event("request"), onBeforeSendHeaders: event("headers") },
    downloads: {
      onCreated: event("download"),
      cancel: async (id) => { cancelled.push(id); },
      removeFile: async () => {}, erase: async () => {},
    },
  };
  const context = vm.createContext({
    browser, console, URL, URLSearchParams, Promise, Date, Math, JSON, Map, Set, Array, Object, String, RegExp,
    setTimeout: () => 0, clearTimeout: () => {}, setInterval: () => 0,
    fetch: async (url) => ({ ok: true, json: async () => (url.endsWith("/rules") ? rules : {}) }),
  });
  for (const f of ["match.js", "social.js", "background.js"]) {
    vm.runInContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files", f), "utf8"), context, { filename: f });
  }
  return { listeners, cancelled };
}

const settle = () => new Promise((done) => setImmediate(done));
async function settled() {
  for (let i = 0; i < 20; i++) {
    await settle();
  }
}

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
  const page = "http://127.0.0.1:8000/bad";
  const plain = "http://127.0.0.1:9000/ok";
  const tabs = [
    { id: 1, url: page, active: true },
    { id: 2, url: plain, active: false },
  ];
  let w = load(tabs);
  await settled();
  w.listeners.message({ type: "page-hits", url: page, matches: [{ id: 1, actions: ["no-downloads"] }] },
    { tab: { id: 1, url: page }, frameId: 0 }, () => {});
  await settled();

  let next = 1;
  async function download(worker, item) {
    const id = next++;
    worker.listeners.download(Object.assign({ id }, item));
    await settled();
    return worker.cancelled.includes(id);
  }
  check("same site (full referrer)", await download(w, { url: "http://127.0.0.1:8000/file", referrer: page }), true);
  check("file from another host (origin as referrer)",
    await download(w, { url: "http://cdn.test/file", referrer: "http://127.0.0.1:8000/" }), true);
  check("no referrer, the matched page in front", await download(w, { url: "http://cdn.test/file", referrer: "" }), true);
  check("file made by the page (blob:)",
    await download(w, { url: "blob:http://127.0.0.1:8000/uuid", referrer: page }), true);
  check("another site's page (origin as referrer)",
    await download(w, { url: "http://cdn.test/file", referrer: "http://127.0.0.1:9000/" }), false);
  check("another site's page (full referrer)",
    await download(w, { url: "http://cdn.test/file", referrer: plain }), false);

  // Chrome stopped the worker and started it again: the page reported once.
  w = load(tabs);
  await settled();
  check("after a worker restart, same site", await download(w, { url: "http://127.0.0.1:8000/file", referrer: page }), true);
  check("after a worker restart, another host",
    await download(w, { url: "http://cdn.test/file", referrer: "http://127.0.0.1:8000/" }), true);

  // The matched page no longer in front: a download without a referrer goes.
  tabs[0].active = false;
  tabs[1].active = true;
  check("no referrer, another page in front", await download(w, { url: "http://cdn.test/file", referrer: "" }), false);

  if (failed) {
    process.exit(1);
  }
  console.log("download background: all passed");
})();
