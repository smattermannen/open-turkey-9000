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

// WebPattern reports whether an entry is for web addresses: without a
// scheme, or with http, https, ftp or *. Others (file://) are left to the
// extension: written as a policy, "file://*" would lose its scheme and match
// every address.
func WebPattern(p string) bool {
	i := strings.Index(p, "://")
	if i <= 0 {
		return true
	}
	switch strings.ToLower(p[:i]) {
	case "http", "https", "ftp", "*":
		return true
	}
	return false
}

func translated(patterns []string) (firefox, chromium []string) {
	for _, p := range ValidPatterns(patterns) {
		if !WebPattern(p) {
			continue
		}
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

// SplitURLEntries sorts a list's entries for address matching: plain
// domains (normalized) and patterns (anything with a "/" or "*"). Entries
// that are neither, such as a lone word meant for page text, are left out.
func SplitURLEntries(entries []string) (domains, patterns []string) {
	for _, e := range entries {
		if strings.ContainsAny(e, "/*") {
			if IsValidPattern(e) {
				patterns = append(patterns, e)
			}
			continue
		}
		if host := NormalizeDomain(e); IsValidDomain(host) {
			domains = append(domains, host)
		}
	}
	return domains, patterns
}

// IsURLEntry reports whether an entry can match addresses.
func IsURLEntry(e string) bool {
	domains, patterns := SplitURLEntries([]string{e})
	return len(domains)+len(patterns) > 0
}

// IsAddressEntry reports whether a list entry is a site or URL entry rather
// than a rendered term (§3.2): a domain (reddit.com), a domain with a path
// (reddit.com/r/funny), or a pattern with * or / (*.xxx, *casino*, *.* for
// every site). A pattern needs more than wildcards, dots and slashes, except
// * and *.*. Anything else (porn, long cat, 18+) is a rendered term. With
// the url scope a site or URL entry matches the whole address (and feeds the
// layers) and a rendered term matches words in it (R-PHRASE-6).
func IsAddressEntry(e string) bool {
	if e != "*" && e != "*.*" && strings.Trim(e, "*./") == "" {
		return false
	}
	return IsURLEntry(e)
}

// FilePatterns keeps the local file entries that can go in a browser policy
// as they are: "file://" plus an absolute path of plain characters, with *
// only at the end ("file:///home/me/games/*"); "file://*" means every local
// file ("file:///*"). Firefox and Chrome both read that form.
func FilePatterns(entries []string) []string {
	var out []string
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if !strings.HasPrefix(strings.ToLower(e), "file://") {
			continue
		}
		path := e[len("file://"):]
		if path == "*" {
			path = "/*"
		}
		if !strings.HasPrefix(path, "/") || strings.Contains(strings.TrimSuffix(path, "*"), "*") {
			continue
		}
		ok := true
		for _, r := range path {
			if r < 0x21 || r == 0x7f || strings.ContainsRune(`"'\<>`+"`", r) {
				ok = false
			}
		}
		if ok {
			out = append(out, "file://"+path)
		}
	}
	return out
}

// FileCovered reports whether a local file entry is spared by one of a
// block's file exceptions: "file://*" spares every file, "file:///a/*"
// everything under /a, otherwise the same entry.
func FileCovered(entry string, exceptions []string) bool {
	want := FilePatterns([]string{entry})
	if len(want) == 0 {
		return true
	}
	for _, x := range FilePatterns(exceptions) {
		if x == want[0] || (strings.HasSuffix(x, "*") && strings.HasPrefix(strings.TrimSuffix(want[0], "*"), strings.TrimSuffix(x, "*"))) {
			return true
		}
	}
	return false
}
