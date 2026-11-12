// Chrome: the same extension, packed for Google Chrome and Chromium.
//
// Chrome only takes Manifest V3, so PackChrome swaps the Firefox manifest for
// one built from it (ChromeManifest) and adds a service worker that loads the
// same background scripts. Off the Chrome Web Store, Chrome installs an
// extension only as a signed .crx (CRX3) that it fetches from an update URL;
// the daemon serves both (see enforce.ServeRules), and policy force-installs
// it (blocker.EnsureChromeSetup). Force-installed is also what lets a
// Manifest V3 extension block requests (webRequestBlocking).
//
// The extension's ID in Chrome comes from the signing key. Each computer
// makes its own, kept by root (ChromeKeyPath): someone who had it could sign
// an extension that does nothing under KC9K's ID.
package webext

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// ChromeKeyPath is the private key the Chrome extension is signed with.
var ChromeKeyPath = "/var/lib/kc9k/chrome-extension.pem"

// ChromeKey loads the signing key from path, making one the first time.
func ChromeKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		// O_EXCL: never replace a key, which would change the extension's ID.
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		if err := pem.Encode(file, block); err != nil {
			file.Close()
			os.Remove(path)
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		return key, nil
	}
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%s holds no key", path)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s is not an RSA key", path)
	}
	return key, nil
}

// ChromeID is the extension ID Chrome gives an extension signed with key:
// the first 16 bytes of the SHA-256 of its public key, each hex digit
// written as a letter from a to p.
func ChromeID(key *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	id := []byte(hex.EncodeToString(sum[:16]))
	for i, c := range id {
		if c <= '9' {
			id[i] = 'a' + c - '0'
		} else {
			id[i] = 'a' + 10 + c - 'a'
		}
	}
	return string(id), nil
}

// firefoxManifest is the parts of manifest.json the Chrome manifest is built
// from, so the two never drift apart.
type firefoxManifest struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	Permissions   []string
	BrowserAction struct {
		DefaultTitle string `json:"default_title"`
		DefaultPopup string `json:"default_popup"`
	} `json:"browser_action"`
	Background struct {
		Scripts []string `json:"scripts"`
	} `json:"background"`
	ContentScripts         []json.RawMessage `json:"content_scripts"`
	WebAccessibleResources []string          `json:"web_accessible_resources"`
}

// chromeIcons are the icon sizes drawn as PNG (Chrome can't show SVG icons).
var chromeIcons = []string{"16", "32", "48", "128"}

// chromeServiceWorker is the background service worker Chrome runs: it
// loads Firefox's background scripts, in the same order.
const chromeServiceWorker = "chrome-worker.js"

// ChromeManifest returns the Manifest V3 manifest for Chrome.
func ChromeManifest() ([]byte, error) {
	data, err := files.ReadFile("files/manifest.json")
	if err != nil {
		return nil, err
	}
	var ff firefoxManifest
	if err := json.Unmarshal(data, &ff); err != nil {
		return nil, err
	}
	icons := map[string]string{}
	for _, size := range chromeIcons {
		icons[size] = "icon-" + size + ".png"
	}
	manifest := map[string]interface{}{
		"manifest_version":       3,
		"name":                   ff.Name,
		"version":                ff.Version,
		"description":            ff.Description,
		"minimum_chrome_version": "120",
		// alarms wakes the service worker should Chrome ever stop it.
		"permissions":      []string{"webRequest", "webRequestBlocking", "storage", "tabs", "idle", "alarms"},
		"host_permissions": []string{"<all_urls>"},
		"action": map[string]interface{}{
			"default_title": ff.BrowserAction.DefaultTitle,
			"default_popup": ff.BrowserAction.DefaultPopup,
			"default_icon":  icons,
		},
		"icons":           icons,
		"background":      map[string]string{"service_worker": chromeServiceWorker},
		"content_scripts": ff.ContentScripts,
		"web_accessible_resources": []map[string][]string{{
			"resources": ff.WebAccessibleResources,
			"matches":   {"<all_urls>"},
		}},
	}
	return json.MarshalIndent(manifest, "", "  ")
}

// chromeWorker returns the service worker script.
func chromeWorker() ([]byte, error) {
	data, err := files.ReadFile("files/manifest.json")
	if err != nil {
		return nil, err
	}
	var ff firefoxManifest
	if err := json.Unmarshal(data, &ff); err != nil {
		return nil, err
	}
	quoted := make([]string, len(ff.Background.Scripts))
	for i, s := range ff.Background.Scripts {
		b, _ := json.Marshal(s)
		quoted[i] = string(b)
	}
	return []byte("// Chrome's background service worker: Firefox's background scripts.\nimportScripts(" +
		strings.Join(quoted, ", ") + ");\n"), nil
}

// PackChromeZip returns the extension's files with the Chrome manifest and
// service worker, as a zip.
func PackChromeZip() ([]byte, error) {
	manifest, err := ChromeManifest()
	if err != nil {
		return nil, err
	}
	worker, err := chromeWorker()
	if err != nil {
		return nil, err
	}
	return pack(map[string][]byte{"manifest.json": manifest, chromeServiceWorker: worker})
}

// PackChrome returns the extension as a CRX3 file signed with key. The same
// key and files always give the same bytes.
//
// The format: "Cr24", version 3, the header's length and the header (a
// CrxFileHeader protocol buffer), then the zip. The header holds the public
// key, the ID (as SignedData), and a signature over
// "CRX3 SignedData\x00", the SignedData's length, the SignedData and the zip.
func PackChrome(key *rsa.PrivateKey) ([]byte, error) {
	archive, err := PackChromeZip()
	if err != nil {
		return nil, err
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(public)
	signedData := protoBytes(1, sum[:16]) // SignedData.crx_id

	var message bytes.Buffer
	message.WriteString("CRX3 SignedData\x00")
	_ = binary.Write(&message, binary.LittleEndian, uint32(len(signedData)))
	message.Write(signedData)
	message.Write(archive)
	digest := sha256.Sum256(message.Bytes())
	signature, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, digest[:])
	if err != nil {
		return nil, err
	}

	proof := append(protoBytes(1, public), protoBytes(2, signature)...) // AsymmetricKeyProof
	header := append(protoBytes(2, proof), protoBytes(10000, signedData)...)

	var out bytes.Buffer
	out.WriteString("Cr24")
	_ = binary.Write(&out, binary.LittleEndian, uint32(3))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(header)))
	out.Write(header)
	out.Write(archive)
	return out.Bytes(), nil
}

// protoBytes encodes a length-delimited protocol buffer field.
func protoBytes(field int, value []byte) []byte {
	out := binary.AppendUvarint(nil, uint64(field)<<3|2)
	out = binary.AppendUvarint(out, uint64(len(value)))
	return append(out, value...)
}

// ChromeUpdateXML is the update manifest that tells Chrome which version of
// the extension is current and where to download it.
func ChromeUpdateXML(id, codebase string) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<gupdate xmlns="http://www.google.com/update2/response" protocol="2.0">
  <app appid="%s">
    <updatecheck codebase="%s" version="%s" />
  </app>
</gupdate>
`, html.EscapeString(id), html.EscapeString(codebase), html.EscapeString(Version())))
}
