package localfiles

// On Linux, inotify tells of the changes to each folder it watches, but
// not of those in the folders in it: the watch of a folder of music
// watches each folder in it, but hidden ones, and those made or renamed
// into it as they come. The system bounds how many folders a user
// watches (fs.inotify.max_user_watches), and how many inotify instances
// they open (fs.inotify.max_user_instances).

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// watchMask is what a watch of a folder tells of: names made, removed and
// renamed, files written and closed, or whose time of change was set, and
// the folder itself removed or renamed.
const watchMask = syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO |
	syscall.IN_CLOSE_WRITE | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF |
	syscall.IN_ONLYDIR | syscall.IN_DONT_FOLLOW | syscall.IN_EXCL_UNLINK

// networkFS are the file systems of network drives, by their magic
// numbers, as statfs tells them: inotify hears only of the changes made
// on this computer, not of those another makes. 9P is also how WSL
// reaches Windows' drives.
var networkFS = map[uint32]bool{
	0x6969:     true, // NFS
	0x517b:     true, // SMB
	0xff534d42: true, // CIFS
	0xfe534d42: true, // SMB2
	0x5346414f: true, // AFS
	0x6b414653: true, // kAFS
	0x73757245: true, // Coda
	0x01021997: true, // 9P
	0x00c36400: true, // Ceph
	0x564c:     true, // NCP
}

// errFolderGone is why a watch ended: its folder was removed, renamed, or
// its drive unmounted.
var errFolderGone = errors.New("the folder went")

// folderWatch is the inotify instance of a folder of music, and the
// folders it watches, by their watch descriptors.
type folderWatch struct {
	root string
	file *os.File
	raw  syscall.RawConn
	tell Changes
	dirs map[int]string
	done chan struct{}
	once sync.Once
}

func watchFolder(root string, tell Changes) (func(), error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err == nil && networkFS[uint32(st.Type)] {
		return nil, ErrNetworkDrive
	}
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if errors.Is(err, syscall.EMFILE) {
		return nil, fmt.Errorf("%w (fs.inotify.max_user_instances)", ErrWatchLimit)
	}
	if err != nil {
		return nil, err
	}
	// A file of the non-blocking descriptor reads through Go's poller,
	// which Close wakes.
	w := &folderWatch{root: filepath.Clean(root), file: os.NewFile(uintptr(fd), "inotify"), tell: tell,
		dirs: map[int]string{}, done: make(chan struct{})}
	if w.raw, err = w.file.SyscallConn(); err == nil {
		err = w.addTree(w.root)
	}
	if err != nil {
		w.file.Close()
		return nil, err
	}
	go w.loop()
	return w.stop, nil
}

// addTree watches dir and the folders in it, but hidden ones. A folder in
// it that can't be watched, as one it isn't allowed to read, is passed
// over; dir itself, and the system's limit, fail.
func (w *folderWatch) addTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil && path == dir:
			return err
		case err != nil:
			return nil
		case !d.IsDir():
			return nil
		case path != w.root && strings.HasPrefix(d.Name(), "."):
			return filepath.SkipDir
		}
		err = w.add(path)
		switch {
		case errors.Is(err, syscall.ENOSPC):
			return fmt.Errorf("%w (fs.inotify.max_user_watches)", ErrWatchLimit)
		case err != nil && path == dir:
			return err
		case err != nil:
			return filepath.SkipDir
		}
		return nil
	})
}

// add watches the folder at path.
func (w *folderWatch) add(path string) error {
	var wd int
	var err error
	if cerr := w.raw.Control(func(fd uintptr) { wd, err = syscall.InotifyAddWatch(int(fd), path, watchMask) }); cerr != nil {
		return cerr
	}
	if err == nil {
		w.dirs[wd] = path
	}
	return err
}

// removeTree stops watching the folder at path and the folders in it, as
// it went, or was renamed: inotify would go on telling of their changes
// by their old paths.
func (w *folderWatch) removeTree(path string) {
	for wd, dir := range w.dirs {
		if dir == path || strings.HasPrefix(dir, path+string(filepath.Separator)) {
			w.raw.Control(func(fd uintptr) { syscall.InotifyRmWatch(int(fd), uint32(wd)) })
			delete(w.dirs, wd)
		}
	}
}

// loop tells of the events inotify reads, until the watch stops, or ends
// as its folder went.
func (w *folderWatch) loop() {
	defer close(w.done)
	buf := make([]byte, 64<<10)
	for {
		n, err := w.file.Read(buf)
		if errors.Is(err, os.ErrClosed) {
			return
		}
		if err == nil {
			err = w.events(buf[:n])
		}
		if err != nil {
			w.tell.Ended(err)
			return
		}
	}
}

// events tells of the inotify_event records of b: of each, its watch
// descriptor, what happened, a cookie that pairs renames, and the length
// of the name that follows, NUL padded.
func (w *folderWatch) events(b []byte) error {
	for off := 0; off+syscall.SizeofInotifyEvent <= len(b); {
		wd := int(int32(binary.NativeEndian.Uint32(b[off:])))
		mask := binary.NativeEndian.Uint32(b[off+4:])
		size := int(binary.NativeEndian.Uint32(b[off+12:]))
		start := off + syscall.SizeofInotifyEvent
		if start+size > len(b) {
			return nil
		}
		name := string(bytes.TrimRight(b[start:start+size], "\x00"))
		off = start + size
		if mask&syscall.IN_Q_OVERFLOW != 0 {
			w.tell.Lost()
			continue
		}
		dir, ok := w.dirs[wd]
		if !ok {
			continue // of a folder no longer watched
		}
		path := dir
		if name != "" {
			path = filepath.Join(dir, name)
		}
		switch {
		case mask&syscall.IN_UNMOUNT != 0:
			return errFolderGone
		case mask&syscall.IN_IGNORED != 0:
			delete(w.dirs, wd)
			if dir == w.root {
				return errFolderGone
			}
		case mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF) != 0:
			// A folder in it is told of by the folder it's in.
			if dir == w.root {
				return errFolderGone
			}
		case mask&syscall.IN_ISDIR != 0 && mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0:
			var err error
			if !strings.HasPrefix(name, ".") {
				err = w.addTree(path)
			}
			w.tell.Moved(path)
			if errors.Is(err, ErrWatchLimit) {
				return err
			}
		case mask&syscall.IN_ISDIR != 0 && mask&(syscall.IN_DELETE|syscall.IN_MOVED_FROM) != 0:
			w.removeTree(path)
			w.tell.Moved(path)
		case mask&syscall.IN_ISDIR != 0:
			// A folder's own changes are its files'.
		case mask&(syscall.IN_CREATE|syscall.IN_DELETE|syscall.IN_MOVED_FROM|syscall.IN_MOVED_TO) != 0:
			w.tell.Moved(path)
		case mask&(syscall.IN_CLOSE_WRITE|syscall.IN_ATTRIB) != 0:
			w.tell.Changed(path)
		}
	}
	return nil
}

// stop stops the watch: closing the instance wakes its read, and drops
// its watches.
func (w *folderWatch) stop() {
	w.once.Do(func() {
		w.file.Close()
		<-w.done
	})
}
