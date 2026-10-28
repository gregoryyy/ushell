package ui

import (
	"os"
	"syscall"
	"unsafe"
)

// IsTerminal: f is a terminal (TIOCGETA succeeds).
func IsTerminal(f *os.File) bool {
	var t syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&t)))
	return err == 0
}
