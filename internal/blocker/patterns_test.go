package blocker

import (
	"strings"
	"testing"
)

func TestTranslatePattern(t *testing.T) {
	cases := []struct{ in, firefox, chromium string }{
		{"*.*", "*://*/*", "*"},
		{"reddit.com/r/funny", "*://*.reddit.com/r/funny*", "reddit.com/r/funny"},
		{"*.xxx", "*://*.xxx/*", "xxx"},
		{"https://example.com/a*", "https://*.example.com/a*", "https://example.com/a"},
		{"google.com/*q=*unicorn*", "*://*.google.com/*q=*unicorn*", ""},
		{"*casino*", "*://*/*casino*", ""},
	}
	for _, c := range cases {
		firefox, chromium := TranslatePattern(c.in)
		if firefox != c.firefox || chromium != c.chromium {
			t.Errorf("%s: got %q, %q; want %q, %q", c.in, firefox, chromium, c.firefox, c.chromium)
		}
	}
}

func TestHasExceptionUnder(t *testing.T) {
	cases := []struct {
		domain string
		list   []string
		want   bool
	}{
		{"reddit.com", []string{"reddit.com/r/programming"}, true},
		{"reddit.com", []string{"https://old.reddit.com/r/x"}, true},
		{"www.reddit.com", []string{"*.reddit.com"}, true},
		{"reddit.com", []string{"notreddit.com"}, false},
		{"reddit.com", nil, false},
	}
	for _, c := range cases {
		if got := HasExceptionUnder(c.domain, c.list); got != c.want {
			t.Errorf("%s %v: got %v", c.domain, c.list, got)
		}
	}
}

func TestExceptionsReachBothBrowsers(t *testing.T) {
	p := BrowserPolicy{BlockDomains: []string{"reddit.com"}, Exceptions: []string{"reddit.com/r/programming"}}
	assertContains(t, firefoxExceptionSet(p), "*://*.reddit.com/r/programming*")
	assertContains(t, chromiumAllowSet(p), "reddit.com/r/programming")
}

// R-LIST-3: the default exception file://* must not reach the browser
// policy, where it would lose its scheme and except every address.
func TestFileExceptionStaysOutOfPolicy(t *testing.T) {
	ff, cr := translated([]string{"file://*", "file:///home/*"})
	if len(ff)+len(cr) != 0 {
		t.Errorf("file entries in the policy: %v %v", ff, cr)
	}
	if ff, _ := translated([]string{"https://*.example.com/x"}); len(ff) != 1 {
		t.Errorf("a web pattern was dropped: %v", ff)
	}
}

// §3.1: local file entries go into the policies as given; a block's file
// exceptions spare what they cover.
func TestFilePatterns(t *testing.T) {
	got := FilePatterns([]string{"file://*", "file:///home/me/games/*", "file:///a/*/b", "https://x.com", "file://rel", "file:///a b"})
	if strings.Join(got, " ") != "file:///* file:///home/me/games/*" {
		t.Errorf("FilePatterns = %q", got)
	}
	if !FileCovered("file:///home/me/games/x.html", []string{"file://*"}) ||
		!FileCovered("file:///home/me/games/x.html", []string{"file:///home/me/*"}) ||
		FileCovered("file:///home/me/games/x.html", []string{"file:///tmp/*"}) {
		t.Error("FileCovered")
	}
	p := BrowserPolicy{BlockFiles: []string{"file:///home/me/games/*"}}
	// Chrome reads a path as a prefix and "*" in it literally (seen live
	// 2026-11-21): the folder without the star.
	if p.Empty() || !contains(firefoxBlockSet(p), "file:///home/me/games/*") || !contains(chromiumBlockSet(p), "file:///home/me/games/") ||
		contains(chromiumBlockSet(p), "file:///home/me/games/*") {
		t.Errorf("policy: %v / %v", firefoxBlockSet(p), chromiumBlockSet(p))
	}
	every := BrowserPolicy{BlockFiles: []string{"file://*"}}
	if !contains(chromiumBlockSet(every), "file:///") || !contains(firefoxBlockSet(every), "file:///*") {
		t.Errorf("every file: %v / %v", firefoxBlockSet(every), chromiumBlockSet(every))
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
