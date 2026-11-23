// Package winres writes the Windows resource object (.syso) that links an
// application manifest into kc9k.exe (ROADMAP §4.3: common controls v6,
// per-monitor DPI aware). The Go linker takes any rsrc_windows_<arch>.syso
// in a main package's folder; this writes the smallest one Windows reads:
// one RT_MANIFEST resource, ID 1, language neutral.
//
// The committed files are checked against cmd/kc9k/kc9k.manifest by
// TestSysoUpToDate; regenerate with UPDATE_SYSO=1 go test ./internal/winres.
package winres

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Machine types and their "address relative to the image" relocation,
// which the linker fills in with the resource section's address (Go's
// cmd/link: loadpe reads it as R_PEIMAGEOFF, addpersrc applies it).
var machines = map[string]struct {
	machine uint16
	reloc   uint16
}{
	"amd64": {0x8664, 0x0003}, // IMAGE_REL_AMD64_ADDR32NB
	"arm64": {0xAA64, 0x0002}, // IMAGE_REL_ARM64_ADDR32NB
	"386":   {0x014C, 0x0007}, // IMAGE_REL_I386_DIR32NB
}

// Arches are the architectures a .syso is committed for.
var Arches = []string{"amd64", "arm64"}

const (
	rtManifest      = 24
	manifestID      = 1 // CREATEPROCESS_MANIFEST_RESOURCE_ID: the process's own
	langNeutral     = 0
	dirHeaderSize   = 16
	dirEntrySize    = 8
	dataEntrySize   = 16
	subdirFlag      = 0x80000000
	sectionReadData = 0x40000040 // IMAGE_SCN_CNT_INITIALIZED_DATA | IMAGE_SCN_MEM_READ
)

// Syso returns a COFF object holding manifest as the program's manifest
// resource for arch.
func Syso(manifest []byte, arch string) ([]byte, error) {
	m, ok := machines[arch]
	if !ok {
		return nil, fmt.Errorf("winres: no machine type for %q", arch)
	}

	// The .rsrc section: three one-entry directories (type, name,
	// language), one data entry, the manifest.
	var sec bytes.Buffer
	le := binary.LittleEndian
	dir := func(id, target uint32) {
		binary.Write(&sec, le, [4]uint32{0, 0, 0, 1 << 16}) // characteristics, time, version, 0 named + 1 ID entry
		binary.Write(&sec, le, [2]uint32{id, target})
	}
	typeDir := uint32(0)
	nameDir := typeDir + dirHeaderSize + dirEntrySize
	langDir := nameDir + dirHeaderSize + dirEntrySize
	dataEntry := langDir + dirHeaderSize + dirEntrySize
	data := dataEntry + dataEntrySize
	dir(rtManifest, subdirFlag|nameDir)
	dir(manifestID, subdirFlag|langDir)
	dir(langNeutral, dataEntry)
	// OffsetToData holds the data's offset in the section; the relocation
	// turns it into an address in the image.
	binary.Write(&sec, le, [4]uint32{data, uint32(len(manifest)), 0, 0})
	sec.Write(manifest)
	for sec.Len()%8 != 0 {
		sec.WriteByte(0)
	}

	const fileHeader, sectionHeader, relocSize = 20, 40, 10
	raw := uint32(fileHeader + sectionHeader)
	relocs := raw + uint32(sec.Len())
	symbols := relocs + relocSize

	var out bytes.Buffer
	// COFF file header: one section, one symbol, no optional header.
	binary.Write(&out, le, struct {
		Machine              uint16
		NumberOfSections     uint16
		TimeDateStamp        uint32
		PointerToSymbolTable uint32
		NumberOfSymbols      uint32
		SizeOfOptionalHeader uint16
		Characteristics      uint16
	}{m.machine, 1, 0, symbols, 1, 0, 0})
	binary.Write(&out, le, struct {
		Name                 [8]byte
		VirtualSize          uint32
		VirtualAddress       uint32
		SizeOfRawData        uint32
		PointerToRawData     uint32
		PointerToRelocations uint32
		PointerToLinenumbers uint32
		NumberOfRelocations  uint16
		NumberOfLinenumbers  uint16
		Characteristics      uint32
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 0, uint32(sec.Len()), raw, relocs, 0, 1, 0, sectionReadData})
	out.Write(sec.Bytes())
	// The data entry's OffsetToData, against symbol 0 (the section).
	binary.Write(&out, le, dataEntry)
	binary.Write(&out, le, uint32(0))
	binary.Write(&out, le, m.reloc)
	// Symbol 0: the section symbol ".rsrc" (static, section 1).
	binary.Write(&out, le, struct {
		Name               [8]byte
		Value              uint32
		SectionNumber      int16
		Type               uint16
		StorageClass       uint8
		NumberOfAuxSymbols uint8
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 1, 0, 3, 0})
	binary.Write(&out, le, uint32(4)) // empty string table
	return out.Bytes(), nil
}
