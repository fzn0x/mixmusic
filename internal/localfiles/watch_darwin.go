//go:build darwin && (amd64 || arm64)

package localfiles

// On macOS, FSEvents tells of the changes to a folder and the folders in
// it, file by file (kFSEventStreamCreateFlagFileEvents), and of the folder
// itself renamed or removed (kFSEventStreamCreateFlagWatchRoot), through
// CoreServices, which MixMusic reaches through purego, as it does Core
// Audio. FSEvents, not kqueue: a kqueue watch takes an open file for each
// folder, and tells only that a folder's list of files changed, not which
// file, nor of a file written in place; FSEvents watches a whole tree in
// one stream, and holds no file open that would keep a drive from being
// ejected. Its callback runs on a dispatch queue of the stream's own.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	fsEventsSinceNow = 0xFFFFFFFFFFFFFFFF

	fsCreateNoDefer    = 0x02
	fsCreateWatchRoot  = 0x04
	fsCreateFileEvents = 0x10

	fsMustScanSubDirs = 0x00000001
	fsUserDropped     = 0x00000002
	fsKernelDropped   = 0x00000004
	fsRootChanged     = 0x00000020
	fsUnmount         = 0x00000080
	fsItemCreated     = 0x00000100
	fsItemRemoved     = 0x00000200
	fsItemMetaMod     = 0x00000400
	fsItemRenamed     = 0x00000800
	fsItemModified    = 0x00001000

	cfStringEncodingUTF8 = 0x08000100
	// mntLocal is statfs's MNT_LOCAL, of a file system on this computer.
	mntLocal = 0x00001000
	// fsLatency is how long, in seconds, FSEvents gathers changes before
	// it tells of them.
	fsLatency = 0.2
)

// fsEventStreamContext is FSEventStreamContext: info is the ID of the
// stream's watch, in fsStreams.
type fsEventStreamContext struct {
	version                          int
	info                             uintptr
	retain, release, copyDescription uintptr
}

var (
	fsOnce sync.Once
	fsErr  error

	fsEventStreamCreate           func(alloc, callback uintptr, ctx *fsEventStreamContext, paths uintptr, since uint64, latency float64, flags uint32) uintptr
	fsEventStreamSetDispatchQueue func(stream, queue uintptr)
	fsEventStreamStart            func(stream uintptr) bool
	fsEventStreamStop             func(stream uintptr)
	fsEventStreamInvalidate       func(stream uintptr)
	fsEventStreamRelease          func(stream uintptr)
	cfStringCreateWithCString     func(alloc uintptr, s string, encoding uint32) uintptr
	cfArrayCreate                 func(alloc uintptr, values *uintptr, n int, callbacks uintptr) uintptr
	cfRelease                     func(ref uintptr)
	dispatchQueueCreate           func(label, attr uintptr) uintptr
	dispatchSyncF                 func(queue, ctx, work uintptr)
	dispatchRelease               func(object uintptr)
	cfTypeArrayCallBacks          uintptr

	// fsCallback is every stream's callback, and fsDone a call that does
	// nothing, run on a stream's queue to wait for the callbacks under
	// way: made once, as callbacks are limited.
	fsCallback, fsDone uintptr
	// fsStreams are the watches, by their IDs.
	fsStreams sync.Map
	fsNext    atomic.Uintptr
)

// loadFSEvents loads what a watch needs: CoreServices' FSEvents,
// CoreFoundation's strings and arrays, and libdispatch's queues.
func loadFSEvents() error {
	fsOnce.Do(func() {
		open := func(path string) uintptr {
			lib, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
			if err != nil && fsErr == nil {
				fsErr = fmt.Errorf("%w (%s: %v)", ErrCantWatch, filepath.Base(path), err)
			}
			return lib
		}
		cs := open("/System/Library/Frameworks/CoreServices.framework/CoreServices")
		cf := open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		sys := open("/usr/lib/libSystem.B.dylib")
		if fsErr != nil {
			return
		}
		purego.RegisterLibFunc(&fsEventStreamCreate, cs, "FSEventStreamCreate")
		purego.RegisterLibFunc(&fsEventStreamSetDispatchQueue, cs, "FSEventStreamSetDispatchQueue")
		purego.RegisterLibFunc(&fsEventStreamStart, cs, "FSEventStreamStart")
		purego.RegisterLibFunc(&fsEventStreamStop, cs, "FSEventStreamStop")
		purego.RegisterLibFunc(&fsEventStreamInvalidate, cs, "FSEventStreamInvalidate")
		purego.RegisterLibFunc(&fsEventStreamRelease, cs, "FSEventStreamRelease")
		purego.RegisterLibFunc(&cfStringCreateWithCString, cf, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&cfArrayCreate, cf, "CFArrayCreate")
		purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
		purego.RegisterLibFunc(&dispatchQueueCreate, sys, "dispatch_queue_create")
		purego.RegisterLibFunc(&dispatchSyncF, sys, "dispatch_sync_f")
		purego.RegisterLibFunc(&dispatchRelease, sys, "dispatch_release")
		var err error
		if cfTypeArrayCallBacks, err = purego.Dlsym(cf, "kCFTypeArrayCallBacks"); err != nil {
			fsErr = fmt.Errorf("%w (kCFTypeArrayCallBacks: %v)", ErrCantWatch, err)
			return
		}
		fsCallback = purego.NewCallback(fsEvents)
		fsDone = purego.NewCallback(func(uintptr) {})
	})
	return fsErr
}

