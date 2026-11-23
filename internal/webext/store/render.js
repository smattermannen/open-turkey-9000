// Draws the Chrome Web Store pictures (ROADMAP 2.17) into this folder: the
// 440×280 small promo tile and four 1280×800 screenshots. The screenshots
// show the extension's real pages and page scripts (block page, toolbar
// panel, search results, dark pages) in Chromium with its APIs stubbed,
// framed with a caption. Needs Node and Playwright (the cloud containers
// have both):
//
//   node internal/webext/store/render.js
"use strict";
const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const OUT = __dirname;
const FILES = path.join(__dirname, "../files");
const read = (f) => fs.readFileSync(path.join(FILES, f), "utf8");
const ICON = read("icon.svg");
const FONT = "Inter, 'DejaVu Sans', sans-serif";
const INK = "#1b1d21";

// The page scripts as the browser runs them, sharing one scope.
const PAGE_SCRIPTS = ["match.js", "social.js", "search.js", "content.js"].map(read).join("\n;\n");

// A stub of the extension APIs: the rules in storage, answers by message type.
function stub({ stored, answers }) {
  const listeners = { addListener() {} };
  window.browser = {
    storage: { local: { get: async () => ({ page: stored }) }, onChanged: listeners },
    runtime: { sendMessage: async (m) => (answers || {})[m && m.type], onMessage: listeners },
    tabs: { query: async () => [(answers || {}).tab].filter(Boolean) },
  };
}

async function shoot(browser, { url, html, files, stored, answers, width, height, scripts, fullPage }) {
  const page = await browser.newPage({ viewport: { width, height }, deviceScaleFactor: 1 });
  await page.route("**/*", (route) => {
    const u = route.request().url();
    if (u === url) {
      return route.fulfill({ status: 200, contentType: "text/html; charset=utf-8", body: html });
    }
    const name = files && u.startsWith(files) ? u.slice(files.length).split("?")[0] : "";
    if (name && fs.existsSync(path.join(FILES, name))) {
      const type = name.endsWith(".js") ? "text/javascript" : name.endsWith(".svg") ? "image/svg+xml" : "text/html";
      return route.fulfill({ status: 200, contentType: type, body: fs.readFileSync(path.join(FILES, name)) });
    }
    return route.abort();
  });
  await page.addInitScript(stub, { stored, answers });
  if (scripts) {
    await page.addInitScript(PAGE_SCRIPTS);
  }
  await page.goto(url);
  await page.waitForTimeout(1200);
  const png = await page.screenshot({ type: "png", fullPage: Boolean(fullPage) });
  await page.close();
  return png;
}

const dataUrl = (png) => "data:image/png;base64," + png.toString("base64");

// A browser window around a picture of its page: tab bar, address, the
// extension's toolbar button (with its badge).
function windowHtml(address, pagePng, width, height, badge, popupPng) {
  return `<div class="win" style="width:${width}px">
    <div class="bar"><span class="dots"><i></i><i></i><i></i></span>
      <span class="addr">${address}</span>
      <span class="btn">${ICON}${badge ? `<b>${badge}</b>` : ""}</span></div>
    <img class="page" src="${dataUrl(pagePng)}" width="${width}" height="${height}">
    ${popupPng ? `<img class="popup" src="${dataUrl(popupPng)}">` : ""}
  </div>`;
}

const FRAME_CSS = `
  * { box-sizing: border-box; }
  body { margin: 0; width: 1280px; height: 800px; overflow: hidden; font-family: ${FONT};
    background: linear-gradient(160deg, #23262c, ${INK} 60%); color: #eceff4; }
  .cap { position: absolute; left: 60px; top: 34px; right: 60px; }
  .cap h1 { margin: 0; font-size: 34px; font-weight: 700; letter-spacing: -0.5px; }
  .cap p { margin: 8px 0 0; font-size: 19px; color: #a7b0c0; }
  .wins { position: absolute; left: 60px; right: 60px; top: 150px; display: flex; gap: 40px; }
  .win { position: relative; border-radius: 12px; overflow: visible; background: #2e3440;
    box-shadow: 0 18px 50px rgba(0,0,0,.45); }
  .bar { height: 44px; display: flex; align-items: center; gap: 14px; padding: 0 14px; }
  .dots i { display: inline-block; width: 12px; height: 12px; border-radius: 50%; background: #4c566a; margin-right: 6px; }
  .addr { flex: 1; height: 28px; border-radius: 14px; background: #3b4252; color: #d8dee9;
    font-size: 14px; line-height: 28px; padding: 0 14px; white-space: nowrap; overflow: hidden; }
  .btn { position: relative; width: 26px; height: 26px; }
  .btn svg { width: 26px; height: 26px; }
  .btn b { position: absolute; right: -10px; bottom: -6px; background: #3b6ea8; color: #fff; font-size: 10px;
    padding: 1px 4px; border-radius: 4px; }
  .page { display: block; border-radius: 0 0 12px 12px; }
  .popup { position: absolute; right: 8px; top: 48px; border-radius: 8px; box-shadow: 0 10px 30px rgba(0,0,0,.5); }
  .label { position: absolute; top: -30px; left: 2px; font-size: 16px; color: #a7b0c0; }`;

