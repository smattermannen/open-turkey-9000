package winapi

// wfp_windows.go — the Windows Filtering Platform, enough for KC9K's
// firewall: a persistent provider and sublayer of KC9K's own, and filters
// in them that block (or permit) by remote address, port and loopback.
// Layouts follow fwpmtypes.h for 64-bit Windows.

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	fwpuclnt                         = windows.NewLazySystemDLL("fwpuclnt.dll")
	procFwpmEngineOpen0              = fwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0             = fwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmTransactionBegin0        = fwpuclnt.NewProc("FwpmTransactionBegin0")
	procFwpmTransactionCommit0       = fwpuclnt.NewProc("FwpmTransactionCommit0")
	procFwpmTransactionAbort0        = fwpuclnt.NewProc("FwpmTransactionAbort0")
	procFwpmProviderAdd0             = fwpuclnt.NewProc("FwpmProviderAdd0")
	procFwpmProviderDeleteByKey0     = fwpuclnt.NewProc("FwpmProviderDeleteByKey0")
	procFwpmSubLayerAdd0             = fwpuclnt.NewProc("FwpmSubLayerAdd0")
	procFwpmSubLayerDeleteByKey0     = fwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFwpmFilterAdd0               = fwpuclnt.NewProc("FwpmFilterAdd0")
	procFwpmFilterDeleteByKey0       = fwpuclnt.NewProc("FwpmFilterDeleteByKey0")
	procFwpmFilterCreateEnumHandle0  = fwpuclnt.NewProc("FwpmFilterCreateEnumHandle0")
	procFwpmFilterEnum0              = fwpuclnt.NewProc("FwpmFilterEnum0")
	procFwpmFilterDestroyEnumHandle0 = fwpuclnt.NewProc("FwpmFilterDestroyEnumHandle0")
	procFwpmFreeMemory0              = fwpuclnt.NewProc("FwpmFreeMemory0")
)

// WFP layers and condition fields (fwpmu.h).
var (
	LayerConnectV4 = windows.GUID{Data1: 0xc38d57d1, Data2: 0x05a7, Data3: 0x4c33, Data4: [8]byte{0x90, 0x4f, 0x7f, 0xbc, 0xee, 0xe6, 0x0e, 0x82}}
	LayerConnectV6 = windows.GUID{Data1: 0x4a72393b, Data2: 0x319f, Data3: 0x44bc, Data4: [8]byte{0x84, 0xc3, 0xba, 0x54, 0xdc, 0xb3, 0xb6, 0xb4}}
	LayerAcceptV4  = windows.GUID{Data1: 0xe1cd9fe7, Data2: 0xf4b5, Data3: 0x4273, Data4: [8]byte{0x96, 0xc0, 0x59, 0x2e, 0x48, 0x7b, 0x86, 0x50}}
	LayerAcceptV6  = windows.GUID{Data1: 0xa3b42c97, Data2: 0x9f04, Data3: 0x4672, Data4: [8]byte{0xb8, 0x7e, 0xce, 0xe9, 0xc4, 0x83, 0x25, 0x7f}}

	FieldRemoteAddress = windows.GUID{Data1: 0xb235ae9a, Data2: 0x1d64, Data3: 0x49b8, Data4: [8]byte{0xa4, 0x4c, 0x5f, 0xf3, 0xd9, 0x09, 0x50, 0x45}}
	FieldRemotePort    = windows.GUID{Data1: 0xc35a604d, Data2: 0xd22b, Data3: 0x4e1a, Data4: [8]byte{0x91, 0xb4, 0x68, 0xf6, 0x74, 0xee, 0x67, 0x4b}}
	FieldFlags         = windows.GUID{Data1: 0x632ce23b, Data2: 0x5167, Data3: 0x435c, Data4: [8]byte{0x86, 0xd7, 0xe9, 0x03, 0x68, 0x4a, 0xa8, 0x0c}}
)

