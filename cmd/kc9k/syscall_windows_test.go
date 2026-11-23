package main

import "syscall"

func syscallN(p uintptr, args ...uintptr) (uintptr, uintptr, error) {
	r1, r2, err := syscall.SyscallN(p, args...)
	return r1, r2, err
}
