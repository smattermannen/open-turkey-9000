package enforce

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/webext"
)

// The rules server has no secret (SPEC §6 2026-11-21 (By Opus)): what keeps
// web pages and rebound host names out is that every route answers only
// requests addressed to RulesAddress, takes posts only with the
// extension's own header, never answers a CORS preflight and never sends a
// CORS header. Held for every route, so a new one can't forget it.
func TestRulesServerGuardsEveryRoute(t *testing.T) {
	webext.ChromeKeyPath = filepath.Join(t.TempDir(), "key.pem")
	d := testDB(t)
	checkIns := 0
	routes := rulesRoutes(d, func(string) { checkIns++ })
	if len(routes) == 0 {
		t.Fatal("no routes")
	}
	send := func(route rulesRoute, method, host string, headers bool) *httptest.ResponseRecorder {
		path := route.path
		if strings.HasSuffix(path, "/") {
			path += "x.png"
		}
		if route.path == "/chrome/" {
			path = chromeCRXPath()
		}
		body := &watchedBody{Reader: strings.NewReader("{}")}
		r := httptest.NewRequest(method, "http://"+host+path, body)
		r.Host = host
		r.RemoteAddr = "127.0.0.1:50000"
		r.Header.Set("Origin", "https://evil.example")
		if headers {
			r.Header.Set(ExtensionHeader, "1")
			r.Header.Set("X-KC9K-Usage", "1")
		}
		w := httptest.NewRecorder()
		route.handler(w, r)
		if body.read {
			t.Errorf("%s %s (host %s, header %v): the request was read before it was refused", method, route.path, host, headers)
		}
		for name := range w.Header() {
			if strings.HasPrefix(strings.ToLower(name), "access-control-") {
				t.Errorf("%s %s (host %s): answered with CORS header %s", method, route.path, host, name)
			}
		}
		return w
	}
	refused := func(w *httptest.ResponseRecorder) bool { return w.Code >= 400 && w.Code < 500 }
	for _, route := range routes {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if w := send(route, method, "evil.example:47613", true); !refused(w) {
				t.Errorf("%s %s with a rebound host name: %d, want refused", method, route.path, w.Code)
			}
		}
		if w := send(route, http.MethodPost, RulesAddress, false); !refused(w) {
			t.Errorf("POST %s without the extension's header: %d, want refused", route.path, w.Code)
		}
		if w := send(route, http.MethodOptions, RulesAddress, false); !refused(w) {
			t.Errorf("CORS preflight to %s: %d, want refused", route.path, w.Code)
		}
	}
	if checkIns != 0 {
		t.Errorf("%d check-ins from refused requests, want none", checkIns)
	}
}

// watchedBody notes whether a handler read the request: a refused request
// must be turned away before anything in it is looked at.
type watchedBody struct {
	io.Reader
	read bool
}

func (b *watchedBody) Read(p []byte) (int, error) {
	b.read = true
	return b.Reader.Read(p)
}
