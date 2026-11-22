package winapi

// wlan_windows.go — the Native Wifi API (wlanapi.dll): access points in
// range for places (ROADMAP W10), and each radio's software switch, which
// Windows' "Wi-Fi off" and airplane mode turn off while the adapter stays
// enabled.

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	wlanapi                   = windows.NewLazySystemDLL("wlanapi.dll")
	procWlanOpenHandle        = wlanapi.NewProc("WlanOpenHandle")
	procWlanCloseHandle       = wlanapi.NewProc("WlanCloseHandle")
	procWlanEnumInterfaces    = wlanapi.NewProc("WlanEnumInterfaces")
	procWlanScan              = wlanapi.NewProc("WlanScan")
	procWlanGetNetworkBssList = wlanapi.NewProc("WlanGetNetworkBssList")
	procWlanQueryInterface    = wlanapi.NewProc("WlanQueryInterface")
	procWlanSetInterface      = wlanapi.NewProc("WlanSetInterface")
	procWlanFreeMemory        = wlanapi.NewProc("WlanFreeMemory")
)

const (
	wlanIntfOpcodeRadioState = 4 // wlan_intf_opcode_radio_state
	dot11BSSTypeAny          = 3
	dot11RadioOn             = 1
)

// Wlan is an open Native Wifi client handle.
type Wlan struct{ h windows.Handle }

// OpenWlan opens the WLAN service (WLAN AutoConfig). Without it (no Wi-Fi,
// Windows Server) it fails.
func OpenWlan() (*Wlan, error) {
	if err := wlanapi.Load(); err != nil {
		return nil, err
	}
	var version uint32
	var h windows.Handle
	if r, _, _ := procWlanOpenHandle.Call(2, 0, uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&h))); r != 0 {
		return nil, fmt.Errorf("WlanOpenHandle: %w", windows.Errno(r))
	}
	return &Wlan{h}, nil
}

// Close closes the handle.
func (w *Wlan) Close() { procWlanCloseHandle.Call(uintptr(w.h), 0) }

// wlanBytes copies n bytes of WLAN-owned memory and frees it.
func wlanBytes(p *byte, n int) []byte {
	out := make([]byte, n)
	copy(out, unsafe.Slice(p, n))
	procWlanFreeMemory.Call(uintptr(unsafe.Pointer(p)))
	return out
}

// Interfaces are the Wi-Fi interfaces' GUIDs.
func (w *Wlan) Interfaces() ([]windows.GUID, error) {
	var p *byte
	if r, _, _ := procWlanEnumInterfaces.Call(uintptr(w.h), 0, uintptr(unsafe.Pointer(&p))); r != 0 {
		return nil, fmt.Errorf("WlanEnumInterfaces: %w", windows.Errno(r))
	}
	n := *(*uint32)(unsafe.Pointer(p))
	ids, err := ParseInterfaceList(wlanBytes(p, 8+int(n)*wlanInterfaceInfoSize))
	if err != nil {
		return nil, err
	}
	out := make([]windows.GUID, len(ids))
	for i, id := range ids {
		out[i] = *(*windows.GUID)(unsafe.Pointer(&id[0]))
	}
	return out, nil
}

// Scan asks the interface to look for networks again; the answer comes
// within seconds, read by BSSList.
func (w *Wlan) Scan(iface windows.GUID) error {
	if r, _, _ := procWlanScan.Call(uintptr(w.h), uintptr(unsafe.Pointer(&iface)), 0, 0, 0); r != 0 {
		return fmt.Errorf("WlanScan: %w", windows.Errno(r))
	}
	return nil
}

// BSSList is every access point the interface saw last.
func (w *Wlan) BSSList(iface windows.GUID) ([]BSS, error) {
	var p *byte
	if r, _, _ := procWlanGetNetworkBssList.Call(uintptr(w.h), uintptr(unsafe.Pointer(&iface)), 0, dot11BSSTypeAny, 0, 0, uintptr(unsafe.Pointer(&p))); r != 0 {
		return nil, fmt.Errorf("WlanGetNetworkBssList: %w", windows.Errno(r))
	}
	total := *(*uint32)(unsafe.Pointer(p))
	return ParseBSSList(wlanBytes(p, int(total)))
}

// Radios are the interface's radios and their switches.
func (w *Wlan) Radios(iface windows.GUID) ([]RadioState, error) {
	var size uint32
	var p *byte
	var kind uint32
	if r, _, _ := procWlanQueryInterface.Call(uintptr(w.h), uintptr(unsafe.Pointer(&iface)), wlanIntfOpcodeRadioState, 0,
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&kind))); r != 0 {
		return nil, fmt.Errorf("WlanQueryInterface: %w", windows.Errno(r))
	}
	return ParseRadioState(wlanBytes(p, int(size)))
}

// SetRadioOn switches one radio's software switch on (airplane mode and
// Windows' "Wi-Fi off" turn it off). A hardware switch can't be turned on.
func (w *Wlan) SetRadioOn(iface windows.GUID, phy uint32) error {
	state := [3]uint32{phy, dot11RadioOn, dot11RadioOn}
	if r, _, _ := procWlanSetInterface.Call(uintptr(w.h), uintptr(unsafe.Pointer(&iface)), wlanIntfOpcodeRadioState,
		unsafe.Sizeof(state), uintptr(unsafe.Pointer(&state[0])), 0); r != 0 {
		return fmt.Errorf("WlanSetInterface: %w", windows.Errno(r))
	}
	return nil
}
