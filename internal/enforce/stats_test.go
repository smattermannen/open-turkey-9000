package enforce

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

func postUsage(h http.Handler, body string, header bool) int {
	req := httptest.NewRequest(http.MethodPost, "http://"+RulesAddress+"/usage", strings.NewReader(body))
	if header {
		req.Header.Set("X-KC9K-Usage", "1")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestUsageHandler(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	h := usageHandler(d)

	if code := postUsage(h, `{"sites":{"example.com":5}}`, false); code != http.StatusForbidden {
		t.Errorf("a report without the header got %d", code)
	}
	body := `{"sites":{"Example.COM":30,"bad name;rm":5,"capped.example":9999,"zero.example":0}}`
	if code := postUsage(h, body, true); code != http.StatusNoContent {
		t.Fatalf("report got %d", code)
	}
	rows, err := d.UsageSince(Day(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, r := range rows {
		got[r.Name] = r.Seconds
	}
	if len(got) != 2 || got["example.com"] != 30 || got["capped.example"] != maxReportSeconds {
		t.Errorf("recorded %v", got)
	}

	if err := d.SetSetting(SettingStats, "off"); err != nil {
		t.Fatal(err)
	}
	postUsage(h, `{"sites":{"example.com":30}}`, true)
	if rows, _ := d.UsageSince(Day(time.Now())); len(rows) != 2 || rows[0].Seconds+rows[1].Seconds != 30+maxReportSeconds {
		t.Errorf("recorded with statistics off: %v", rows)
	}
}