// fsStream is the FSEvents stream of a folder of music. FSEvents tells of
// the paths of real, which root, as the settings name it, is with its
// links followed: /private/var for /var.
type fsStream struct {
	id            uintptr
	root, real    string
	tell          Changes
	stream, queue uintptr
	ended         atomic.Bool
	once          sync.Once
}

func watchFolder(root string, tell Changes) (func(), error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err == nil && st.Flags&mntLocal == 0 {
		return nil, ErrNetworkDrive
	}
	if err := loadFSEvents(); err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		real = root
	}
	s := &fsStream{id: fsNext.Add(1), root: root, real: real, tell: tell}
	fsStreams.Store(s.id, s)
	path := cfStringCreateWithCString(0, real, cfStringEncodingUTF8)
	paths := cfArrayCreate(0, &path, 1, cfTypeArrayCallBacks)
	cfRelease(path)
	ctx := fsEventStreamContext{info: s.id}
	s.stream = fsEventStreamCreate(0, fsCallback, &ctx, paths, fsEventsSinceNow, fsLatency,
		fsCreateNoDefer|fsCreateWatchRoot|fsCreateFileEvents)
	cfRelease(paths)
	if s.stream == 0 {
		fsStreams.Delete(s.id)
		return nil, errors.New("FSEvents made no stream")
	}
	s.queue = dispatchQueueCreate(0, 0)
	fsEventStreamSetDispatchQueue(s.stream, s.queue)
	if !fsEventStreamStart(s.stream) {
		s.stop()
		return nil, errors.New("FSEvents didn't start the stream")
	}
	return s.stop, nil
}

// fsEvents is the callback of every stream: numEvents paths, in C
// strings, and their flags.
func fsEvents(_, info, numEvents uintptr, paths **byte, flags *uint32, _ uintptr) {
	v, ok := fsStreams.Load(info)
	if !ok || numEvents == 0 || paths == nil || flags == nil {
		return
	}
	s := v.(*fsStream)
	ps, fs := unsafe.Slice(paths, numEvents), unsafe.Slice(flags, numEvents)
	for i := range ps {
		if s.ended.Load() {
			return
		}
		s.event(cString(ps[i]), fs[i])
	}
}

// event tells of an event of the stream: at path, what flags tell.
func (s *fsStream) event(path string, flags uint32) {
	switch {
	case flags&(fsUserDropped|fsKernelDropped) != 0:
		s.tell.Lost()
		return
	case flags&fsRootChanged != 0:
		// The folder of music was renamed or removed: its watch ends.
		s.ended.Store(true)
		s.tell.Ended(errors.New("the folder went"))
		return
	case flags&fsUnmount != 0:
		s.tell.Lost() // a drive mounted in the folder went
		return
	}
	p, ok := s.local(path)
	if !ok {
		return
	}
	switch {
	case flags&fsMustScanSubDirs != 0 && p == s.root:
		s.tell.Lost()
	case p == s.root && flags&(fsItemRemoved|fsItemRenamed) == 0:
		// The folder of music itself, made or written in. FSEvents tells a
		// path's flags gathered over its recent past, so a folder made just
		// before its watch started tells it was made. It's there, as its
		// watch is: what changed in it, its files tell.
		s.tell.Changed(p)
	case flags&(fsMustScanSubDirs|fsItemCreated|fsItemRemoved|fsItemRenamed) != 0:
		s.tell.Moved(p)
	case flags&(fsItemModified|fsItemMetaMod) != 0:
		s.tell.Changed(p)
	}
}

// local is the path in the folder of music, as the settings name it, of a
// path FSEvents told of, false for one beyond it. macOS's disks don't tell
// a name's case.
func (s *fsStream) local(path string) (string, bool) {
	path = strings.TrimSuffix(path, "/")
	if len(path) < len(s.real) || !strings.EqualFold(path[:len(s.real)], s.real) {
		return "", false
	}
	rest := path[len(s.real):]
	if rest != "" && rest[0] != '/' {
		return "", false
	}
	return s.root + rest, true
}

// stop stops the stream, waits for its callbacks under way, and releases
// it and its queue.
func (s *fsStream) stop() {
	s.once.Do(func() {
		fsStreams.Delete(s.id)
		s.ended.Store(true)
		fsEventStreamStop(s.stream)
		fsEventStreamInvalidate(s.stream)
		dispatchSyncF(s.queue, 0, fsDone)
		fsEventStreamRelease(s.stream)
		dispatchRelease(s.queue)
	})
}

// cString is the C string at p.
func cString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
