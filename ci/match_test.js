// Tests the extension's matching (internal/webext/files/match.js) with Node:
//   node ci/match_test.js
"use strict";
const fs = require("fs");
const path = require("path");
const vm = require("vm");

vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files/match.js"), "utf8"));
vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../internal/webext/files/social.js"), "utf8"));

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
  // Word combinations: a word of one group right next to one of the other.
  ["cars(red, big, blue, beautiful)", "red beautiful cars", true],
  ["cars(red, big, blue, beautiful)", "Big cars for sale", true],
  ["cars(red, big, blue, beautiful)", "cars, big and red", false],
  ["cars(red, big, blue, beautiful)", "red cars", true],
  ["cars(red, big, blue, beautiful)", "cheap cars", false],
  ["cars(red, big, blue, beautiful)", "red car", false],
  ["car*(red, big)", "red carpets", true],
  ["car*(red, big)", "big   cars", true],
  ["car*(red, big)", "big scars", false],
  ["(car, lambo*, ferrari)(red, big, beautiful)", "a beautiful Lamborghini", true],
  ["(car, lambo*, ferrari)(red, big, beautiful)", "ferrari red", true],
  ["(car, lambo*, ferrari)(red, big, beautiful)", "a red toyota", false],
  ["(car, lambo*, ferrari)(red, big, beautiful)", "a cheap red car", true],
  ["(red, big) cars", "big cars", true],
  ["car(cheap)", "cheap car", true],
  ["car(cheap)", "car cheap", true],
  ["car(cheap)", "cheap reliable car", false],
  ["car(cheap)", "cheapest car", false],
  ["car(cheap)", "cheap", false],
  ["car(cheap)", "car", false],
  ["car (very cheap)", "a very cheap car", true],
  ["car (very cheap)", "a cheap car", false],
  // Several boxes, each needing a number of different words of it.
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "a big nice sport car", true],
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "small sheep nice nieces", true],
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "a big sport car", false],
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "a big big sport car", false],
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "big nice car, sport", false],
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "big nice car and sport", false],
  ["(car, lambo*, sheep)(big, small, nice):2(sport, hello, *niece*)", "big nice car", false],
  ["(car, lambo*) (big, small, nice):2 (sport, hello)", "Hello, a nice big Lamborghini sport", true],
  ["(a, b, c):2", "c a", true],
  ["(a, b, c):2", "a a", false],
  ["(a, b, c):3", "b c a b", true],
  // A word in two boxes goes to one of them.
  ["(red, big)(big, car)", "big", false],
  ["(red, big)(big, car)", "big red", true],
  ["(red, big)(big, car)", "big big", false],
  ["(red, big):2(big, car)", "red big car", true],
  ["(red, big):2(big, car)", "red big", false],
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

// Hits count repeats: block at 1 different word but 5 times in all.
const often = kc9kCompileLists([{
  url: [], title: [], text: ["nsfw", "18+"], except: none,
  actions: [{ action: "block", distinct: 1, hits: 5 }, { action: "greyscale", distinct: 2, hits: 3 }],
}]);
check("too few hits", kc9kListActions(often, at("https://a.com/", { text: "nsfw nsfw nsfw nsfw" })), {});
check("enough hits", kc9kListActions(often, at("https://a.com/", { text: "nsfw nsfw nsfw nsfw NSFW" })), { block: true });
check("hits and different words", kc9kListActions(often, at("https://a.com/", { text: "nsfw 18+ nsfw" })), { greyscale: true });
check("a run counts once", kc9kTextHits(kc9kKeywordMatchers(["(car)(red, big, blue):2"]), "red big blue car. big car, red. blue red car big"), { distinct: 1, total: 2 });
check("a combination counts once per pair", kc9kTextHits(kc9kKeywordMatchers(["car(red, big)"]), "red car, big car, big red car, car"), { distinct: 1, total: 3 });
check("not a combination", ["cars", "cars(red, )", "a, b(c)", "(a)", "(a, b):0", "(a)(b", "a(b)c:2", "(a):x", ")(a)(b)", "(a))(b)"].map(kc9kCombination), [null, null, null, null, null, null, null, null, null, null]);
check("combination boxes", kc9kCombination(" (car, lambo*) ( red ,big beautiful):2 (x) cars"), [
  { terms: ["car", "lambo*"], min: 1 }, { terms: ["red", "big beautiful"], min: 2 }, { terms: ["x"], min: 1 }, { terms: ["cars"], min: 1 }]);
