package keypad

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

// TestPlusPressed takes a press of the keypad's + with Ctrl, as it repeats
// too, and nothing else: its release, other keys, other hooks' codes, and
// Ctrl with Alt or the Windows key, which other shortcuts use.
func TestPlusPressed(t *testing.T) {
	const press, repeat, release = 0x004E0001, 0x404E0001, 0xC04E0001
	keys := func(down ...uintptr) func(uintptr) bool {
		return func(vk uintptr) bool {
			for _, d := range down {
				if d == vk {
					return true
				}
			}
			return false
		}
	}
	for _, c := range []struct {
		what  string
		code  int32
		vk    uintptr
		flags uintptr
		held  []uintptr
		want  bool
	}{
		{"Ctrl and +", hcAction, vkAdd, press, []uintptr{vkControl}, true},
		{"Ctrl and + held", hcAction, vkAdd, repeat, []uintptr{vkControl}, true},
		{"Ctrl, Shift and +", hcAction, vkAdd, press, []uintptr{vkControl, 0x10}, true},
		{"+ let go", hcAction, vkAdd, release, []uintptr{vkControl}, false},
		{"+ alone", hcAction, vkAdd, press, nil, false},
		{"Ctrl, Alt and +", hcAction, vkAdd, press, []uintptr{vkControl, vkMenu}, false},
		{"Ctrl, Windows and +", hcAction, vkAdd, press, []uintptr{vkControl, vkLWin}, false},
		{"Ctrl and the keypad's −", hcAction, 0x6D, press, []uintptr{vkControl}, false},
		{"a hook's other code", 3, vkAdd, press, []uintptr{vkControl}, false},
	} {
		if got := plusPressed(c.code, c.vk, c.flags, keys(c.held...)); got != c.want {
			t.Errorf("%s: %v, want %v", c.what, got, c.want)
		}
	}
}

// TestWatch hooks a window's thread, and lets go of it.
func TestWatch(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user32 := syscall.NewLazyDLL("user32.dll")
	class, _ := syscall.UTF16PtrFromString("STATIC")
	hwnd, _, errno := syscall.SyscallN(user32.NewProc("CreateWindowExW").Addr(), 0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 10, 10, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("CreateWindowEx: %v", errno)
	}
	defer syscall.SyscallN(user32.NewProc("DestroyWindow").Addr(), hwnd)
	stop, err := Watch(hwnd, func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := watchers.Load(hwnd); !ok {
		t.Error("the window isn't watched")
	}
	stop()
	stop()
	if _, ok := watchers.Load(hwnd); ok {
		t.Error("the window is still watched")
	}
	if _, err := Watch(0x7FFFFFF0, func() bool { return true }); err == nil {
		t.Error("watched no window")
	}
}
