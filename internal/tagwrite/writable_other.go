//go:build !unix && !windows

package tagwrite

import "io/fs"

// writableFile tells whether the file's permissions let it be written.
func writableFile(path string, info fs.FileInfo) error {
	if info.Mode().Perm()&0o200 == 0 {
		return ErrReadOnly
	}
	return nil
}

// writableDir finds no reason a file can't be made in dir: the write
// tells.
func writableDir(dir string) error { return nil }
