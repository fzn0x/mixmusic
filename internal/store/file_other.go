//go:build !windows

package store

import "os"

// ReadFile reads the file at path, as os.ReadFile does: elsewhere than on
// Windows, an editor saves over a file open to read.
func ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// rename renames from over to, as os.Rename does: elsewhere than on
// Windows, a file open to read doesn't keep another from replacing it.
func rename(from, to string) error { return os.Rename(from, to) }
