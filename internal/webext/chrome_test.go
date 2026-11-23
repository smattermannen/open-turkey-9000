package webext

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChromeKeyIsMadeOnceAndKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")
	first, err := ChromeKey(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 { // Windows: the data folder's DACL
		t.Errorf("key file mode %v, want 0600", info.Mode().Perm())
	}
	again, err := ChromeKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Equal(again) {
		t.Error("the key changed when read again")
	}
	id, err := ChromeID(&first.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 32 || strings.Trim(id, "abcdefghijklmnop") != "" {
		t.Errorf("ID %q: want 32 letters from a to p", id)
	}
}

// chromeZip reads the zip inside a .crx, checking the header's layout.
func chromeZip(t *testing.T, crx []byte) *zip.Reader {
	t.Helper()
	if !bytes.HasPrefix(crx, []byte("Cr24")) || binary.LittleEndian.Uint32(crx[4:8]) != 3 {
		t.Fatalf("not a CRX3 file: % x", crx[:8])
	}
	start := 12 + int(binary.LittleEndian.Uint32(crx[8:12]))
	archive, err := zip.NewReader(bytes.NewReader(crx[start:]), int64(len(crx)-start))
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

func readZipped(t *testing.T, archive *zip.Reader, name string) []byte {
	t.Helper()
	file, err := archive.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPackChrome(t *testing.T) {
	key, err := ChromeKey(filepath.Join(t.TempDir(), "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	crx, err := PackChrome(key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := PackChrome(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(crx, again) {
		t.Error("packing twice gave different files")
	}

	archive := chromeZip(t, crx)
	var manifest struct {
		ManifestVersion int    `json:"manifest_version"`
		Version         string `json:"version"`
		Background      struct {
			ServiceWorker string `json:"service_worker"`
		} `json:"background"`
		Permissions    []string          `json:"permissions"`
		Icons          map[string]string `json:"icons"`
		ContentScripts []struct {
			JS []string `json:"js"`
		} `json:"content_scripts"`
	}
	if err := json.Unmarshal(readZipped(t, archive, "manifest.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ManifestVersion != 3 || manifest.Version != Version() {
		t.Errorf("manifest %d, version %s: want 3, %s", manifest.ManifestVersion, manifest.Version, Version())
	}
	if !strings.Contains(strings.Join(manifest.Permissions, " "), "webRequestBlocking") {
		t.Error("no webRequestBlocking: Chrome would not let the extension block pages")
	}
	worker := string(readZipped(t, archive, manifest.Background.ServiceWorker))
	if !strings.Contains(worker, `importScripts("compat.js", "match.js", "social.js", "background.js")`) {
		t.Errorf("the service worker doesn't load Firefox's background scripts: %s", worker)
	}
	// Everything the manifest names is in the package.
	names := []string{}
	for _, icon := range manifest.Icons {
		names = append(names, icon)
	}
	for _, scripts := range manifest.ContentScripts {
		names = append(names, scripts.JS...)
	}
	for _, name := range names {
		readZipped(t, archive, name)
	}
	if len(manifest.ContentScripts) == 0 || manifest.ContentScripts[0].JS[0] != "compat.js" {
		t.Error("content scripts don't start with compat.js")
	}
}

// 2.17: what the Chrome Web Store takes (make chrome-store-zip): Manifest V3
// at the root of a plain zip, no key or update_url (the store gives the ID
// and serves the updates), the 128 px icon, a description of at most 132
// characters.
func TestChromeStoreZip(t *testing.T) {
	data, err := PackChromeStoreZip()
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(readZipped(t, archive, "manifest.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"key", "update_url", "browser_specific_settings"} {
		if _, ok := manifest[field]; ok {
			t.Errorf("the store package has %q", field)
		}
	}
	if manifest["manifest_version"] != float64(3) {
		t.Errorf("manifest_version %v, want 3", manifest["manifest_version"])
	}
	if d, _ := manifest["description"].(string); d == "" || len([]rune(d)) > 132 {
		t.Errorf("description %q: the store takes 1 to 132 characters", d)
	}
	if name, _ := manifest["name"].(string); name == "" || len([]rune(name)) > 75 {
		t.Errorf("name %q: the store takes 1 to 75 characters", name)
	}
	readZipped(t, archive, "icon-128.png")
	readZipped(t, archive, chromeServiceWorker)
	for _, f := range archive.File {
		if f.ModifiedDate>>5&0xf == 0 || f.ModifiedDate&0x1f == 0 {
			t.Errorf("%s has no real date (month or day 0)", f.Name)
		}
	}
	again, _ := PackChromeStoreZip()
	if !bytes.Equal(data, again) {
		t.Error("the same files gave a different zip")
	}
}

func TestChromeUpdateXML(t *testing.T) {
	xml := string(ChromeUpdateXML("abc", "http://127.0.0.1:47613/chrome/kc9k-1.crx"))
	for _, want := range []string{`appid="abc"`, `codebase="http://127.0.0.1:47613/chrome/kc9k-1.crx"`, `version="` + Version() + `"`} {
		if !strings.Contains(xml, want) {
			t.Errorf("update manifest lacks %s:\n%s", want, xml)
		}
	}
}
