//go:build darwin && (amd64 || arm64)

package nowplaying

// macOS shows what plays through MPNowPlayingInfoCenter, in Control Center
// and on the lock screen, and passes on what Control Center, the media keys
// and headphones ask through MPRemoteCommandCenter. MediaPlayer is driven
// through the Objective-C runtime with purego, without cgo, on the main
// thread, which also calls the commands' handlers.

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	mpPlaybackStatePlaying = 1
	mpPlaybackStatePaused  = 2
	mpPlaybackStateStopped = 3
	// MPRemoteCommandHandlerStatusSuccess and …CommandFailed.
	mpCommandSuccess = 0
	mpCommandFailed  = 200
	// MPNowPlayingInfoMediaTypeAudio.
	mpMediaTypeAudio     = 1
	nsUTF8StringEncoding = 4
)

// cgSize is CGSize.
type cgSize struct{ width, height float64 }

var (
	mpOnce sync.Once
	mpErr  error

	poolPush func() uintptr
	poolPop  func(pool uintptr)

	// msgSend is objc_msgSend, of which these are typed variants.
	msgSend      uintptr
	msgID        func(obj objc.ID, sel objc.SEL) objc.ID
	msgIDID      func(obj objc.ID, sel objc.SEL, a objc.ID) objc.ID
	msgSetID     func(obj objc.ID, sel objc.SEL, a objc.ID)
	msgSetIDID   func(obj objc.ID, sel objc.SEL, a, b objc.ID)
	msgSetUint   func(obj objc.ID, sel objc.SEL, v uint)
	msgUintID    func(obj objc.ID, sel objc.SEL, v uint) objc.ID
	msgFloatID   func(obj objc.ID, sel objc.SEL, v float64) objc.ID
	msgFloat     func(obj objc.ID, sel objc.SEL) float64
	msgBytesID   func(obj objc.ID, sel objc.SEL, p unsafe.Pointer, n uint, encoding uint) objc.ID
	msgDataID    func(obj objc.ID, sel objc.SEL, p unsafe.Pointer, n uint) objc.ID
	msgSize      func(obj objc.ID, sel objc.SEL) cgSize
	msgArtworkID func(obj objc.ID, sel objc.SEL, size cgSize, handler objc.Block) objc.ID
	msgBlockID   func(obj objc.ID, sel objc.SEL, handler objc.Block) objc.ID

	// The keys of the now playing info, NSStrings.
	keyTitle, keyArtist, keyAlbum, keyDuration, keyArtwork,
	keyElapsed, keyRate, keyDefaultRate, keyMediaType objc.ID

	selMu    sync.Mutex
	selCache = map[string]objc.SEL{}
)

func loadMediaPlayer() error {
	mpOnce.Do(func() {
		for _, framework := range []string{"Foundation", "AppKit"} {
			if _, err := purego.Dlopen("/System/Library/Frameworks/"+framework+".framework/"+framework, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
				mpErr = fmt.Errorf("%w: %s: %w", ErrUnsupported, framework, err)
				return
			}
		}
		mp, err := purego.Dlopen("/System/Library/Frameworks/MediaPlayer.framework/MediaPlayer", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			mpErr = fmt.Errorf("%w: MediaPlayer: %w", ErrUnsupported, err)
			return
		}
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			mpErr = fmt.Errorf("%w: libobjc: %w", ErrUnsupported, err)
			return
		}
		if msgSend, err = purego.Dlsym(lib, "objc_msgSend"); err != nil {
			mpErr = fmt.Errorf("%w: libobjc: %w", ErrUnsupported, err)
			return
		}
		for _, fn := range []any{&msgID, &msgIDID, &msgSetID, &msgSetIDID, &msgSetUint, &msgUintID, &msgFloatID,
			&msgFloat, &msgBytesID, &msgDataID, &msgSize, &msgArtworkID, &msgBlockID} {
			purego.RegisterFunc(fn, msgSend)
		}
		purego.RegisterLibFunc(&poolPush, lib, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&poolPop, lib, "objc_autoreleasePoolPop")
		for name, key := range map[string]*objc.ID{
			"MPMediaItemPropertyTitle":                    &keyTitle,
			"MPMediaItemPropertyArtist":                   &keyArtist,
			"MPMediaItemPropertyAlbumTitle":               &keyAlbum,
			"MPMediaItemPropertyPlaybackDuration":         &keyDuration,
			"MPMediaItemPropertyArtwork":                  &keyArtwork,
			"MPNowPlayingInfoPropertyElapsedPlaybackTime": &keyElapsed,
			"MPNowPlayingInfoPropertyPlaybackRate":        &keyRate,
			"MPNowPlayingInfoPropertyDefaultPlaybackRate": &keyDefaultRate,
			"MPNowPlayingInfoPropertyMediaType":           &keyMediaType,
		} {
			p, err := purego.Dlsym(mp, name)
			if err != nil {
				mpErr = fmt.Errorf("%w: MediaPlayer: %w", ErrUnsupported, err)
				return
			}
			// p is the address of an NSString *const.
			*key = **(**objc.ID)(unsafe.Pointer(&p))
		}
		for _, name := range []string{"MPNowPlayingInfoCenter", "MPRemoteCommandCenter", "MPMediaItemArtwork"} {
			if class(name) == 0 {
				mpErr = fmt.Errorf("%w: MediaPlayer has no %s", ErrUnsupported, name)
				return
			}
		}
	})
	return mpErr
}

