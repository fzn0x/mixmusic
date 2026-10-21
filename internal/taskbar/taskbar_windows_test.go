//go:build windows && (amd64 || arm64)

package taskbar

import (
	"image/color"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var (
	ole32               = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx  = ole32.NewProc("CoInitializeEx")
	procCoUninitialize  = ole32.NewProc("CoUninitialize")
	procGetModuleHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procPeekMessageW     = user32.NewProc("PeekMessageW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procDrawIconEx       = user32.NewProc("DrawIconEx")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

const (
	coinitApartmentThreaded = 0x2
	wsPopup                 = 0x80000000
	wsExAppWindow           = 0x00040000
	wsExNoActivate          = 0x08000000
	swShowNoActivate        = 4
	pmRemove                = 0x1
	diNormal                = 0x3
)

// wndClassEx is WNDCLASSEXW.
type wndClassEx struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background uintptr
	menuName, className                *uint16
	iconSm                             uintptr
}

// message is MSG.
type message struct {
	hwnd           uintptr
	message        uint32
	wParam, lParam uintptr
	time           uint32
	pt             struct{ x, y int32 }
	private        uint32
}

// testClass registers the class of the tests' windows, once.
var testClass = sync.OnceValue(func() *uint16 {
	name, _ := syscall.UTF16PtrFromString("MixMusicTaskbarTest")
	instance, _, _ := syscall.SyscallN(procGetModuleHandle.Addr(), 0)
	wc := wndClassEx{wndProc: procDefWindowProcW.Addr(), instance: instance, className: name}
	wc.size = uint32(unsafe.Sizeof(wc))
	syscall.SyscallN(procRegisterClassExW.Addr(), uintptr(unsafe.Pointer(&wc)))
	return name
})

// apartment has the test's thread in COM's single-threaded apartment, as a
// window's thread is, until the test ends.
func apartment(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	if r, _, _ := syscall.SyscallN(procCoInitializeEx.Addr(), 0, coinitApartmentThreaded); int32(r) < 0 {
		t.Fatalf("CoInitializeEx: %#x", r)
	}
	t.Cleanup(func() {
		syscall.SyscallN(procCoUninitialize.Addr())
		runtime.UnlockOSThread()
	})
}

// testWindow makes a window on the test's thread, which goes with the
// test. Shown, it has a taskbar button: off the screen, it takes no focus.
func testWindow(t *testing.T, shown bool) uintptr {
	t.Helper()
	title, _ := syscall.UTF16PtrFromString("MixMusic taskbar test")
	instance, _, _ := syscall.SyscallN(procGetModuleHandle.Addr(), 0)
	x := uintptr(0)
	if shown {
		x = uintptr(int32ToUintptr(-32000))
	}
	hwnd, _, err := syscall.SyscallN(procCreateWindowExW.Addr(), wsExAppWindow|wsExNoActivate, uintptr(unsafe.Pointer(testClass())),
		uintptr(unsafe.Pointer(title)), wsPopup, x, x, 200, 100, 0, 0, instance, 0)
	if hwnd == 0 {
		t.Fatalf("CreateWindowEx: %v", err)
	}
	t.Cleanup(func() { syscall.SyscallN(procDestroyWindow.Addr(), hwnd) })
	if shown {
		syscall.SyscallN(procShowWindow.Addr(), hwnd, swShowNoActivate)
	}
	return hwnd
}

func int32ToUintptr(v int32) uintptr { return uintptr(int64(v)) }

// pump handles the thread's messages until ok, or fails after 5 s.
func pump(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); {
		var m message
		for {
			r, _, _ := syscall.SyscallN(procPeekMessageW.Addr(), uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
			if r == 0 {
				break
			}
			syscall.SyscallN(procDispatchMessageW.Addr(), uintptr(unsafe.Pointer(&m)))
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// attachTo attaches a bar to hwnd, or skips the test where there's no
// taskbar, as without Explorer.
func attachTo(t *testing.T, hwnd uintptr, click func(Button)) *Bar {
	t.Helper()
	bar, err := Attach(hwnd, click)
	if err != nil && strings.Contains(err.Error(), "the shell's taskbar") {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return bar
}

// TestBar passes on the clicks of the buttons, and goes with its window.
func TestBar(t *testing.T) {
	apartment(t)
	hwnd := testWindow(t, false)
	var clicked []Button
	bar := attachTo(t, hwnd, func(b Button) { clicked = append(clicked, b) })

	// Clicks come as WM_COMMAND, with THBN_CLICKED and the button's ID.
	for _, b := range []Button{Next, PlayPause, Previous} {
		syscall.SyscallN(procSendMessageW.Addr(), hwnd, wmCommand, uintptr(thbnClicked<<16|ids[b]), 0)
	}
	// A menu's command is the window's.
	syscall.SyscallN(procSendMessageW.Addr(), hwnd, wmCommand, uintptr(ids[Next]), 0)
	if want := []Button{Next, PlayPause, Previous}; !slices.Equal(clicked, want) {
		t.Errorf("clicked %v, want %v", clicked, want)
	}

	tips := func(st state) (tips []string, flags []uint32) {
		for _, b := range bar.b.buttons(st) {
			tips = append(tips, syscall.UTF16ToString(b.tip[:]))
			flags = append(flags, b.flags)
			if b.icon == 0 {
				t.Errorf("%q has no icon", syscall.UTF16ToString(b.tip[:]))
			}
		}
		return tips, flags
	}
	if got, flags := tips(state{playing: true, track: true}); !slices.Equal(got, []string{"Previous", "Pause", "Next"}) ||
		!slices.Equal(flags, []uint32{thbfEnabled, thbfEnabled, thbfEnabled}) {
		t.Errorf("playing, the buttons are %q, %v", got, flags)
	}
	if got, flags := tips(state{}); !slices.Equal(got, []string{"Previous", "Play", "Next"}) ||
		!slices.Equal(flags, []uint32{thbfDisabled, thbfDisabled, thbfDisabled}) {
		t.Errorf("without a track, the buttons are %q, %v", got, flags)
	}

	syscall.SyscallN(procDestroyWindow.Addr(), hwnd)
	if _, ok := bars.Load(hwnd); ok {
		t.Error("the bar stays after its window")
	}
}

// TestBarOnTaskbar puts the buttons on a window's taskbar button, once the
// shell says it has one: the button shows briefly on the taskbar.
func TestBarOnTaskbar(t *testing.T) {
	if tray, _, _ := syscall.SyscallN(procFindWindowW.Addr(), uintptr(unsafe.Pointer(trayClass)), 0); tray == 0 {
		t.Skip("no taskbar: Explorer isn't running")
	}
	apartment(t)
	hwnd := testWindow(t, true)
	bar := attachTo(t, hwnd, func(Button) {})
	bar.Show(true, true)
	pump(t, "the buttons on the taskbar button", func() bool { return bar.b.created && bar.b.added })
}

var (
	procFindWindowW = user32.NewProc("FindWindowW")
	// trayClass is the class of the taskbar's window.
	trayClass, _ = syscall.UTF16PtrFromString("Shell_TrayWnd")
)

// TestIcon makes icons whose colours Windows takes as they are: drawn on
// black, an icon is as bright as its glyph is opaque.
func TestIcon(t *testing.T) {
	const size = 32
	img := glyphPlay.render(size, color.NRGBA{255, 255, 255, 255})
	icon := newIcon(img)
	if icon == 0 {
		t.Fatal("no icon")
	}
	defer syscall.SyscallN(procDestroyIcon.Addr(), icon)
	dc, _, _ := syscall.SyscallN(procCreateCompatibleDC.Addr(), 0)
	defer syscall.SyscallN(procDeleteDC.Addr(), dc)
	bi := bitmapInfo{size: 40, width: size, height: -size, planes: 1, bitCount: 32}
	var bits unsafe.Pointer
	bmp, _, _ := syscall.SyscallN(procCreateDIBSection.Addr(), dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 {
		t.Fatal("no bitmap")
	}
	defer syscall.SyscallN(procDeleteObject.Addr(), bmp)
	old, _, _ := syscall.SyscallN(procSelectObject.Addr(), dc, bmp)
	defer syscall.SyscallN(procSelectObject.Addr(), dc, old)
	if r, _, err := syscall.SyscallN(procDrawIconEx.Addr(), dc, 0, 0, icon, size, size, 0, 0, diNormal); r == 0 {
		t.Fatalf("DrawIconEx: %v", err)
	}
	px := unsafe.Slice((*byte)(bits), size*size*4)
	partly := 0
	for i := range size * size {
		a, got := int(img.Pix[4*i+3]), int(px[4*i+2])
		if d := got - a; d > 24 || d < -24 {
			t.Fatalf("pixel %d, %d: drawn %d, its alpha %d", i%size, i/size, got, a)
		}
		if a > 0 && a < 255 {
			partly++
		}
	}
	if partly == 0 {
		t.Fatal("no pixel is partly in")
	}
}
