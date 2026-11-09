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

// Distinct keywords count once each, and thresholds apply per action.
const total = cases.length + keywordCases.length + 6;
const text = "nsfw nsfw nsfw nsfw 18+";
const check = (name, got, want) => {
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    failed++;
    console.log(`FAIL: ${name}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
  }
};
check("repeats count once", kc9kCountKeywords(text, ["nsfw", "18+", "porn"]), 2);
const rules = [{ keywords: ["nsfw", "18+", "porn"], block: 3, greyscale: 1, no_images: 2, except: ["wikipedia.org"] }];
check("thresholds", kc9kKeywordActions(rules, "https://example.com/", text), { block: false, greyscale: true, noImages: true });
check("three different block", kc9kKeywordActions(rules, "https://example.com/", text + " porn").block, true);
check("excepted site", kc9kKeywordActions(rules, "https://en.wikipedia.org/wiki/X", text + " porn"), { block: false, greyscale: false, noImages: false });
check("untaken action", kc9kKeywordActions([{ keywords: ["nsfw"], block: 0, greyscale: 1, no_images: 0 }], "https://a.com/", "nsfw").block, false);
check("not a web page", kc9kKeywordActions(rules, "about:blank", text).greyscale, false);

console.log(failed ? `${failed} of ${total} failed` : `all ${total} passed`);
process.exit(failed ? 1 : 0);
