// Runs the extension's page script (match.js, social.js, search.js, content.js) in a
// real Chromium page, with the extension APIs it uses stubbed, so what it
// does to a page can be checked without installing KC9K. Needs Node and
// Playwright (the cloud containers have both; the test laptop doesn't):
//
//   node ci/content_harness_test.js
//
// §3.9 no-images covers every image, animated GIFs included: also pictures a
// style sheet draws as content (::before/::after { content: url(…) }, or an
// element's own content), in shadow roots and in content added later; a
// ::before with text keeps it.
"use strict";
const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const FILES = path.join(__dirname, "../internal/webext/files");
const GIF = Buffer.from("R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==", "base64");
let failures = 0;
function check(name, ok, detail) {
  console.log((ok ? "ok   " : "FAIL ") + name + (ok || detail === undefined ? "" : ": " + JSON.stringify(detail)));
  failures += ok ? 0 : 1;
}

const PAGE = `<!doctype html><title>Pictures</title>
<style>
  .a::before { content: url(/x.gif); }
  .b::after { content: url(/x.gif) !important; }
  .c { content: url(/x.gif); }
  .set::before { content: image-set(url(/x.gif) 1x); }
  .t::before { content: "kept text"; }
</style>
<p>Some text so the page has a body.</p>
<div class="a" id="a"></div><div class="b" id="b"></div><div class="c" id="c"></div>
<div class="set" id="set"></div><div class="t" id="t"></div><div id="host"></div>
<script>
  const root = document.getElementById("host").attachShadow({ mode: "open" });
  root.innerHTML = '<style>.s::before { content: url(/x.gif); }</style><div class="s" id="s"></div><img src="/x.gif">';
  setTimeout(() => {
    const late = document.createElement("div");
    late.className = "a"; late.id = "late";
    document.body.appendChild(late);
  }, 300);
</script>`;

function rules(actions) {
  return {
    lists: [{
      id: 1, block: "b", name: "Pictures", url: ["pictures.example"], title: [], text: [],
      actions, except: { url: [], title: [], text: [] },
    }],
  };
}

