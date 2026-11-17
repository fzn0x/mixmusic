package localfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// WalkRoot walks root as filepath.WalkDir does, but when root is itself a
// link (a symbolic link, or a directory junction on Windows) it walks the
// folder it points to, and tells the paths under root: the folder's own
// path stays the one the settings and the index know it by. Links inside
// root are not followed, so a link to a parent folder makes no loop.
func WalkRoot(root string, fn fs.WalkDirFunc) error {
	real, err := resolve(root)
	if err != nil || filepath.Clean(real) == filepath.Clean(root) {
		return filepath.WalkDir(root, fn)
	}
	root, real = filepath.Clean(root), filepath.Clean(real)
	return filepath.WalkDir(real, func(path string, d fs.DirEntry, err error) error {
		if path == real {
			return fn(root, d, err)
		}
		if rel, rerr := filepath.Rel(real, path); rerr == nil {
			path = filepath.Join(root, rel)
		}
		return fn(path, d, err)
	})
}

// sameFolder reports whether a and b are the same folder reached by two
// paths, as a link and the folder it points to.
func sameFolder(a, b string) bool {
	ra, err := resolve(a)
	if err != nil {
		return false
	}
	rb, err := resolve(b)
	return err == nil && ra == rb
}

// resolve is the folder a path leads to, past its links. EvalSymlinks
// leaves a Windows directory junction as it is, which Lstat tells as
// neither a folder nor a link, so a junction is followed by what it
// points to, as a symbolic link is.
func resolve(path string) (string, error) {
	for range 16 {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		if info, err := os.Lstat(real); err != nil || info.Mode()&fs.ModeIrregular == 0 {
			return real, err
		}
		if info, err := os.Stat(real); err != nil || !info.IsDir() {
			return real, err
		}
		to, err := os.Readlink(real)
		if err != nil {
			return real, nil
		}
		if !filepath.IsAbs(to) {
			to = filepath.Join(filepath.Dir(real), to)
		}
		path = to
	}
	return "", errors.New("too many links")
}
