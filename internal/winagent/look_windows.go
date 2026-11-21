package winagent

// look_windows.go — what the circadian rhythm changes on a desktop
// (ROADMAP W6), done by the agent as the signed-in person: the screens'
// gamma ramps (colour temperature and software brightness), the
// wallpaper, dark mode, whether Windows' Night light is on, and external
// screens' brightness over DDC/CI.

import (
	"encoding/json"
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Ops of W6.
const (
	OpGetGamma     = "get-gamma"      // → []Ramp (one per screen)
	OpSetGamma     = "set-gamma"      // Ramp → screens set
	OpGetLook      = "get-look"       // → Look
	OpSetWallpaper = "set-wallpaper"  // string path
	OpSetDark      = "set-dark"       // bool
	OpMonitors     = "monitors"       // → []Monitor
	OpSetMonitor   = "set-monitor"    // Monitor (Index, Level)
	OpNightLightOn = "night-light-on" // → bool
)

// Ramp is a gamma ramp: 256 values per colour.
type Ramp struct {
	Red   []uint16 `json:"r"`
	Green []uint16 `json:"g"`
	Blue  []uint16 `json:"b"`
}

// Look is what a circadian preview may change, to put back.
type Look struct {
	Wallpaper   string `json:"wallpaper"`
	AppsLight   int    `json:"apps_light"`   // -1 unknown
	SystemLight int    `json:"system_light"` // -1 unknown
}

// Monitor is one external screen with DDC/CI brightness.
type Monitor struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Min   uint32 `json:"min"`
	Max   uint32 `json:"max"`
	Level uint32 `json:"level"`
}

var (
	dxva2                                   = windows.NewLazySystemDLL("dxva2.dll")
	procGetDC                               = user32.NewProc("GetDC")
	procReleaseDC                           = user32.NewProc("ReleaseDC")
	procEnumDisplayMonitors                 = user32.NewProc("EnumDisplayMonitors")
	procSystemParametersInfoW               = user32.NewProc("SystemParametersInfoW")
	procSendMessageTimeoutW                 = user32.NewProc("SendMessageTimeoutW")
	procGetDeviceGammaRamp                  = gdi32.NewProc("GetDeviceGammaRamp")
	procSetDeviceGammaRamp                  = gdi32.NewProc("SetDeviceGammaRamp")
	procCreateDCW                           = gdi32.NewProc("CreateDCW")
	procDeleteDC                            = gdi32.NewProc("DeleteDC")
	procGetMonitorInfoW                     = user32.NewProc("GetMonitorInfoW")
	procGetNumberOfPhysicalMonitorsFromHMON = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procGetPhysicalMonitorsFromHMONITOR     = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procDestroyPhysicalMonitors             = dxva2.NewProc("DestroyPhysicalMonitors")
	procGetMonitorBrightness                = dxva2.NewProc("GetMonitorBrightness")
	procSetMonitorBrightness                = dxva2.NewProc("SetMonitorBrightness")
)

type monitorInfoEx struct {
	size    uint32
	monitor [4]int32
	work    [4]int32
	flags   uint32
	device  [32]uint16
}

type physicalMonitor struct {
	handle      windows.Handle
	description [128]uint16
}

