package webext

// signed.go — the extension as Mozilla signed it, for release Firefox
// (ROADMAP 2.12; SPEC §7 "Signing the extension for Firefox"): release
// Firefox installs only Mozilla-signed add-ons, LibreWolf any. Once the
// owner's Mozilla account signs a version (unlisted), its file goes in
// signed/ as kc9k-<version>.xpi and every Firefox-family browser gets
// that one; until then Firefox is not a supported browser.

import (
	"archive/zip"
	"bytes"
	"embed"
	"io/fs"
	"sync"
)

//go:embed all:signed
var signedFiles embed.FS

var signedOnce struct {
	sync.Once
	data []byte
}

// SignedXPI is this version's Mozilla-signed extension, if it is here: a
// file carrying Mozilla's signature (META-INF/mozilla.rsa) and exactly the
// extension's own files besides its signature, so a stale or edited one is
// never used.
func SignedXPI() ([]byte, bool) {
	signedOnce.Do(func() {
		data, err := signedFiles.ReadFile("signed/kc9k-" + Version() + ".xpi")
		if err == nil && signedMatches(data) {
			signedOnce.data = data
		}
	})
	return signedOnce.data, signedOnce.data != nil
}

// MozillaSigned reports whether SignedXPI has this version.
func MozillaSigned() bool {
	_, ok := SignedXPI()
	return ok
}

// signedMatches checks a signed .xpi against the embedded files.
func signedMatches(data []byte) bool {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	inside := map[string][]byte{}
	signature := false
	for _, f := range r.File {
		if f.Name == "META-INF/mozilla.rsa" {
			signature = true
		}
		if len(f.Name) >= 9 && f.Name[:9] == "META-INF/" || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		var b bytes.Buffer
		_, err = b.ReadFrom(rc)
		rc.Close()
		if err != nil {
			return false
		}
		inside[f.Name] = b.Bytes()
	}
	if !signature {
		return false
	}
	own := 0
	err = fs.WalkDir(files, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		want, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		own++
		if got, ok := inside[path[len("files/"):]]; !ok || !bytes.Equal(got, want) {
			return fs.ErrNotExist
		}
		return nil
	})
	return err == nil && own == len(inside)
}
