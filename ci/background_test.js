// Tests the extension's background script (internal/webext/files/
// background.js) with Node, the browser APIs it uses stubbed:
//   node ci/background_test.js
// Frames (R-LIST-4): what a frame's report may get, frameHit. The ad
// trigger (§3.9): noteAdRequest. Local files' block page reason (§3.1).
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const listeners = {};
const event = (name) => ({ addListener: (fn) => { listeners[name] = fn; } });
globalThis.browser = {
  runtime: { getURL: (p) => "moz-extension://id/" + p, getManifest: () => ({ version: "1.0.0" }), onMessage: event("message"),
    getPlatformInfo: async () => ({}) },
  extension: { isAllowedIncognitoAccess: async () => true },
  tabs: {
    onCreated: event("tabCreated"), onUpdated: event("tabUpdated"), onRemoved: event("tabRemoved"),
    onActivated: event("tabActivated"), query: async () => [], update: async () => {}, remove: async () => {},
    get: async (id) => ({ id, url: tabUrls[id] }),
  },
  storage: { local: { get: async () => ({}), set: async () => {} } },
  webRequest: { onBeforeRequest: event("request"), onBeforeSendHeaders: event("headers") },
};
const tabUrls = {};
globalThis.fetch = async () => ({ ok: false, json: async () => ({}) }); // the daemon isn't there
globalThis.setInterval = () => 0;

for (const f of ["match.js", "social.js", "background.js"]) {
  vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files", f), "utf8"), { filename: f });
}

let failed = 0;
let passed = 0;
function check(what, got, want) {
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    console.error("FAIL", what, "got", JSON.stringify(got), "want", JSON.stringify(want));
    failed++;
  } else {
    passed++;
  }
}

const list = (id, extra) => Object.assign({
  id, block: "b", name: "L" + id, url: [], title: [], text: ["casino"],
  actions: [{ action: "block", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] },
}, extra);

