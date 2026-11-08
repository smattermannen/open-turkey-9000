// Package webext embeds the browser extension and packs it as an .xpi.
package webext

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"sort"
)

//go:embed files
var files embed.FS

// ID is the extension's id in browser policies.
const ID = "blocker@kc9k"

// Dir is where the packed extension is installed.
const Dir = "/usr/share/kc9k"

// Version returns the extension's version, from its manifest.
func Version() string {
	data, err := files.ReadFile("files/manifest.json")
	if err != nil {
		return "0"
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) != nil || manifest.Version == "" {
		return "0"
	}
	return manifest.Version
}

// Path is where this version of the extension is installed. The file name
// carries the version, so the browser policy points somewhere new with every
// version and browsers always install the update. That matters: the daemon
// closes browsers whose extension doesn't check in the current way.
func Path() string {
	return filepath.Join(Dir, "kc9k-"+Version()+".xpi")
}

// StaleFiles returns installed extension files other than this version's,
// including the unversioned kc9k.xpi of earlier releases.
func StaleFiles() []string {
	matches, _ := filepath.Glob(filepath.Join(Dir, "kc9k*.xpi"))
	var stale []string
	for _, m := range matches {
		if m != Path() {
			stale = append(stale, m)
		}
	}
	return stale
}

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
