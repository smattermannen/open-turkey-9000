// Runs the extension's background script as IronFox (Firefox for Android) would: a
// browser object without the windows and idle APIs, and rules as KC9K on Android serves
// them. Checks address blocking, SafeSearch (also for POST searches) and web results.
//   node ci/android_background_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const listeners = {};
const event = (name) => ({ addListener: (fn) => { listeners[name] = fn; } });
const fetched = [];
const rules = {
  lists: [{
    id: 1, block: "phone", name: "Blocked", url: ["reddit.com", "youtube.com/shorts"], title: [], text: [],
    actions: [{ action: "block", distinct: 1, hits: 1 }],
    except: { url: ["reddit.com/r/programming"], title: [], text: [] },
  }],
  quota: [], web_only: true, safesearch: true, titles: [], social: [],
};
globalThis.browser = {
  runtime: { getURL: (p) => "moz-extension://id/" + p, getManifest: () => ({ version: "1.15.0" }), onMessage: event("message") },
  extension: { isAllowedIncognitoAccess: async () => true },
  tabs: {
    onCreated: event("tabCreated"), onUpdated: event("tabUpdated"), onRemoved: event("tabRemoved"),
    onActivated: event("tabActivated"), query: async () => [], update: async () => {}, remove: async () => {},
  },
  storage: { local: { get: async () => ({}), set: async () => {} } },
  webRequest: { onBeforeRequest: event("request"), onBeforeSendHeaders: event("headers") },
};
globalThis.fetch = async (url, init) => {
  fetched.push({ url, headers: (init && init.headers) || {} });
  return { ok: true, json: async () => (url.endsWith("/rules") ? rules : {}) };
};
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

(async () => {
  await new Promise((done) => setTimeout(done, 50)); // the first poll
  const checkIn = fetched.find((f) => f.url.endsWith("/rules"));
  check("check-in headers", checkIn && [checkIn.headers["X-KC9K-Extension"], checkIn.headers["X-KC9K-Version"], checkIn.headers["X-KC9K-Private"]], ["1", "1.15.0", "1"]);

  const req = (url, extra) => listeners.request(Object.assign({ url, type: "main_frame", tabId: 1, method: "GET" }, extra || {}));
  const blockedTo = (r) => Boolean(r && r.redirectUrl && r.redirectUrl.startsWith("moz-extension://id/blocked.html"));
  check("reddit blocked", blockedTo(req("https://www.reddit.com/r/funny")), true);
  check("exception spared", req("https://www.reddit.com/r/programming/"), {});
  check("shorts blocked", blockedTo(req("https://m.youtube.com/shorts/x")), true);
  check("other site", req("https://example.com/"), {});
  check("ddg POST", req("https://duckduckgo.com/", { method: "POST", requestBody: { formData: { q: ["cats"] } } }),
    { redirectUrl: "https://safe.duckduckgo.com/?q=cats&kp=1" });
  check("ddg after redirect, still POST", req("https://safe.duckduckgo.com/?q=cats&kp=1", { method: "POST", requestBody: { formData: { q: ["cats"] } } }), {});
  check("google safe + web only", req("https://www.google.com/search?q=cats&tbm=isch"),
    { redirectUrl: "https://www.google.com/search?q=cats&safe=active&udm=14" });
  check("google done", req("https://www.google.com/search?q=cats&safe=active&udm=14"), {});
  const sp = req("https://www.startpage.com/sp/search", { method: "POST", requestBody: { formData: { query: ["x"] } } });
  check("startpage blocked", Boolean(sp.redirectUrl && sp.redirectUrl.includes("reason=safesearch")), true);
  const yt = listeners.headers({ url: "https://www.youtube.com/", requestHeaders: [{ name: "YouTube-Restrict", value: "Off" }] });
  check("youtube restricted", yt.requestHeaders, [{ name: "YouTube-Restrict", value: "Strict" }]);
  // The Android-only paths: no windows or idle API.
  await Promise.all(timers.map((fn) => fn()));
  if (failed) {
    process.exit(1);
  }
  console.log("android background: all passed");
})();
