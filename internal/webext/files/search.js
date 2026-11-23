// Search results (R-PAGE-2): a result leading to an address the lists take
// away is taken off the search results page ("reddit.com" blacklisted: no
// Reddit results). Loaded before content.js, which runs it.
//
// A results page is known by its address (KC9K_SEARCH_ENGINES). Each link
// in its results area is followed through the engine's own redirect
// (kc9kResultTarget); one leading where a visit would be taken away hides
// its result: the largest part of the page around the link that holds no
// result kept and no search box (kc9kResultOf). The engines' markup changes
// often; this needs none of it but where the results are.

// own: the engine's own sites (its links are menus, more searches, pages
// of results), never results. results: where the results are, else the
// whole page.
const KC9K_SEARCH_ENGINES = [
  { name: "google", host: /^([a-z0-9-]+\.)*google(\.[a-z]{2,3}){1,2}$/, path: /^\/search$/,
    own: /(^|\.)(google(\.[a-z]{2,3}){1,2}|gstatic\.com)$/, results: "#rso, #tads, #tadsb, #bottomads, #rhs, #botstuff" },
  { name: "bing", host: /^([a-z0-9-]+\.)?bing\.com$/, path: /^\/((news|videos)\/)?search$/,
    own: /(^|\.)bing\.com$/, results: "#b_results, #b_context" },
  { name: "duckduckgo", host: /^([a-z0-9-]+\.)?duckduckgo\.com$/, path: /^\/(html\/?)?$/, query: "q",
    own: /(^|\.)duckduckgo\.com$/, results: "[data-testid=\"mainline\"], .react-results--main, #links" },
  { name: "brave", host: /^search\.brave\.com$/, path: /^\/(search|news|videos)$/,
    own: /(^|\.)brave\.com$/, results: "#results" },
  { name: "yahoo", host: /^([a-z0-9-]+\.)*search\.yahoo\.com$/, path: /^\/(search|.*\/search)$/,
    own: /(^|\.)yahoo\.com$/, results: "#web, #right" },
  { name: "startpage", host: /^([a-z0-9-]+\.)?startpage\.com$/, path: /^\/(sp|do|rvd)?\/?search$/,
    own: /(^|\.)startpage\.com$/, results: ".w-gl, #main" },
  { name: "ecosia", host: /^(www\.)?ecosia\.org$/, path: /^\/search$/,
    own: /(^|\.)ecosia\.org$/, results: ".mainline, main" },
  { name: "yandex", host: /^((www\.)?yandex\.[a-z]{2,3}(\.[a-z]{2})?|ya\.ru)$/, path: /^\/search\/?$/,
    own: /(^|\.)(yandex\.[a-z]{2,3}(\.[a-z]{2})?|ya\.ru)$/, results: "#search-result, .content__left" },
  { name: "mojeek", host: /^(www\.)?mojeek\.com$/, path: /^\/search$/,
    own: /(^|\.)mojeek\.com$/, results: ".results-standard, .results" },
  // SearXNG instances, known the way SafeSearch knows them.
  { name: "searxng", host: /^(search\.disroot\.org|searx(ng)?\..+)$/, path: /^\/search$/,
    own: null, results: "#urls, #results" },
];

// kc9kSearchEngine returns the engine whose results page rawUrl is, or null.
function kc9kSearchEngine(rawUrl) {
  let u;
  try {
    u = new URL(rawUrl);
  } catch (error) {
    return null;
  }
  if (!/^https?:$/.test(u.protocol)) {
    return null;
  }
  const host = u.hostname.toLowerCase();
  return KC9K_SEARCH_ENGINES.find((e) => e.host.test(host) && e.path.test(u.pathname) &&
    (!e.query || u.searchParams.has(e.query))) || null;
}

function kc9kWebAddress(text) {
  return typeof text === "string" && /^https?:\/\/[^/\s]+/i.test(text) ? text : "";
}

