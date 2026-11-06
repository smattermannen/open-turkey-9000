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

function kc9kMatches(filter, url) {
  if (!url || !/^(https?|ftp):/i.test(url)) {
    return false;
  }
  const text = kc9kTarget(url);
  return filter.match.some((rule) => rule.test(text)) && !filter.except.some((rule) => rule.test(text));
}
