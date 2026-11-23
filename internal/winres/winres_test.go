package winres

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const kc9kDir = "../../cmd/kc9k"

func manifest(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(kc9kDir, "kc9k.manifest"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestManifestWellFormed: Windows refuses to start a program whose
// manifest doesn't parse ("side-by-side configuration is incorrect").
func TestManifestWellFormed(t *testing.T) {
	m := manifest(t)
	d := xml.NewDecoder(bytes.NewReader(m))
	for {
		if _, err := d.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("kc9k.manifest: %v", err)
		}
	}
	for _, want := range []string{"Microsoft.Windows.Common-Controls", `version="6.0.0.0"`, "PerMonitorV2", "true/pm", `level="asInvoker"`} {
		if !bytes.Contains(m, []byte(want)) {
			t.Errorf("kc9k.manifest lacks %s", want)
		}
	}
}

// TestSysoUpToDate: the committed .syso files are what Syso makes of the
// manifest. UPDATE_SYSO=1 rewrites them.
func TestSysoUpToDate(t *testing.T) {
	m := manifest(t)
	for _, arch := range Arches {
		want, err := Syso(m, arch)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(kc9kDir, "rsrc_windows_"+arch+".syso")
		if os.Getenv("UPDATE_SYSO") == "1" {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s is stale or missing (%v): UPDATE_SYSO=1 go test ./internal/winres", path, err)
		}
	}
}

// TestManifestLinked builds kc9k.exe for Windows and reads the manifest
// back out of its resource section, as Windows' loader does.
func TestManifestLinked(t *testing.T) {
	if testing.Short() {
		t.Skip("builds kc9k.exe")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, arch := range Arches {
		exe := filepath.Join(t.TempDir(), "kc9k.exe")
		cmd := exec.Command(goBin, "build", "-o", exe, ".")
		cmd.Dir = kc9kDir
		cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", arch, err, out)
		}
		got := readManifest(t, exe)
		if !bytes.Equal(got, manifest(t)) {
			t.Errorf("%s: manifest in kc9k.exe differs from kc9k.manifest (%d bytes)", arch, len(got))
		}
	}
}

// readManifest walks the image's resource directory: RT_MANIFEST, ID 1,
// first language, data entry (an address in the image).
func readManifest(t *testing.T, exe string) []byte {
	t.Helper()
	f, err := pe.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var dd pe.DataDirectory
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		dd = oh.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE]
	case *pe.OptionalHeader32:
		dd = oh.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE]
	}
	if dd.VirtualAddress == 0 {
		t.Fatal("no resource directory in kc9k.exe")
	}
	var sec *pe.Section
	for _, s := range f.Sections {
		if dd.VirtualAddress >= s.VirtualAddress && dd.VirtualAddress < s.VirtualAddress+s.VirtualSize {
			sec = s
		}
	}
	if sec == nil || !strings.HasPrefix(sec.Name, ".rsrc") {
		t.Fatalf("resource directory not in .rsrc (%v)", sec)
	}
	data, err := sec.Data()
	if err != nil {
		t.Fatal(err)
	}
	base := dd.VirtualAddress - sec.VirtualAddress
	le := binary.LittleEndian
	find := func(dir, id uint32, any bool) uint32 {
		named := uint32(le.Uint16(data[base+dir+12:]))
		ids := uint32(le.Uint16(data[base+dir+14:]))
		for i := named; i < named+ids; i++ {
			e := base + dir + 16 + 8*i
			if any || le.Uint32(data[e:]) == id {
				return le.Uint32(data[e+4:])
			}
		}
		t.Fatalf("resource %d not found", id)
		return 0
	}
	name := find(0, rtManifest, false) &^ subdirFlag
	lang := find(name, manifestID, false) &^ subdirFlag
	entry := find(lang, 0, true)
	rva, size := le.Uint32(data[base+entry:]), le.Uint32(data[base+entry+4:])
	off := rva - sec.VirtualAddress
	if uint64(off)+uint64(size) > uint64(len(data)) {
		t.Fatalf("manifest at %#x+%d outside .rsrc", rva, size)
	}
	return data[off : off+size]
}
