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

// kc9kCountKeywords returns how many of the keywords occur in text; a keyword
// found many times counts once.
function kc9kCountKeywords(text, keywords) {
  let found = 0;
  for (const keyword of keywords) {
    if (String(keyword).replace(/\*/g, "").trim() && kc9kKeywordRegex(keyword).test(text)) {
      found++;
    }
  }
  return found;
}

// kc9kKeywordActions works out which actions the page keyword rules take on
// a page at url with this text: { block, greyscale, noImages }. Each rule is
// one block's keywords with, per action, how many different ones it takes
// (0: the block doesn't take that action).
function kc9kKeywordActions(rules, url, text) {
  const out = { block: false, greyscale: false, noImages: false };
  if (!url || !/^https?:/i.test(url)) {
    return out;
  }
  for (const rule of rules || []) {
    const keywords = rule.keywords || [];
    if (keywords.length === 0) {
      continue;
    }
    const except = kc9kFilter({ match: rule.except || [] });
    if (except.match.length > 0 && kc9kMatches(except, url)) {
      continue;
    }
    const found = kc9kCountKeywords(text, keywords);
    if (found === 0) {
      continue;
    }
    out.block = out.block || (rule.block > 0 && found >= rule.block);
    out.greyscale = out.greyscale || (rule.greyscale > 0 && found >= rule.greyscale);
    out.noImages = out.noImages || (rule.no_images > 0 && found >= rule.no_images);
  }
  return out;
}
