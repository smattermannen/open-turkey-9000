package winapi

// accounts_windows.go — the computer's local accounts (netapi32): the
// Guest account for "close guest profiles" (ROADMAP W14) and accounts made
// or turned on during strict mode (W22).

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	netapi32             = windows.NewLazySystemDLL("netapi32.dll")
	procNetUserEnum      = netapi32.NewProc("NetUserEnum")
	procNetUserSetInfo   = netapi32.NewProc("NetUserSetInfo")
	procNetApiBufferFree = netapi32.NewProc("NetApiBufferFree")
)

const (
	ufAccountDisable    = 0x2
	filterNormalAccount = 0x2
)

// Account is one local account.
type Account struct {
	Name     string
	RID      uint32
	Disabled bool
}

// userInfo20 is USER_INFO_20.
type userInfo20 struct {
	name, fullName, comment *uint16
	flags, userID           uint32
}

// LocalAccounts lists the computer's local accounts (normal accounts: not
// computer or trust accounts).
func LocalAccounts() ([]Account, error) {
	var out []Account
	var resume uint32
	for {
		var buf *userInfo20
		var read, total uint32
		r, _, _ := procNetUserEnum.Call(0, 20, filterNormalAccount, uintptr(unsafe.Pointer(&buf)),
			0xFFFFFFFF, uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&resume)))
		if r != 0 && r != 234 { // ERROR_MORE_DATA
			return nil, fmt.Errorf("NetUserEnum: %w", windows.Errno(r))
		}
		if buf != nil {
			for _, u := range unsafe.Slice(buf, read) {
				out = append(out, Account{Name: windows.UTF16PtrToString(u.name), RID: u.userID, Disabled: u.flags&ufAccountDisable != 0})
			}
			procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))
		}
		if r == 0 {
			return out, nil
		}
	}
}

// SetAccountDisabled turns a local account off or on (USER_INFO_1008 keeps
// its other flags: they are read first).
func SetAccountDisabled(name string, disabled bool) error {
	accounts, err := LocalAccounts()
	if err != nil {
		return err
	}
	var flags uint32
	found := false
	for _, a := range accounts {
		if a.Name == name {
			found = true
			flags, err = accountFlags(name)
			if err != nil {
				return err
			}
		}
	}
	if !found {
		return fmt.Errorf("no local account %q", name)
	}
	if disabled {
		flags |= ufAccountDisable
	} else {
		flags &^= ufAccountDisable
	}
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	info := struct{ flags uint32 }{flags}
	var parmErr uint32
	if r, _, _ := procNetUserSetInfo.Call(0, uintptr(unsafe.Pointer(n)), 1008, uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&parmErr))); r != 0 {
		return fmt.Errorf("NetUserSetInfo %s: %w", name, windows.Errno(r))
	}
	return nil
}

// accountFlags reads an account's flags (USER_INFO_1).
func accountFlags(name string) (uint32, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	var buf *byte
	if err := windows.NetUserGetInfo(nil, n, 1, &buf); err != nil {
		return 0, fmt.Errorf("NetUserGetInfo %s: %w", name, err)
	}
	defer windows.NetApiBufferFree(buf)
	// USER_INFO_1: name, password (pointers), password_age, priv (DWORDs),
	// home_dir, comment (pointers), flags.
	return *(*uint32)(unsafe.Add(unsafe.Pointer(buf), 4*unsafe.Sizeof(uintptr(0))+8)), nil
}
