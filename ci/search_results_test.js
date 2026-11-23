// R-PAGE-2: search results leading to a blacklisted address are taken off
// search results pages, the others untouched. Each engine's results page
// (ci/search_results/<engine>.html, markup modelled on the engine's own) is
// served at its real address in a real Chromium, with the extension's page
// scripts (match.js, social.js, search.js, content.js) and its APIs
// stubbed, as ci/content_harness_test.js does. Needs Node and Playwright
// (the cloud containers have both):
//
//   node ci/search_results_test.js
//
// Elements marked data-expect="gone" must not be shown, ever (checked each
// animation frame: R-PAGE-1), and those marked "kept" must be.
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");
const { chromium } = require("playwright");

const FILES = path.join(__dirname, "../internal/webext/files");
const FIXTURES = path.join(__dirname, "search_results");
let failures = 0;
function check(name, ok, detail) {
  console.log((ok ? "ok   " : "FAIL ") + name + (ok || detail === undefined ? "" : ": " + JSON.stringify(detail)));
  failures += ok ? 0 : 1;
}

// The rules: a block taking Reddit and X away, Reddit's r/allowed spared by
// the block's exception; a list that only greys a site out (its results
// stay: visiting it isn't taken away).
const LISTS = [
  { id: 1, block: "Social", name: "Sites", url: ["reddit.com", "x.com"], title: [], text: [],
    actions: [{ action: "block", distinct: 1, hits: 1 }], except: { url: ["reddit.com/r/allowed"], title: [], text: [] } },
  { id: 2, block: "Looks", name: "Grey", url: ["looks.example"], title: [], text: [],
    actions: [{ action: "greyscale", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] } },
];

// The address parts, in Node: which pages are results pages, where links lead.
function unitChecks() {
  const context = { URL, URLSearchParams, atob, console };
  vm.createContext(context);
  for (const f of ["match.js", "search.js"]) {
    vm.runInContext(fs.readFileSync(path.join(FILES, f), "utf8"), context, { filename: f });
  }
  const engine = (u) => (vm.runInContext("kc9kSearchEngine", context)(u) || {}).name || null;
  const target = (h, b) => vm.runInContext("kc9kResultTarget", context)(h, b);
  const pages = JSON.parse(fs.readFileSync(path.join(FIXTURES, "pages.json"), "utf8"));
  for (const [name, url] of Object.entries(pages)) {
    if (name !== "_") {
      check(`${name}: its results page is known`, engine(url) === name.replace(/-html$/, ""), engine(url));
    }
  }
  for (const url of ["https://www.google.com/maps?q=x", "https://duckduckgo.com/", "https://www.bing.com/images/search?q=x",
    "https://www.reddit.com/search?q=x", "https://search.example.org/search?q=x", "ftp://www.google.com/search"]) {
    check(`not a results page: ${url}`, engine(url) === null, engine(url));
  }
  const g = "https://www.google.com/search?q=x";
  for (const [href, base, want] of [
    ["/url?q=https://x.com/a&sa=U", g, "https://x.com/a"],
    ["/url?sa=t&url=https://x.com/b", g, "https://x.com/b"],
    ["https://www.google.com/amp/s/www.reddit.com/r/a/", g, "https://www.reddit.com/r/a/"],
    ["https://www.bing.com/ck/a?!&&p=1&u=a1aHR0cHM6Ly94LmNvbS9zb21lb25l&ntb=1", g, "https://x.com/someone"],
    ["https://www.bing.com/aclk?ld=e8&u=aHR0cHM6Ly93d3cucmVkZGl0LmNvbS9wcm9tbw", g, "https://www.reddit.com/promo"],
    ["//duckduckgo.com/l/?uddg=https%3A%2F%2Fx.com%2Fa&rut=1", "https://html.duckduckgo.com/html/?q=x", "https://x.com/a"],
    ["https://duckduckgo.com/y.js?ad_domain=reddit.com&u3=x", g, "https://reddit.com/"],
    ["https://r.search.yahoo.com/_ylt=A/RV=2/RU=https%3a%2f%2fx.com%2fa/RK=2/RS=x-", g, "https://x.com/a"],
    ["https://web.archive.org/web/2024/https://www.reddit.com/r/a/", g, "https://www.reddit.com/r/a/"],
    ["https://en.wikipedia.org/wiki/X", g, "https://en.wikipedia.org/wiki/X"],
    ["/search?q=more", g, "https://www.google.com/search?q=more"],
    ["javascript:void(0)", g, ""],
    ["mailto:a@example.org", g, ""],
  ]) {
    check(`leads to ${want || "nothing"}: ${href}`, target(href, base) === want, target(href, base));
  }
}