async function main() {
  const url = "http://pictures.example/page";
  const browser = await chromium.launch();
  // One script, as the extension's content scripts share one scope (each
  // init script here would get its own).
  const script = ["match.js", "social.js", "search.js", "content.js"].map((f) => fs.readFileSync(path.join(FILES, f), "utf8")).join("\n;\n");

  async function open(page) {
    const p = await browser.newPage();
    // The site, served here: the page and its picture.
    await p.route("http://pictures.example/**", (route) => route.request().url().endsWith("/x.gif")
      ? route.fulfill({ status: 200, contentType: "image/gif", body: GIF })
      : route.fulfill({ status: 200, contentType: "text/html", body: PAGE }));
    // What the extension gives a page script: the rules in storage, and a
    // background script that answers every message with nothing.
    await p.addInitScript((stored) => {
      const listeners = { addListener() {} };
      window.browser = {
        storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
        runtime: { sendMessage: async () => undefined, onMessage: listeners },
      };
    }, page);
    await p.addInitScript(script);
    p.on("pageerror", (error) => check("no page script error", false, error.message));
    await p.goto(url);
    await p.waitForTimeout(1500); // the late element, idle-time scans
    return p;
  }

  const content = (p, selector, pseudo, inShadow) => p.evaluate(([sel, ps, sh]) => {
    const root = sh ? document.getElementById("host").shadowRoot : document;
    const el = root.querySelector(sel);
    return el ? getComputedStyle(el, ps).content : "missing";
  }, [selector, pseudo, inShadow]);

  const p = await open(rules([{ action: "no-images", distinct: 1, hits: 1 }]));
  check("::before picture emptied", await content(p, "#a", "::before") === '""', await content(p, "#a", "::before"));
  check("::after picture emptied, also !important", await content(p, "#b", "::after") === '""', await content(p, "#b", "::after"));
  check("an element's own content picture emptied", await content(p, "#c", null) === "normal", await content(p, "#c", null));
  check("image-set() emptied", await content(p, "#set", "::before") === '""', await content(p, "#set", "::before"));
  check("::before text kept", await content(p, "#t", "::before") === '"kept text"', await content(p, "#t", "::before"));
  check("in a shadow root", await content(p, "#s", "::before", true) === '""', await content(p, "#s", "::before", true));
  check("added later", await content(p, "#late", "::before") === '""', await content(p, "#late", "::before"));
  check("an image in a shadow root hidden", await p.evaluate(() =>
    getComputedStyle(document.getElementById("host").shadowRoot.querySelector("img")).visibility) === "hidden");
  await p.close();

  // Principle 9: a big page is looked at in small pieces (no long task,
  // over 50 ms, while it is scanned), and the picture at its end is found.
  const big = await browser.newPage();
  await big.route("http://pictures.example/**", (route) => route.request().url().endsWith("/x.gif")
    ? route.fulfill({ status: 200, contentType: "image/gif", body: GIF })
    : route.fulfill({ status: 200, contentType: "text/html", body: "<!doctype html><title>Big</title>" +
      "<style>.a::before { content: url(/x.gif); }</style>" +
      "<div>" + "<span>word</span>".repeat(20000) + "</div><div class=\"a\" id=\"a\"></div>" }));
  await big.addInitScript(() => {
    window.longTasks = [];
    new PerformanceObserver((list) => window.longTasks.push(...list.getEntries().map((e) => e.duration)))
      .observe({ entryTypes: ["longtask"] });
  });
  await big.addInitScript((stored) => {
    const listeners = { addListener() {} };
    window.browser = {
      storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
      runtime: { sendMessage: async () => undefined, onMessage: listeners },
    };
  }, rules([{ action: "no-images", distinct: 1, hits: 1 }]));
  await big.addInitScript(script);
  await big.goto("http://pictures.example/big");
  await big.waitForFunction(() => document.getElementById("a") &&
    getComputedStyle(document.getElementById("a"), "::before").content === '""', null, { timeout: 15000 })
    .then(() => check("a 20,000-element page scanned to its end", true),
      () => check("a 20,000-element page scanned to its end", false));
  const long = await big.evaluate(() => window.longTasks);
  // The page's own parsing may be one long task; the scan adds none.
  check("no long task from the scan", long.length <= 1, long);
  await big.close();

  // Principle 9: a huge page (40,000 words) with 1,000 rendered terms is
  // judged in pieces in idle time: no long task, several pieces, and the
  // term near its end still found and reported.
  const huge = await browser.newPage();
  const paragraphs = [];
  for (let i = 0; i < 400; i++) {
    const words = [];
    for (let k = 0; k < 100; k++) {
      words.push("word" + ((i * 100 + k) % 997));
    }
    if (i === 395) {
      words.push("zebra");
    }
    paragraphs.push("<p>" + words.join(" ") + ".</p>");
  }
  await huge.route("http://words.example/**", (route) => route.fulfill({ status: 200, contentType: "text/html",
    body: "<!doctype html><title>Huge</title>" + paragraphs.join("") }));
  const terms = ["zebra"];
  for (let i = 0; i < 999; i++) {
    terms.push(["term" + i, "pre" + i + "*", "*suf" + i, "long term" + i][i % 4]);
  }
  await huge.addInitScript(() => {
    window.longTasks = [];
    new PerformanceObserver((list) => window.longTasks.push(...list.getEntries().map((e) => e.duration)))
      .observe({ entryTypes: ["longtask"] });
    // The page held up: the most a 5 ms timer is late, once the page is
    // parsed (long tasks are only reported from 50 ms).
    window.stall = 0;
    // The extension's own work: the longest run of one of its callbacks
    // (idle time, timers, observers, events), set up before it loads.
    window.ownWork = 0;
    const timed = (fn) => function (...args) {
      const t0 = performance.now();
      try {
        return fn.apply(this, args);
      } finally {
        window.ownWork = Math.max(window.ownWork, performance.now() - t0);
      }
    };
    const ric = window.requestIdleCallback;
    window.requestIdleCallback = (fn, opts) => ric.call(window, timed(fn), opts);
    const st = window.setTimeout;
    window.setTimeoutOwn = st;
    window.setTimeout = (fn, ms, ...rest) => st.call(window, typeof fn === "function" ? timed(fn) : fn, ms, ...rest);
    const MO = window.MutationObserver;
    window.MutationObserver = class extends MO { constructor(fn) { super(timed(fn)); } };
    const add = EventTarget.prototype.addEventListener;
    EventTarget.prototype.addEventListener = function (type, fn, opts) {
      return add.call(this, type, typeof fn === "function" ? timed(fn) : fn, opts);
    };
    document.addEventListener("DOMContentLoaded", () => {
      let last = performance.now();
      const tick = () => {
        const now = performance.now();
        window.stall = Math.max(window.stall, now - last - 5);
        last = now;
        if (!window.stopTicks) {
          window.setTimeoutOwn(tick, 5);
        }
      };
      window.setTimeoutOwn(tick, 5);
    });
  });
  await huge.addInitScript((stored) => {
    const listeners = { addListener() {} };
    window.hits = [];
    window.browser = {
      storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
      runtime: { sendMessage: async (m) => { if (m.type === "page-hits") window.hits.push(m); }, onMessage: listeners },
    };
  }, { lists: [{ id: 7, block: "b", name: "Words", url: [], title: [], text: terms,
    actions: [{ action: "greyscale", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] } }] });
  await huge.addInitScript(script);
  await huge.goto("http://words.example/huge");
  await huge.waitForFunction(() => window.hits.length > 0, null, { timeout: 15000 })
    .then(() => check("a huge page's term near the end found", true),
      () => check("a huge page's term near the end found", false));
  const hugeLong = await huge.evaluate(() => window.longTasks);
  check("no long task from judging it", hugeLong.length <= 1, hugeLong);
  const stall = await huge.evaluate(() => { window.stopTicks = true; return window.stall; });
  const own = await huge.evaluate(() => window.ownWork);
  console.log("     the page was held up at most " + stall.toFixed(1) + " ms at a time (the browser's own " +
    "parsing and painting of a 40,000-word page included); the extension's work at most " + own.toFixed(1) + " ms");
  // Principle 9 is about the extension's own work: the page's figure also
  // holds the browser laying out 40,000 words, which it does with or
  // without the extension, so it is shown, not checked.
  check("the extension never works over 16 ms at a time (principle 9)", own <= 16, own);
  const hiddenAt = Date.now();
  await huge.waitForFunction(() => getComputedStyle(document.body).opacity === "1", null, { timeout: 5000 })
    .then(() => check("and shown once judged", true, Date.now() - hiddenAt),
      () => check("and shown once judged", false));
  await huge.close();

  // R-PAGE-1: a page kept hidden while judged shows its own background, not
  // a white flash (the root at opacity 0 took the canvas with it).
  const dark = await browser.newPage({ viewport: { width: 200, height: 200 } });
  await dark.route("http://dark.example/**", (route) => route.fulfill({ status: 200, contentType: "text/html",
    body: "<!doctype html><title>Dark</title><body style='background:#123;color:#fff'><p>plain words</p>" +
      "<img src='/slow.gif'></body>" }));
  await dark.addInitScript((stored) => {
    const listeners = { addListener() {} };
    window.browser = {
      storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
      runtime: { sendMessage: async () => undefined, onMessage: listeners },
    };
  }, { lists: [{ id: 8, block: "b", name: "Words", url: [], title: [], text: ["zebra"],
    actions: [{ action: "greyscale", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] } }] });
  await dark.addInitScript(script);
  // The picture never loads, so the page waits to be judged.
  let releaseGif;
  await dark.route("http://dark.example/slow.gif", (route) => new Promise((resolve) => {
    releaseGif = () => resolve(route.fulfill({ status: 200, contentType: "image/gif", body: GIF }));
  }));
  await dark.goto("http://dark.example/page", { waitUntil: "domcontentloaded" });
  const hiddenNow = await dark.evaluate(() => getComputedStyle(document.body).opacity);
  const pixel = (await dark.screenshot({ clip: { x: 150, y: 150, width: 1, height: 1 } })).toString("base64");
  const rgb = await dark.evaluate(async (data) => {
    const img = new Image();
    img.src = "data:image/png;base64," + data;
    await img.decode();
    const c = document.createElement("canvas");
    c.width = c.height = 1;
    const x = c.getContext("2d");
    x.drawImage(img, 0, 0);
    return Array.from(x.getImageData(0, 0, 1, 1).data.slice(0, 3));
  }, pixel);
  check("a page being judged is hidden", hiddenNow === "0", hiddenNow);
  check("and keeps its own background, no white flash", rgb.join(",") === "17,34,51", rgb);
  if (releaseGif) {
    releaseGif();
  }
  await dark.waitForFunction(() => getComputedStyle(document.body).opacity === "1", null, { timeout: 5000 })
    .then(() => check("then shown", true), () => check("then shown", false));
  await dark.close();

  // R-LIST-4: rendered visible text only (hidden and collapsed text doesn't
  // count, text far down does), and each frame on its own text: a frame
  // whose text a list blocks gets the block page; the page around it stays.
  const textPage = (body) => async (route) => route.fulfill({ status: 200, contentType: "text/html", body });
  async function judge(top, frames, inner) {
    const t = await browser.newPage();
    await t.route("http://text.example/top", textPage(top));
    await t.route("http://frame.example/**", textPage(frames));
    await t.route("http://inner.example/**", textPage(inner || ""));
    await t.addInitScript((stored) => {
      const listeners = { addListener() {} };
      window.sent = [];
      window.browser = {
        storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
        runtime: { sendMessage: async (m) => {
          window.sent.push(m);
          return m.type === "frame-hits" ? { blocked: "about:blank#kc9k-blocked" } : undefined;
        }, onMessage: listeners },
      };
    }, { lists: [{ id: 3, block: "b", name: "Words", url: [], title: [], text: ["casino"],
      actions: [{ action: "block", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] } }] });
    await t.addInitScript(script);
    await t.goto("http://text.example/top");
    await t.waitForTimeout(2500);
    return t;
  }
  const sentOf = async (t, type) => (await t.evaluate(() => window.sent)).filter((m) => m.type === type);
  let t = await judge("<!doctype html><p>Plain words.</p><div style=\"display:none\">casino</div>" +
    "<details><summary>More</summary>casino</details><p style=\"visibility:hidden\">casino</p>", "");
  check("hidden and collapsed text isn't read", (await sentOf(t, "page-hits")).length === 0, await sentOf(t, "page-hits"));
  await t.close();
  t = await judge("<!doctype html><p>Plain words.</p><div style=\"height:5000px\"></div><p>casino far down</p>", "");
  check("text far down is read", (await sentOf(t, "page-hits")).length === 1, await sentOf(t, "page-hits"));
  await t.close();
  t = await judge("<!doctype html><p>Plain words.</p><iframe src=\"http://frame.example/f\"></iframe>",
    "<!doctype html><p>Play casino games</p>");
  check("a frame's text isn't the page's", (await sentOf(t, "page-hits")).length === 0, await sentOf(t, "page-hits"));
  const frameHits = await t.frames()[1].evaluate(() => window.sent.filter((m) => m.type === "frame-hits")).catch(() => null);
  const frameUrl = t.frames()[1] ? t.frames()[1].url() : "";
  check("the frame judges its own text, and gets the block page",
    frameUrl.includes("kc9k-blocked") || (frameHits && frameHits.length === 1), { frameUrl, frameHits });
  await t.close();

  // A frame's report, wherever it is: what the frame with that address sent.
  const frameSent = async (t, host) => {
    for (const f of t.frames()) {
      if (f.url().includes(host)) {
        return f.evaluate(() => window.sent.filter((m) => m.type === "frame-hits")).catch(() => null);
      }
    }
    return null;
  };
  const blockedFrames = (t) => t.frames().filter((f) => f.url().includes("kc9k-blocked")).length;
  // R-LIST-4 in a frame too: its hidden and collapsed text isn't read.
  t = await judge("<!doctype html><p>Plain words.</p><iframe src=\"http://frame.example/h\"></iframe>",
    "<!doctype html><p>Plain frame.</p><div style=\"display:none\">casino</div><details><summary>More</summary>casino</details>");
  check("a frame's hidden text isn't read", blockedFrames(t) === 0 && ((await frameSent(t, "frame.example")) || []).length === 0,
    await frameSent(t, "frame.example"));
  await t.close();
  // A frame inside a frame is judged on its own text; the frame around it
  // and the page stay.
  t = await judge("<!doctype html><p>Plain words.</p><iframe src=\"http://frame.example/o\"></iframe>",
    "<!doctype html><p>Outer frame.</p><iframe src=\"http://inner.example/i\"></iframe>",
    "<!doctype html><p>Play casino games</p>");
  check("a frame in a frame judges its own text", blockedFrames(t) === 1 && t.frames().some((f) => f.url().includes("frame.example/o")),
    t.frames().map((f) => f.url()));
  check("and the frame around it reports nothing", ((await frameSent(t, "frame.example/o")) || []).length === 0,
    await frameSent(t, "frame.example/o"));
  check("nor the page", (await sentOf(t, "page-hits")).length === 0, await sentOf(t, "page-hits"));
  await t.close();
  // R-LIST-5 in a frame: look-alike letters (Cyrillic, small capitals) and
  // an invisible character inside the word.
  for (const word of ["саsіnо", "ᴄᴀsɪɴᴏ", "cas\u200Bino"]) {
    t = await judge("<!doctype html><p>Plain words.</p><iframe src=\"http://frame.example/l\"></iframe>",
      "<!doctype html><meta charset=\"utf-8\"><p>Play " + word + " games</p>");
    check("a frame's look-alike " + JSON.stringify(word) + " is read as Latin", blockedFrames(t) === 1, t.frames().map((f) => f.url()));
    await t.close();
  }

  // §3.9 ad trigger for a block with text exceptions: the page itself sees
  // the ad network it loaded from (resource timing) and judges it with the
  // exceptions; a page they spare isn't taken.
  async function judgeAds(body) {
    const a = await browser.newPage();
    await a.route("http://news.example/**", textPage(body));
    await a.route("http://ads.example/**", (route) => route.fulfill({ status: 200, contentType: "text/javascript", body: "" }));
    await a.addInitScript((stored) => {
      const listeners = { addListener() {} };
      window.sent = [];
      window.browser = {
        storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
        runtime: { sendMessage: async (m) => { window.sent.push(m); return undefined; }, onMessage: listeners },
      };
    }, { lists: [{ id: 5, block: "b", name: "Ads", url: [], title: [], text: [], ads: ["ads.example"],
      actions: [{ action: "block", distinct: 1, hits: 1 }], except: { url: [], title: [], text: ["homework"] } }] });
    await a.addInitScript(script);
    await a.goto("http://news.example/page");
    await a.waitForTimeout(2000);
    const hits = (await a.evaluate(() => window.sent)).filter((m) => m.type === "page-hits");
    await a.close();
    return hits;
  }
  let adHits = await judgeAds("<!doctype html><p>Today's news</p><script src=\"http://ads.example/ad.js\"></script>");
  check("a page loading a listed ad network is judged by it", adHits.length === 1 && adHits[0].matches[0].id === 5, adHits);
  adHits = await judgeAds("<!doctype html><p>Homework help</p><script src=\"http://ads.example/ad.js\"></script>");
  check("and spared by the block's text exception", adHits.length === 0, adHits);
  adHits = await judgeAds("<!doctype html><p>Today's news</p>");
  check("no ad network: nothing", adHits.length === 0, adHits);

  // §3.1: a local file is judged by its text only while the rules say so
  // (tamper "allow file:// blocking"), and spared by a file exception.
  const os = require("os");
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "kc9k-file-"));
  const local = path.join(dir, "page.html");
  fs.writeFileSync(local, "<!doctype html><meta charset=\"utf-8\"><title>Notes</title><p>Play casino games</p>");
  async function judgeFile(filePages, exceptFiles) {
    const f = await browser.newPage();
    await f.addInitScript((stored) => {
      const listeners = { addListener() {} };
      window.sent = [];
      window.browser = {
        storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
        runtime: { sendMessage: async (m) => { window.sent.push(m); return undefined; }, onMessage: listeners },
      };
    }, { lists: [{ id: 4, block: "b", name: "Words", url: [], title: [], text: ["casino"], file_pages: filePages,
      actions: [{ action: "block", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [], files: exceptFiles } }] });
    await f.addInitScript(script);
    await f.goto("file://" + local);
    await f.waitForTimeout(2000);
    const hits = (await f.evaluate(() => window.sent)).filter((m) => m.type === "page-hits");
    await f.close();
    return hits;
  }
  let fileHits = await judgeFile(true, []);
  check("a local file is judged by its text", fileHits.length === 1 && fileHits[0].url.startsWith("file://"), fileHits);
  fileHits = await judgeFile(false, []);
  check("not while file:// blocking is off", fileHits.length === 0, fileHits);
  fileHits = await judgeFile(true, ["file://*"]);
  check("and spared by file://*", fileHits.length === 0, fileHits);
  fs.rmSync(dir, { recursive: true, force: true });

  // Without no-images nothing is touched.
  const plain = await open(rules([{ action: "greyscale", distinct: 1, hits: 1 }]));
  check("other looks leave pictures alone", (await content(plain, "#a", "::before")).includes("url("),
    await content(plain, "#a", "::before"));
  check("and mark nothing", await plain.evaluate(() => document.querySelectorAll("[data-kc9k-noimg]").length) === 0);
  await plain.close();

  // The toolbar button's popup (owner, 2026-11-16): "Exempt from dark
  // pages" when dark pages are on, and back again.
  const popupHtml = fs.readFileSync(path.join(FILES, "popup.html"), "utf8")
    .replace('<script src="compat.js"></script>', "")
    .replace('<script src="popup.js"></script>', "<script>" + fs.readFileSync(path.join(FILES, "popup.js"), "utf8") + "</script>");
  const pop = await browser.newPage();
  await pop.route("http://popup.example/**", (route) => route.fulfill({ status: 200, contentType: "text/html", body: popupHtml }));
  await pop.addInitScript(() => {
    window.sent = [];
    let exempt = false;
    window.browser = { runtime: { sendMessage: async (m) => {
      window.sent.push(m);
      if (m.type === "popup") {
        return { up: true, url: "https://news.example.com/x", site: "news.example.com", acting: [], left: [], blocks: [],
          dark: { exempt } };
      }
      if (m.type === "dark-exempt") {
        exempt = m.exempt;
        return { ok: true, site: "news.example.com", exempt: m.exempt };
      }
      return undefined;
    } } };
  });
  await pop.goto("http://popup.example/popup.html");
  await pop.waitForSelector("#dark-go");
  check("the dark pages row shows", await pop.isVisible("#dark-go"));
  check("its button says Exempt from dark pages", (await pop.textContent("#dark-go")) === "Exempt from dark pages",
    await pop.textContent("#dark-go"));
  await pop.click("#dark-go");
  await pop.waitForFunction(() => document.getElementById("dark-go").textContent === "Show it dark again");
  const sent = await pop.evaluate(() => window.sent.filter((m) => m.type === "dark-exempt"));
  check("the click asks for the exemption", sent.length === 1 && sent[0].exempt === true &&
    sent[0].url === "https://news.example.com/x", sent);
  check("and says it is done", (await pop.textContent("#dark-result")).includes("left as it is"),
    await pop.textContent("#dark-result"));
  await pop.click("#dark-go");
  await pop.waitForFunction(() => document.getElementById("dark-go").textContent === "Exempt from dark pages");
  check("and back again", true);
  await pop.close();

  await browser.close();
  console.log(failures ? `FAILED: ${failures}` : "all passed");
  process.exit(failures ? 1 : 0);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