check("one box needing two", kc9kCombination("(a, b):2"), [{ terms: ["a", "b"], min: 2 }]);
check("hits are whole words",kc9kTextHits(kc9kKeywordMatchers(["porn", "*porn*"]), "porn pornhub PORN"), { distinct: 2, total: 5 });

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

// The strongest page action wins; device actions and the lists to report.
const many = kc9kCompileLists([
  { id: 1, url: ["a.com"], actions: [{ action: "block" }, { action: "greyscale" }] },
  { id: 2, url: ["a.com/x"], actions: [{ action: "redirect", param: "https://calm.example/" }] },
  { id: 3, url: ["a.com/x/y"], actions: [{ action: "monk", minutes: 30 }, { action: "image", param: "abc.png" }] },
  { id: 4, url: ["grey.com"], actions: [{ action: "greyscale" }] },
]);
const outcome = (url) => kc9kOutcome(kc9kListMatches(many, { url }));
check("block alone", outcome("https://a.com/"), { device: false, page: "block", param: "", lists: [{ id: 1, actions: ["block"] }] });
check("redirect beats block", [outcome("https://a.com/x").page, outcome("https://a.com/x").param], ["redirect", "https://calm.example/"]);
check("image and a device action", [outcome("https://a.com/x/y").page, outcome("https://a.com/x/y").device, outcome("https://a.com/x/y").lists.length], ["image", true, 3]);
check("greyscale takes nothing away", kc9kTakesAway(outcome("https://grey.com/")), false);

const total = cases.length + keywordCases.length + checks;
console.log(failed ? `${failed} of ${total} failed` : `all ${total} passed`);
process.exit(failed ? 1 : 0);

// Proxies of any kind: a full address in the query or the path.
{
  const f = kc9kFilter({ match: ["reddit.com"], except: [] });
  const proxied = [
    ["https://someproxy.example/browse.php?u=https%3A%2F%2Fwww.reddit.com%2Fr%2Ffunny", true],
    ["https://proxy.example/?url=https://old.reddit.com/", true],
    ["https://proxy.example/go?b64=" + Buffer.from("https://reddit.com/r/x").toString("base64"), true],
    ["https://proxy.example/fetch/https://reddit.com/r/x", true],
    ["https://proxy.example/?url=https://wikipedia.org/", false],
    // Searching for an address is not visiting it.
    ["https://www.google.com/search?q=https://reddit.com/", false],
    ["https://duckduckgo.com/?q=https%3A%2F%2Freddit.com", false],
    ["https://example.com/?ref=reddit.com", false],
  ];
  for (const [url, want] of proxied) {
    if (kc9kMatches(f, url) !== want) {
      console.error("FAIL proxy", url, "want", want);
      process.exitCode = 1;
    }
  }
  if (kc9kDirectly(() => kc9kMatches(f, "https://proxy.example/?url=https://reddit.com/")) !== false) {
    console.error("FAIL kc9kDirectly still unwraps");
    process.exitCode = 1;
  }
}

// Keyword test mode: a list in test mode never acts, and reports what it found.
{
  const compiled = kc9kCompileLists([{
    id: 7, block: "b", name: "Words", test: true, url: [], title: [], text: ["casino", "poker", "*bet*"],
    actions: [{ action: "greyscale", distinct: 1, hits: 1 }, { action: "block", distinct: 2, hits: 5 }],
    except: { url: [], title: [], text: [] },
  }]);
  const page = { url: "https://example.com/", title: "", text: "casino casino poker and betting" };
  const report = kc9kTestReport(compiled, page);
  const ok = kc9kListMatches(compiled, page).length === 0 && report.length === 1 &&
    report[0].distinct === 3 && report[0].total === 4 && report[0].entries[0].entry === "casino" &&
    report[0].entries[0].count === 2 && report[0].actions[0].met === true && report[0].actions[1].met === false;
  if (!ok) {
    console.error("FAIL test mode report", JSON.stringify(report));
    process.exitCode = 1;
  }
  if (kc9kTimeLeft(42) !== "42s" || kc9kTimeLeft(125) !== "2m" || kc9kTimeLeft(3 * 3600) !== "3h") {
    console.error("FAIL kc9kTimeLeft");
    process.exitCode = 1;
  }
}