async function frame(browser, name, caption, sub, windows) {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
  await page.setContent(`<!doctype html><html><head><meta charset="utf-8"><style>${FRAME_CSS}</style></head>
    <body><div class="cap"><h1>${caption}</h1><p>${sub}</p></div><div class="wins">${windows}</div></body></html>`);
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(OUT, name), type: "png" });
  await page.close();
  console.log("wrote", name);
}

const ARTICLE = (title, dark) => `<!doctype html><html><head><meta charset="utf-8"><title>${title}</title><style>
  body { margin: 0; font: 17px/1.6 Georgia, 'DejaVu Serif', serif; background: #fff; color: #222; }
  header { padding: 16px 48px; border-bottom: 1px solid #ddd; font: 700 22px ${FONT}; }
  main { padding: 28px 48px; max-width: 760px; }
  h1 { font: 700 34px/1.2 ${FONT}; margin: 0 0 14px; }
  .photo { height: 150px; border-radius: 8px; background: linear-gradient(135deg, #f6d365, #fda085); margin: 18px 0; }
  ${dark ? `@media (prefers-color-scheme: dark) { body { background: #15171a; color: #ddd; } header { border-color: #333; } }` : ""}
</style></head><body><header>The Daily Example</header><main><h1>${title}</h1>
<p>Morning light, evening dark: our bodies keep time by the sky. Screens that stay bright and blue at night
tell the body it is still day, and sleep comes later.</p><div class="photo"></div>
<p>Small changes help: warmer colours after sunset, dimmer screens, dark pages at night.</p></main></body></html>`;

const RESULTS = (q, items) => `<!doctype html><html><head><meta charset="utf-8"><title>${q} - Search</title><style>
  body { margin: 0; font: 15px/1.45 ${FONT}; background: #fff; color: #202124; }
  form { padding: 18px 28px; border-bottom: 1px solid #e3e3e3; }
  input { width: 380px; font: inherit; padding: 9px 16px; border: 1px solid #ccc; border-radius: 22px; }
  #urls { padding: 10px 28px; }
  article { margin: 0 0 20px; }
  .url { color: #4d5156; font-size: 13px; }
  h3 { margin: 2px 0; font-size: 19px; font-weight: 500; } h3 a { color: #1a0dab; text-decoration: none; }
  p { margin: 2px 0; color: #4d5156; font-size: 14px; }
</style></head><body><form role="search" action="/search"><input name="q" value="${q}"></form>
<div id="urls" role="main">${items.map(([url, title, text]) =>
  `<article><div class="url">${url.replace(/^https:\/\//, "").replace(/\/$/, "")}</div><h3><a href="${url}">${title}</a></h3><p>${text}</p></article>`).join("")}
</div></body></html>`;

async function main() {
  const browser = await chromium.launch();

  // Small promo tile, 440×280.
  const tile = await browser.newPage({ viewport: { width: 440, height: 280 }, deviceScaleFactor: 1 });
  await tile.setContent(`<!doctype html><html><head><meta charset="utf-8"><style>
    body { margin: 0; width: 440px; height: 280px; display: flex; align-items: center; gap: 22px; padding: 0 34px;
      box-sizing: border-box; font-family: ${FONT}; background: linear-gradient(160deg, #2a2d34, ${INK} 70%); color: #eceff4; }
    svg { width: 132px; height: 132px; flex: none; filter: drop-shadow(0 8px 18px rgba(0,0,0,.4)); }
    h1 { margin: 0; font-size: 31px; line-height: 1.08; font-weight: 800; letter-spacing: -0.5px; }
    p { margin: 10px 0 0; font-size: 16px; color: #a7b0c0; }
  </style></head><body>${ICON}<div><h1>Killer Chicken<br>9000</h1><p>Your blocks, inside Chrome.</p></div></body></html>`);
  await tile.screenshot({ path: path.join(OUT, "small-tile.png"), type: "png" });
  await tile.close();
  console.log("wrote small-tile.png");

  // 1: the block page.
  const ext = "chrome-extension://kc9k/";
  const ends = new Date();
  ends.setHours(17, 0, 0, 0);
  const blocked = await shoot(browser, {
    url: "http://kc9k.test/blocked.html?url=https%3A%2F%2Fvideos.example%2Fwatch",
    html: read("blocked.html"), files: "http://kc9k.test/", width: 1160, height: 560,
    answers: { why: { block: "Work hours", list: "Video sites", ends: Math.floor(ends / 1000) }, "still-blocked": true },
  });
  await frame(browser, "screenshot-1-blocked.png", "Blocked sites stay blocked",
    "While a block you set in Killer Chicken 9000 is on, its sites go to the block page, and come back when it ends.",
    windowHtml(ext + "blocked.html", blocked, 1160, 560));

  // 2: the toolbar button's panel over a page.
  const tab = { url: "https://news.example/today", title: "The Daily Example" };
  const article = await shoot(browser, { url: tab.url, html: ARTICLE("Why evenings feel shorter"), width: 1160, height: 560,
    stored: { lists: [] }, scripts: true });
  const panel = await shoot(browser, {
    url: "http://kc9k.test/popup.html", html: read("popup.html"), files: "http://kc9k.test/", width: 328, height: 100, fullPage: true,
    answers: {
      tab,
      popup: { up: true, url: tab.url, site: "news.example", blocks: ["Work hours", "Evenings", "Weekend"],
        left: [{ text: "'Work hours' blocks this in 12m" }],
        acting: [{ block: "Evenings", list: "News", actions: ["mute"] }], dark: { exempt: false } },
    },
  });
  await frame(browser, "screenshot-2-toolbar.png", "Know how long you have",
    "The toolbar button shows the time until a site is blocked, what acts on it, and adds it to a block.",
    windowHtml(tab.url, article, 1160, 560, "12m", panel));

  // 3: search results with and without the blocked site.
  const searchUrl = "https://searx.example.org/search?q=study+tips";
  const results = RESULTS("study tips", [
    ["https://www.reddit.com/r/GetStudying/", "r/GetStudying - Reddit", "A community for students sharing study tips."],
    ["https://en.wikipedia.org/wiki/Study_skills", "Study skills - Wikipedia", "Study skills are approaches applied to learning."],
    ["https://www.reddit.com/r/college/comments/study/", "Best study tips? : r/college", "120 comments · posted by a student"],
    ["https://learning.example/pomodoro", "The Pomodoro technique, explained", "Work for 25 minutes, then take a short break."],
    ["https://library.example/guides/notes", "How to take good notes", "A guide from the university library."],
    ["https://videos.example/watch?v=study", "Study with me: 2 hours", "A video of someone studying, with a timer."],
  ]);
  const blockReddit = [{ id: 1, block: "Work hours", name: "Social media", url: ["reddit.com", "videos.example"], title: [], text: [],
    actions: [{ action: "block", distinct: 1, hits: 1 }], except: { url: [], title: [], text: [] } }];
  const before = await shoot(browser, { url: searchUrl, html: results, width: 540, height: 560, stored: { lists: [] }, scripts: true });
  const after = await shoot(browser, { url: searchUrl, html: results, width: 540, height: 560, stored: { lists: blockReddit }, scripts: true });
  await frame(browser, "screenshot-3-search.png", "No way around it through search",
    "Results leading to sites you blocked (here Reddit and a video site) are taken off search pages.",
    `<div style="position:relative">${windowHtml("searx.example.org/search?q=study+tips", before, 540, 560)}<span class="label">Without a block</span></div>` +
    `<div style="position:relative">${windowHtml("searx.example.org/search?q=study+tips", after, 540, 560)}<span class="label">With the block on</span></div>`);

  // 4: dark pages.
  const darkRule = { on: true, follow_system: false, exempt: [] };
  const light = await shoot(browser, { url: tab.url, html: ARTICLE("Why evenings feel shorter"), width: 540, height: 560,
    stored: { lists: [] }, scripts: true });
  const darkened = await shoot(browser, { url: tab.url, html: ARTICLE("Why evenings feel shorter"), width: 540, height: 560,
    stored: { lists: [], dark: darkRule }, scripts: true });
  await frame(browser, "screenshot-4-dark.png", "Dark pages at night",
    "With the circadian rhythm's dark mode on, sites show their own dark design, and light pages are shown dark.",
    `<div style="position:relative">${windowHtml(tab.url, light, 540, 560)}<span class="label">Daytime</span></div>` +
    `<div style="position:relative">${windowHtml(tab.url, darkened, 540, 560)}<span class="label">Dark mode on</span></div>`);

  await browser.close();
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
