//go:build windows && (amd64 || arm64)

package taskbar

// Windows puts the buttons on the thumbnail through the shell's
// ITaskbarList3, once the window has its taskbar button: the shell sends
// the window TaskbarButtonCreated then, and again when the button comes
// back, as when the window shows after it hid in the tray, or when Explorer
// restarts. The buttons' clicks come to the window as WM_COMMAND, which a
// window procedure of the bar's own, put in front of the window's, takes.
// All of it runs on the window's thread, which Show reaches with a message
// of the bar's own.

import (
	"fmt"
	"image"
	"image/color"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

var (
	clsidTaskbarList = guid{0x56FDF344, 0xFD6D, 0x11D0, [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidITaskbarList3 = guid{0xEA1AFB91, 0x9E28, 0x4B86, [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEF, 0xAF}}
)

const (
	clsctxInprocServer = 0x1

	// The methods of ITaskbarList3 called, by their slot in its vtable
	// (ShObjIdl_core.h).
	slotRelease               = 2
	slotHrInit                = 3
	slotThumbBarAddButtons    = 15
	slotThumbBarUpdateButtons = 16

	// THUMBBUTTON's mask and flags, and the notification of a click.
	thbIcon      = 0x2
	thbTooltip   = 0x4
	thbFlags     = 0x8
	thbfEnabled  = 0x0
	thbfDisabled = 0x1
	thbnClicked  = 0x1800

	wmCommand       = 0x0111
	wmSettingChange = 0x001A
	wmDPIChanged    = 0x02E0
	wmNCDestroy     = 0x0082
	// gwlpWndProc is GWLP_WNDPROC, -4.
	gwlpWndProc = ^uintptr(3)
	smCxSmIcon  = 49
	msgfltAllow = 1
)

var (
	procCoCreateInstance = syscall.NewLazyDLL("ole32.dll").NewProc("CoCreateInstance")

	user32                     = syscall.NewLazyDLL("user32.dll")
	procGetWindowLongPtrW      = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW      = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW        = user32.NewProc("CallWindowProcW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	procDestroyIcon            = user32.NewProc("DestroyIcon")
	procGetDpiForWindow        = user32.NewProc("GetDpiForWindow")
	procGetSystemMetricsForDpi = user32.NewProc("GetSystemMetricsForDpi")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")

	procChangeWindowMessageFilterEx = user32.NewProc("ChangeWindowMessageFilterEx")

	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap     = gdi32.NewProc("CreateBitmap")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
)

// thumbButton is THUMBBUTTON, which the SDK packs on 8 bytes, as Go lays it
// out: 552 bytes.
type thumbButton struct {
	mask, id, bitmap uint32
	icon             uintptr
	tip              [260]uint16
	flags            uint32
}

// bitmapInfo is BITMAPINFO, of a 32-bit bitmap.
type bitmapInfo struct {
	size                         uint32
	width, height                int32
	planes, bitCount             uint16
	compression, sizeImage       uint32
	xPelsPerMeter, yPelsPerMeter int32
	clrUsed, clrImportant        uint32
	colors                       [1]uint32
}

// iconInfo is ICONINFO.
type iconInfo struct {
	icon               int32
	xHotspot, yHotspot uint32
	mask, color        uintptr
}

// ids are the command IDs of the buttons, by Button.
var ids = [...]uint32{Previous: 1, PlayPause: 2, Next: 3}

// The icons of the buttons, by their index in a bar's icons.
const (
	iconPrevious = iota
	iconPlay
	iconPause
	iconNext
)

var glyphs = [...]glyph{iconPrevious: glyphPrevious, iconPlay: glyphPlay, iconPause: glyphPause, iconNext: glyphNext}

// bar is the buttons of a window's thumbnail. What follows mu is any
// goroutine's; the rest is the window thread's.
type bar struct {
	hwnd    uintptr
	click   func(Button)
	list    unsafe.Pointer // the shell's ITaskbarList3
	oldProc uintptr
	// added is set once the shell took the buttons, which it does even
	// before the window has its taskbar button, and keeps nowhere then;
	// created once the shell said the window has it.
	added, created bool
	// icons are made for the taskbar's theme, light or dark, and size.
	icons [len(glyphs)]uintptr
	light bool
	size  int32

	mu   sync.Mutex
	want state
}

var (
	// bars are the bars by their window, which barProc finds them in.
	bars sync.Map
	// barWndProc is barProc as a callback, which Go makes a limited number
	// of: once.
	barWndProc = sync.OnceValue(func() uintptr { return syscall.NewCallback(barProc) })
	// msgButtonCreated is the shell's message telling a window that its
	// taskbar button exists, msgShow the bars' own, which has a bar show
	// what Show asked.
	msgButtonCreated = sync.OnceValue(func() uint32 { return registerMessage("TaskbarButtonCreated") })
	msgShow          = sync.OnceValue(func() uint32 { return registerMessage("MixMusicTaskbarButtons") })
)

func registerMessage(name string) uint32 {
	p, _ := syscall.UTF16PtrFromString(name)
	r, _, _ := syscall.SyscallN(procRegisterWindowMessageW.Addr(), uintptr(unsafe.Pointer(p)))
	return uint32(r)
}

// check turns an HRESULT into an error.
func check(r uintptr) error {
	if int32(r) < 0 {
		return fmt.Errorf("taskbar: Windows: error %#08x", uint32(r))
	}
	return nil
}

// method returns the method at slot i of the shell's taskbar list.
func (b *bar) method(i int) uintptr {
	return (*[slotThumbBarUpdateButtons + 1]uintptr)(*(*unsafe.Pointer)(b.list))[i]
}

func attach(hwnd uintptr, click func(Button)) (*bar, error) {
	if err := procCoCreateInstance.Find(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	b := &bar{hwnd: hwnd, click: click}
	r, _, _ := syscall.SyscallN(procCoCreateInstance.Addr(), uintptr(unsafe.Pointer(&clsidTaskbarList)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidITaskbarList3)), uintptr(unsafe.Pointer(&b.list)))
	if err := check(r); err != nil {
		return nil, fmt.Errorf("taskbar: the shell's taskbar: %w", err)
	}
	if r, _, _ := syscall.SyscallN(b.method(slotHrInit), uintptr(b.list)); check(r) != nil {
		b.free()
		return nil, fmt.Errorf("taskbar: the shell's taskbar: %w", check(r))
	}
	b.makeIcons()
	// Run as an administrator, the window gets nothing from Explorer, which
	// runs as the user, but what it lets through: the news of its taskbar
	// button, and the buttons' clicks.
	if procChangeWindowMessageFilterEx.Find() == nil {
		for _, m := range []uint32{msgButtonCreated(), wmCommand} {
			syscall.SyscallN(procChangeWindowMessageFilterEx.Addr(), hwnd, uintptr(m), msgfltAllow, 0)
		}
	}
	b.oldProc, _, _ = syscall.SyscallN(procGetWindowLongPtrW.Addr(), hwnd, gwlpWndProc)
	bars.Store(hwnd, b)
	if r, _, errno := syscall.SyscallN(procSetWindowLongPtrW.Addr(), hwnd, gwlpWndProc, barWndProc()); r == 0 {
		bars.Delete(hwnd)
		b.free()
		return nil, fmt.Errorf("taskbar: the window's procedure: %w", errno)
	}
	// The window has its taskbar button already, unless it hides: then
	// the buttons come with TaskbarButtonCreated.
	b.apply()
	return b, nil
}

// barProc is the window procedure in front of the windows' own, on their
// thread.
func barProc(hwnd, msg, wp, lp uintptr) uintptr {
	v, ok := bars.Load(hwnd)
	if !ok {
		r, _, _ := syscall.SyscallN(procDefWindowProcW.Addr(), hwnd, msg, wp, lp)
		return r
	}
	b := v.(*bar)
	switch m := uint32(msg); {
	case m == wmCommand && wp>>16&0xFFFF == thbnClicked:
		for btn, id := range ids {
			if uint32(wp&0xFFFF) == id {
				b.click(Button(btn))
				return 0
			}
		}
	case m == msgShow():
		b.apply()
		return 0
	case m == msgButtonCreated():
		// A new taskbar button, without the buttons.
		b.added, b.created = false, true
		b.apply()
	case m == wmSettingChange || m == wmDPIChanged:
		if old, ok := b.makeIcons(); ok {
			b.apply()
			destroyIcons(old)
		}
	case m == wmNCDestroy:
		syscall.SyscallN(procSetWindowLongPtrW.Addr(), hwnd, gwlpWndProc, b.oldProc)
		bars.Delete(hwnd)
		b.free()
	}
	r, _, _ := syscall.SyscallN(procCallWindowProcW.Addr(), b.oldProc, hwnd, msg, wp, lp)
	return r
}

// show has the window's thread show st.
func (b *bar) show(st state) {
	b.mu.Lock()
	changed := b.want != st
	b.want = st
	b.mu.Unlock()
	if changed {
		syscall.SyscallN(procPostMessageW.Addr(), b.hwnd, uintptr(msgShow()), 0, 0)
	}
}

// apply puts the buttons on the thumbnail as want asks: added the first
// time, updated after. Without a taskbar button, both fail, until
// TaskbarButtonCreated.
func (b *bar) apply() {
	b.mu.Lock()
	st := b.want
	b.mu.Unlock()
	buttons := b.buttons(st)
	if !b.added {
		r, _, _ := syscall.SyscallN(b.method(slotThumbBarAddButtons), uintptr(b.list), b.hwnd, uintptr(len(buttons)),
			uintptr(unsafe.Pointer(&buttons[0])))
		if check(r) == nil {
			b.added = true
			return
		}
	}
	// Added already, as before the bar knew of it: a button takes them
	// once.
	r, _, _ := syscall.SyscallN(b.method(slotThumbBarUpdateButtons), uintptr(b.list), b.hwnd, uintptr(len(buttons)),
		uintptr(unsafe.Pointer(&buttons[0])))
	if check(r) == nil {
		b.added = true
	}
}

// buttons are the buttons that show st.
func (b *bar) buttons(st state) [len(ids)]thumbButton {
	flags := uint32(thbfEnabled)
	if !st.track {
		flags = thbfDisabled
	}
	toggle, tip := b.icons[iconPlay], "Play"
	if st.playing {
		toggle, tip = b.icons[iconPause], "Pause"
	}
	return [len(ids)]thumbButton{
		Previous:  newButton(ids[Previous], b.icons[iconPrevious], "Previous", flags),
		PlayPause: newButton(ids[PlayPause], toggle, tip, flags),
		Next:      newButton(ids[Next], b.icons[iconNext], "Next", flags),
	}
}

func newButton(id uint32, icon uintptr, tip string, flags uint32) thumbButton {
	t := thumbButton{mask: thbIcon | thbTooltip | thbFlags, id: id, icon: icon, flags: flags}
	copy(t.tip[:len(t.tip)-1], utf16.Encode([]rune(tip)))
	return t
}

// makeIcons makes the icons for the taskbar's theme and size, unless they
// are made for them, and returns the icons they replace, which go once the
// buttons let go of them.
func (b *bar) makeIcons() (old [len(glyphs)]uintptr, changed bool) {
	light, size := taskbarLight(), b.iconSize()
	if b.icons[0] != 0 && light == b.light && size == b.size {
		return old, false
	}
	// The thumbnails are dark or light as the taskbar is.
	ink := color.NRGBA{255, 255, 255, 255}
	if light {
		ink = color.NRGBA{0x1f, 0x1f, 0x1f, 255}
	}
	old = b.icons
	for i, g := range glyphs {
		b.icons[i] = newIcon(g.render(int(size), ink))
	}
	b.light, b.size = light, size
	return old, true
}

// iconSize is the size of a small icon at the window's DPI.
func (b *bar) iconSize() int32 {
	if procGetDpiForWindow.Find() == nil && procGetSystemMetricsForDpi.Find() == nil {
		if dpi, _, _ := syscall.SyscallN(procGetDpiForWindow.Addr(), b.hwnd); dpi != 0 {
			if s, _, _ := syscall.SyscallN(procGetSystemMetricsForDpi.Addr(), smCxSmIcon, dpi); int32(s) > 0 {
				return int32(s)
			}
		}
	}
	if s, _, _ := syscall.SyscallN(procGetSystemMetrics.Addr(), smCxSmIcon); int32(s) > 0 {
		return int32(s)
	}
	return 16
}

// free lets go of the icons and the shell's taskbar list.
func (b *bar) free() {
	destroyIcons(b.icons)
	b.icons = [len(glyphs)]uintptr{}
	if b.list != nil {
		syscall.SyscallN(b.method(slotRelease), uintptr(b.list))
		b.list = nil
	}
}

func destroyIcons(icons [len(glyphs)]uintptr) {
	for _, h := range icons {
		if h != 0 {
			syscall.SyscallN(procDestroyIcon.Addr(), h)
		}
	}
}

// taskbarLight tells whether the taskbar is light, and the thumbnails over
// it: Windows' theme of its own parts, apart from the apps'.
func taskbarLight() bool {
	path, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_READ, &key) != nil {
		return false
	}
	defer syscall.RegCloseKey(key)
	name, _ := syscall.UTF16PtrFromString("SystemUsesLightTheme")
	var v, typ uint32
	n := uint32(unsafe.Sizeof(v))
	if syscall.RegQueryValueEx(key, name, nil, &typ, (*byte)(unsafe.Pointer(&v)), &n) != nil || typ != syscall.REG_DWORD {
		return false
	}
	return v != 0
}

// newIcon makes an icon of img, with its alpha, or returns 0.
func newIcon(img *image.NRGBA) uintptr {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	// Top-down: a negative height.
	bi := bitmapInfo{size: 40, width: int32(w), height: -int32(h), planes: 1, bitCount: 32}
	var bits unsafe.Pointer
	colour, _, _ := syscall.SyscallN(procCreateDIBSection.Addr(), 0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if colour == 0 {
		return 0
	}
	defer syscall.SyscallN(procDeleteObject.Addr(), colour)
	// Icons take their colours unpremultiplied, as BGRA.
	px := unsafe.Slice((*byte)(bits), w*h*4)
	for i := range w * h {
		px[4*i], px[4*i+1], px[4*i+2], px[4*i+3] = img.Pix[4*i+2], img.Pix[4*i+1], img.Pix[4*i], img.Pix[4*i+3]
	}
	mask, _, _ := syscall.SyscallN(procCreateBitmap.Addr(), uintptr(w), uintptr(h), 1, 1, 0)
	if mask == 0 {
		return 0
	}
	defer syscall.SyscallN(procDeleteObject.Addr(), mask)
	ii := iconInfo{icon: 1, mask: mask, color: colour}
	icon, _, _ := syscall.SyscallN(procCreateIconIndirect.Addr(), uintptr(unsafe.Pointer(&ii)))
	return icon
}