// kc9kResultTarget returns where a link on a results page leads: through
// the engines' redirects (Google's /url?q= and /amp/s/, Bing's
// /ck/a?u=a1<base64>, DuckDuckGo's /l/?uddg= and its ads' ad_domain,
// Yahoo's /RU=…/RK=), else an address carried inside it (kc9kUnwrap:
// ?u=<base64>, ?adurl=, web archives…), else the link itself. "" for what
// isn't a web address.
function kc9kResultTarget(href, base) {
  let u;
  try {
    u = new URL(href, base);
  } catch (error) {
    return "";
  }
  if (!/^https?:$/.test(u.protocol)) {
    return "";
  }
  const host = u.hostname.toLowerCase();
  let target = "";
  if (/(^|\.)google(\.[a-z]{2,3}){1,2}$/.test(host)) {
    if (u.pathname === "/url") {
      target = kc9kWebAddress(u.searchParams.get("q")) || kc9kWebAddress(u.searchParams.get("url"));
    } else if (u.pathname.startsWith("/amp/s/")) {
      target = "https://" + u.pathname.slice("/amp/s/".length) + u.search;
    }
  } else if (/(^|\.)bing\.com$/.test(host) && u.pathname === "/ck/a") {
    const value = u.searchParams.get("u") || "";
    if (value.startsWith("a1")) {
      try {
        target = kc9kWebAddress(atob(value.slice(2).replace(/-/g, "+").replace(/_/g, "/")));
      } catch (error) {
        target = "";
      }
    }
  } else if (/(^|\.)duckduckgo\.com$/.test(host) && u.pathname === "/l/") {
    target = kc9kWebAddress(u.searchParams.get("uddg"));
  } else if (/(^|\.)duckduckgo\.com$/.test(host) && u.pathname === "/y.js" &&
    /^[a-z0-9.-]+\.[a-z]{2,}$/i.test(u.searchParams.get("ad_domain") || "")) {
    target = "https://" + u.searchParams.get("ad_domain") + "/";
  } else if (/(^|\.)search\.yahoo\.com$/.test(host)) {
    const m = u.pathname.match(/\/RU=([^/]+)\//);
    if (m) {
      try {
        target = kc9kWebAddress(decodeURIComponent(m[1]));
      } catch (error) {
        target = "";
      }
    }
  }
  return target || kc9kUnwrap(u.href)[0] || u.href;
}

// What the page needs to stay usable: a result is never hidden together
// with one of these.
const KC9K_RESULT_GUARD = "input, textarea, select, form, header, nav, footer, [role=\"navigation\"], [role=\"search\"]";
const KC9K_REMOVED_ATTR = "data-kc9k-removed";
const KC9K_RESULTS_STYLE_ID = "kc9k-results-style";

// kc9kSearchResults keeps one results page's results judged. leadsAway(url)
// says whether a visit to url would be taken away. judge() hides the
// results leading away (again after the page changed), forget() shows
// everything again (the lists changed).
function kc9kSearchResults(doc, engine, pageUrl, leadsAway) {
  const verdicts = new Map(); // link target → "own", "away" or "kept"
  const removed = new Set();
  const pageHost = (() => {
    try {
      return new URL(pageUrl).hostname.toLowerCase();
    } catch (error) {
      return "";
    }
  })();

  function verdict(link) {
    const href = link.getAttribute("href");
    if (!href) {
      return "own";
    }
    const target = kc9kResultTarget(href, pageUrl);
    if (!target) {
      return "own";
    }
    let known = verdicts.get(target);
    if (known === undefined) {
      let host = "";
      try {
        host = new URL(target).hostname.toLowerCase();
      } catch (error) {
        host = "";
      }
      known = !host || host === pageHost || (engine.own && engine.own.test(host)) ? "own" : leadsAway(target) ? "away" : "kept";
      verdicts.set(target, known);
    }
    return known;
  }

  function keepsSome(el) {
    return [...el.querySelectorAll("a[href]")].some((a) => verdict(a) === "kept") ||
      (el.matches("a[href]") && verdict(el) === "kept");
  }

  // kc9kResultOf: from the link up, while the part around it holds no
  // result kept and no search box, and stays inside the results area.
  function resultOf(link, root) {
    let node = link;
    for (let parent = node.parentElement; parent && parent !== root; parent = node.parentElement) {
      if (parent.matches(KC9K_RESULT_GUARD) || parent.querySelector(KC9K_RESULT_GUARD) || keepsSome(parent)) {
        break;
      }
      node = parent;
    }
    return node;
  }

  function ensureStyle() {
    if (doc.getElementById(KC9K_RESULTS_STYLE_ID) || !doc.documentElement) {
      return;
    }
    const style = doc.createElement("style");
    style.id = KC9K_RESULTS_STYLE_ID;
    style.textContent = "[" + KC9K_REMOVED_ATTR + "] { display: none !important; }";
    doc.documentElement.appendChild(style);
  }

  function roots() {
    const found = [...doc.querySelectorAll(engine.results)];
    // A results area inside another is judged with it.
    const outer = found.filter((el) => !found.some((other) => other !== el && other.contains(el)));
    return outer.length > 0 ? outer : doc.body ? [doc.body] : [];
  }

  function show(el) {
    el.removeAttribute(KC9K_REMOVED_ATTR);
    removed.delete(el);
  }

  function judge() {
    // What was hidden before the rest of the page came in may hold a
    // result kept now: shown again, and its links judged afresh below.
    for (const el of [...removed]) {
      if (!el.isConnected || !el.hasAttribute(KC9K_REMOVED_ATTR)) {
        removed.delete(el);
      } else if (keepsSome(el)) {
        show(el);
      }
    }
    let hid = false;
    for (const root of roots()) {
      for (const link of root.querySelectorAll("a[href]")) {
        if (link.closest("[" + KC9K_REMOVED_ATTR + "]") || verdict(link) !== "away") {
          continue;
        }
        const result = resultOf(link, root);
        result.setAttribute(KC9K_REMOVED_ATTR, "");
        removed.add(result);
        hid = true;
      }
    }
    if (hid || removed.size > 0) {
      ensureStyle();
    }
  }

  function forget() {
    for (const el of [...removed]) {
      show(el);
    }
    verdicts.clear();
  }

  return { judge, forget };
}
