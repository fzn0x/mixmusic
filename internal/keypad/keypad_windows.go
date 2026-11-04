//go:build windows

package keypad

// A keyboard hook of the window's thread alone, WH_KEYBOARD, sees each key
// message the thread takes from its queue before the window does, and may
// take it instead: the keys of other programs never come by.

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
)

const (
	whKeyboard = 2
	hcAction   = 0

	vkAdd     = 0x6B
	vkControl = 0x11
	vkMenu    = 0x12
	vkLWin    = 0x5B
	vkRWin    = 0x5C
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procSetWindowsHookExW        = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx           = user32.NewProc("CallNextHookEx")
	procGetKeyState              = user32.NewProc("GetKeyState")
	procGetActiveWindow          = user32.NewProc("GetActiveWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

var (
	// watchers are what runs at a press, by the window it is for.
	watchers sync.Map
	// hookProc is keyboardProc as a callback, which Go makes a limited
	// number of: once.
	hookProc = sync.OnceValue(func() uintptr { return syscall.NewCallback(keyboardProc) })
)

func watch(hwnd uintptr, plus func() bool) (func(), error) {
	thread, _, _ := syscall.SyscallN(procGetWindowThreadProcessId.Addr(), hwnd, 0)
	if thread == 0 {
		return nil, errors.New("keypad: no such window")
	}
	watchers.Store(hwnd, plus)
	hook, _, errno := syscall.SyscallN(procSetWindowsHookExW.Addr(), whKeyboard, hookProc(), 0, thread)
	if hook == 0 {
		watchers.Delete(hwnd)
		return nil, fmt.Errorf("keypad: the keyboard hook: %w", errno)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			syscall.SyscallN(procUnhookWindowsHookEx.Addr(), hook)
			watchers.Delete(hwnd)
		})
	}, nil
}

// keyboardProc is the hook's procedure, on the window's thread.
func keyboardProc(code, wp, lp uintptr) uintptr {
	if plusPressed(int32(code), wp, lp, held) {
		active, _, _ := syscall.SyscallN(procGetActiveWindow.Addr())
		if plus, ok := watchers.Load(active); ok && plus.(func() bool)() {
			// Taken: the window gets neither the press nor its character.
			return 1
		}
	}
	r, _, _ := syscall.SyscallN(procCallNextHookEx.Addr(), 0, code, wp, lp)
	return r
}

// plusPressed tells whether a call of the hook, with its code, virtual key
// and flags, is a press of the keypad's + with Ctrl held, without Alt or
// the Windows key: of a key down, held as it repeats too, whose release
// sets bit 31 of the flags.
func plusPressed(code int32, vk, flags uintptr, held func(vk uintptr) bool) bool {
	return code == hcAction && vk == vkAdd && flags&(1<<31) == 0 &&
		held(vkControl) && !held(vkMenu) && !held(vkLWin) && !held(vkRWin)
}

// held tells whether the window's thread saw the key vk go down, and not
// up since: as the key message the hook sees left it.
func held(vk uintptr) bool {
	state, _, _ := syscall.SyscallN(procGetKeyState.Addr(), vk)
	return int16(state) < 0
}
