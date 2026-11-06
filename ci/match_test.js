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
console.log(failed ? `${failed} of ${cases.length} failed` : `all ${cases.length} passed`);
process.exit(failed ? 1 : 0);