// sel returns the selector of name, cached.
func sel(name string) objc.SEL {
	selMu.Lock()
	defer selMu.Unlock()
	s, ok := selCache[name]
	if !ok {
		s = objc.RegisterName(name)
		selCache[name] = s
	}
	return s
}

func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

// withPool runs fn in an autorelease pool, on its thread throughout.
func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := poolPush()
	defer poolPop(pool)
	fn()
}

func autorelease(obj objc.ID) objc.ID { return msgID(obj, sel("autorelease")) }
func release(obj objc.ID)             { msgID(obj, sel("release")) }

// nsString returns an autoreleased NSString of s.
func nsString(s string) objc.ID {
	str := msgBytesID(msgID(class("NSString"), sel("alloc")), sel("initWithBytes:length:encoding:"),
		unsafe.Pointer(unsafe.StringData(s)), uint(len(s)), nsUTF8StringEncoding)
	runtime.KeepAlive(s)
	return autorelease(str)
}

func nsNumber(v float64) objc.ID {
	return msgFloatID(class("NSNumber"), sel("numberWithDouble:"), v)
}

// commands are the commands of MPRemoteCommandCenter MixMusic handles, but
// changePlaybackPositionCommand's seek.
var commands = []struct {
	name string
	cmd  Command
}{
	{"playCommand", Play},
	{"pauseCommand", Pause},
	{"togglePlayPauseCommand", Toggle},
	{"nextTrackCommand", Next},
	{"previousTrackCommand", Previous},
	// As the media keys' stop.
	{"stopCommand", Pause},
}

// macControls is Now Playing.
type macControls struct {
	ask func(Request)
	run func(func())

	// targets are the commands with their handlers, made on the main
	// thread, which removes them; blocks the handlers.
	targets [][2]objc.ID
	blocks  []objc.Block

	mu    sync.Mutex
	shown *Info
	// image is the artwork of the track imageFor, an NSImage retained; the
	// artwork's handler hands it out until it changes.
	image    objc.ID
	imageFor string
	// fetching is the track whose artwork downloads.
	fetching string
}

func open(ask func(Request), run func(func())) (platform, error) {
	if err := loadMediaPlayer(); err != nil {
		return nil, err
	}
	m := &macControls{ask: ask, run: run}
	m.run(func() { withPool(m.register) })
	return m, nil
}

// register handles the commands.
func (m *macControls) register() {
	center := msgID(class("MPRemoteCommandCenter"), sel("sharedCommandCenter"))
	for _, c := range commands {
		r := Request{Command: c.cmd}
		m.handle(msgID(center, sel(c.name)), objc.NewBlock(func(objc.Block, objc.ID) int {
			m.ask(r)
			return mpCommandSuccess
		}))
	}
	m.handle(msgID(center, sel("changePlaybackPositionCommand")), objc.NewBlock(func(_ objc.Block, event objc.ID) int {
		t := msgFloat(event, sel("positionTime"))
		if math.IsNaN(t) || math.IsInf(t, 0) || t < 0 {
			return mpCommandFailed
		}
		m.ask(Request{Command: Seek, Position: time.Duration(t * float64(time.Second))})
		return mpCommandSuccess
	}))
}

func (m *macControls) handle(command objc.ID, block objc.Block) {
	target := msgBlockID(command, sel("addTargetWithHandler:"), block)
	m.targets = append(m.targets, [2]objc.ID{command, target})
	m.blocks = append(m.blocks, block)
}

func (m *macControls) close() {
	m.run(func() {
		withPool(func() {
			for _, t := range m.targets {
				msgSetID(t[0], sel("removeTarget:"), t[1])
			}
			for _, b := range m.blocks {
				b.Release()
			}
			m.targets, m.blocks = nil, nil
		})
	})
	m.mu.Lock()
	m.dropImage()
	m.mu.Unlock()
}