// WFP constants.
const (
	fwpEmpty          = 0
	fwpUint8          = 1
	fwpUint16         = 2
	fwpUint32         = 3
	fwpByteArray16    = 11
	fwpMatchEqual     = 0
	fwpMatchFlagsAll  = 6
	actionBlock       = 0x1001
	actionPermit      = 0x1002
	flagPersistent    = 0x1
	rpcCAuthnWinNT    = 10
	FlagIsLoopback    = 0x1
	fwpEAlreadyExists = 0x80320009
	fwpENotFound      = 0x80320002 // FWP_E_*_NOT_FOUND share 0x8032000x; checked by range
)

type displayData struct {
	name, description *uint16
}

type byteBlob struct {
	size uint32
	data *byte
}

type fwpValue struct {
	kind  uint32
	value uint64 // the union: a number, or a pointer
}

type conditionValue fwpValue

type filterCondition struct {
	field     windows.GUID
	matchType uint32
	value     conditionValue
}

type provider struct {
	key         windows.GUID
	display     displayData
	flags       uint32
	data        byteBlob
	serviceName *uint16
}

type sublayer struct {
	key         windows.GUID
	display     displayData
	flags       uint32
	providerKey *windows.GUID
	data        byteBlob
	weight      uint16
}

type action struct {
	kind uint32
	guid windows.GUID
}

type filter struct {
	key             windows.GUID
	display         displayData
	flags           uint32
	providerKey     *windows.GUID
	providerData    byteBlob
	layer           windows.GUID
	sublayer        windows.GUID
	weight          fwpValue
	numConditions   uint32
	conditions      *filterCondition
	action          action
	context         [16]byte // rawContext / providerContextKey
	reserved        *windows.GUID
	id              uint64
	effectiveWeight fwpValue
}

type session struct {
	key        windows.GUID
	display    displayData
	flags      uint32
	txnWaitMs  uint32
	processID  uint32
	sid        *windows.SID
	username   *uint16
	kernelMode int32
}

type filterEnumTemplate struct {
	providerKey     *windows.GUID
	layer           windows.GUID
	enumType        uint32
	flags           uint32
	providerContext uintptr
	numConditions   uint32
	conditions      *filterCondition
	actionMask      uint32
	calloutKey      *windows.GUID
}

type v6AddrMask struct {
	addr   [16]byte
	prefix uint8
}

// Sizes the code relies on (64-bit fwpmtypes.h).
var _ = [1]int{}[unsafe.Sizeof(filter{})-200]
var _ = [1]int{}[unsafe.Sizeof(filterCondition{})-40]
var _ = [1]int{}[unsafe.Sizeof(sublayer{})-72]
var _ = [1]int{}[unsafe.Sizeof(provider{})-64]
var _ = [1]int{}[unsafe.Sizeof(session{})-72]
var _ = [1]int{}[unsafe.Sizeof(filterEnumTemplate{})-72]

func hresult(r uintptr) error {
	if r == 0 {
		return nil
	}
	return WFPError(uint32(r))
}

// WFPError is an error code from the filtering engine.
type WFPError uint32

func (e WFPError) Error() string {
	return fmt.Sprintf("filtering engine error 0x%08x: %v", uint32(e), windows.Errno(e))
}

// NotFound reports whether err says an object isn't there.
func NotFound(err error) bool {
	var e WFPError
	if !errors.As(err, &e) {
		return false
	}
	switch uint32(e) {
	case 0x80320002, 0x80320003, 0x80320005, 0x80320007, 0x80320008:
		return true // callout, filter, provider, sublayer, layer... not found
	}
	return false
}

// Engine is an open session with the filtering engine.
type Engine struct{ h windows.Handle }

// OpenEngine opens a (non-dynamic) session: what it adds as persistent
// stays after the session and across restarts.
func OpenEngine() (*Engine, error) {
	var h windows.Handle
	s := session{txnWaitMs: 10000}
	r, _, _ := procFwpmEngineOpen0.Call(0, rpcCAuthnWinNT, 0, uintptr(unsafe.Pointer(&s)), uintptr(unsafe.Pointer(&h)))
	if err := hresult(r); err != nil {
		return nil, err
	}
	return &Engine{h: h}, nil
}

// Close ends the session.
func (e *Engine) Close() { procFwpmEngineClose0.Call(uintptr(e.h)) }

