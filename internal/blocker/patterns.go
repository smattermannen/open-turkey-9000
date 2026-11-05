package blocker

import "strings"

// TranslatePattern turns one Cold Turkey style entry into Firefox and Chromium
// filters. Entries can be "reddit.com/r/funny", "*.xxx", "*.*", or a keyword
// like "*casino*".
//
// Firefox understands wildcards anywhere in a path. Chromium's policy format
// only prefix-matches paths and has no keyword matching, so an entry it can't
// express returns "" there rather than a filter that matches too much.
func TranslatePattern(p string) (firefox, chromium string) {
	if p == "*" || p == "*.*" || p == "*://*/*" {
		return "*://*/*", "*"
	}
	scheme, rest := "*", p
	if i := strings.Index(p, "://"); i > 0 {
		scheme, rest = p[:i], p[i+3:]
	}
	host, path := rest, ""
	if i := strings.Index(rest, "/"); i >= 0 {
		host, path = rest[:i], rest[i:]
	}

	bare := strings.TrimPrefix(host, "*.")
	if bare == "" || bare == "*" || strings.Contains(bare, "*") {
		term := strings.Trim(host+path, "*/")
		if bare == "*" && path != "" {
			term = strings.Trim(path, "*/")
		}
		return "*://*/*" + term + "*", ""
	}
	bare = stripWWWPrefix(bare)

	firefoxPath := path
	if firefoxPath == "" {
		firefoxPath = "/*"
	} else if !strings.HasSuffix(firefoxPath, "*") {
		firefoxPath += "*"
	}
	firefox = scheme + "://*." + bare + firefoxPath

	chromiumPath := strings.TrimSuffix(path, "*")
	if strings.Contains(chromiumPath, "*") {
		return firefox, ""
	}
	chromium = bare + chromiumPath
	if scheme != "*" {
		chromium = scheme + "://" + chromium
	}
	return firefox, chromium
}

// BrowserLimited reports whether an entry only works in Firefox.
func BrowserLimited(p string) bool {
	_, chromium := TranslatePattern(p)
	return chromium == ""
}

func translated(patterns []string) (firefox, chromium []string) {
	for _, p := range ValidPatterns(patterns) {
		ff, cr := TranslatePattern(p)
		firefox = append(firefox, ff)
		if cr != "" {
			chromium = append(chromium, cr)
		}
	}
	return firefox, chromium
}

// exceptionHost returns the host an exception entry applies to.
func exceptionHost(e string) string {
	if i := strings.Index(e, "://"); i >= 0 {
		e = e[i+3:]
	}
	if i := strings.Index(e, "/"); i >= 0 {
		e = e[:i]
	}
	return stripWWWPrefix(strings.TrimPrefix(e, "*."))
}

// HasExceptionUnder reports whether any exception falls under domain. Such a
// domain can't stay in /etc/hosts or the firewall, which block a whole host:
// the browser policy enforces it instead, so the exception can take effect.
func HasExceptionUnder(domain string, exceptions []string) bool {
	domain = stripWWWPrefix(domain)
	for _, e := range exceptions {
		host := exceptionHost(e)
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