func (m *macControls) show(next *Info) {
	m.mu.Lock()
	if next != nil && m.shown != nil {
		if c := diff(m.shown, next); !c.track && !c.playing && !c.seeked {
			// Now Playing has no volume, and moves the position on itself.
			m.mu.Unlock()
			return
		}
	}
	m.shown = next
	if next == nil || next.ID != m.imageFor {
		m.dropImage()
	}
	fetch := next != nil && next.Artwork != "" && m.imageFor == "" && m.fetching != next.ID
	if fetch {
		m.fetching = next.ID
	}
	m.mu.Unlock()
	m.run(func() { withPool(m.update) })
	if fetch {
		go m.fetchArtwork(next.ID, next.Artwork)
	}
}

// dropImage lets go of the artwork; m.mu is held.
func (m *macControls) dropImage() {
	if m.image != 0 {
		release(m.image)
	}
	m.image, m.imageFor = 0, ""
}

// update shows what m.shown holds, on the main thread.
func (m *macControls) update() {
	m.mu.Lock()
	info, image := m.shown, m.image
	if info != nil && m.imageFor != info.ID {
		image = 0
	}
	m.mu.Unlock()
	// MediaPlayer may call the artwork's handler, which takes m.mu, from
	// here on.
	center := msgID(class("MPNowPlayingInfoCenter"), sel("defaultCenter"))
	if info == nil {
		msgSetID(center, sel("setNowPlayingInfo:"), 0)
		msgSetUint(center, sel("setPlaybackState:"), mpPlaybackStateStopped)
		return
	}
	dict := msgID(class("NSMutableDictionary"), sel("dictionary"))
	set := func(key, value objc.ID) {
		if value != 0 { // setObject:forKey: throws on nil
			msgSetIDID(dict, sel("setObject:forKey:"), value, key)
		}
	}
	set(keyTitle, nsString(info.Title))
	set(keyArtist, nsString(strings.Join(info.Artists, ", ")))
	if info.Album != "" {
		set(keyAlbum, nsString(info.Album))
	}
	if info.Duration > 0 {
		set(keyDuration, nsNumber(info.Duration.Seconds()))
	}
	set(keyElapsed, nsNumber(info.position(time.Now()).Seconds()))
	rate, state := 0.0, uint(mpPlaybackStatePaused)
	if info.Playing {
		rate, state = 1, mpPlaybackStatePlaying
	}
	set(keyRate, nsNumber(rate))
	set(keyDefaultRate, nsNumber(1))
	set(keyMediaType, msgUintID(class("NSNumber"), sel("numberWithUnsignedInteger:"), mpMediaTypeAudio))
	if image != 0 {
		set(keyArtwork, m.artwork(info.ID, msgSize(image, sel("size"))))
	}
	msgSetID(center, sel("setNowPlayingInfo:"), dict)
	msgSetUint(center, sel("setPlaybackState:"), state)
}

// artwork returns an autoreleased MPMediaItemArtwork of the image of the
// track id, whose handler hands out the image while it is that track's.
func (m *macControls) artwork(id string, size cgSize) objc.ID {
	handler := objc.NewBlock(func(objc.Block, cgSize) objc.ID {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.image == 0 || m.imageFor != id {
			return 0
		}
		return autorelease(msgID(m.image, sel("retain")))
	})
	defer handler.Release()
	art := msgArtworkID(msgID(class("MPMediaItemArtwork"), sel("alloc")), sel("initWithBoundsSize:requestHandler:"), size, handler)
	return autorelease(art)
}

// fetchArtwork downloads the artwork of the track id, and shows it if the
// track still shows.
func (m *macControls) fetchArtwork(id, rawURL string) {
	data, err := download(rawURL)
	if err != nil || len(data) == 0 {
		m.mu.Lock()
		if m.fetching == id {
			m.fetching = ""
		}
		m.mu.Unlock()
		return
	}
	m.run(func() {
		withPool(func() {
			d := msgDataID(class("NSData"), sel("dataWithBytes:length:"), unsafe.Pointer(&data[0]), uint(len(data)))
			runtime.KeepAlive(data)
			image := msgIDID(msgID(class("NSImage"), sel("alloc")), sel("initWithData:"), d)
			m.mu.Lock()
			if m.fetching == id {
				m.fetching = ""
			}
			if image == 0 || m.shown == nil || m.shown.ID != id {
				m.mu.Unlock()
				if image != 0 {
					release(image)
				}
				return
			}
			m.dropImage()
			m.image, m.imageFor = image, id
			m.mu.Unlock()
			m.update()
		})
	})
}
