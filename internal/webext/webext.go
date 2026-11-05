// Package webext embeds the browser extension and packs it as an .xpi.
package webext

import (
	"archive/zip"
	"bytes"
	"embed"
	"io/fs"
	"sort"
)

//go:embed files
var files embed.FS

// ID is the extension's id in browser policies.
const ID = "blocker@kc9k"

// Path is where the packed extension is installed.
const Path = "/usr/share/kc9k/kc9k.xpi"

// Pack returns the extension as an .xpi. The output is the same on every call,
// so the daemon can tell whether the installed file is current.
func Pack() ([]byte, error) {
	entries, err := fs.ReadDir(files, "files")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	for _, name := range names {
		data, err := files.ReadFile("files/" + name)
		if err != nil {
			return nil, err
		}
		w, err := archive.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
