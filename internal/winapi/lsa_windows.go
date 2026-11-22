package winapi

// lsa_windows.go — user rights (the Local Security Authority's account
// rights): strict mode takes privileges from Administrators while it holds
// (ROADMAP W21). Rights apply to tokens made after the change: the next
// sign-in.

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procLsaOpenPolicy             = advapi32.NewProc("LsaOpenPolicy")
	procLsaClose                  = advapi32.NewProc("LsaClose")
	procLsaEnumerateAccountRights = advapi32.NewProc("LsaEnumerateAccountRights")
	procLsaAddAccountRights       = advapi32.NewProc("LsaAddAccountRights")
	procLsaRemoveAccountRights    = advapi32.NewProc("LsaRemoveAccountRights")
	procLsaFreeMemory             = advapi32.NewProc("LsaFreeMemory")
	procLsaNtStatusToWinError     = advapi32.NewProc("LsaNtStatusToWinError")
)

type lsaUnicodeString struct {
	Length, MaximumLength uint16
	Buffer                *uint16
}

type lsaObjectAttributes struct {
	Length                   uint32
	RootDirectory            uintptr
	ObjectName               uintptr
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

func lsaErr(what string, status uintptr) error {
	code, _, _ := procLsaNtStatusToWinError.Call(status)
	return fmt.Errorf("%s: %w", what, windows.Errno(code))
}

func lsaOpen() (uintptr, error) {
	var attrs lsaObjectAttributes
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h uintptr
	const access = 0x1 | 0x10 | 0x800 // view local information, create account, lookup names
	if r, _, _ := procLsaOpenPolicy.Call(0, uintptr(unsafe.Pointer(&attrs)), access, uintptr(unsafe.Pointer(&h))); r != 0 {
		return 0, lsaErr("LsaOpenPolicy", r)
	}
	return h, nil
}

// AccountRights are the rights (privileges' names) given to sid itself.
func AccountRights(sid *windows.SID) ([]string, error) {
	h, err := lsaOpen()
	if err != nil {
		return nil, err
	}
	defer procLsaClose.Call(h)
	var list *lsaUnicodeString
	var n uint32
	r, _, _ := procLsaEnumerateAccountRights.Call(h, uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&list)), uintptr(unsafe.Pointer(&n)))
	if r == 0xC0000034 { // STATUS_OBJECT_NAME_NOT_FOUND: no rights at all
		return nil, nil
	}
	if r != 0 {
		return nil, lsaErr("LsaEnumerateAccountRights", r)
	}
	defer procLsaFreeMemory.Call(uintptr(unsafe.Pointer(list)))
	var out []string
	for _, s := range unsafe.Slice(list, n) {
		out = append(out, windows.UTF16ToString(unsafe.Slice(s.Buffer, s.Length/2)))
	}
	return out, nil
}

func lsaStrings(rights []string) ([]lsaUnicodeString, error) {
	out := make([]lsaUnicodeString, len(rights))
	for i, r := range rights {
		p, err := windows.UTF16PtrFromString(r)
		if err != nil {
			return nil, err
		}
		out[i] = lsaUnicodeString{uint16(2 * len(r)), uint16(2*len(r) + 2), p}
	}
	return out, nil
}

// SetAccountRights adds (add) or removes rights of sid.
func SetAccountRights(sid *windows.SID, rights []string, add bool) error {
	if len(rights) == 0 {
		return nil
	}
	h, err := lsaOpen()
	if err != nil {
		return err
	}
	defer procLsaClose.Call(h)
	list, err := lsaStrings(rights)
	if err != nil {
		return err
	}
	var r uintptr
	if add {
		r, _, _ = procLsaAddAccountRights.Call(h, uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&list[0])), uintptr(len(list)))
	} else {
		r, _, _ = procLsaRemoveAccountRights.Call(h, uintptr(unsafe.Pointer(sid)), 0, uintptr(unsafe.Pointer(&list[0])), uintptr(len(list)))
	}
	if r != 0 {
		return lsaErr("account rights", r)
	}
	return nil
}
