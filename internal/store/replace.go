package store

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Replace writes the file at path anew, safely: write writes the new file
// beside it, in its folder, which Replace then puts on the disk and
// renames over the old one, so that the file is the old one or the new
// one, whole, whatever happens meanwhile. The new file gets the
// permissions perm. When write fails, or the disk does, the old file stays
// as it was, and the new one goes.
//
// On Windows, the rename replaces a file that other programs read, as long
// as they let it be deleted meanwhile, as OpenShared does: they read the
// old file until they close it.
func Replace(path string, perm fs.FileMode, write func(f *os.File) error) error {
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return err
	}
	kept := false
	defer func() {
		if !kept {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err := write(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := renameOver(root, filepath.Base(tmp.Name()), name); err != nil {
		return err
	}
	kept = true
	syncDir(dir)
	return nil
}
