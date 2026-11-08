package enforce

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/smattermannen/kc9k/internal/db"
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
