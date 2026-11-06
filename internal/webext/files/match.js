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
