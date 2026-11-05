// Package keypad hands a window Ctrl and the keypad's + on Windows, which
// MyGo gives its windows as no key, as it has none for it (#251): Ctrl and
// the keypad's − reach them as Ctrl and −, which zoom out, while Ctrl and
// the keypad's + couldn't zoom in.
package keypad

import "errors"

// ErrUnsupported is returned where the keypad's + needs no help, or can't
// have it: on macOS a menu's key takes it, and Linux has no hook here.
var ErrUnsupported = errors.New("keypad: not supported here")

// Watch has plus run at each press of Ctrl and the keypad's +, without Alt
// or the Windows key, while the window hwnd is the active one; plus tells
// whether it took the press, which the window then doesn't get. Watch runs
// on the window's thread, which plus runs on too; stop undoes it.
func Watch(hwnd uintptr, plus func() bool) (stop func(), err error) { return watch(hwnd, plus) }
