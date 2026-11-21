// Package winapi holds the few Win32 calls golang.org/x/sys/windows
// doesn't wrap, loaded from the system directory only.
package winapi

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32 = windows.NewLazySystemDLL("advapi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")

	procImpersonateNamedPipeClient = advapi32.NewProc("ImpersonateNamedPipeClient")
	procImpersonateLoggedOnUser    = advapi32.NewProc("ImpersonateLoggedOnUser")
	procWaitNamedPipeW             = kernel32.NewProc("WaitNamedPipeW")
	procGetLastInputInfo           = user32.NewProc("GetLastInputInfo")
)

// ImpersonateNamedPipeClient makes the calling thread act as the pipe's
// client. The caller locks the OS thread and calls RevertToSelf.
func ImpersonateNamedPipeClient(pipe windows.Handle) error {
	if r, _, err := procImpersonateNamedPipeClient.Call(uintptr(pipe)); r == 0 {
		return err
	}
	return nil
}

// ImpersonateLoggedOnUser makes the calling thread act as token's user.
func ImpersonateLoggedOnUser(token windows.Token) error {
	if r, _, err := procImpersonateLoggedOnUser.Call(uintptr(token)); r == 0 {
		return err
	}
	return nil
}

// WaitNamedPipe waits up to timeoutMs for an instance of the pipe.
func WaitNamedPipe(name string, timeoutMs uint32) error {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if r, _, err := procWaitNamedPipeW.Call(uintptr(unsafe.Pointer(p)), uintptr(timeoutMs)); r == 0 {
		return err
	}
	return nil
}

type lastInputInfo struct {
	size uint32
	time uint32
}

// IdleMillis is how long this session has gone without input, in ms.
func IdleMillis() (uint32, bool) {
	info := lastInputInfo{size: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info))); r == 0 {
		return 0, false
	}
	return uint32(windows.DurationSinceBoot().Milliseconds()) - info.time, true
}

// EnablePrivilege turns on a privilege the process token holds but has
// off (an administrator's SeRestorePrivilege, to make SYSTEM an owner).
func EnablePrivilege(name string) error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
		return err
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	if err := windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil); err != nil {
		return err
	}
	if windows.GetLastError() == windows.ERROR_NOT_ALL_ASSIGNED {
		return windows.ERROR_NOT_ALL_ASSIGNED
	}
	return nil
}