(async () => {
  // R-LIST-4: a frame whose own text a list blocks gets the block page; the
  // background decides from its own rules, never from what the report says.
  apply({ lists: [
    list(1),
    list(2, { actions: [{ action: "greyscale", distinct: 1, hits: 1 }] }),
    list(3, { test: true }),
  ] });
  const frame = { tab: { id: 7, url: "https://news.example/" }, frameId: 3 };
  const blocked = (r) => Boolean(r && typeof r.blocked === "string" && r.blocked.startsWith("moz-extension://id/blocked.html"));
  check("a frame blocked by its text", blocked(frameHit({ url: "https://ads.example/f", matches: [{ id: 1, actions: ["block"] }] }, frame)), true);
  check("a look alone doesn't take the frame", frameHit({ url: "https://ads.example/f", matches: [{ id: 2, actions: ["greyscale"] }] }, frame), null);
  check("an action the list hasn't is ignored", frameHit({ url: "https://ads.example/f", matches: [{ id: 2, actions: ["block"] }] }, frame), null);
  check("a list in test mode reports, never blocks", frameHit({ url: "https://ads.example/f", matches: [{ id: 3, actions: ["block"] }] }, frame), null);
  check("an unknown list is ignored", frameHit({ url: "https://ads.example/f", matches: [{ id: 99, actions: ["block"] }] }, frame), null);
  check("the top frame isn't a frame", frameHit({ url: "https://news.example/", matches: [{ id: 1, actions: ["block"] }] },
    { tab: frame.tab, frameId: 0 }), null);
  check("no tab, no answer", frameHit({ url: "https://ads.example/f", matches: [{ id: 1, actions: ["block"] }] }, { frameId: 3 }), null);
  check("only web pages", frameHit({ url: "file:///home/u/x.html", matches: [{ id: 1, actions: ["block"] }] }, frame), null);
  check("a malformed report", frameHit({ url: "https://ads.example/f", matches: "block" }, frame), null);
  check("actions not a list", frameHit({ url: "https://ads.example/f", matches: [{ id: 1, actions: "block" }] }, frame), null);

  // §3.9 ad trigger (noteAdRequest): a page's third-party request to a
  // listed ad network matches that list once per page; what isn't an ad
  // request of a web page is left alone.
  apply({ lists: [
    list(10, { text: [], ads: ["exoclick.com"] }),
    list(11, { text: [], ads: ["trafficjunky.net"], except: { url: [], title: [], text: ["homework"] } }),
  ] });
  const hits = [];
  globalThis.pageHit = (message, sender) => hits.push({ tab: sender.tab.id, ids: message.matches.map((m) => m.id) });
  const settle = () => new Promise((done) => setTimeout(done, 10));
  const ad = async (details) => {
    hits.length = 0;
    noteAdRequest(Object.assign({ tabId: 5, type: "script", url: "https://syndication.exoclick.com/ad.js",
      documentUrl: "https://tube.example/v/1" }, details));
    await settle();
    return hits.map((h) => h.ids.join(","));
  };
  tabUrls[5] = "https://tube.example/v/1";
  check("an ad network's script matches", await ad({}), ["10"]);
  check("once per page", await ad({ url: "https://cdn.exoclick.com/x.js" }), []);
  tabUrls[5] = "https://tube.example/v/2";
  check("again on another page", await ad({ documentUrl: "https://tube.example/v/2" }), ["10"]);
  check("Chrome's initiator stands for the page", await ad({ documentUrl: undefined, initiator: "https://other.example" }), ["10"]);
  check("no page known: nothing", await ad({ tabId: 6, documentUrl: undefined }), []);
  check("not from a tab", await ad({ tabId: -1, documentUrl: "https://x.example/" }), []);
  check("a page load itself isn't an ad", await ad({ tabId: 7, type: "main_frame", documentUrl: "https://y.example/" }), []);
  check("first-party: the site's own subdomain", await ad({ tabId: 8, url: "https://ads.tube.example/a.js",
    documentUrl: "https://tube.example/" }), []);
  check("the site itself", await ad({ tabId: 8, url: "https://syndication.exoclick.com/a.js",
    documentUrl: "https://exoclick.com/" }), []);
  check("only web pages", await ad({ tabId: 9, documentUrl: "moz-extension://id/blocked.html" }), []);
  check("a malformed address", await ad({ tabId: 9, url: "not a url" }), []);
  check("a list whose block has text exceptions is left to the page", await ad({ tabId: 12,
    url: "https://ht.trafficjunky.net/x", documentUrl: "https://tube.example/" }), []);
  adSeen.clear();
  listeners.tabRemoved && listeners.tabRemoved(5);
  apply({ lists: [list(12, { ads: [] })] });
  check("no list has ads: nothing is looked at", await ad({ tabId: 13 }), []);

  // §3.1: a local file is judged in the background by its address and
  // title (no request for it reaches onBeforeRequest); the block page names
  // the address when that alone took it.
  const sent = [];
  browser.tabs.update = async (id, props) => { sent.push(props.url); };
  apply({ lists: [list(20, { title: ["casino"], text: [], files: ["file:///home/u/games/*"], file_pages: true })] });
  blockTitledTab({ id: 21, url: "file:///home/u/games/chess.html", title: "Chess" });
  blockTitledTab({ id: 22, url: "file:///home/u/notes/casino.html", title: "Casino night" });
  blockTitledTab({ id: 23, url: "file:///home/u/notes/plan.html", title: "Plan" });
  check("local files: by address, by title, neither", sent, [
    "moz-extension://id/blocked.html?url=" + encodeURIComponent("file:///home/u/games/chess.html"),
    "moz-extension://id/blocked.html?url=" + encodeURIComponent("file:///home/u/notes/casino.html") + "&reason=title",
  ]);

  console.log(failed ? `${failed} of ${passed + failed} failed` : `all ${passed} passed`);
  process.exit(failed ? 1 : 0);
})();
