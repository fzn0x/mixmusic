//go:build windows && (amd64 || arm64)

package nowplaying

// Windows shows what plays through the SystemMediaTransportControls of a
// hidden window of MixMusic's own: in the media flyout of the volume keys
// and of Quick Settings, and on the lock screen. They pass on the flyout's
// buttons and seeks, and the media keys Windows sends them. The controls
// live on a thread of their own, in COM's multithreaded apartment, which
// handles their window's messages; the system calls the delegates on
// threads of its own.

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	iidISystemMediaTransportControlsInterop            = guid{0xDDB0472D, 0xC911, 0x4A1F, [8]byte{0x86, 0xD9, 0xDC, 0x3D, 0x71, 0xA9, 0x5F, 0x5A}}
	iidISystemMediaTransportControls                   = guid{0x99FA3FF4, 0x1742, 0x42A6, [8]byte{0x90, 0x2E, 0x08, 0x7D, 0x41, 0xF9, 0x65, 0xEC}}
	iidISystemMediaTransportControls2                  = guid{0xEA98D2F6, 0x7F3C, 0x4AF2, [8]byte{0xA5, 0x86, 0x72, 0x88, 0x98, 0x08, 0xEF, 0xB1}}
	iidIMusicDisplayProperties2                        = guid{0x00368462, 0x97D3, 0x44B9, [8]byte{0xB0, 0x0F, 0x00, 0x8A, 0xFC, 0xEF, 0xAF, 0x18}}
	iidISystemMediaTransportControlsTimelineProperties = guid{0x5125316A, 0xC3A2, 0x475B, [8]byte{0x85, 0x07, 0x93, 0x53, 0x4D, 0xC8, 0x8F, 0x15}}
	iidIRandomAccessStream                             = guid{0x905A0FE1, 0xBC53, 0x11DF, [8]byte{0x8C, 0x49, 0x00, 0x1E, 0x4F, 0xC6, 0x86, 0xDA}}
	iidIRandomAccessStreamReferenceStatics             = guid{0x857309DC, 0x3FBF, 0x4E7D, [8]byte{0x98, 0x6F, 0xEF, 0x3B, 0x1A, 0x07, 0xA9, 0x64}}
	// The TypedEventHandlers of the controls' button presses and seeks.
	iidButtonPressedHandler           = guid{0x0557E996, 0x7B23, 0x5BAE, [8]byte{0xAA, 0x81, 0xEA, 0x0D, 0x67, 0x11, 0x43, 0xA4}}
	iidPositionChangeRequestedHandler = guid{0x44E34F15, 0xBDC0, 0x50A7, [8]byte{0xAC, 0xE4, 0x39, 0xE9, 0x1F, 0xB7, 0x53, 0xF1}}
)

const (
	// MediaPlaybackStatus.
	statusClosed  = 0
	statusPlaying = 3
	statusPaused  = 4
	// MediaPlaybackType.
	playbackMusic = 1
	// SystemMediaTransportControlsButton.
	buttonPlay     = 0
	buttonPause    = 1
	buttonStop     = 2
	buttonNext     = 6
	buttonPrevious = 7

	// tick is the unit of a TimeSpan.
	tick = 100 * time.Nanosecond

	wsPopup = 0x80000000
	// wmRun has the controls' thread run what waits in work.
	wmRun = 0x8000 + 1 // WM_APP + 1
)

type iSystemMediaTransportControlsInterop struct {
	vtbl *iSystemMediaTransportControlsInteropVtbl
}

type iSystemMediaTransportControlsInteropVtbl struct {
	iInspectableVtbl
	getForWindow uintptr
}

type iSystemMediaTransportControls struct {
	vtbl *iSystemMediaTransportControlsVtbl
}

type iSystemMediaTransportControlsVtbl struct {
	iInspectableVtbl
	getPlaybackStatus, putPlaybackStatus, getDisplayUpdater, getSoundLevel,
	getIsEnabled, putIsEnabled, getIsPlayEnabled, putIsPlayEnabled,
	getIsStopEnabled, putIsStopEnabled, getIsPauseEnabled, putIsPauseEnabled,
	getIsRecordEnabled, putIsRecordEnabled, getIsFastForwardEnabled, putIsFastForwardEnabled,
	getIsRewindEnabled, putIsRewindEnabled, getIsPreviousEnabled, putIsPreviousEnabled,
	getIsNextEnabled, putIsNextEnabled, getIsChannelUpEnabled, putIsChannelUpEnabled,
	getIsChannelDownEnabled, putIsChannelDownEnabled,
	addButtonPressed, removeButtonPressed uintptr
}

