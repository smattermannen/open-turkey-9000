// Tests the extension's matching (internal/webext/files/match.js) with Node:
//   node ci/match_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files/match.js"), "utf8"));

const filter = kc9kFilter({ match: ["reddit.com", "my-site.com", "*casino*"], except: ["reddit.com/r/programming"] });
const cases = [
  ["https://www.reddit.com/", true],
  ["https://reddit.com/r/programming/x", false],
  ["https://example.com/", false],
  ["https://news.example.com/bigcasino", true],
  ["not a url", false],
  // Google Translate's proxy: "-" stands for "." and "--" for "-".
  ["https://www-reddit-com.translate.goog/r/funny?_x_tr_sl=auto", true],
  ["https://my--site-com.translate.goog/", true],
  ["https://www-reddit-com.translate.goog/r/programming", false],
  ["https://en-wikipedia-org.translate.goog/wiki/X", false],
  // Web archives.
  ["https://web.archive.org/web/20240101000000/https://www.reddit.com/r/funny/", true],
  ["https://web.archive.org/web/2024id_/https:/reddit.com/", true],
  ["https://web.archive.org/web/*/reddit.com*", true],
  ["https://web.archive.org/web/2024/https://wikipedia.org/", false],
  ["https://archive.ph/newest/https://old.reddit.com/", true],
  ["https://archive.ph/AbCdE", false],
];

const allowlist = kc9kFilter({ match: ["*.*"], except: ["wikipedia.org"] });
cases.push(["https://news.ycombinator.com/", true, allowlist]);
cases.push(["https://en.wikipedia.org/wiki/X", false, allowlist]);

let failed = 0;
for (const [url, want, f] of cases) {
  const got = kc9kMatches(f || filter, url);
  if (got !== want) {
    failed++;
    console.log(`FAIL: ${url}: got ${got}, want ${want}`);
  }
}
// Page keywords: [keyword, text, want].
const keywordCases = [
  ["18+", "This site is 18+ only", true],
  ["18+", "Item 1118+ in stock", false],
  ["18+", "page 18+3", true],
  ["porn", "free porn here", true],
  ["porn", "Pornhub", false],
  ["porn", "PORN.", true],
  ["*porn*", "visit pornhub today", true],
  ["porn*", "a pornstar", true],
  ["porn*", "xporn", false],
  ["*porn", "xporn", true],
  ["nsfw", "NSFW content", true],
  ["nsfw", "nsfwish", false],
  ["free money", "get free   money now", true],
  ["free money", "freemoney", false],
  ["hentai", "ヘンタイ hentai!", true],
  ["café", "the cafés", false],
  ["café", "le café ouvert", true],
];
for (const [keyword, text, want] of keywordCases) {
  const got = kc9kCountKeywords(text, [keyword]) === 1;
  if (got !== want) {
    failed++;
    console.log(`FAIL: keyword ${keyword} in "${text}": got ${got}, want ${want}`);
  }
}

// Lists: distinct keywords count once each, and thresholds apply per action.
let checks = 0;
const text = "nsfw nsfw nsfw nsfw 18+";
const check = (name, got, want) => {
  checks++;
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    failed++;
    console.log(`FAIL: ${name}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
  }
};
const none = { url: [], title: [], text: [] };
check("repeats count once", kc9kCountKeywords(text, ["nsfw", "18+", "porn"]), 2);
const words = kc9kCompileLists([{
  url: [], title: [], text: ["nsfw", "18+", "porn"],
  actions: [{ action: "block", distinct: 3 }, { action: "greyscale", distinct: 1 }, { action: "no-images", distinct: 2 }],
  except: { url: ["wikipedia.org"], title: [], text: [] },
}]);
const at = (url, extra) => Object.assign({ url, title: "" }, extra);
check("thresholds", kc9kListActions(words, at("https://example.com/", { text })), { greyscale: true, "no-images": true });
check("three different block", kc9kListActions(words, at("https://example.com/", { text: text + " porn" })).block, true);
check("excepted site", kc9kListActions(words, at("https://en.wikipedia.org/wiki/X", { text: text + " porn" })), {});
check("text unknown yet", kc9kListActions(words, { url: "https://example.com/" }), {});
check("not a web page", kc9kListActions(words, at("about:blank", { text })), {});

// An address list takes every action at once, whatever the thresholds.
const sites = kc9kCompileLists([{
  url: ["reddit.com", "*casino*"], title: ["youtube"], text: [],
  actions: [{ action: "block", distinct: 5 }, { action: "greyscale", distinct: 5 }], except: none,
}]);
check("address", kc9kListActions(sites, { url: "https://old.reddit.com/r/x" }), { block: true, greyscale: true });
check("address pattern", kc9kListActions(sites, { url: "https://bigcasino.example/" }).block, true);
check("title", kc9kListActions(sites, { url: "https://example.com/", title: "YouTube - Video" }).block, true);
check("title is a whole word", kc9kListActions(sites, { url: "https://example.com/", title: "myyoutube" }), {});
check("other site", kc9kListActions(sites, { url: "https://example.com/", title: "News", text: "youtube" }), {});

// Exceptions by title or text wait until the page is read, then spare it.
const spared = kc9kCompileLists([{
  url: ["youtube.com"], title: [], text: [], actions: [{ action: "block", distinct: 1 }],
  except: { url: [], title: ["lecture"], text: ["tutorial"] },
}]);
check("waits for the title", kc9kListActions(spared, { url: "https://youtube.com/watch" }), {});
check("waits for the text", kc9kListActions(spared, { url: "https://youtube.com/watch", title: "Cats" }), {});
check("blocked once read", kc9kListActions(spared, { url: "https://youtube.com/watch", title: "Cats", text: "cats" }).block, true);
check("title exception", kc9kListActions(spared, { url: "https://youtube.com/watch", title: "Lecture 3", text: "" }), {});
check("text exception", kc9kListActions(spared, { url: "https://youtube.com/watch", title: "X", text: "a tutorial" }), {});
check("reads the page", [kc9kReadsPage(spared), kc9kReadsPage(kc9kCompileLists([{ url: ["a.com"], actions: [] }]))], [true, false]);

const total = cases.length + keywordCases.length + checks;
console.log(failed ? `${failed} of ${total} failed` : `all ${total} passed`);
process.exit(failed ? 1 : 0);
