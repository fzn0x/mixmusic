package localfiles

// On Windows, ReadDirectoryChangesW tells of the changes to a folder and
// the folders in it, by their names in it, through an I/O completion
// port: one read of a folder of music at a time, asked for again as soon
// as one comes.

import (
	"encoding/binary"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	// watchFilter is what a read tells of: names made, removed and
	// renamed, of files and of folders, and files written.
	watchFilter = syscall.FILE_NOTIFY_CHANGE_FILE_NAME | syscall.FILE_NOTIFY_CHANGE_DIR_NAME |
		syscall.FILE_NOTIFY_CHANGE_SIZE | syscall.FILE_NOTIFY_CHANGE_LAST_WRITE
	// changesSize is how many bytes of changes a read holds: 64 KB, past
	// which the read of a folder on a network drive fails.
	changesSize = 64 << 10
	// errorNotifyEnumDir is ERROR_NOTIFY_ENUM_DIR: more changed than the
	// system kept.
	errorNotifyEnumDir syscall.Errno = 1022

	// The kinds of drives GetDriveTypeW tells.
	driveRemovable = 2
	driveRemote    = 4
	driveCDROM     = 5
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDriveTypeW      = kernel32.NewProc("GetDriveTypeW")
	getVolumePathNameW = kernel32.NewProc("GetVolumePathNameW")
)

// errStopping is why a watch asks for no more changes: it stops.
var errStopping = errors.New("the watch stops")

// folderWatch is the watch of a folder of music.
type folderWatch struct {
	root       string
	dir, port  syscall.Handle
	tell       Changes
	buf        []uint32 // the changes, of DWORDs, as the system aligns them
	overlapped syscall.Overlapped
	done       chan struct{}
	once       sync.Once

	mu       sync.Mutex
	stopping bool
}

func watchFolder(root string, tell Changes) (func(), error) {
	if err := driveError(driveTypeOf(root)); err != nil {
		return nil, err
	}
	name, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return nil, err
	}
	dir, err := syscall.CreateFile(name, syscall.FILE_LIST_DIRECTORY,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	}
	port, err := syscall.CreateIoCompletionPort(dir, 0, 0, 1)
	if err != nil {
		syscall.CloseHandle(dir)
		return nil, err
	}
	w := &folderWatch{root: filepath.Clean(root), dir: dir, port: port, tell: tell,
		buf: make([]uint32, changesSize/4), done: make(chan struct{})}
	// The first read is asked for here, so that what changes once the
	// watch returns is told, and a folder that can't be watched fails.
	if err := w.read(); err != nil {
		syscall.CloseHandle(port)
		syscall.CloseHandle(dir)
		return nil, err
	}
	go w.loop()
	return w.stop, nil
}

// read asks for the next changes, unless the watch stops.
func (w *folderWatch) read() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopping {
		return errStopping
	}
	w.overlapped = syscall.Overlapped{}
	return syscall.ReadDirectoryChanges(w.dir, (*byte)(unsafe.Pointer(&w.buf[0])), changesSize, true, watchFilter, nil, &w.overlapped, 0)
}

// loop tells of each read's changes, and asks for the next, until the
// watch stops, or ends as the folder went.
func (w *folderWatch) loop() {
	defer close(w.done)
	for {
		var n, key uint32
		var o *syscall.Overlapped
		err := syscall.GetQueuedCompletionStatus(w.port, &n, &key, &o, syscall.INFINITE)
		w.mu.Lock()
		stopping := w.stopping
		w.mu.Unlock()
		switch {
		case stopping:
			return
		case errors.Is(err, errorNotifyEnumDir):
			w.tell.Lost()
		case err != nil:
			// As when the folder was removed, or its drive unplugged.
			w.tell.Ended(err)
			return
		case n == 0:
			w.tell.Lost() // more changed than the read could hold
		default:
			w.tellOf(n)
		}
		if err := w.read(); err != nil {
			if !errors.Is(err, errStopping) {
				w.tell.Ended(err)
			}
			return
		}
	}
}

// tellOf tells of the changes of the n bytes a read gave:
// FILE_NOTIFY_INFORMATION records, each of the name of what changed in
// the folder, in UTF-16.
func (w *folderWatch) tellOf(n uint32) {
	b := unsafe.Slice((*byte)(unsafe.Pointer(&w.buf[0])), len(w.buf)*4)[:n]
	for off := 0; off+12 <= len(b); {
		next := int(binary.LittleEndian.Uint32(b[off:]))
		action := binary.LittleEndian.Uint32(b[off+4:])
		size := int(binary.LittleEndian.Uint32(b[off+8:]))
		if off+12+size > len(b) {
			return
		}
		units := make([]uint16, size/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(b[off+12+2*i:])
		}
		path := w.longName(filepath.Join(w.root, string(utf16.Decode(units))))
		switch action {
		case syscall.FILE_ACTION_MODIFIED:
			w.tell.Changed(path)
		case syscall.FILE_ACTION_ADDED, syscall.FILE_ACTION_REMOVED,
			syscall.FILE_ACTION_RENAMED_OLD_NAME, syscall.FILE_ACTION_RENAMED_NEW_NAME:
			w.tell.Moved(path)
		}
		if next == 0 {
			return
		}
		off += next
	}
}

// longName is path, of a file in the folder, with its long names: Windows
// tells a change by the short name, as PROGRA~1, a program reached the
// file by, which the index doesn't hold the file by.
func (w *folderWatch) longName(path string) string {
	if !strings.Contains(path[len(w.root):], "~") {
		return path
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buf := make([]uint16, syscall.MAX_PATH)
	for {
		n, err := syscall.GetLongPathName(p, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return path // gone, as a file removed is
		}
		if int(n) <= len(buf) {
			long := syscall.UTF16ToString(buf[:n])
			if len(long) < len(w.root) || !strings.EqualFold(long[:len(w.root)], w.root) {
				return path
			}
			return w.root + long[len(w.root):]
		}
		buf = make([]uint16, n)
	}
}

// stop stops the watch: the read under way is cancelled, and the watch's
// handles closed once it is.
func (w *folderWatch) stop() {
	w.once.Do(func() {
		w.mu.Lock()
		w.stopping = true
		syscall.CancelIoEx(w.dir, nil)
		w.mu.Unlock()
		<-w.done
		syscall.CloseHandle(w.port)
		syscall.CloseHandle(w.dir)
	})
}

// driveTypeOf tells the kind of the drive root is on, as GetDriveTypeW
// does: driveRemote for a share of another computer.
func driveTypeOf(root string) uint32 {
	vol := filepath.VolumeName(root)
	if strings.HasPrefix(vol, `\\`) && !strings.HasPrefix(vol, `\\?\`) && !strings.HasPrefix(vol, `\\.\`) {
		return driveRemote // \\server\share
	}
	name, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0
	}
	// The drive of a folder a drive is mounted in is that drive.
	mount := make([]uint16, len(root)+syscall.MAX_PATH)
	if r, _, _ := getVolumePathNameW.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&mount[0])), uintptr(len(mount))); r == 0 {
		if vol == "" {
			return 0
		}
		mount, _ = syscall.UTF16FromString(vol + `\`)
	}
	kind, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(&mount[0])))
	return uint32(kind)
}

// driveError is why a folder on a drive of kind isn't watched: one on a
// network drive, whose server may not tell of what other computers
// change, and one on a removable drive, which an open watch keeps from
// being ejected.
func driveError(kind uint32) error {
	switch kind {
	case driveRemote:
		return ErrNetworkDrive
	case driveRemovable, driveCDROM:
		return ErrRemovableDrive
	}
	return nil
}
