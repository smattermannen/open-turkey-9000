"use strict";

// Shared by the background script, the content script and the block page:
// turns list entries into matchers and tests URLs against them.

function kc9kEscapeRegex(text) {
  return text.replace(/[.+?^${}()|[\]\\]/g, "\\$&");
}

// Entries work as in Cold Turkey: reddit.com, reddit.com/r/funny, *.xxx, *.*,
// and keywords such as *casino*.
function kc9kCompile(entry) {
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

// A compiled filter: an entry in match that no entry in except matches.
function kc9kFilter(raw) {
  return {
    match: ((raw && raw.match) || []).map(kc9kCompile),
    except: ((raw && raw.except) || []).map(kc9kCompile),
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
  return filter.match.some((rule) => rule.test(text)) && !filter.except.some((rule) => rule.test(text));
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
  return (keywords || []).filter(kc9kUsableKeyword).map(kc9kKeywordMatcher);
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
