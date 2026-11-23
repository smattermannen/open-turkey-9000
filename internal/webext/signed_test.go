package webext

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"testing"
)

// signedCopy packs the embedded files plus META-INF entries, as Mozilla's
// signing returns them; edit changes one file first.
func signedCopy(t *testing.T, withSignature bool, edit string) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	err := fs.WalkDir(files, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, _ := files.ReadFile(path)
		name := path[len("files/"):]
		if name == edit {
			data = append(append([]byte{}, data...), ' ')
		}
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	meta := []string{"META-INF/manifest.mf", "META-INF/mozilla.sf"}
	if withSignature {
		meta = append(meta, "META-INF/mozilla.rsa")
	}
	for _, m := range meta {
		w, _ := z.Create(m)
		w.Write([]byte("x"))
	}
	z.Close()
	return out.Bytes()
}

// 2.12: Firefox gets the signed file only when it is Mozilla's signature
// over exactly this version's files.
func TestSignedXPIMatches(t *testing.T) {
	if !signedMatches(signedCopy(t, true, "")) {
		t.Error("a signed copy of these files was refused")
	}
	if signedMatches(signedCopy(t, false, "")) {
		t.Error("a copy without Mozilla's signature was taken")
	}
	if signedMatches(signedCopy(t, true, "manifest.json")) {
		t.Error("a signed copy of other files (stale version) was taken")
	}
	if signedMatches([]byte("not a zip")) {
		t.Error("garbage was taken")
	}
	// No file for this version is committed yet: Firefox waits (SPEC §7).
	if _, err := signedFiles.ReadFile("signed/kc9k-" + Version() + ".xpi"); err == nil && !MozillaSigned() {
		t.Errorf("signed/kc9k-%s.xpi is there but doesn't match this version's files", Version())
	}
}