type iSystemMediaTransportControls2 struct {
	vtbl *iSystemMediaTransportControls2Vtbl
}

type iSystemMediaTransportControls2Vtbl struct {
	iInspectableVtbl
	getAutoRepeatMode, putAutoRepeatMode, getShuffleEnabled, putShuffleEnabled,
	getPlaybackRate, putPlaybackRate, updateTimelineProperties,
	addPlaybackPositionChangeRequested, removePlaybackPositionChangeRequested uintptr
}

type iDisplayUpdater struct{ vtbl *iDisplayUpdaterVtbl }

type iDisplayUpdaterVtbl struct {
	iInspectableVtbl
	getType, putType, getAppMediaID, putAppMediaID, getThumbnail, putThumbnail,
	getMusicProperties, getVideoProperties, getImageProperties, copyFromFileAsync,
	clearAll, update uintptr
}

type iMusicDisplayProperties struct {
	vtbl *iMusicDisplayPropertiesVtbl
}

type iMusicDisplayPropertiesVtbl struct {
	iInspectableVtbl
	getTitle, putTitle, getAlbumArtist, putAlbumArtist, getArtist, putArtist uintptr
}

type iMusicDisplayProperties2 struct {
	vtbl *iMusicDisplayProperties2Vtbl
}

type iMusicDisplayProperties2Vtbl struct {
	iInspectableVtbl
	getAlbumTitle, putAlbumTitle uintptr
}

type iTimelineProperties struct{ vtbl *iTimelinePropertiesVtbl }

type iTimelinePropertiesVtbl struct {
	iInspectableVtbl
	getStartTime, putStartTime, getEndTime, putEndTime, getMinSeekTime, putMinSeekTime,
	getMaxSeekTime, putMaxSeekTime, getPosition, putPosition uintptr
}

type iButtonPressedEventArgs struct {
	vtbl *iButtonPressedEventArgsVtbl
}

type iButtonPressedEventArgsVtbl struct {
	iInspectableVtbl
	getButton uintptr
}

type iPositionChangeRequestedEventArgs struct {
	vtbl *iPositionChangeRequestedEventArgsVtbl
}

type iPositionChangeRequestedEventArgsVtbl struct {
	iInspectableVtbl
	getRequestedPlaybackPosition uintptr
}

type iStreamReferenceStatics struct {
	vtbl *iStreamReferenceStaticsVtbl
}

