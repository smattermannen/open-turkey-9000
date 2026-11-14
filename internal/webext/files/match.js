"use strict";

// Shared by the background script, the content script and the block page:
// turns list entries into matchers and tests URLs against them.

function kc9kEscapeRegex(text) {
  return text.replace(/[.+?^${}()|[\]\\]/g, "\\$&");
}

// Entries work as in Cold Turkey: reddit.com, reddit.com/r/funny, *.xxx, *.*,
// and keywords such as *casino*.
// An entry for another scheme (file://*) never matches a web address: with
// its scheme stripped it would read as "*", every address.
function kc9kCompile(entry) {
  const scheme = /^([a-z*]+):\/\//i.exec(entry);
  if (scheme && !/^(https?|ftp|\*)$/i.test(scheme[1])) {
    return /(?!)/;
  }
  const value = entry.replace(/^[a-z*]+:\/\//i, "").replace(/^www\./, "").toLowerCase();
  if (value === "*" || value === "*.*") {
    return /.*/;
  }
  const pattern = kc9kEscapeRegex(value).replace(/\*/g, ".*");
  if (value.startsWith("*.")) {
    return new RegExp("^[^/]*\\." + pattern.slice(4) + (value.includes("/") ? "" : "(/|$)"));
  }
  if (value.startsWith("*")) {
    return new RegExp("^" + pattern);
  }
  return new RegExp("^([^/]*\\.)?" + pattern + (value.includes("/") ? "" : "(/|$)"));
}

function kc9kTarget(url) {
  return url.replace(/^[a-z-]+:\/\//i, "").replace(/^www\./i, "").toLowerCase();
}

// kc9kIndex compiles entries into a lookup that costs about the same with 10
// entries or 100,000 (principle 9): plain sites ("reddit.com") and wildcard
// subdomains ("*.reddit.com", subdomains only) go in sets looked up by each
// suffix of the address's host, entries with a path but no wildcard in their
// domain ("reddit.com/r/funny") are filed under their domain, and only the
// rest ("*casino*", "*.*") are tried one by one. It answers exactly as
// testing every kc9kCompile'd entry would (ci/match_test.js checks).
function kc9kIndex(entries) {
  const index = { length: 0, sites: new Set(), subs: new Set(), byDomain: new Map(), rest: [] };
  for (const entry of entries || []) {
    index.length++;
    const scheme = /^([a-z*]+):\/\//i.exec(entry);
    if (scheme && !/^(https?|ftp|\*)$/i.test(scheme[1])) {
      continue; // file://* and the like never match a web address
    }
    const value = entry.replace(/^[a-z*]+:\/\//i, "").replace(/^www\./, "").toLowerCase();
    const slash = value.indexOf("/");
    const domain = (slash < 0 ? value : value.slice(0, slash)).replace(/^\*\./, "");
    if (value === "*" || value === "*.*" || domain === "" || domain.includes("*")) {
      index.rest.push(kc9kCompile(entry));
    } else if (slash < 0) {
      (value.startsWith("*.") ? index.subs : index.sites).add(domain);
    } else {
      const rules = index.byDomain.get(domain) || [];
      rules.push(kc9kCompile(entry));
      index.byDomain.set(domain, rules);
    }
  }
  return index;
}

// kc9kIndexTest reports whether an index matches a target (kc9kTarget).
function kc9kIndexTest(index, text) {
  if (index.length === 0) {
    return false;
  }
  const slash = text.indexOf("/");
  const host = slash < 0 ? text : text.slice(0, slash);
  if (index.sites.size > 0 || index.subs.size > 0 || index.byDomain.size > 0) {
    for (let at = -1; ; at = host.indexOf(".", at + 1)) {
      const suffix = host.slice(at + 1);
      if (index.sites.has(suffix) || (at >= 0 && index.subs.has(suffix))) {
        return true;
      }
      const rules = index.byDomain.get(suffix);
      if (rules && rules.some((rule) => rule.test(text))) {
        return true;
      }
      if (host.indexOf(".", at + 1) < 0) {
        break;
      }
    }
  }
  return index.rest.some((rule) => rule.test(text));
}

// A compiled filter: an entry in match that no entry in except matches.
function kc9kFilter(raw) {
  return {
    match: kc9kIndex((raw && raw.match) || []),
    except: kc9kIndex((raw && raw.except) || []),
  };
}

// kc9kEmbedded turns an address written inside another one into a full URL:
// "https://reddit.com/x", "https:/reddit.com/x" or "reddit.com/x".
function kc9kEmbedded(text) {
  let value = text;
  if (/^https?%3a/i.test(value)) {
    try {
      value = decodeURIComponent(value);
    } catch (error) {
      return "";
    }
  }
  // A Wayback search such as reddit.com* lists a site's captures.
  value = value.replace(/^(https?):\/+/i, "$1://").replace(/\*+$/, "");
  return /^https?:\/\//i.test(value) ? value : "https://" + value;
}

// kc9kUnwrap returns the sites shown through a proxy address, so a blocked
// site can't be reached through Google Translate's proxy or a web archive.
function kc9kUnwrap(url) {
  let u;
  try {
    u = new URL(url);
  } catch (error) {
    return [];
  }
  const host = u.hostname.toLowerCase();
  const out = [];
  // Google Translate: www-reddit-com.translate.goog shows www.reddit.com.
  // A "-" in the label stands for ".", and "--" for "-".
  if (host.endsWith(".translate.goog")) {
    const label = host.slice(0, -".translate.goog".length);
    const site = label.replace(/--/g, "\u0000").replace(/-/g, ".").replace(/\u0000/g, "-");
    out.push("https://" + site + u.pathname + u.search);
  }
  // The Wayback Machine: web.archive.org/web/<time>/<address>.
  if (host === "web.archive.org" || host === "wayback.archive.org") {
    const m = (u.pathname + u.search).match(/^\/web\/[^/]*\/(.+)$/);
    if (m) {
      out.push(kc9kEmbedded(m[1]));
    }
  }
  // archive.today and its mirrors: archive.ph/newest/<address>.
  if (/^archive\.(ph|is|today|li|vn|fo|md)$/.test(host)) {
    const m = (u.pathname + u.search).match(/^\/(?:newest|oldest|timegate|timemap\/[a-z]+|\d{4,14})\/(.+)$/);
    if (m) {
      out.push(kc9kEmbedded(m[1]));
    }
  }
  // Any other proxy: a full web address carried in the query (?url=https://…,
  // also percent-encoded or base64) or after a slash in the path. A search
  // for an address is not a visit to it, so search pages are left alone.
  const searching = /(^|\/)(search|results?)(\/|$)/i.test(u.pathname);
  for (const [name, value] of u.searchParams) {
    if (searching || /^(q|query|search|text|p|s|k|keywords?)$/i.test(name)) {
      continue;
    }
    let inner = value;
    if (/^aHR0c(HM6Ly|DovL)/.test(inner)) {
      try {
        inner = atob(inner.replace(/-/g, "+").replace(/_/g, "/"));
      } catch (error) {
        continue;
      }
    }
    if (/^https?:\/\/[^/\s]+\.[^/\s]+/i.test(inner)) {
      out.push(inner);
    }
  }
  const inPath = u.pathname.match(/\/(https?:\/{1,2}[^/\s]+\.[^\s]+)$/i);
  if (inPath) {
    out.push(kc9kEmbedded(inPath[1] + u.search));
  }
  return out.filter(Boolean);
}

// kc9kUnwrapOff turns the proxy unwrapping off for a moment, to tell a page
// blocked for its own address from one blocked for the site it shows.
let kc9kUnwrapOff = false;

function kc9kDirectly(run) {
  kc9kUnwrapOff = true;
  try {
    return run();
  } finally {
    kc9kUnwrapOff = false;
  }
}

function kc9kMatchesOne(filter, url) {
  if (!url || !/^(https?|ftp):/i.test(url)) {
    return false;
  }
  const text = kc9kTarget(url);
  return kc9kIndexTest(filter.match, text) && !kc9kIndexTest(filter.except, text);
}

// kc9kMatches reports whether the filter applies to url or to a site shown
// through it.
function kc9kMatches(filter, url) {
  return kc9kMatchesOne(filter, url) ||
    (!kc9kUnwrapOff && kc9kUnwrap(url).some((inner) => kc9kMatchesOne(filter, inner)));
}

// Page keywords. A plain keyword counts only as a whole word: "porn" misses
// "pornhub" and "18+" misses "1118+". A * stands for any letters or digits,
// so "*porn*" hits "pornhub". Spaces match any run of whitespace. Case is
// ignored.
const KC9K_WORD = "[\\p{L}\\p{N}]";

// kc9kTermSource is the pattern for one word or phrase.
function kc9kTermSource(term) {
  const value = String(term).trim().toLowerCase();
  const body = value
    .split("*")
    .map((part) => kc9kEscapeRegex(part).replace(/\s+/g, "\\s+"))
    .join(KC9K_WORD + "*");
  const before = /^[\p{L}\p{N}]/u.test(value) ? "(?<!" + KC9K_WORD + ")" : "";
  const after = /[\p{L}\p{N}]$/u.test(value) ? "(?!" + KC9K_WORD + ")" : "";
  return before + body + after;
}

// Word combinations: boxes of words side by side, each a list in brackets
// with how many different words of it are needed (":2"; 1 if left out), or
// one bare word or phrase: "cars(red, big)", "(red, big) cars",
// "(car, lambo*) (big, small, nice):2 (sport, *niece*)". A combination counts
// where a run of words, with only spaces between them and each a word of
// some box, holds the needed number of different words of every box, in any
// order: car(cheap) counts in "cheap car" and "car cheap", but not in
// "cheap reliable car". Each such run counts once toward hits. Keep in step
// with pageCombination in internal/cli/keywords.go.
const KC9K_MAX_BOXES = 9;
const KC9K_MAX_BOX_MIN = 9;
const KC9K_MAX_COMBINATION_RUN = 64;

// kc9kCombination returns a combination's boxes, [{ terms, min }], or null
// when the keyword isn't a well-formed one.
function kc9kCombination(keyword) {
  const value = String(keyword).trim();
  if (!/[()]/.test(value)) {
    return null;
  }
  const boxes = [];
  const bare = (text) => {
    const term = text.trim();
    if (term === "") {
      return true;
    }
    if (/[,:()]/.test(term) || !kc9kUsableKeyword(term)) {
      return false;
    }
    boxes.push({ terms: [term], min: 1 });
    return true;
  };
  const group = /\(([^()]*)\)(?:\s*:\s*(\d+))?/g;
  let last = 0;
  let m;
  while ((m = group.exec(value)) !== null) {
    if (!bare(value.slice(last, m.index))) {
      return null;
    }
    const terms = m[1].split(",").map((term) => term.trim());
    const min = m[2] === undefined ? 1 : Number(m[2]);
    if (!terms.every(kc9kUsableKeyword) || min < 1 || min > KC9K_MAX_BOX_MIN) {
      return null;
    }
    boxes.push({ terms, min });
    last = group.lastIndex;
  }
  const rest = value.slice(last);
  if (/[()]/.test(rest) || !bare(rest)) {
    return null;
  }
  const needed = boxes.reduce((sum, box) => sum + box.min, 0);
  return needed >= 2 && boxes.length <= KC9K_MAX_BOXES ? boxes : null;
}

// kc9kCanFill reports whether words, each with the boxes it belongs to, can
// give every box its minimum when each word goes to one box (a matching of
// the boxes' places to words).
function kc9kCanFill(words, boxes) {
  const places = [];
  boxes.forEach((box, b) => {
    for (let i = 0; i < box.min; i++) {
      places.push(b);
    }
  });
  if (places.length > words.length) {
    return false;
  }
  const owner = new Array(words.length).fill(-1);
  const take = (p, seen) => {
    for (let w = 0; w < words.length; w++) {
      if (!seen[w] && words[w].includes(places[p])) {
        seen[w] = true;
        if (owner[w] < 0 || take(owner[w], seen)) {
          owner[w] = p;
          return true;
        }
      }
    }
    return false;
  };
  return places.every((b, p) => take(p, new Array(words.length).fill(false)));
}

// kc9kCombinationMatcher counts a combination's runs of words in a text.
function kc9kCombinationMatcher(boxes) {
  const terms = [];
  boxes.forEach((box, b) => box.terms.forEach((term) => terms.push({ term, box: b })));
  const sources = terms.map((t) => kc9kTermSource(t.term));
  const whole = sources.map((source) => new RegExp("^(?:" + source + ")$", "iu"));
  // Longer terms first, so "big beautiful" is read before "big".
  const order = sources.map((source, i) => i).sort((a, b) => terms[b].term.length - terms[a].term.length);
  const any = order.map((i) => "(?:" + sources[i] + ")").join("|");
  const count = (input) => {
    const text = String(input);
    const finder = new RegExp(any, "giu");
    const known = new Map(); // a word -> the boxes it belongs to
    let found = 0;
    let run = [];
    let end = -1;
    const close = () => {
      const words = new Map();
      for (const word of run) {
        const key = word.toLowerCase().replace(/\s+/g, " ");
        if (!known.has(key)) {
          known.set(key, [...new Set(terms.filter((t, i) => whole[i].test(word)).map((t) => t.box))]);
        }
        words.set(key, known.get(key));
      }
      if (kc9kCanFill([...words.values()], boxes)) {
        found++;
      }
      run = [];
    };
    let m;
    while ((m = finder.exec(text)) !== null) {
      if (m[0] === "") {
        finder.lastIndex++;
        continue;
      }
      const joined = run.length > 0 && run.length < KC9K_MAX_COMBINATION_RUN &&
        /^\s+$/u.test(text.slice(end, m.index));
      if (run.length > 0 && !joined) {
        close();
      }
      run.push(m[0]);
      end = m.index + m[0].length;
    }
    if (run.length > 0) {
      close();
    }
    return found;
  };
  return { count, test: (text) => count(text) > 0 };
}

// kc9kKeywordMatcher turns a keyword into { test(text), count(text) }:
// whether it occurs in a text, and how many times.
function kc9kKeywordMatcher(keyword) {
  const boxes = kc9kCombination(keyword);
  if (boxes) {
    return kc9kCombinationMatcher(boxes);
  }
  const source = kc9kTermSource(keyword);
  const regex = new RegExp(source, "iu");
  return {
    test: (text) => regex.test(text),
    count: (text) => (String(text).match(new RegExp(source, "giu")) || []).length,
  };
}

// kc9kUsableKeyword drops entries that are only wildcards or blank.
function kc9kUsableKeyword(keyword) {
  return String(keyword).replace(/\*/g, "").trim() !== "";
}

function kc9kKeywordMatchers(keywords) {
  const usable = (keywords || []).filter(kc9kUsableKeyword);
  const matchers = usable.map(kc9kKeywordMatcher);
  matchers.index = kc9kWordIndex(usable);
  return matchers;
}

// kc9kWordIndex files the one-word terms of a list (letters and digits, with
// * for any letters or digits) so that a text is read once, word by word,
// whatever the number of terms (principle 9): exact words in a map,
// "porn*" by prefix and "*porn" by suffix, other one-word patterns tested
// per word. It counts exactly as the terms' regular expressions do: one
// count per word that matches. Terms of several words, with other
// characters, and combinations are left to their own matcher (rest).
function kc9kWordIndex(terms) {
  const index = {
    exact: new Map(), prefix: new Map(), suffix: new Map(),
    globStart: new Map(), globEnd: new Map(), globs: [], rest: [],
  };
  const add = (map, key, i) => {
    const list = map.get(key) || [];
    list.push(i);
    map.set(key, list);
  };
  terms.forEach((term, i) => {
    const value = String(term).trim().toLowerCase();
    if (kc9kCombination(term) || !/^[\p{L}\p{N}*]+$/u.test(value)) {
      index.rest.push(i);
      return;
    }
    const parts = value.split("*");
    if (parts.length === 1) {
      add(index.exact, value, i);
    } else if (parts.length === 2 && parts[1] === "" && parts[0] !== "") {
      add(index.prefix, parts[0], i);
    } else if (parts.length === 2 && parts[0] === "" && parts[1] !== "") {
      add(index.suffix, parts[1], i);
    } else if (parts[0] !== "") {
      add(index.globStart, parts[0], { i, parts });
    } else if (parts[parts.length - 1] !== "") {
      add(index.globEnd, parts[parts.length - 1], { i, parts });
    } else {
      index.globs.push({ i, parts });
    }
  });
  const lengths = (map) => [...new Set([...map.keys()].map((k) => k.length))];
  index.prefixLengths = lengths(index.prefix);
  index.suffixLengths = lengths(index.suffix);
  index.globStartLengths = lengths(index.globStart);
  index.globEndLengths = lengths(index.globEnd);
  return index;
}

// kc9kGlobWord reports whether a whole word fits a pattern split at its *s:
// the first part starts it, the last ends it, the others come in order.
function kc9kGlobWord(word, parts) {
  if (!word.startsWith(parts[0]) || !word.endsWith(parts[parts.length - 1])) {
    return false;
  }
  let at = parts[0].length;
  for (let p = 1; p < parts.length - 1; p++) {
    const found = word.indexOf(parts[p], at);
    if (found < 0) {
      return false;
    }
    at = found + parts[p].length;
  }
  return at <= word.length - parts[parts.length - 1].length;
}

// kc9kIndexCounts reads a text once and returns how many times each indexed
// term occurs (by its position in the list); terms in rest are left out.
function kc9kIndexCounts(index, text) {
  const counts = new Map();
  const hit = (list) => {
    for (const i of list) {
      counts.set(i, (counts.get(i) || 0) + 1);
    }
  };
  const words = String(text).toLowerCase().match(/[\p{L}\p{N}]+/gu) || [];
  for (const word of words) {
    const exact = index.exact.get(word);
    if (exact) {
      hit(exact);
    }
    for (const n of index.prefixLengths) {
      const list = n <= word.length && index.prefix.get(word.slice(0, n));
      if (list) {
        hit(list);
      }
    }
    for (const n of index.suffixLengths) {
      const list = n <= word.length && index.suffix.get(word.slice(word.length - n));
      if (list) {
        hit(list);
      }
    }
    const globs = (list) => {
      for (const glob of list || []) {
        if (kc9kGlobWord(word, glob.parts)) {
          hit([glob.i]);
        }
      }
    };
    for (const n of index.globStartLengths) {
      if (n <= word.length) {
        globs(index.globStart.get(word.slice(0, n)));
      }
    }
    for (const n of index.globEndLengths) {
      if (n <= word.length) {
        globs(index.globEnd.get(word.slice(word.length - n)));
      }
    }
    globs(index.globs);
  }
  return counts;
}

// kc9kCountKeywords returns how many of the keywords occur in text; a keyword
// found many times counts once.
function kc9kCountKeywords(text, keywords) {
  return kc9kKeywordMatchers(keywords).filter((matcher) => matcher.test(text)).length;
}

// kc9kTextHits counts, in text, how many of the matchers occur (each once)
// and how many times they occur in all.
function kc9kTextHits(matchers, text) {
  let distinct = 0;
  let total = 0;
  if (matchers.index) {
    for (const n of kc9kIndexCounts(matchers.index, text).values()) {
      distinct++;
      total += n;
    }
    for (const i of matchers.index.rest) {
      const n = matchers[i].count(text);
      if (n > 0) {
        distinct++;
        total += n;
      }
    }
    return { distinct, total };
  }
  for (const matcher of matchers) {
    const n = matcher.count(text);
    if (n > 0) {
      distinct++;
      total += n;
    }
  }
  return { distinct, total };
}

// kc9kCompileLists compiles the daemon's blacklists (see the daemon's
// ListRule): what each looks for in a page's address, title and text, its
// actions, and its block's exceptions.
function kc9kCompileLists(lists) {
  return (lists || []).map((list) => {
    const except = list.except || {};
    return {
      id: list.id,
      block: list.block,
      name: list.name || "",
      test: Boolean(list.test),
      titleRaw: (list.title || []).filter(kc9kUsableKeyword),
      textRaw: (list.text || []).filter(kc9kUsableKeyword),
      url: kc9kFilter({ match: list.url || [] }),
      title: kc9kKeywordMatchers(list.title),
      text: kc9kKeywordMatchers(list.text),
      // A phrase set: its phrases, matched where its scope says.
      phrases: (list.phrases || []).map((phrase) => ({
        name: kc9kPhraseText(phrase),
        url: kc9kPhraseMatcher(phrase, "url"),
        page: kc9kPhraseMatcher(phrase),
      })),
      phraseScope: list.phrase_scope || [],
      actions: list.actions || [],
      except: {
        url: kc9kFilter({ match: except.url || [] }),
        title: kc9kKeywordMatchers(except.title),
        text: kc9kKeywordMatchers(except.text),
      },
    };
  });
}

// kc9kReadsPage reports whether any list needs the page's title or text.
function kc9kReadsPage(compiled) {
  return compiled.some((list) => list.title.length > 0 || list.text.length > 0 ||
    (list.phrases.length > 0 && (list.phraseScope.includes("title") || list.phraseScope.includes("text"))) ||
    list.except.title.length > 0 || list.except.text.length > 0);
}

// kc9kListMatches works out which lists act on a page, and with which
// actions: [{ list, actions: [action objects] }]. page holds its url, and its
// title and text when they are known (undefined otherwise). A list whose
// exceptions look at a part of the page that isn't known yet is left until
// it is, so a page an exception spares isn't blocked before it is read.
// A match in the address or the title takes every action of the list; in
// the text, an action needs its number of different entries (distinct) and
// of times they occur in all (hits).
function kc9kListMatches(compiled, page) {
  const out = [];
  if (!page || !page.url || !/^(https?|ftp):/i.test(page.url)) {
    return out;
  }
  const hasTitle = typeof page.title === "string";
  const hasText = typeof page.text === "string";
  for (const list of compiled) {
    const except = list.except;
    if (list.test) {
      continue; // shown, not run: see kc9kTestReport
    }
    if ((except.title.length > 0 && !hasTitle) || (except.text.length > 0 && !hasText)) {
      continue;
    }
    if ((except.url.match.length > 0 && kc9kMatches(except.url, page.url)) ||
      (hasTitle && except.title.some((matcher) => matcher.test(page.title))) ||
      (hasText && except.text.some((matcher) => matcher.test(page.text)))) {
      continue;
    }
    if (list.phrases.length > 0) {
      // A phrase set: an action needs its number of different phrases.
      const n = kc9kPhrasesFound(list, page).length;
      const acts = list.actions.filter((action) => n > 0 && n >= (action.distinct || 1));
      if (acts.length > 0) {
        out.push({ list, actions: acts });
      }
      continue;
    }
    const whole = (list.url.match.length > 0 && kc9kMatches(list.url, page.url)) ||
      (hasTitle && list.title.some((matcher) => matcher.test(page.title)));
    const found = hasText && list.text.length > 0 ? kc9kTextHits(list.text, page.text) : { distinct: 0, total: 0 };
    const actions = list.actions.filter((action) => whole ||
      (found.distinct > 0 && found.distinct >= (action.distinct || 1) && found.total >= (action.hits || 1)));
    if (actions.length > 0) {
      out.push({ list, actions });
    }
  }
  return out;
}

// kc9kPhraseText writes a phrase as it is typed: [a, b] [c]? :N.
function kc9kPhraseText(phrase) {
  const sets = phrase.sets || [];
  const necessary = sets.filter((set, i) => i < 2 || set.necessary).length;
  const text = sets.map((set, i) => "[" + (set.terms || []).join(", ") + "]" + (i >= 2 && !set.necessary ? "?" : "")).join(" ");
  return phrase.length && phrase.length !== necessary ? text + " :" + phrase.length : text;
}

// kc9kPhrasesFound returns the names of a phrase set's phrases that match
// the page where the set looks (only the parts of the page known so far).
function kc9kPhrasesFound(list, page) {
  const scope = list.phraseScope;
  return list.phrases.filter((p) =>
    (scope.includes("url") && p.url.test(page.url)) ||
    (scope.includes("title") && typeof page.title === "string" && p.page.test(page.title)) ||
    (scope.includes("text") && typeof page.text === "string" && p.page.test(page.text))).map((p) => p.name);
}

// kc9kTestReport describes, for each list in keyword test mode, what it
// finds on a page: [{ block, name, spared, address, title, entries: [{ entry,
// count }], distinct, total, actions: [{ action, distinct, hits, met }] }].
// spared is true when one of the block's exceptions matches the page.
function kc9kTestReport(compiled, page) {
  const out = [];
  if (!page || !page.url || !/^(https?|ftp):/i.test(page.url)) {
    return out;
  }
  const title = typeof page.title === "string" ? page.title : "";
  const text = typeof page.text === "string" ? page.text : "";
  for (const list of compiled) {
    if (!list.test) {
      continue;
    }
    const except = list.except;
    const spared = (except.url.match.length > 0 && kc9kMatches(except.url, page.url)) ||
      except.title.some((matcher) => matcher.test(title)) || except.text.some((matcher) => matcher.test(text));
    const address = list.url.match.length > 0 && kc9kMatches(list.url, page.url);
    const inTitle = list.titleRaw.filter((entry, i) => list.title[i].test(title));
    const entries = [];
    let total = 0;
    for (const name of list.phrases.length > 0 ? kc9kPhrasesFound(list, { url: page.url, title, text }) : []) {
      entries.push({ entry: name, count: 1 });
      total++;
    }
    list.textRaw.forEach((entry, i) => {
      const n = list.text[i].count(text);
      if (n > 0) {
        entries.push({ entry, count: n });
        total += n;
      }
    });
    entries.sort((a, b) => b.count - a.count);
    const whole = address || inTitle.length > 0;
    out.push({
      block: list.block, name: list.name, spared, address, title: inTitle, entries,
      distinct: entries.length, total,
      actions: list.actions.map((action) => ({
        action: action.action, distinct: action.distinct || 1, hits: action.hits || 1,
        met: !spared && (whole || (entries.length > 0 && entries.length >= (action.distinct || 1) && total >= (action.hits || 1))),
      })),
    });
  }
  return out;
}

// kc9kTimeLeft renders seconds as a few characters for the toolbar badge.
function kc9kTimeLeft(seconds) {
  if (seconds < 60) {
    return Math.max(0, Math.round(seconds)) + "s";
  }
  if (seconds < 100 * 60) {
    return Math.round(seconds / 60) + "m";
  }
  return Math.round(seconds / 3600) + "h";
}

// kc9kListActions is which actions the lists take on a page: an object with
// a true value for each action name.
function kc9kListActions(compiled, page) {
  const out = {};
  for (const match of kc9kListMatches(compiled, page)) {
    for (const action of match.actions) {
      out[action.action] = true;
    }
  }
  return out;
}

// Actions that take the page away (rather than change how it looks), and
// those that reach past the browser tab.
const KC9K_PAGE_ACTIONS = ["close-tab", "image", "redirect", "block"];
const KC9K_DEVICE_ACTIONS = ["close-browser", "logout", "shutdown", "monk"];

// kc9kOutcome turns matches into what to do: { device, page, param, lists }.
// device is true when any device action applies (the tab and the browser
// close); page is the strongest page action ("close-tab", "image",
// "redirect", "block" or ""), and param its address or image. lists holds
// [{ id, actions: [names] }] for the matches that take the page away, to
// report to the daemon.
function kc9kOutcome(matches) {
  const out = { device: false, page: "", param: "", lists: [] };
  let rank = KC9K_PAGE_ACTIONS.length;
  for (const match of matches) {
    const names = match.actions.map((action) => action.action);
    const away = names.filter((name) => KC9K_PAGE_ACTIONS.includes(name) || KC9K_DEVICE_ACTIONS.includes(name));
    if (away.length === 0) {
      continue;
    }
    out.lists.push({ id: match.list.id, actions: away });
    for (const action of match.actions) {
      if (KC9K_DEVICE_ACTIONS.includes(action.action)) {
        out.device = true;
      }
      const r = KC9K_PAGE_ACTIONS.indexOf(action.action);
      if (r >= 0 && r < rank) {
        rank = r;
        out.page = action.action;
        out.param = action.param || "";
      }
    }
  }
  return out;
}

// kc9kTakesAway reports whether an outcome takes the page away.
function kc9kTakesAway(outcome) {
  return outcome.device || outcome.page !== "";
}

// SafeSearch ("safesearch" in the rules; KC9K on Android sends it): a
// search gets its engine's strictest setting, and search engines whose
// setting can't be forced from the address are blocked. form holds a POST
// search's fields (webRequest requestBody.formData): IronFox sends its
// searches as POST, so they are moved into the address, which then carries
// the setting whether the request stays a POST or not. Returns { block: true },
// { redirect: address } or null when the address is already right.
const KC9K_UNSAFE_SEARCH = /^(([a-z0-9-]+\.)*(startpage\.com|ecosia\.org|mojeek\.com|marginalia-search\.com|marginalia\.nu)|(www\.)?yandex\.[a-z]{2,3}(\.[a-z]{2})?|ya\.ru)$/;

function kc9kSafeSearch(rawUrl, form) {
  let url;
  try {
    url = new URL(rawUrl);
  } catch (error) {
    return null;
  }
  if (url.protocol !== "https:" && url.protocol !== "http:") {
    return null;
  }
  const host = url.hostname.toLowerCase();
  const path = url.pathname;
  if (KC9K_UNSAFE_SEARCH.test(host)) {
    return { block: true };
  }
  let setting = null; // [parameter, value]
  let safeHost = host;
  if (/^([a-z0-9-]+\.)*google(\.[a-z]{2,3}){1,2}$/.test(host) && path === "/search") {
    setting = ["safe", "active"];
  } else if (/^(www\.)?bing\.com$/.test(host) && /^\/((images|videos|news)\/)?search$/.test(path)) {
    setting = ["adlt", "strict"];
  } else if (/^([a-z0-9-]+\.)?duckduckgo\.com$/.test(host) && /^\/((html|lite)\/?)?$/.test(path)) {
    setting = ["kp", "1"];
    // safe.duckduckgo.com is always strict, cookies or not.
    if (host === "duckduckgo.com" || host === "www.duckduckgo.com") {
      safeHost = "safe.duckduckgo.com";
    }
  } else if (host === "search.brave.com" && /^\/(search|images|videos|news)$/.test(path)) {
    setting = ["safesearch", "strict"];
  } else if (/^([a-z0-9-]+\.)*search\.yahoo\.com$/.test(host) && /^\/(search|.*\/search)$/.test(path)) {
    setting = ["vm", "r"];
  } else if ((host === "search.disroot.org" || /^searx(ng)?\./.test(host)) && path === "/search") {
    setting = ["safesearch", "2"]; // SearXNG
  }
  if (!setting) {
    return null;
  }
  const fields = form && typeof form === "object" ? Object.keys(form) : [];
  // Only fields not in the address yet count as a change: a redirected
  // POST that stays a POST comes back with the same form.
  let changed = safeHost !== host;
  for (const name of fields) {
    if (!url.searchParams.has(name)) {
      for (const value of [].concat(form[name])) {
        url.searchParams.append(name, String(value));
        changed = true;
      }
    }
  }
  if (url.searchParams.get(setting[0]) !== setting[1]) {
    url.searchParams.set(setting[0], setting[1]);
    changed = true;
  }
  if (!changed) {
    return null;
  }
  return { redirect: url.protocol + "//" + safeHost + (url.port ? ":" + url.port : "") + url.pathname + url.search };
}

// Phrases (SPEC R-PHRASE-1 to 9). A phrase is word sets, each a list of
// terms and necessary or optional; a match takes one term from each
// necessary set, plus terms from optional sets until it has `length` terms,
// one term per set at most. The terms may come in any order, directly after
// each other with only blank space between them; punctuation breaks a match,
// and so does the end of a sentence (. ! ? or a line break: a break between
// parts of a page). An apostrophe inside a word is part of it, and a hyphen
// between letters is blank space. A bridge, "big-" … "-gorilla", lets up to
// 20 words of the same sentence (commas allowed) come between a "big-" and
// a "-gorilla" of another set, in that order; "-strong-" is a middle link.
// Terms are words or several words, and * matches inside a word.
const KC9K_BRIDGE_WORDS = 20;

// kc9kPhraseTokens splits a text into sentences of words, each word with
// the gap before it: "space" (only blank space), "comma" or "punct".
function kc9kPhraseTokens(text) {
  const sentences = [];
  let words = [];
  let gap = "space";
  const value = String(text).toLowerCase()
    .replace(/(\p{L})-(?=\p{L})/gu, "$1 "); // long-cat is long cat
  const re = /([\p{L}\p{N}]+(?:['’][\p{L}\p{N}]+)*)|(\s+)|([.!?]+)|(,)|([^\s\p{L}\p{N}])/gu;
  let m;
  const end = () => {
    if (words.length > 0) {
      sentences.push(words);
    }
    words = [];
    gap = "space";
  };
  while ((m = re.exec(value)) !== null) {
    if (m[1] !== undefined) {
      words.push({ word: m[1].replace(/’/g, "'"), gap });
      gap = "space";
    } else if (m[2] !== undefined) {
      if (/[\n\r]/.test(m[2])) {
        end();
      }
    } else if (m[3] !== undefined) {
      end();
    } else if (m[4] !== undefined) {
      gap = gap === "punct" ? "punct" : "comma";
    } else {
      gap = "punct";
    }
  }
  end();
  return sentences;
}

// kc9kPhraseTerm parses one term: its words (with * patterns) and bridges.
function kc9kPhraseTerm(raw) {
  let value = String(raw).trim().toLowerCase();
  const into = value.startsWith("-");
  const out = value.endsWith("-") && value.length > 1;
  value = value.replace(/^-/, "").replace(/-$/, "").trim();
  const words = value.split(/\s+/).filter(Boolean).map((w) => {
    const parts = w.split("*");
    return parts.length === 1 ? { exact: w } : { parts };
  });
  return { words, into, out };
}

function kc9kPhraseWordMatches(pattern, word) {
  return pattern.exact !== undefined ? pattern.exact === word : kc9kGlobWord(word, pattern.parts);
}

// kc9kPhraseMatcher compiles a phrase { sets: [{ terms: [...], necessary }],
// length } into { test(text) } for text (scope "url": the address,
// decoded, with its separators as blank space).
function kc9kPhraseMatcher(phrase, scope) {
  const sets = (phrase.sets || []).map((set, i) => ({
    terms: (set.terms || []).map(kc9kPhraseTerm).filter((t) => t.words.length > 0),
    necessary: i < 2 || Boolean(set.necessary),
  }));
  const necessary = sets.filter((s) => s.necessary).length;
  const length = Math.min(Math.max(phrase.length || necessary, necessary), sets.length);
  const prepare = (input) => {
    if (scope !== "url") {
      return input;
    }
    let text = String(input);
    try {
      text = decodeURIComponent(text.replace(/\+/g, " "));
    } catch (error) {
      // keep it as it is
    }
    return text.replace(/^[a-z]+:\/\//i, "").replace(/[-_/.?=&+#:,]/g, " ");
  };
  const test = (input) => {
    for (const words of kc9kPhraseTokens(prepare(input))) {
      // Every place a term occurs: [{ start, end, set, into, out }].
      const at = words.map(() => []);
      sets.forEach((set, s) => {
        for (const term of set.terms) {
          for (let i = 0; i + term.words.length <= words.length; i++) {
            let ok = true;
            for (let k = 0; k < term.words.length && ok; k++) {
              ok = kc9kPhraseWordMatches(term.words[k], words[i + k].word) &&
                (k === 0 || words[i + k].gap === "space");
            }
            if (ok) {
              at[i].push({ start: i, end: i + term.words.length - 1, set: s, into: term.into, out: term.out });
            }
          }
        }
      });
      // Chains of term places, each set used once.
      const extend = (last, used, count, covered) => {
        if (covered === necessary && count >= length && !last.out) {
          return true;
        }
        if (count >= sets.length) {
          return false;
        }
        const next = [];
        if (last.out) {
          // Over a bridge: up to 20 words, commas allowed, to a "-term".
          for (let j = last.end + 1; j < words.length && j <= last.end + 1 + KC9K_BRIDGE_WORDS; j++) {
            if (j > last.end + 1 && words[j - 1].gap === "punct") {
              break;
            }
            if (words[j].gap === "punct") {
              break;
            }
            next.push(...at[j].filter((p) => p.into));
          }
        } else if (last.end + 1 < words.length && words[last.end + 1].gap === "space") {
          next.push(...at[last.end + 1].filter((p) => !p.into));
        }
        for (const p of next) {
          if (used & (1 << p.set)) {
            continue;
          }
          if (extend(p, used | (1 << p.set), count + 1, covered + (sets[p.set].necessary ? 1 : 0))) {
            return true;
          }
        }
        return false;
      };
      for (const places of at) {
        for (const p of places) {
          if (!p.into && extend(p, 1 << p.set, 1, sets[p.set].necessary ? 1 : 0)) {
            return true;
          }
        }
      }
    }
    return false;
  };
  return { test };
}

// kc9kPhraseCheck says why a phrase can't be used, or "" (R-PHRASE-1, 8).
function kc9kPhraseCheck(phrase) {
  const sets = phrase.sets || [];
  if (sets.length < 2) {
    return "a phrase needs at least two word sets";
  }
  // Which word sets hold a bridge's start (big-) and end (-gorilla).
  const terms = sets.map((set) => (set.terms || []).map(kc9kPhraseTerm));
  const outs = terms.map((ts) => ts.some((t) => t.out));
  const ins = terms.map((ts) => ts.some((t) => t.into));
  const elsewhere = (has, ...skip) => has.some((h, i) => h && !skip.includes(i));
  for (let i = 0; i < sets.length; i++) {
    for (const term of terms[i]) {
      if (term.into && term.out) {
        if (sets.length < 3) {
          return "a middle link (-term-) needs at least three word sets";
        }
        if (!outs.some((o, a) => o && a !== i && elsewhere(ins, i, a))) {
          return "a middle link needs a term ending in - and one starting with -, each in another set";
        }
      } else if ((term.into && !elsewhere(outs, i)) || (term.out && !elsewhere(ins, i))) {
        return "a bridge needs both ends: a term ending in - and one starting with - in another set";
      }
    }
  }
  return "";
}
