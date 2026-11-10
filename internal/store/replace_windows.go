package store

import (
	"os"
	"time"
)

// OpenShared opens the file at path to read it, letting other programs
// write it, delete it or rename another file over it meanwhile, as
// Replace does: the file stays readable as it was until it's closed.
func OpenShared(path string) (*os.File, error) { return openShared(path) }

// renameOver renames from over to, in root. os.Root's rename replaces to
// with POSIX semantics where the file system has them, as NTFS does, so
// that a program reading to through a handle that shares its deletion,
// as OpenShared's, goes on reading the old file; elsewhere, it renames as
// MoveFileEx does. A program that has to open without sharing its
// deletion, as an editor, a search indexer or an antivirus may for a
// moment, makes it try again for about 100 ms.
func renameOver(root *os.Root, from, to string) error {
	var err error
	for try := range renameTries {
		if try > 0 {
			time.Sleep(renameWait)
		}
		if err = root.Rename(from, to); err == nil || !busy(err) {
			return err
		}
	}
	return err
}

// syncDir puts a rename in dir on the disk: Windows does with the rename.
func syncDir(dir string) {}
