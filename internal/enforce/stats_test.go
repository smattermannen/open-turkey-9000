package enforce

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
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

// §4.9 statistics per user: site time goes to the user whose browser
// reported it, besides the totals.
func TestUsagePerUser(t *testing.T) {
	d, err := db.OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	who := "anna"
	old := clientUser
	clientUser = func(string) string { return who }
	defer func() { clientUser = old }()
	h := usageHandler(d)
	postUsage(h, `{"sites":{"example.com":30}}`, true)
	who = "ben"
	postUsage(h, `{"sites":{"example.com":10,"other.example":5}}`, true)
	who = ""
	postUsage(h, `{"sites":{"example.com":1}}`, true)
	today := Day(time.Now())
	if rows, _ := d.UserUsageSince(today, "anna"); len(rows) != 1 || rows[0].Seconds != 30 {
		t.Errorf("anna: %v", rows)
	}
	if rows, _ := d.UserUsageSince(today, "ben"); len(rows) != 2 {
		t.Errorf("ben: %v", rows)
	}
	if users, _ := d.UsageUsers(); len(users) != 2 || users[0] != "anna" || users[1] != "ben" {
		t.Errorf("users: %v", users)
	}
	if rows, _ := d.UsageSince(today); len(rows) != 2 || rows[0].Seconds != 41 {
		t.Errorf("totals: %v", rows)
	}
	if err := d.ClearUsage(); err != nil {
		t.Fatal(err)
	}
	if users, _ := d.UsageUsers(); len(users) != 0 {
		t.Error("clear kept per-user rows")
	}
}

// ClientUser reads the uid of a real connection's socket.
func TestClientUserLive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Skip(err)
	}
	defer conn.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	me, _ := user.Current()
	got := blocker.ClientUser(conn.LocalAddr().String(), port)
	want := me.Username[strings.LastIndex(me.Username, `\`)+1:] // Windows: DOMAIN\name
	if me.Uid != "0" && got != want {
		t.Errorf("ClientUser = %q; want %q", got, me.Username)
	}
}
