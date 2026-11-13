//go:build roottest

// Run as root on a disposable machine with Playwright's Chromium (Chrome for
// Testing, which reads its policies from /etc/opt/chrome_for_testing):
//
//	go test -tags roottest -run Chrome ./internal/enforce
//
// Not at the same time as the blocker package's root tests (use -p 1): their
// extension guard closes this test's Chrome, which has a debugging port.
package enforce

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/webext"
)

// Chrome installs the extension from the daemon through policy, checks in
// with it, and the extension takes a blocked page away and reports it.
func TestChromeRunsTheExtension(t *testing.T) {
	matches, _ := filepath.Glob("/opt/pw-browsers/chromium-*/chrome-linux*/chrome")
	if len(matches) == 0 {
		t.Skip("no Chromium to test with")
	}
	policy := "/etc/opt/chrome_for_testing/policies/managed/kc9k-setup.json"
	if _, err := os.Stat(policy); err == nil {
		t.Fatalf("%s exists: not a disposable machine?", policy)
	}
	saved := blocker.ChromeSetupPaths
	blocker.ChromeSetupPaths = []string{policy}
	t.Cleanup(func() { blocker.ChromeSetupPaths = saved; os.Remove(policy) })
	webext.ChromeKeyPath = filepath.Join(t.TempDir(), "key.pem")

	d := testDB(t)
	if err := d.CreateBlock("news"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, d, "news", "Blocked", "example.com")
	words, err := d.CreateList("news", db.ListBlacklist, "Words", db.Where{Text: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddListEntries(words, []string{"casino"}); err != nil {
		t.Fatal(err)
	}
	// A local site whose text is blocked (the computer may have no network).
	site, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(site, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<title>Games</title><p>The best casino online.</p>")
	}))
	t.Cleanup(func() { site.Close() })
	if err := d.ActivateBlock("news", false, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	go func() { t.Error(ServeRules(d)) }()

	id, _, err := chromeExtension()
	if err != nil {
		t.Fatal(err)
	}
	if err := blocker.EnsureChromeSetup(blocker.ChromeSetup{ExtensionID: id, UpdateURL: ChromeUpdateURL}); err != nil {
		t.Fatal(err)
	}

	// An ordinary user's Chrome, with a debugging port only so the test can
	// open a tab and see where it ends up.
	// (Not t.TempDir: its parent is root's alone.)
	home, err := os.MkdirTemp("", "kc9k-chrome-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	if err := os.Chown(home, 1500, 1500); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(matches[0], "--headless", "--no-sandbox", "--disable-gpu", "--remote-debugging-port=9334",
		"--user-data-dir="+home+"/profile", "about:blank")
	cmd.Env = []string{"HOME=" + home}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1500, Gid: 1500}}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	waitFor(t, 90*time.Second, "the extension to check in", func() bool {
		_, ok := LastExtensionCheckIn(d)
		return ok
	})

	// A blocked address: the request is redirected (Chrome may first
	// upgrade it to https).
	blockPage := "chrome-extension://" + id + "/blocked.html?url="
	openTab(t, "http://example.com/page")
	waitFor(t, 15*time.Second, "the block page", func() bool {
		return hasTab(t, func(tab string) bool {
			return strings.HasPrefix(tab, blockPage) && strings.HasSuffix(tab, url.QueryEscape("://example.com/page"))
		})
	})
	waitFor(t, 10*time.Second, "the hit to be logged", func() bool {
		events, err := d.RecentEvents(0, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range events {
			if e.Block == "news" && strings.Contains(e.Detail, "example.com") {
				return true
			}
		}
		return false
	})

	// Blocked text: the content script reads the page and the background
	// takes it away.
	page := "http://" + site.Addr().String() + "/words"
	openTab(t, page)
	waitFor(t, 15*time.Second, "the page with blocked text to go", func() bool {
		return hasTab(t, func(tab string) bool { return tab == blockPage+url.QueryEscape(page)+"&reason=keyword" })
	})

	// Chrome stops a service worker after 30 seconds without work: the
	// extension must still be there, with its rules, later on.
	time.Sleep(45 * time.Second)
	openTab(t, "http://www.example.com/later")
	waitFor(t, 15*time.Second, "the block page after a while", func() bool {
		return hasTab(t, func(tab string) bool {
			return strings.HasPrefix(tab, blockPage) && strings.HasSuffix(tab, url.QueryEscape("://www.example.com/later"))
		})
	})

	// The block page asks the extension whether its site is still blocked
	// (a message answered through sendResponse), and goes back when it isn't.
	// A page taken away for its text stays away.
	if err := d.DeactivateBlock("news"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 20*time.Second, "the block pages to go back", func() bool {
		return !hasTab(t, func(tab string) bool {
			return strings.HasPrefix(tab, blockPage) && !strings.HasSuffix(tab, "&reason=keyword")
		})
	})
	if !hasTab(t, func(tab string) bool { return strings.HasSuffix(tab, "&reason=keyword") }) {
		t.Error("the page taken away for its text went back")
	}
}

func openTab(t *testing.T, address string) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPut, "http://127.0.0.1:9334/json/new?"+url.QueryEscape(address), nil)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
}

func hasTab(t *testing.T, match func(string) bool) bool {
	for _, tab := range chromeTabs(t) {
		if match(tab) {
			return true
		}
	}
	return false
}

func chromeTabs(t *testing.T) []string {
	response, err := http.Get("http://127.0.0.1:9334/json/list")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	var tabs []struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(data, &tabs)
	var out []string
	for _, tab := range tabs {
		out = append(out, tab.URL)
	}
	return out
}

func waitFor(t *testing.T, limit time.Duration, what string, done func() bool) {
	t.Helper()
	for end := time.Now().Add(limit); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
		if done() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}
