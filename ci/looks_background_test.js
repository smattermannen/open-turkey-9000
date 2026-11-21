// Look actions (SPEC §3.9: they change a page, and only while it earns
// them) drawn by the background script's user sheet, with fake browser APIs
// that behave as Chrome's: the same sheet inserted twice into a document is
// there twice, and removeCSS takes one copy away. Found in Google Chrome on
// the desktop, 2026-11-20 (B10): after a page that earned no-video from its
// text, the next page without the word kept its embedded players hidden.
//   node ci/looks_background_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const rules = {
  lists: [{
    id: 1, block: "lk", name: "Words", url: [], title: [], text: ["kcninelookword"],
    actions: [{ action: "no-video", distinct: 1, hits: 1 }, { action: "no-images", distinct: 1, hits: 1 }],
    except: { url: [], title: [], text: [] },
  }],
  quota: [], titles: [], social: [],
};

// sheets: frame id → the user sheets in its document now, in order.
function load(tab) {
  const listeners = {};
  const event = (name) => ({ addListener: (fn) => { listeners[name] = fn; } });
  const sheets = new Map([[0, []], [5, []]]);
  const frames = (target) => (target.allFrames ? [...sheets.keys()] : target.frameIds);
  const browser = {
    runtime: {
      getURL: (p) => "chrome-extension://id/" + p, getManifest: () => ({ version: "1", manifest_version: 2 }),
      onMessage: event("message"),
    },
    extension: { isAllowedIncognitoAccess: async () => false },
    tabs: {
      onCreated: event("tabCreated"), onUpdated: event("tabUpdated"), onRemoved: event("tabRemoved"),
      onActivated: event("tabActivated"), update: async () => {}, remove: async () => {},
      query: async () => [tab], get: async () => tab, sendMessage: async () => {},
    },
    scripting: {
      insertCSS: async ({ target, css }) => frames(target).forEach((f) => sheets.get(f).push(css)),
      removeCSS: async ({ target, css }) => frames(target).forEach((f) => {
        const list = sheets.get(f);
        const at = list.lastIndexOf(css);
        if (at >= 0) {
          list.splice(at, 1); // one copy, the last one, as Chrome does
        }
      }),
    },
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
    vm.runInContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files", f), "utf8"), context, { filename: f });
  }
  return { listeners, sheets };
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
  const word = "http://127.0.0.1:8000/word";
  const plain = "http://127.0.0.1:8000/plain";
  const tab = { id: 1, url: word, title: "" };
  const w = load(tab);
  await settled();
  const ask = async (frameId) => {
    w.listeners.message({ type: "tab-looks" }, { tab, frameId }, () => {});
    await settled();
  };
  const updated = async (change) => {
    Object.assign(tab, change.url ? { url: change.url } : {});
    w.listeners.tabUpdated(tab.id, change, tab);
    await settled();
  };
  const hidden = (frameId) => w.sheets.get(frameId).filter((css) => css.includes("display: none")).length > 0;

  // A page whose text earns no-video: its frames ask as they start, the
  // tab finishes loading, the page reports its words.
  await updated({ status: "loading", url: word });
  await ask(0);
  await ask(5);
  w.listeners.message({ type: "page-hits", url: word, matches: [{ id: 1, actions: ["no-video", "no-images"] }] },
    { tab, frameId: 0 }, () => {});
  await settled();
  await updated({ status: "complete" });
  check("the page with the word hides its players", [hidden(0), hidden(5)], [true, true]);

  // The next page, without the word: the look is carried until it is read,
  // its documents start empty and ask, the tab loads, the page is judged.
  await updated({ url: plain });
  w.sheets.set(0, []);
  w.sheets.set(5, []);
  await updated({ status: "loading" });
  await ask(0);
  await ask(5);
  await updated({ status: "complete" });
  check("while it is read, the carried look hides", hidden(0), true);
  w.listeners.message({ type: "page-judged", url: plain }, { tab, frameId: 0 }, () => {});
  await settled();
  check("once read, the next page shows its players (no sheet left in any frame)",
    [w.sheets.get(0), w.sheets.get(5)], [[], []]);

  // The word again on the same page, then the block switched off while it
  // shows (the rules no longer have the list): nothing stays hidden.
  w.listeners.message({ type: "page-hits", url: plain, matches: [{ id: 1, actions: ["no-video"] }] },
    { tab, frameId: 0 }, () => {});
  await settled();
  await updated({ status: "complete" });
  check("the word again hides the players", hidden(0), true);
  await updated({ url: plain + "?next" });
  w.listeners.message({ type: "page-judged", url: plain + "?next" }, { tab, frameId: 0 }, () => {});
  await settled();
  check("an address the word wasn't found on shows them", [w.sheets.get(0), w.sheets.get(5)], [[], []]);

  if (failed) {
    process.exit(1);
  }
  console.log("looks background: all passed");
})();
