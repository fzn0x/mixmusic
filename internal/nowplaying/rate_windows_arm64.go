//go:build windows

package nowplaying

// setRate can't tell the controls the rate on Windows on Arm, whose
// put_PlaybackRate takes its double in a floating-point register, D0,
// which syscall.SyscallN never loads: the flyout moves the position on at
// 1×, and update shows where the track is again whenever that drifts.
func (w *winControls) setRate(float64) {}
