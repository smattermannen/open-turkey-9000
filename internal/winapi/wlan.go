package winapi

// wlan.go — reading the Native Wifi API's answers (wlan_windows.go): pure
// byte layouts, so they are tested on every system.

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// BSS is one access point in range: its SSID and BSSID (upper case,
// colons), as nmcli gives them on Linux.
type BSS struct {
	SSID, BSSID string
}

// wlanBSSEntrySize is sizeof(WLAN_BSS_ENTRY); entries follow
// WLAN_BSS_LIST's two DWORDs (dwTotalSize, dwNumberOfItems).
const wlanBSSEntrySize = 360

// ParseBSSList reads a WLAN_BSS_LIST: per entry DOT11_SSID (ULONG length,
// 32 bytes) at 0, the BSSID (6 bytes) at 40.
func ParseBSSList(buf []byte) ([]BSS, error) {
	if len(buf) < 8 {
		return nil, fmt.Errorf("BSS list too short (%d bytes)", len(buf))
	}
	n := int(binary.LittleEndian.Uint32(buf[4:8]))
	if n < 0 || 8+n*wlanBSSEntrySize > len(buf) {
		return nil, fmt.Errorf("BSS list of %d entries in %d bytes", n, len(buf))
	}
	out := make([]BSS, 0, n)
	for i := 0; i < n; i++ {
		e := buf[8+i*wlanBSSEntrySize:]
		l := int(binary.LittleEndian.Uint32(e[0:4]))
		if l > 32 {
			l = 32
		}
		mac := e[40:46]
		parts := make([]string, 6)
		for j, b := range mac {
			parts[j] = fmt.Sprintf("%02X", b)
		}
		out = append(out, BSS{SSID: string(e[4 : 4+l]), BSSID: strings.Join(parts, ":")})
	}
	return out, nil
}

// wlanInterfaceInfoSize is sizeof(WLAN_INTERFACE_INFO): GUID, WCHAR[256],
// WLAN_INTERFACE_STATE.
const wlanInterfaceInfoSize = 16 + 512 + 4

// ParseInterfaceList reads a WLAN_INTERFACE_INFO_LIST (dwNumberOfItems,
// dwIndex, entries) into the interfaces' GUIDs, 16 bytes each.
func ParseInterfaceList(buf []byte) ([][16]byte, error) {
	if len(buf) < 8 {
		return nil, fmt.Errorf("interface list too short (%d bytes)", len(buf))
	}
	n := int(binary.LittleEndian.Uint32(buf[0:4]))
	if n < 0 || 8+n*wlanInterfaceInfoSize > len(buf) {
		return nil, fmt.Errorf("interface list of %d entries in %d bytes", n, len(buf))
	}
	out := make([][16]byte, n)
	for i := range out {
		copy(out[i][:], buf[8+i*wlanInterfaceInfoSize:])
	}
	return out, nil
}

// RadioState is one radio's software and hardware switch (DOT11_RADIO_STATE:
// 0 unknown, 1 on, 2 off).
type RadioState struct {
	Phy                uint32
	Software, Hardware uint32
}

// ParseRadioState reads a WLAN_RADIO_STATE: dwNumberOfPhys, then per radio
// dwPhyIndex, software and hardware state.
func ParseRadioState(buf []byte) ([]RadioState, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("radio state too short (%d bytes)", len(buf))
	}
	n := int(binary.LittleEndian.Uint32(buf[0:4]))
	if n < 0 || n > 64 || 4+n*12 > len(buf) {
		return nil, fmt.Errorf("radio state of %d radios in %d bytes", n, len(buf))
	}
	out := make([]RadioState, n)
	for i := range out {
		e := buf[4+i*12:]
		out[i] = RadioState{binary.LittleEndian.Uint32(e[0:4]), binary.LittleEndian.Uint32(e[4:8]), binary.LittleEndian.Uint32(e[8:12])}
	}
	return out, nil
}