// Social media settings.
if (typeof kc9kSocialRedirect === "function") {
  const on = ["youtube:no-shorts", "youtube:no-home", "instagram:messages-only", "x:no-explore", "youtube:playlist=PLabcdefghij12"];
  const redirects = [
    ["https://www.youtube.com/shorts/abc", "https://www.youtube.com/"],
    ["https://www.youtube.com/", "https://www.youtube.com/feed/subscriptions"],
    ["https://www.youtube.com/feed/subscriptions", ""],
    ["https://www.instagram.com/explore/", "https://www.instagram.com/direct/inbox/"],
    ["https://www.instagram.com/direct/t/123", ""],
    ["https://www.instagram.com/accounts/login/", ""],
    ["https://x.com/explore/tabs/trending", "https://x.com/home"],
    ["https://twitter.com/home", ""],
    ["https://example.com/shorts/x", ""],
  ];
  for (const [url, want] of redirects) {
    if (kc9kSocialRedirect(on, url) !== want) {
      console.error("FAIL social redirect", url, "got", kc9kSocialRedirect(on, url), "want", want);
      process.exitCode = 1;
    }
  }
  const playlist = [
    ["https://www.youtube.com/watch?v=abc&list=PLabcdefghij12", false],
    ["https://www.youtube.com/watch?v=abc", true],
    ["https://www.youtube.com/watch?v=abc&list=PLother00000000", true],
    ["https://www.youtube.com/playlist?list=PLabcdefghij12", false],
    ["https://www.youtube.com/results?search_query=x", false],
    ["https://example.com/watch?v=abc", false],
  ];
  for (const [url, want] of playlist) {
    if (kc9kPlaylistBlocks(on, url) !== want) {
      console.error("FAIL playlist", url, "want", want);
      process.exitCode = 1;
    }
  }
  if (kc9kPlaylistBlocks(["youtube:no-shorts"], "https://www.youtube.com/watch?v=abc") ||
    !kc9kSocialCss(on, "https://www.youtube.com/watch?v=x").includes("ytd-reel-shelf-renderer") ||
    kc9kSocialCss(on, "https://example.com/") !== "") {
    console.error("FAIL social css or playlist without playlists");
    process.exitCode = 1;
  }
}

// SafeSearch: each engine's strict setting, POST searches moved into the
// address, engines that can't be made strict blocked, nothing else touched.
{
  const cases = [
    ["https://www.google.com/search?q=x", null, "https://www.google.com/search?q=x&safe=active"],
    ["https://www.google.co.uk/search?q=x&safe=off", null, "https://www.google.co.uk/search?q=x&safe=active"],
    ["https://www.google.com/search?q=x&safe=active", null, null],
    ["https://www.google.com/maps", null, null],
    ["https://www.bing.com/images/search?q=x", null, "https://www.bing.com/images/search?q=x&adlt=strict"],
    ["https://duckduckgo.com/", { q: ["x y"] }, "https://safe.duckduckgo.com/?q=x+y&kp=1"],
    ["https://duckduckgo.com/?q=x", null, "https://safe.duckduckgo.com/?q=x&kp=1"],
    ["https://safe.duckduckgo.com/?q=x&kp=1", { q: ["x"] }, null],
    ["https://html.duckduckgo.com/html/", { q: ["x"] }, "https://html.duckduckgo.com/html/?q=x&kp=1"],
    ["https://noai.duckduckgo.com/?q=x&kp=-2", null, "https://noai.duckduckgo.com/?q=x&kp=1"],
    ["https://search.brave.com/search?q=x", null, "https://search.brave.com/search?q=x&safesearch=strict"],
    ["https://search.yahoo.com/search?p=x", null, "https://search.yahoo.com/search?p=x&vm=r"],
    ["https://search.disroot.org/search", { q: ["x"] }, "https://search.disroot.org/search?q=x&safesearch=2"],
    ["https://example.com/search?q=x", null, null],
    ["about:blank", null, null],
  ];
  for (const [url, form, want] of cases) {
    const got = kc9kSafeSearch(url, form);
    const redirect = got && got.redirect ? got.redirect : null;
    if (redirect !== want || (got && got.block)) {
      console.error("FAIL safesearch", url, "got", JSON.stringify(got), "want", want);
      process.exitCode = 1;
    }
  }
  for (const url of ["https://www.startpage.com/sp/search", "https://www.mojeek.com/search?q=x",
    "https://www.ecosia.org/search?q=x", "https://yandex.ru/search/?text=x", "https://marginalia-search.com/search"]) {
    const got = kc9kSafeSearch(url, null);
    if (!got || !got.block) {
      console.error("FAIL safesearch should block", url, JSON.stringify(got));
      process.exitCode = 1;
    }
  }
}