type iStreamReferenceStaticsVtbl struct {
	iInspectableVtbl
	createFromFile, createFromURI, createFromStream uintptr
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procSHCreateMemStream                  = syscall.NewLazyDLL("shlwapi.dll").NewProc("SHCreateMemStream")
	procCreateRandomAccessStreamOverStream = syscall.NewLazyDLL("shcore.dll").NewProc("CreateRandomAccessStreamOverStream")
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

// windowClass registers the class of the controls' windows, once: the
// system's DefWindowProc handles their messages.
var windowClass = sync.OnceValues(func() (*uint16, error) {
	name, err := syscall.UTF16PtrFromString("MixMusicMediaControls")
	if err != nil {
		return nil, err
	}
	instance, _, _ := syscall.SyscallN(procGetModuleHandleW.Addr(), 0)
	wc := wndClassEx{wndProc: procDefWindowProcW.Addr(), instance: instance, className: name}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := syscall.SyscallN(procRegisterClassExW.Addr(), uintptr(unsafe.Pointer(&wc))); r == 0 {
		return nil, fmt.Errorf("nowplaying: the media controls' window class: %w", err)
	}
	return name, nil
})

// buttonCommands are the commands of the controls' buttons.
var buttonCommands = map[int32]Command{
	buttonPlay:  Play,
	buttonPause: Pause,
	// As the media keys' stop.
	buttonStop:     Pause,
	buttonNext:     Next,
	buttonPrevious: Previous,
}

// winControls is the media controls of a hidden window.
type winControls struct {
	ask  func(Request)
	hwnd uintptr
	// work is what the thread runs next; done is closed once it ended.
	work chan func()
	done chan struct{}

	// What follows is the thread's.
	smtc     *iSystemMediaTransportControls
	smtc2    *iSystemMediaTransportControls2
	updater  *iDisplayUpdater
	music    *iMusicDisplayProperties
	music2   *iMusicDisplayProperties2
	timeline *iTimelineProperties
	streams  *iStreamReferenceStatics
	// buttons and seeks are the delegates the controls call, once added,
	// with the tokens that remove them.
	buttons, seeks           *delegate
	buttonsToken, seeksToken int64
	// shown is what shows, as the flyout may move it on: at 1×.
	shown *Info
	// rate is the rate the controls were told last, or 0.
	rate float64
	// artwork is the address of the artwork shown, fetching that of the
	// artwork that downloads.
	artwork, fetching string
	// on is set while the controls are on. They last went off at offAt,
	// and nothing shows since cleared; settling turns them on or off
	// later.
	on             bool
	offAt, cleared time.Time
	settling       *time.Timer
}

// settleAfter is how long the controls stay on once nothing shows, and
// off once they went off: turned off and on again in quick succession, as
// when the player stops a track and loads the next, the system may lose
// them, and the next controls of MixMusic's with them.
const settleAfter = 2 * time.Second

func open(ask func(Request), _ func(func())) (platform, error) {
	if err := procRoInitialize.Find(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	w := &winControls{ask: ask, work: make(chan func(), 1), done: make(chan struct{})}
	ready := make(chan error, 1)
	go w.run(ready)
	if err := <-ready; err != nil {
		<-w.done
		return nil, err
	}
	return w, nil
}

// run is the controls' thread: it makes them, then runs what show and
// close send among the window's messages, until close ends it.
func (w *winControls) run(ready chan<- error) {
	defer close(w.done)
	// The thread stays locked: it ends with the goroutine, and COM's
	// apartment with it.
	runtime.LockOSThread()
	r, _, _ := syscall.SyscallN(procRoInitialize.Addr(), roInitMultithreaded)
	if err := check(r); err != nil {
		ready <- fmt.Errorf("nowplaying: Windows' media controls: %w", err)
		return
	}
	defer syscall.SyscallN(procRoUninitialize.Addr())
	if err := w.start(); err != nil {
		w.stop()
		ready <- err
		return
	}
	ready <- nil
	var m message
	for {
		r, _, _ := syscall.SyscallN(procGetMessageW.Addr(), uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		if m.hwnd == w.hwnd && m.message == wmRun {
			(<-w.work)()
			continue
		}
		syscall.SyscallN(procDispatchMessageW.Addr(), uintptr(unsafe.Pointer(&m)))
	}
}

// do runs fn on the controls' thread, and waits for it; once the thread
// ended, fn doesn't run.
func (w *winControls) do(fn func()) {
	done := make(chan struct{})
	select {
	case w.work <- func() { fn(); close(done) }:
	case <-w.done:
		return
	}
	if r, _, _ := syscall.SyscallN(procPostMessageW.Addr(), w.hwnd, wmRun, 0, 0); r == 0 {
		// The window is gone: the thread ends.
		<-w.done
		return
	}
	select {
	case <-done:
	case <-w.done:
	}
}

// start makes the window and its controls, which show nothing yet.
func (w *winControls) start() error {
	class, err := windowClass()
	if err != nil {
		return err
	}
	instance, _, _ := syscall.SyscallN(procGetModuleHandleW.Addr(), 0)
	title, _ := syscall.UTF16PtrFromString("MixMusic")
	hwnd, _, errno := syscall.SyscallN(procCreateWindowExW.Addr(), 0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsPopup, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("nowplaying: the media controls' window: %w", errno)
	}
	w.hwnd = hwnd
	var interop *iSystemMediaTransportControlsInterop
	if err := activationFactory("Windows.Media.SystemMediaTransportControls", &iidISystemMediaTransportControlsInterop, unsafe.Pointer(&interop)); err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	defer release(unsafe.Pointer(interop))
	r, _, _ := syscall.SyscallN(interop.vtbl.getForWindow, uintptr(unsafe.Pointer(interop)), w.hwnd,
		uintptr(unsafe.Pointer(&iidISystemMediaTransportControls)), uintptr(unsafe.Pointer(&w.smtc)))
	if err := check(r); err != nil {
		return fmt.Errorf("nowplaying: Windows' media controls: %w", err)
	}
	if err := w.listen(); err != nil {
		return fmt.Errorf("nowplaying: Windows' media controls: %w", err)
	}
	if err := w.reach(); err != nil {
		return fmt.Errorf("nowplaying: Windows' media controls: %w", err)
	}
	return nil
}

// listen turns on the buttons and the seek bar, and handles them. The
// controls themselves, off at first, go on with the first track.
func (w *winControls) listen() error {
	s := w.smtc
	for _, put := range []uintptr{s.vtbl.putIsPlayEnabled, s.vtbl.putIsPauseEnabled, s.vtbl.putIsStopEnabled,
		s.vtbl.putIsNextEnabled, s.vtbl.putIsPreviousEnabled} {
		r, _, _ := syscall.SyscallN(put, uintptr(unsafe.Pointer(s)), 1)
		if err := check(r); err != nil {
			return err
		}
	}
	buttons := newDelegate(iidButtonPressedHandler, w.buttonPressed)
	r, _, _ := syscall.SyscallN(s.vtbl.addButtonPressed, uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(buttons)),
		uintptr(unsafe.Pointer(&w.buttonsToken)))
	if err := check(r); err != nil {
		return err
	}
	w.buttons = buttons
	if err := queryInterface(unsafe.Pointer(s), &iidISystemMediaTransportControls2, unsafe.Pointer(&w.smtc2)); err != nil {
		return err
	}
	seeks := newDelegate(iidPositionChangeRequestedHandler, w.seekRequested)
	r, _, _ = syscall.SyscallN(w.smtc2.vtbl.addPlaybackPositionChangeRequested, uintptr(unsafe.Pointer(w.smtc2)),
		uintptr(unsafe.Pointer(seeks)), uintptr(unsafe.Pointer(&w.seeksToken)))
	if err := check(r); err != nil {
		return err
	}
	w.seeks = seeks
	return activate("Windows.Media.SystemMediaTransportControlsTimelineProperties",
		&iidISystemMediaTransportControlsTimelineProperties, unsafe.Pointer(&w.timeline))
}

// reach gets what shows the track: the display updater, its music
// properties, and the factory of the artwork's reference.
func (w *winControls) reach() error {
	r, _, _ := syscall.SyscallN(w.smtc.vtbl.getDisplayUpdater, uintptr(unsafe.Pointer(w.smtc)), uintptr(unsafe.Pointer(&w.updater)))
	if err := check(r); err != nil {
		return err
	}
	r, _, _ = syscall.SyscallN(w.updater.vtbl.putType, uintptr(unsafe.Pointer(w.updater)), playbackMusic)
	if err := check(r); err != nil {
		return err
	}
	r, _, _ = syscall.SyscallN(w.updater.vtbl.getMusicProperties, uintptr(unsafe.Pointer(w.updater)), uintptr(unsafe.Pointer(&w.music)))
	if err := check(r); err != nil {
		return err
	}
	if err := queryInterface(unsafe.Pointer(w.music), &iidIMusicDisplayProperties2, unsafe.Pointer(&w.music2)); err != nil {
		return err
	}
	return activationFactory("Windows.Storage.Streams.RandomAccessStreamReference", &iidIRandomAccessStreamReferenceStatics, unsafe.Pointer(&w.streams))
}

// stop takes the delegates out, lets go of the controls and destroys the
// window, on the thread.
func (w *winControls) stop() {
	if w.buttons != nil {
		syscall.SyscallN(w.smtc.vtbl.removeButtonPressed, uintptr(unsafe.Pointer(w.smtc)), uintptr(w.buttonsToken))
	}
	if w.seeks != nil {
		syscall.SyscallN(w.smtc2.vtbl.removePlaybackPositionChangeRequested, uintptr(unsafe.Pointer(w.smtc2)), uintptr(w.seeksToken))
	}
	for _, p := range []unsafe.Pointer{unsafe.Pointer(w.streams), unsafe.Pointer(w.timeline), unsafe.Pointer(w.music2),
		unsafe.Pointer(w.music), unsafe.Pointer(w.updater), unsafe.Pointer(w.smtc2), unsafe.Pointer(w.smtc)} {
		release(p)
	}
	w.streams, w.timeline, w.music2, w.music, w.updater, w.smtc2, w.smtc = nil, nil, nil, nil, nil, nil, nil
	if w.hwnd != 0 {
		syscall.SyscallN(procDestroyWindow.Addr(), w.hwnd)
	}
}

func (w *winControls) close() {
	w.do(func() {
		// Controls let go of while on, the system goes on showing.
		if w.shown != nil {
			w.clear()
		}
		if w.settling != nil {
			w.settling.Stop()
		}
		if w.on {
			syscall.SyscallN(w.smtc.vtbl.putIsEnabled, uintptr(unsafe.Pointer(w.smtc)), 0)
		}
		w.stop()
		syscall.SyscallN(procPostQuitMessage.Addr(), 0)
	})
	<-w.done
}

func (w *winControls) show(next *Info) { w.do(func() { w.update(next) }) }

// update shows next, or nothing for nil, on the thread.
func (w *winControls) update(next *Info) {
	// The controls are told the rate where they can be, but whether the
	// flyout moves the position on at it, rather than at 1×, it doesn't
	// say: where the track is shows again whenever it drifts from 1×.
	var shown *Info
	if next != nil {
		at1 := *next
		at1.Rate = 0
		shown = &at1
	}
	c := diff(w.shown, shown)
	rate := next != nil && next.rate() != w.rate
	if next != nil && w.shown != nil && !c.track && !c.playing && !c.seeked && !rate {
		// The flyout has no volume, and moves the position on itself.
		return
	}
	w.shown = shown
	defer w.settle()
	if next == nil {
		w.clear()
		w.cleared, w.rate = time.Now(), 0
		return
	}
	if c.track {
		w.describe(next)
	}
	if rate {
		w.setRate(next.rate())
		w.rate = next.rate()
	}
	status := uintptr(statusPaused)
	if next.Playing {
		status = statusPlaying
	}
	syscall.SyscallN(w.smtc.vtbl.putPlaybackStatus, uintptr(unsafe.Pointer(w.smtc)), status)
	w.place(next)
}

// clear shows nothing: closed, the controls play nothing.
func (w *winControls) clear() {
	syscall.SyscallN(w.smtc.vtbl.putPlaybackStatus, uintptr(unsafe.Pointer(w.smtc)), statusClosed)
	syscall.SyscallN(w.updater.vtbl.clearAll, uintptr(unsafe.Pointer(w.updater)))
	syscall.SyscallN(w.updater.vtbl.update, uintptr(unsafe.Pointer(w.updater)))
	w.artwork = ""
}

// settle turns the controls on while something shows, and off, out of the
// flyout, while nothing does: off settleAfter after nothing showed, and on
// again no sooner than settleAfter after they went off.
func (w *winControls) settle() {
	if w.settling != nil {
		w.settling.Stop()
		w.settling = nil
	}
	on := w.shown != nil
	if on == w.on {
		return
	}
	since := w.cleared
	if on {
		since = w.offAt
	}
	if wait := settleAfter - time.Since(since); wait > 0 {
		var t *time.Timer
		t = time.AfterFunc(wait, func() {
			w.do(func() {
				if w.settling == t {
					w.settling = nil
					w.settle()
				}
			})
		})
		w.settling = t
		return
	}
	v := uintptr(0)
	if on {
		v = 1
	}
	syscall.SyscallN(w.smtc.vtbl.putIsEnabled, uintptr(unsafe.Pointer(w.smtc)), v)
	w.on = on
	if !on {
		w.offAt = time.Now()
	}
}

// describe shows the track of info: its title, artists and album, and its
// artwork once it downloaded.
func (w *winControls) describe(info *Info) {
	set := func(fn uintptr, this unsafe.Pointer, s string) {
		h, err := newHString(s)
		if err != nil {
			return
		}
		defer h.free()
		syscall.SyscallN(fn, uintptr(this), uintptr(h))
	}
	// ClearAll forgets the type.
	syscall.SyscallN(w.updater.vtbl.putType, uintptr(unsafe.Pointer(w.updater)), playbackMusic)
	set(w.music.vtbl.putTitle, unsafe.Pointer(w.music), info.Title)
	set(w.music.vtbl.putArtist, unsafe.Pointer(w.music), strings.Join(info.Artists, ", "))
	set(w.music2.vtbl.putAlbumTitle, unsafe.Pointer(w.music2), info.Album)
	if info.Artwork != w.artwork {
		w.artwork = ""
		syscall.SyscallN(w.updater.vtbl.putThumbnail, uintptr(unsafe.Pointer(w.updater)), 0)
		if info.Artwork != "" && info.Artwork != w.fetching {
			w.fetching = info.Artwork
			go w.fetchArtwork(info.Artwork)
		}
	}
	syscall.SyscallN(w.updater.vtbl.update, uintptr(unsafe.Pointer(w.updater)))
}

// fetchArtwork downloads the artwork at rawURL, and shows it if it's still
// the artwork of the track that shows.
func (w *winControls) fetchArtwork(rawURL string) {
	data, err := download(rawURL)
	w.do(func() {
		if w.fetching == rawURL {
			w.fetching = ""
		}
		if err != nil || len(data) == 0 || w.smtc == nil || w.shown == nil || w.shown.Artwork != rawURL {
			return
		}
		ref := w.thumbnail(data)
		if ref == nil {
			return
		}
		syscall.SyscallN(w.updater.vtbl.putThumbnail, uintptr(unsafe.Pointer(w.updater)), uintptr(unsafe.Pointer(ref)))
		release(unsafe.Pointer(ref))
		syscall.SyscallN(w.updater.vtbl.update, uintptr(unsafe.Pointer(w.updater)))
		w.artwork = rawURL
	})
}

// thumbnail returns a reference to a stream of the image data, or nil.
func (w *winControls) thumbnail(data []byte) *iInspectable {
	mem, _, _ := syscall.SyscallN(procSHCreateMemStream.Addr(), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	if mem == 0 {
		return nil
	}
	// The stream, which holds a copy of data, is the system's: reaching it
	// through a uintptr is safe.
	stream := *(*unsafe.Pointer)(unsafe.Pointer(&mem))
	var random *iInspectable
	r, _, _ := syscall.SyscallN(procCreateRandomAccessStreamOverStream.Addr(), uintptr(stream), 0,
		uintptr(unsafe.Pointer(&iidIRandomAccessStream)), uintptr(unsafe.Pointer(&random)))
	release(stream)
	if check(r) != nil {
		return nil
	}
	defer release(unsafe.Pointer(random))
	var ref *iInspectable
	r, _, _ = syscall.SyscallN(w.streams.vtbl.createFromStream, uintptr(unsafe.Pointer(w.streams)), uintptr(unsafe.Pointer(random)),
		uintptr(unsafe.Pointer(&ref)))
	if check(r) != nil {
		return nil
	}
	return ref
}

// place shows where the track of info is, and how long it is, on the seek
// bar.
func (w *winControls) place(info *Info) {
	t := w.timeline
	end := uintptr(max(info.Duration, 0) / tick)
	for _, v := range []struct{ put, value uintptr }{
		{t.vtbl.putStartTime, 0},
		{t.vtbl.putMinSeekTime, 0},
		{t.vtbl.putEndTime, end},
		{t.vtbl.putMaxSeekTime, end},
		{t.vtbl.putPosition, uintptr(info.position(time.Now()) / tick)},
	} {
		syscall.SyscallN(v.put, uintptr(unsafe.Pointer(t)), v.value)
	}
	syscall.SyscallN(w.smtc2.vtbl.updateTimelineProperties, uintptr(unsafe.Pointer(w.smtc2)), uintptr(unsafe.Pointer(t)))
}

// buttonPressed passes on a button the controls' user pressed.
func (w *winControls) buttonPressed(args unsafe.Pointer) {
	var b int32
	r, _, _ := syscall.SyscallN((*iButtonPressedEventArgs)(args).vtbl.getButton, uintptr(args), uintptr(unsafe.Pointer(&b)))
	if c, ok := buttonCommands[b]; ok && check(r) == nil {
		w.ask(Request{Command: c})
	}
}

// seekRequested passes on a seek the controls' user asked.
func (w *winControls) seekRequested(args unsafe.Pointer) {
	var ticks int64
	r, _, _ := syscall.SyscallN((*iPositionChangeRequestedEventArgs)(args).vtbl.getRequestedPlaybackPosition, uintptr(args),
		uintptr(unsafe.Pointer(&ticks)))
	if check(r) == nil && ticks >= 0 {
		w.ask(Request{Command: Seek, Position: time.Duration(ticks) * tick})
	}
}