// Transaction runs fn in one transaction: all of it applies or none.
func (e *Engine) Transaction(fn func() error) error {
	if err := hresult(call(procFwpmTransactionBegin0, uintptr(e.h), 0)); err != nil {
		return err
	}
	if err := fn(); err != nil {
		procFwpmTransactionAbort0.Call(uintptr(e.h))
		return err
	}
	return hresult(call(procFwpmTransactionCommit0, uintptr(e.h)))
}

func call(p *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...)
	return r
}

// AddProvider adds a persistent provider (already there is fine).
func (e *Engine) AddProvider(key windows.GUID, name string, sd *windows.SECURITY_DESCRIPTOR) error {
	n, _ := windows.UTF16PtrFromString(name)
	p := provider{key: key, display: displayData{name: n}, flags: flagPersistent}
	err := hresult(call(procFwpmProviderAdd0, uintptr(e.h), uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(sd))))
	if isExists(err) {
		return nil
	}
	return err
}

// AddSublayer adds a persistent sublayer of provider (already there is
// fine). The highest weight is evaluated first.
func (e *Engine) AddSublayer(key, providerKey windows.GUID, name string, weight uint16, sd *windows.SECURITY_DESCRIPTOR) error {
	n, _ := windows.UTF16PtrFromString(name)
	s := sublayer{key: key, display: displayData{name: n}, flags: flagPersistent, providerKey: &providerKey, weight: weight}
	err := hresult(call(procFwpmSubLayerAdd0, uintptr(e.h), uintptr(unsafe.Pointer(&s)), uintptr(unsafe.Pointer(sd))))
	if isExists(err) {
		return nil
	}
	return err
}

// DeleteSublayer deletes a sublayer (its filters must be gone).
func (e *Engine) DeleteSublayer(key windows.GUID) error {
	err := hresult(call(procFwpmSubLayerDeleteByKey0, uintptr(e.h), uintptr(unsafe.Pointer(&key))))
	if NotFound(err) {
		return nil
	}
	return err
}

// DeleteProvider deletes a provider (its sublayers and filters must be
// gone).
func (e *Engine) DeleteProvider(key windows.GUID) error {
	err := hresult(call(procFwpmProviderDeleteByKey0, uintptr(e.h), uintptr(unsafe.Pointer(&key))))
	if NotFound(err) {
		return nil
	}
	return err
}

func isExists(err error) bool {
	var e WFPError
	return errors.As(err, &e) && uint32(e) == fwpEAlreadyExists
}

// Condition is one filter condition. Conditions on the same field are
// ORed; different fields are ANDed.
type Condition struct {
	Field windows.GUID
	// One of:
	V4                    uint32 // an IPv4 address, host order
	V6                    *[16]byte
	Port                  uint16
	FlagsAll              uint32 // the flags field has all these set
	isV4, isPort, isFlags bool
}

// V4Address matches remote IPv4 address ip (4 bytes).
func V4Address(ip [4]byte) Condition {
	return Condition{Field: FieldRemoteAddress, V4: uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3]), isV4: true}
}

// V6Address matches remote IPv6 address ip.
func V6Address(ip [16]byte) Condition {
	return Condition{Field: FieldRemoteAddress, V6: &ip}
}

// RemotePort matches the remote port.
func RemotePort(port uint16) Condition {
	return Condition{Field: FieldRemotePort, Port: port, isPort: true}
}

// Loopback matches loopback traffic.
func Loopback() Condition {
	return Condition{Field: FieldFlags, FlagsAll: FlagIsLoopback, isFlags: true}
}

// Filter is a filter to add.
type Filter struct {
	Key        windows.GUID
	Name       string
	Layer      windows.GUID
	Sublayer   windows.GUID
	Provider   windows.GUID
	Weight     uint8 // within the sublayer; higher first
	Permit     bool
	Conditions []Condition
}

