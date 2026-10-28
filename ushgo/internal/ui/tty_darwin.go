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

// setEcho turns terminal echo on or off; it returns a function restoring the previous state.
func setEcho(f *os.File, on bool) func() {
	var t syscall.Termios
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&t))); err != 0 {
		return func() {}
	}
	saved := t
	if on {
		t.Lflag |= syscall.ECHO
	} else {
		t.Lflag &^= syscall.ECHO
	}
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCSETA, uintptr(unsafe.Pointer(&t)))
	return func() { syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCSETA, uintptr(unsafe.Pointer(&saved))) }
}
