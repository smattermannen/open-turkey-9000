// Checks that the rules KC9K on Android serves work in the extension's own matcher:
//   (cd android && ./gradlew testDebugUnitTest) && node ci/android_rules_test.js
"use strict";
const fs = require("fs");
const os = require("os");
const path = require("path");
const vm = require("vm");

vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files/match.js"), "utf8"));
const rules = JSON.parse(fs.readFileSync(path.join(os.tmpdir(), "kc9k-android-rules.json"), "utf8"));
const lists = kc9kCompileLists(rules.lists);
const cases = [
  ["https://www.reddit.com/", true],
  ["https://old.reddit.com/r/funny", true],
  ["https://www.reddit.com/r/programming/comments/x", false],
  ["https://www.youtube.com/shorts/abc", true],
  ["https://www.youtube.com/watch?v=abc", false],
  ["https://bigcasino.example/", true],
  ["https://site.xxx/", true],
  ["https://web.archive.org/web/2024/https://reddit.com/", true],
  ["https://example.com/", false],
];
let failed = 0;
for (const [url, want] of cases) {
  const got = kc9kTakesAway(kc9kOutcome(kc9kListMatches(lists, { url })));
  if (got !== want) {
    console.error("FAIL", url, "got", got, "want", want);
    failed++;
  }
}
if (!rules.safesearch || !rules.web_only) {
  console.error("FAIL safesearch/web_only flags", rules.safesearch, rules.web_only);
  failed++;
}
if (failed) {
  process.exit(1);
}
console.log("android rules: all " + cases.length + " passed");