// AddFilter adds a persistent filter.
func (e *Engine) AddFilter(f Filter, sd *windows.SECURITY_DESCRIPTOR) error {
	conds := make([]filterCondition, len(f.Conditions))
	var keep []any // keeps pointed-to values alive through the call
	for i, c := range f.Conditions {
		fc := filterCondition{field: c.Field, matchType: fwpMatchEqual}
		switch {
		case c.isV4:
			fc.value = conditionValue{kind: fwpUint32, value: uint64(c.V4)}
		case c.V6 != nil:
			v := c.V6
			keep = append(keep, v)
			fc.value = conditionValue{kind: fwpByteArray16, value: uint64(uintptr(unsafe.Pointer(v)))}
		case c.isPort:
			fc.value = conditionValue{kind: fwpUint16, value: uint64(c.Port)}
		case c.isFlags:
			fc.matchType = fwpMatchFlagsAll
			fc.value = conditionValue{kind: fwpUint32, value: uint64(c.FlagsAll)}
		default:
			return errors.New("empty filter condition")
		}
		conds[i] = fc
	}
	n, _ := windows.UTF16PtrFromString(f.Name)
	pk := f.Provider
	flt := filter{
		key: f.Key, display: displayData{name: n}, flags: flagPersistent, providerKey: &pk,
		layer: f.Layer, sublayer: f.Sublayer,
		weight:        fwpValue{kind: fwpUint8, value: uint64(f.Weight)},
		numConditions: uint32(len(conds)),
		action:        action{kind: actionBlock},
	}
	if f.Permit {
		flt.action.kind = actionPermit
	}
	if len(conds) > 0 {
		flt.conditions = &conds[0]
	}
	var id uint64
	r := call(procFwpmFilterAdd0, uintptr(e.h), uintptr(unsafe.Pointer(&flt)), uintptr(unsafe.Pointer(sd)), uintptr(unsafe.Pointer(&id)))
	runtime.KeepAlive(keep)
	runtime.KeepAlive(conds)
	runtime.KeepAlive(&flt)
	return hresult(r)
}

// DeleteFilter deletes a filter by key; one not there is fine.
func (e *Engine) DeleteFilter(key windows.GUID) error {
	err := hresult(call(procFwpmFilterDeleteByKey0, uintptr(e.h), uintptr(unsafe.Pointer(&key))))
	if NotFound(err) {
		return nil
	}
	return err
}

// FilterInfo is what KC9K reads back of a filter.
type FilterInfo struct {
	Key        windows.GUID
	Name       string
	Permit     bool
	Conditions int
}

// Filters lists provider's filters in layer.
func (e *Engine) Filters(providerKey, layer windows.GUID) ([]FilterInfo, error) {
	pk := providerKey
	tmpl := filterEnumTemplate{providerKey: &pk, layer: layer, enumType: 1 /* FWP_FILTER_ENUM_OVERLAPPING */, actionMask: 0xFFFFFFFF}
	var enum windows.Handle
	if err := hresult(call(procFwpmFilterCreateEnumHandle0, uintptr(e.h), uintptr(unsafe.Pointer(&tmpl)), uintptr(unsafe.Pointer(&enum)))); err != nil {
		return nil, err
	}
	defer procFwpmFilterDestroyEnumHandle0.Call(uintptr(e.h), uintptr(enum))
	var out []FilterInfo
	for {
		var entries **filter
		var n uint32
		if err := hresult(call(procFwpmFilterEnum0, uintptr(e.h), uintptr(enum), 256, uintptr(unsafe.Pointer(&entries)), uintptr(unsafe.Pointer(&n)))); err != nil {
			return nil, err
		}
		if n == 0 {
			if entries != nil {
				procFwpmFreeMemory0.Call(uintptr(unsafe.Pointer(&entries)))
			}
			return out, nil
		}
		for _, f := range unsafe.Slice(entries, n) {
			if f.providerKey == nil || *f.providerKey != providerKey {
				continue
			}
			out = append(out, FilterInfo{Key: f.key, Name: windows.UTF16PtrToString(f.display.name),
				Permit: f.action.kind == actionPermit, Conditions: int(f.numConditions)})
		}
		procFwpmFreeMemory0.Call(uintptr(unsafe.Pointer(&entries)))
	}
}
