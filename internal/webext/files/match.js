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
  return out.filter(Boolean);
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
  return kc9kMatchesOne(filter, url) || kc9kUnwrap(url).some((inner) => kc9kMatchesOne(filter, inner));
}

// Page keywords. A plain keyword counts only as a whole word: "porn" misses
// "pornhub" and "18+" misses "1118+". A * stands for any letters or digits,
// so "*porn*" hits "pornhub". Spaces match any run of whitespace. Case is
// ignored.
const KC9K_WORD = "[\\p{L}\\p{N}]";

function kc9kKeywordRegex(keyword) {
  const value = String(keyword).trim().toLowerCase();
  const body = value
    .split("*")
    .map((part) => kc9kEscapeRegex(part).replace(/\s+/g, "\\s+"))
    .join(KC9K_WORD + "*");
  const before = /^[\p{L}\p{N}]/u.test(value) ? "(?<!" + KC9K_WORD + ")" : "";
  const after = /[\p{L}\p{N}]$/u.test(value) ? "(?!" + KC9K_WORD + ")" : "";
  return new RegExp(before + body + after, "iu");
}

// kc9kUsableKeyword drops entries that are only wildcards or blank.
function kc9kUsableKeyword(keyword) {
  return String(keyword).replace(/\*/g, "").trim() !== "";
}

function kc9kKeywordRegexes(keywords) {
  return (keywords || []).filter(kc9kUsableKeyword).map(kc9kKeywordRegex);
}

// kc9kCountKeywords returns how many of the keywords occur in text; a keyword
// found many times counts once.
function kc9kCountKeywords(text, keywords) {
  return kc9kKeywordRegexes(keywords).filter((regex) => regex.test(text)).length;
}

// kc9kCompileLists compiles the daemon's blacklists (see the daemon's
// ListRule): what each looks for in a page's address, title and text, its
// actions, and its block's exceptions.
function kc9kCompileLists(lists) {
  return (lists || []).map((list) => {
    const except = list.except || {};
    return {
      url: kc9kFilter({ match: list.url || [] }),
      title: kc9kKeywordRegexes(list.title),
      text: kc9kKeywordRegexes(list.text),
      actions: list.actions || [],
      except: {
        url: kc9kFilter({ match: except.url || [] }),
        title: kc9kKeywordRegexes(except.title),
        text: kc9kKeywordRegexes(except.text),
      },
    };
  });
}

// kc9kReadsPage reports whether any list needs the page's title or text.
function kc9kReadsPage(compiled) {
  return compiled.some((list) => list.title.length > 0 || list.text.length > 0 ||
    list.except.title.length > 0 || list.except.text.length > 0);
}

// kc9kListActions works out which actions the lists take on a page: an
// object with a true value for each action name. page holds its url, and its
// title and text when they are known (undefined otherwise). A list whose
// exceptions look at a part of the page that isn't known yet is left until
// it is, so a page an exception spares isn't blocked before it is read.
// A match in the address or the title takes every action of the list; in
// the text, an action needs its number of different entries.
function kc9kListActions(compiled, page) {
  const out = {};
  if (!page || !page.url || !/^(https?|ftp):/i.test(page.url)) {
    return out;
  }
  const hasTitle = typeof page.title === "string";
  const hasText = typeof page.text === "string";
  for (const list of compiled) {
    const except = list.except;
    if ((except.title.length > 0 && !hasTitle) || (except.text.length > 0 && !hasText)) {
      continue;
    }
    if ((except.url.match.length > 0 && kc9kMatches(except.url, page.url)) ||
      (hasTitle && except.title.some((regex) => regex.test(page.title))) ||
      (hasText && except.text.some((regex) => regex.test(page.text)))) {
      continue;
    }
    const whole = (list.url.match.length > 0 && kc9kMatches(list.url, page.url)) ||
      (hasTitle && list.title.some((regex) => regex.test(page.title)));
    const found = hasText && list.text.length > 0 ? list.text.filter((regex) => regex.test(page.text)).length : 0;
    for (const action of list.actions) {
      if (whole || (found > 0 && found >= (action.distinct || 1))) {
        out[action.action] = true;
      }
    }
  }
  return out;
}
