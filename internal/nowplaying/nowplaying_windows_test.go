//go:build windows && (amd64 || arm64)

package nowplaying

// The test reads the controls back as Windows' media flyout does, through
// GlobalSystemMediaTransportControlsSessionManager, and presses their
// buttons there. Its objects are called by the slots of their methods, from
// the Windows SDK's windows.media.control.h.

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var (
	iidIGlobalSystemMediaTransportControlsSessionManagerStatics = guid{0x2050C4EE, 0x11A0, 0x57DE, [8]byte{0xAE, 0xD7, 0xC9, 0x7C, 0x70, 0x33, 0x82, 0x45}}
	iidIAsyncInfo                                               = guid{0x00000036, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
)

const (
	// AsyncStatus.
	asyncStarted   = 0
	asyncCompleted = 1
	// GlobalSystemMediaTransportControlsSessionPlaybackStatus.
	sessionClosed  = 0
	sessionPlaying = 4
	sessionPaused  = 5

	// The slots of the methods called.
	slotRequestAsync         = 6  // the manager's statics
	slotGetSessions          = 7  // the manager
	slotGetAt                = 6  // IVectorView
	slotGetSize              = 7  // IVectorView
	slotGetStatus            = 7  // IAsyncInfo
	slotGetResults           = 8  // IAsyncOperation
	slotSourceApp            = 6  // a session
	slotGetMediaProperties   = 7  // a session
	slotGetTimeline          = 8  // a session
	slotGetPlaybackInfo      = 9  // a session
	slotTryPlay              = 10 // a session
	slotTryPause             = 11 // a session
	slotTryStop              = 12 // a session
	slotTrySkipNext          = 16 // a session
	slotTrySkipPrevious      = 17 // a session
	slotTryChangePosition    = 24 // a session
	slotTitle                = 6  // media properties
	slotArtist               = 9  // media properties
	slotAlbumTitle           = 10 // media properties
	slotThumbnail            = 15 // media properties
	slotPlaybackStatus       = 7  // playback info
	slotPlaybackRate         = 10 // playback info
	slotReferenceValue       = 6  // IReference
	slotTimelineEndTime      = 7  // timeline properties
	slotTimelineMaxSeekTime  = 9  // timeline properties
	slotTimelinePosition     = 10 // timeline properties
	slotTimelineLastUpdated  = 11 // timeline properties
	maxSlot                  = 32
	sessionPropertiesTimeout = 10 * time.Second
)

// method returns the method at slot i of the object p's vtable.
func method(p unsafe.Pointer, i int) uintptr {
	return (*[maxSlot]uintptr)(*(*unsafe.Pointer)(p))[i]
}

// await waits for the IAsyncOperation op, and puts its results in *out.
func await(t *testing.T, op, out unsafe.Pointer) {
	t.Helper()
	var info unsafe.Pointer
	if err := queryInterface(op, &iidIAsyncInfo, unsafe.Pointer(&info)); err != nil {
		t.Fatal(err)
	}
	defer release(info)
	for deadline := time.Now().Add(sessionPropertiesTimeout); ; time.Sleep(10 * time.Millisecond) {
		var status int32
		r, _, _ := syscall.SyscallN(method(info, slotGetStatus), uintptr(info), uintptr(unsafe.Pointer(&status)))
		if err := check(r); err != nil {
			t.Fatal(err)
		}
		if status == asyncCompleted {
			break
		}
		if status != asyncStarted {
			t.Fatalf("the operation ended with status %d", status)
		}
		if time.Now().After(deadline) {
			t.Fatal("the operation didn't end")
		}
	}
	r, _, _ := syscall.SyscallN(method(op, slotGetResults), uintptr(op), uintptr(out))
	if err := check(r); err != nil {
		t.Fatal(err)
	}
}

// sessions returns the sessions the flyout shows.
func sessions(t *testing.T) []unsafe.Pointer {
	t.Helper()
	var statics unsafe.Pointer
	if err := activationFactory("Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager",
		&iidIGlobalSystemMediaTransportControlsSessionManagerStatics, unsafe.Pointer(&statics)); err != nil {
		t.Fatal(err)
	}
	defer release(statics)
	var op unsafe.Pointer
	r, _, _ := syscall.SyscallN(method(statics, slotRequestAsync), uintptr(statics), uintptr(unsafe.Pointer(&op)))
	if err := check(r); err != nil {
		t.Fatal(err)
	}
	defer release(op)
	var manager unsafe.Pointer
	await(t, op, unsafe.Pointer(&manager))
	defer release(manager)
	var list unsafe.Pointer
	r, _, _ = syscall.SyscallN(method(manager, slotGetSessions), uintptr(manager), uintptr(unsafe.Pointer(&list)))
	if err := check(r); err != nil {
		t.Fatal(err)
	}
	defer release(list)
	var n uint32
	syscall.SyscallN(method(list, slotGetSize), uintptr(list), uintptr(unsafe.Pointer(&n)))
	all := make([]unsafe.Pointer, n)
	for i := range n {
		syscall.SyscallN(method(list, slotGetAt), uintptr(list), uintptr(i), uintptr(unsafe.Pointer(&all[i])))
	}
	return all
}

// flyout is what the flyout shows of a session.
type flyout struct {
	title, artist, album string
	thumbnail            bool
	status               int32
	// rate is the playback rate, or 0 when there is none.
	rate             float64
	end, maxSeek, at time.Duration
	updated          time.Time
}

// read returns what the flyout shows of the session s.
func read(t *testing.T, s unsafe.Pointer) flyout {
	t.Helper()
	var f flyout
	var op unsafe.Pointer
	r, _, _ := syscall.SyscallN(method(s, slotGetMediaProperties), uintptr(s), uintptr(unsafe.Pointer(&op)))
	if err := check(r); err != nil {
		t.Fatal(err)
	}
	defer release(op)
	var props unsafe.Pointer
	await(t, op, unsafe.Pointer(&props))
	defer release(props)
	f.title, f.artist, f.album = getString(props, slotTitle), getString(props, slotArtist), getString(props, slotAlbumTitle)
	var thumbnail unsafe.Pointer
	syscall.SyscallN(method(props, slotThumbnail), uintptr(props), uintptr(unsafe.Pointer(&thumbnail)))
	f.thumbnail = thumbnail != nil
	release(thumbnail)

	var info unsafe.Pointer
	syscall.SyscallN(method(s, slotGetPlaybackInfo), uintptr(s), uintptr(unsafe.Pointer(&info)))
	if info != nil {
		syscall.SyscallN(method(info, slotPlaybackStatus), uintptr(info), uintptr(unsafe.Pointer(&f.status)))
		var rate unsafe.Pointer
		syscall.SyscallN(method(info, slotPlaybackRate), uintptr(info), uintptr(unsafe.Pointer(&rate)))
		if rate != nil {
			syscall.SyscallN(method(rate, slotReferenceValue), uintptr(rate), uintptr(unsafe.Pointer(&f.rate)))
			release(rate)
		}
		release(info)
	}
	var timeline unsafe.Pointer
	syscall.SyscallN(method(s, slotGetTimeline), uintptr(s), uintptr(unsafe.Pointer(&timeline)))
	if timeline != nil {
		var end, maxSeek, at, updated int64
		syscall.SyscallN(method(timeline, slotTimelineEndTime), uintptr(timeline), uintptr(unsafe.Pointer(&end)))
		syscall.SyscallN(method(timeline, slotTimelineMaxSeekTime), uintptr(timeline), uintptr(unsafe.Pointer(&maxSeek)))
		syscall.SyscallN(method(timeline, slotTimelinePosition), uintptr(timeline), uintptr(unsafe.Pointer(&at)))
		syscall.SyscallN(method(timeline, slotTimelineLastUpdated), uintptr(timeline), uintptr(unsafe.Pointer(&updated)))
		release(timeline)
		f.end, f.maxSeek, f.at = time.Duration(end)*tick, time.Duration(maxSeek)*tick, time.Duration(at)*tick
		// A DateTime counts ticks from 1601, 11644473600 s before 1970.
		f.updated = time.Unix(0, (updated-11644473600e7)*int64(tick))
	}
	return f
}

// getString returns the HSTRING the getter at slot i of p returns.
func getString(p unsafe.Pointer, i int) string {
	var h hstring
	r, _, _ := syscall.SyscallN(method(p, i), uintptr(p), uintptr(unsafe.Pointer(&h)))
	if check(r) != nil || h == 0 {
		return ""
	}
	defer h.free()
	var n uint32
	buf, _, _ := syscall.SyscallN(procWindowsGetStringRawBuffer.Addr(), uintptr(h), uintptr(unsafe.Pointer(&n)))
	// The buffer is the system's: reading it through a uintptr is safe.
	return syscall.UTF16ToString(unsafe.Slice(*(**uint16)(unsafe.Pointer(&buf)), n))
}

// own returns the flyout's sessions of the test's executable, whose
// controls the test made: it reads nothing of other apps' sessions.
func own(t *testing.T) []unsafe.Pointer {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var mine []unsafe.Pointer
	for _, s := range sessions(t) {
		if strings.EqualFold(getString(s, slotSourceApp), filepath.Base(exe)) {
			mine = append(mine, s)
			continue
		}
		release(s)
	}
	return mine
}

// find waits for the test's session that shows title, and returns it.
func find(t *testing.T, title string) unsafe.Pointer {
	t.Helper()
	for deadline := time.Now().Add(sessionPropertiesTimeout); ; time.Sleep(50 * time.Millisecond) {
		var found unsafe.Pointer
		for _, s := range own(t) {
			if found == nil && read(t, s).title == title {
				found = s
				continue
			}
			release(s)
		}
		if found != nil {
			return found
		}
		if time.Now().After(deadline) {
			t.Fatalf("the flyout shows no %q", title)
		}
	}
}

// try calls the session's method at slot i, which asks the controls
// something, as its button does, and waits for it.
func try(t *testing.T, s unsafe.Pointer, i int) {
	t.Helper()
	var op unsafe.Pointer
	r, _, _ := syscall.SyscallN(method(s, i), uintptr(s), uintptr(unsafe.Pointer(&op)))
	if err := check(r); err != nil {
		t.Fatal(err)
	}
	defer release(op)
	var ok byte
	await(t, op, unsafe.Pointer(&ok))
	if ok == 0 {
		t.Errorf("the session's method %d didn't reach the controls", i)
	}
}

// trySeek asks the controls to move to position, as the seek bar does.
func trySeek(t *testing.T, s unsafe.Pointer, position time.Duration) {
	t.Helper()
	var op unsafe.Pointer
	r, _, _ := syscall.SyscallN(method(s, slotTryChangePosition), uintptr(s), uintptr(position/tick), uintptr(unsafe.Pointer(&op)))
	if err := check(r); err != nil {
		t.Fatal(err)
	}
	defer release(op)
	var ok byte
	await(t, op, unsafe.Pointer(&ok))
	if ok == 0 {
		t.Error("the seek didn't reach the controls")
	}
}

func TestMediaFlyout(t *testing.T) {
	got := make(chan Request, requests)
	c, err := Open(func(r Request) { got <- r }, nil)
	if errors.Is(err, ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// The test's own calls, as the flyout's.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if r, _, _ := syscall.SyscallN(procRoInitialize.Addr(), roInitMultithreaded); check(r) != nil {
		t.Fatal(check(r))
	}
	defer syscall.SyscallN(procRoUninitialize.Addr())

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	}))
	defer srv.Close()
	client := artworkClient
	artworkClient = srv.Client()
	defer func() { artworkClient = client }()

	info := Info{
		ID:       "yt:abc",
		Title:    fmt.Sprintf("Song %d", time.Now().UnixNano()),
		Artists:  []string{"Ann", "Bo"},
		Album:    "Album",
		Artwork:  srv.URL + "/art.png",
		Duration: 200 * time.Second,
		Position: 10 * time.Second,
		At:       time.Now(),
		Playing:  true,
	}
	c.Show(&info)
	s := find(t, info.Title)
	defer release(s)
	f := read(t, s)
	if f.artist != "Ann, Bo" || f.album != "Album" {
		t.Errorf("the flyout shows %+v", f)
	}
	if f.status != sessionPlaying {
		t.Errorf("status %d, want playing", f.status)
	}
	if f.end != 200*time.Second || f.maxSeek != 200*time.Second {
		t.Errorf("the seek bar ends at %v, seeks up to %v", f.end, f.maxSeek)
	}
	// Where the track was when the flyout learned it.
	if at := f.at - f.updated.Sub(info.At); at < 9*time.Second || at > 11*time.Second {
		t.Errorf("position %v at %v, want about 10 s at %v", f.at, f.updated, info.At)
	}
	eventually(t, "the artwork", func() bool { return read(t, s).thumbnail })

	// At 2×, from 20 s: the controls are told the rate where Windows lets
	// them, on x64, and where the track is.
	faster := info
	faster.Rate, faster.Position, faster.At = 2, 20*time.Second, time.Now()
	c.Show(&faster)
	eventually(t, "the position at 2×", func() bool {
		f := read(t, s)
		return f.at >= 19*time.Second && f.at <= 22*time.Second
	})
	if f := read(t, s); runtime.GOARCH == "amd64" && f.rate != 2 {
		t.Errorf("the flyout's rate %v at 2×", f.rate)
	}
	c.Show(&info)
	eventually(t, "the position at 1×", func() bool {
		f := read(t, s)
		return f.at < 19*time.Second && (runtime.GOARCH != "amd64" || f.rate == 1)
	})

	// The buttons ask the player, which shows what it does: the flyout
	// offers only what changes it.
	press := func(slot int, want Command) {
		t.Helper()
		try(t, s, slot)
		if r := request(t, got); r.Command != want {
			t.Errorf("the session's method %d asked %+v, want %v", slot, r, want)
		}
	}
	press(slotTryPause, Pause)
	paused := info
	paused.Playing, paused.At = false, time.Now()
	c.Show(&paused)
	eventually(t, "showing paused", func() bool { return read(t, s).status == sessionPaused })
	press(slotTryPlay, Play)
	c.Show(&info)
	eventually(t, "showing playing", func() bool { return read(t, s).status == sessionPlaying })
	press(slotTrySkipNext, Next)
	press(slotTrySkipPrevious, Previous)
	press(slotTryStop, Pause)
	trySeek(t, s, 42*time.Second)
	if r := request(t, got); r.Command != Seek || r.Position != 42*time.Second {
		t.Errorf("the seek bar asked %+v", r)
	}

	c.Show(nil)
	eventually(t, "leaving the flyout", func() bool { return read(t, s).status == sessionClosed })

	// Between two tracks, the player stops one and loads the next: the
	// controls go off and on again at once, which Show might merge.
	next := info
	next.ID, next.Title, next.At = "yt:def", info.Title+" next", time.Now()
	c.sys.show(&info)
	c.sys.show(nil)
	c.sys.show(&next)
	s = find(t, next.Title)
	defer release(s)

	// Off before Close, which the app never does: controls closed while
	// on hide the next controls of the same app from the flyout a while.
	c.sys.show(nil)
	eventually(t, "leaving the flyout again", func() bool { return read(t, s).status == sessionClosed })
}