// displays are the screens' device names (\\.\DISPLAY1 ...) and monitors.
func displays() (names []string, handles []uintptr) {
	cb := syscall.NewCallback(func(h, hdc, rc, data uintptr) uintptr {
		info := monitorInfoEx{size: uint32(unsafe.Sizeof(monitorInfoEx{}))}
		if r, _, _ := procGetMonitorInfoW.Call(h, uintptr(unsafe.Pointer(&info))); r != 0 {
			names = append(names, windows.UTF16ToString(info.device[:]))
			handles = append(handles, h)
		}
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	return names, handles
}

// screenDC opens a device context for one screen.
func screenDC(name string) uintptr {
	n, _ := windows.UTF16PtrFromString(name)
	dc, _, _ := procCreateDCW.Call(0, uintptr(unsafe.Pointer(n)), 0, 0)
	return dc
}

func init() {
	Register(OpGetGamma, func(json.RawMessage) (any, error) {
		names, _ := displays()
		var out []Ramp
		for _, n := range names {
			dc := screenDC(n)
			if dc == 0 {
				continue
			}
			var buf [3 * 256]uint16
			r, _, _ := procGetDeviceGammaRamp.Call(dc, uintptr(unsafe.Pointer(&buf[0])))
			procDeleteDC.Call(dc)
			if r != 0 {
				out = append(out, Ramp{Red: append([]uint16(nil), buf[:256]...), Green: append([]uint16(nil), buf[256:512]...), Blue: append([]uint16(nil), buf[512:]...)})
			}
		}
		return out, nil
	})
	Register(OpSetGamma, func(raw json.RawMessage) (any, error) {
		var ramp Ramp
		if err := json.Unmarshal(raw, &ramp); err != nil {
			return nil, err
		}
		if len(ramp.Red) != 256 || len(ramp.Green) != 256 || len(ramp.Blue) != 256 {
			return nil, errors.New("a ramp has 256 values per colour")
		}
		var buf [3 * 256]uint16
		copy(buf[:256], ramp.Red)
		copy(buf[256:512], ramp.Green)
		copy(buf[512:], ramp.Blue)
		names, _ := displays()
		set := 0
		var lastErr error
		for _, n := range names {
			dc := screenDC(n)
			if dc == 0 {
				continue
			}
			r, _, err := procSetDeviceGammaRamp.Call(dc, uintptr(unsafe.Pointer(&buf[0])))
			procDeleteDC.Call(dc)
			if r != 0 {
				set++
			} else {
				lastErr = err
			}
		}
		if set == 0 && lastErr != nil {
			return 0, lastErr
		}
		return set, nil
	})
	Register(OpGetLook, func(json.RawMessage) (any, error) {
		look := Look{AppsLight: -1, SystemLight: -1}
		buf := make([]uint16, 1024)
		const spiGetDeskWallpaper = 0x0073
		if r, _, _ := procSystemParametersInfoW.Call(spiGetDeskWallpaper, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])), 0); r != 0 {
			look.Wallpaper = windows.UTF16ToString(buf)
		}
		if k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE); err == nil {
			if v, _, err := k.GetIntegerValue("AppsUseLightTheme"); err == nil {
				look.AppsLight = int(v)
			}
			if v, _, err := k.GetIntegerValue("SystemUsesLightTheme"); err == nil {
				look.SystemLight = int(v)
			}
			k.Close()
		}
		return look, nil
	})
	Register(OpSetWallpaper, func(raw json.RawMessage) (any, error) {
		var path string
		if err := json.Unmarshal(raw, &path); err != nil {
			return nil, err
		}
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return nil, err
		}
		const spiSetDeskWallpaper, spifUpdateIniFile, spifSendChange = 0x0014, 0x1, 0x2
		if r, _, err := procSystemParametersInfoW.Call(spiSetDeskWallpaper, 0, uintptr(unsafe.Pointer(p)), spifUpdateIniFile|spifSendChange); r == 0 {
			return nil, err
		}
		return nil, nil
	})
	Register(OpSetDark, func(raw json.RawMessage) (any, error) {
		var dark bool
		if err := json.Unmarshal(raw, &dark); err != nil {
			return nil, err
		}
		return nil, setDark(dark)
	})
	Register(OpNightLightOn, func(json.RawMessage) (any, error) { return nightLightOn(), nil })
	Register(OpMonitors, func(json.RawMessage) (any, error) { return monitors(-1, 0) })
	Register(OpSetMonitor, func(raw json.RawMessage) (any, error) {
		var m Monitor
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		_, err := monitors(m.Index, m.Level)
		return nil, err
	})
}

const personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`

// setDark switches apps and Windows to dark (or light) and tells open
// windows.
func setDark(dark bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, personalizeKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	v := uint32(1)
	if dark {
		v = 0
	}
	if err := k.SetDWordValue("AppsUseLightTheme", v); err != nil {
		return err
	}
	if err := k.SetDWordValue("SystemUsesLightTheme", v); err != nil {
		return err
	}
	theme, _ := windows.UTF16PtrFromString("ImmersiveColorSet")
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	var result uintptr
	procSendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(theme)), smtoAbortIfHung, 2000, uintptr(unsafe.Pointer(&result)))
	return nil
}

// nightLightOn reads Windows' Night light state from the user's settings
// store (byte 18 of its state blob is 0x15 while on).
func nightLightOn() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\CloudStore\Store\DefaultAccount\Current\default$windows.data.bluelightreduction.bluelightreductionstate\windows.data.bluelightreduction.bluelightreductionstate`,
		registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	data, _, err := k.GetBinaryValue("Data")
	return err == nil && len(data) > 18 && data[18] == 0x15
}

// monitors lists external screens with DDC/CI brightness; with index >=
// 0 it sets that one's brightness to level instead.
func monitors(index int, level uint32) ([]Monitor, error) {
	if dxva2.Load() != nil {
		return nil, errors.New("dxva2.dll is missing")
	}
	_, hmons := displays()
	var out []Monitor
	i := 0
	var setErr error
	for _, h := range hmons {
		var n uint32
		if r, _, _ := procGetNumberOfPhysicalMonitorsFromHMON.Call(h, uintptr(unsafe.Pointer(&n))); r == 0 || n == 0 {
			continue
		}
		phys := make([]physicalMonitor, n)
		if r, _, _ := procGetPhysicalMonitorsFromHMONITOR.Call(h, uintptr(n), uintptr(unsafe.Pointer(&phys[0]))); r == 0 {
			continue
		}
		for _, p := range phys {
			var lo, cur, hi uint32
			if r, _, _ := procGetMonitorBrightness.Call(uintptr(p.handle), uintptr(unsafe.Pointer(&lo)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&hi))); r != 0 {
				if i == index {
					if r, _, err := procSetMonitorBrightness.Call(uintptr(p.handle), uintptr(level)); r == 0 {
						setErr = err
					}
				}
				out = append(out, Monitor{Index: i, Name: windows.UTF16ToString(p.description[:]), Min: lo, Max: hi, Level: cur})
				i++
			}
		}
		procDestroyPhysicalMonitors.Call(uintptr(n), uintptr(unsafe.Pointer(&phys[0])))
	}
	return out, setErr
}
