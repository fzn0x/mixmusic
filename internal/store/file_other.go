//go:build !windows

package store

import "os"

// ReadFile reads the file at path, as os.ReadFile does: elsewhere than on
// Windows, an editor saves over a file open to read.
func ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
