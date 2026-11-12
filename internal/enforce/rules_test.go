package enforce

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/webext"
)

// Only the extension's own requests (with the header) count as check-ins;
// anything else still gets the rules but proves nothing.
func TestRulesHandlerCheckIns(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var checkIns []string
	handler := rulesHandler(d, func(addr string) { checkIns = append(checkIns, addr) })

	request := func(header bool, host string) int {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+"/rules", nil)
		r.Host = host
		r.RemoteAddr = "127.0.0.1:50000"
		if header {
			r.Header.Set(ExtensionHeader, "1")
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w.Code
	}

	if code := request(false, RulesAddress); code != http.StatusOK || len(checkIns) != 0 {
		t.Fatalf("plain request: code %d, %d check-ins; want 200 and none", code, len(checkIns))
	}
	if code := request(true, RulesAddress); code != http.StatusOK || len(checkIns) != 1 {
		t.Fatalf("extension request: code %d, %d check-ins; want 200 and one", code, len(checkIns))
	}
	if code := request(true, "evil.example:47613"); code != http.StatusForbidden || len(checkIns) != 1 {
		t.Fatalf("rebound host: code %d, %d check-ins; want 403 and still one", code, len(checkIns))
	}
}

// Chrome reads the update manifest and the .crx from the daemon, only at
// the daemon's own address.
func TestChromeExtensionHandlers(t *testing.T) {
	webext.ChromeKeyPath = filepath.Join(t.TempDir(), "key.pem")
	get := func(handler http.HandlerFunc, host, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
		r.Host = host
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	id, crx, err := chromeExtension()
	if err != nil {
		t.Fatal(err)
	}

	w := get(chromeUpdateHandler, RulesAddress, chromeUpdatePath+"?x=id%3D"+id)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `appid="`+id+`"`) ||
		!strings.Contains(w.Body.String(), "http://"+RulesAddress+chromeCRXPath()) {
		t.Fatalf("update manifest: %d %s", w.Code, w.Body.String())
	}
	if w := get(chromeExtensionHandler, RulesAddress, chromeCRXPath()); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), crx) {
		t.Fatalf(".crx: %d, %d bytes", w.Code, w.Body.Len())
	}
	if w := get(chromeExtensionHandler, RulesAddress, "/chrome/kc9k-0.1.crx"); w.Code != http.StatusNotFound {
		t.Errorf("another version: %d, want 404", w.Code)
	}
	if w := get(chromeUpdateHandler, "evil.example:47613", chromeUpdatePath); w.Code != http.StatusForbidden {
		t.Errorf("rebound host, update manifest: %d, want 403", w.Code)
	}
	if w := get(chromeExtensionHandler, "evil.example:47613", chromeCRXPath()); w.Code == http.StatusOK {
		t.Error("rebound host got the .crx")
	}

	rules, err := BuildRules(testDB(t))
	if err != nil {
		t.Fatal(err)
	}
	if rules.ExtensionVersion != webext.Version() {
		t.Errorf("rules carry extension version %q, want %q", rules.ExtensionVersion, webext.Version())
	}
}