async function main() {
  unitChecks();
  const manifest = JSON.parse(fs.readFileSync(path.join(FILES, "manifest.json"), "utf8"));
  const js = manifest.content_scripts[0].js;
  check("manifest: search.js runs before content.js", js.indexOf("search.js") >= 0 && js.indexOf("search.js") < js.indexOf("content.js"), js);

  const pages = JSON.parse(fs.readFileSync(path.join(FIXTURES, "pages.json"), "utf8"));
  const script = ["match.js", "social.js", "search.js", "content.js"].map((f) => fs.readFileSync(path.join(FILES, f), "utf8")).join("\n;\n");
  const browser = await chromium.launch();

  async function open(name, stored) {
    const p = await browser.newPage();
    const html = fs.readFileSync(path.join(FIXTURES, name + ".html"), "utf8");
    // Nothing leaves the machine: the engine's page is served here, the rest refused.
    await p.route("**/*", (route) => route.request().url() === pages[name]
      ? route.fulfill({ status: 200, contentType: "text/html", body: html })
      : route.abort());
    await p.addInitScript((page) => {
      const listeners = { addListener() {} };
      window.kc9kChanged = [];
      window.browser = {
        storage: { local: { get: async () => ({ page }) }, onChanged: { addListener: (fn) => window.kc9kChanged.push(fn) } },
        runtime: { sendMessage: async () => undefined, onMessage: listeners },
      };
      // R-PAGE-1: what each animation frame would paint.
      window.kc9kPainted = new Set();
      const frame = () => {
        if (!document.getElementById("kc9k-hide")) {
          for (const el of document.querySelectorAll("[data-expect=gone]")) {
            if (el.getClientRects().length > 0) {
              window.kc9kPainted.add(el.outerHTML.slice(0, 120));
            }
          }
        }
        requestAnimationFrame(frame);
      };
      requestAnimationFrame(frame);
    }, stored);
    await p.addInitScript(script);
    p.on("pageerror", (error) => check(`${name}: no page script error`, false, error.message));
    await p.goto(pages[name]);
    await p.waitForTimeout(900); // DuckDuckGo adds more results after 400 ms
    return p;
  }

  const shown = (p) => p.evaluate(() => [...document.querySelectorAll("[data-expect]")].map((el) => ({
    expect: el.dataset.expect, shown: el.getClientRects().length > 0, what: el.textContent.trim().slice(0, 50),
  })));

  for (const name of Object.keys(pages).filter((n) => n !== "_")) {
    const p = await open(name, { lists: LISTS });
    const marks = await shown(p);
    const gone = marks.filter((m) => m.expect === "gone");
    const kept = marks.filter((m) => m.expect === "kept");
    check(`${name}: results leading to Reddit or X are gone (${gone.length})`, gone.length > 0 && gone.every((m) => !m.shown),
      gone.filter((m) => m.shown));
    check(`${name}: the others are shown (${kept.length})`, kept.length > 0 && kept.every((m) => m.shown),
      kept.filter((m) => !m.shown));
    const painted = await p.evaluate(() => [...window.kc9kPainted]);
    check(`${name}: a result taken off was never painted`, painted.length === 0, painted);
    // The block switched off: everything is back.
    await p.evaluate(() => window.kc9kChanged.forEach((fn) => fn({ page: { newValue: { lists: [] } } })));
    await p.waitForTimeout(50);
    const after = await shown(p);
    check(`${name}: with the block off every result is back`, after.every((m) => m.shown), after.filter((m) => !m.shown));
    await p.close();
  }

  // No lists at all: nothing is touched.
  const p = await open("google", { lists: [] });
  const marks = await shown(p);
  check("no lists: every result shown", marks.every((m) => m.shown), marks.filter((m) => !m.shown));
  check("no lists: nothing marked", await p.evaluate(() => document.querySelectorAll("[data-kc9k-removed]").length) === 0);
  await p.close();

  await browser.close();
  console.log(failures ? `search results: ${failures} failed` : "search results: all passed");
  process.exit(failures ? 1 : 0);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
