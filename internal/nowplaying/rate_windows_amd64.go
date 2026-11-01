//go:build windows

package nowplaying

import (
	"math"
	"syscall"
	"unsafe"
)

// setRate tells the controls the rate the track plays at. put_PlaybackRate
// takes a double, which Windows on x64 passes in the second argument's
// floating-point register, XMM1: syscall.SyscallN copies the first four
// arguments there as well as to the integer ones.
func (w *winControls) setRate(rate float64) {
	syscall.SyscallN(w.smtc2.vtbl.putPlaybackRate, uintptr(unsafe.Pointer(w.smtc2)), uintptr(math.Float64bits(rate)))
}
