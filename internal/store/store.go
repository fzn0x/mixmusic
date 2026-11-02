// Package store keeps values in JSON files. Writes are atomic: a file is
// either the old value or the new one, never half of each, even when the
// app is killed while it saves.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// File holds a value of type T, kept in a JSON file.
type File[T any] struct {
	path  string
	mu    sync.RWMutex
	value T
	// version counts the changes Update made.
	version uint64
}

// Open loads the value kept at path, or def when the file does not exist
// yet. A file that cannot be decoded is moved aside to path + ".broken",
// so a corrupt file never keeps the app from starting.
func Open[T any](path string, def T) (*File[T], error) {
	f := &File[T]{path: path, value: def}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return f, nil
	case err != nil:
		return nil, err
	}
	if err := json.Unmarshal(data, &f.value); err != nil {
		f.value = def
		if err := os.Rename(path, path+".broken"); err != nil {
			return nil, fmt.Errorf("store: %s is corrupt and cannot be moved aside: %w", path, err)
		}
	}
	return f, nil
}

// Path returns the file the value is kept in.
func (f *File[T]) Path() string { return f.path }

// Get returns the value. Callers must not change what it references, such
// as slices and maps: change the value with Update.
func (f *File[T]) Get() T {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.value
}

// Update changes the value with fn and saves it. When fn returns an error,
// nothing changes and the error is returned.
func (f *File[T]) Update(fn func(v *T) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var next T
	if err := clone(&next, f.value); err != nil {
		return err
	}
	if err := fn(&next); err != nil {
		return err
	}
	if err := writeJSON(f.path, next); err != nil {
		return err
	}
	f.value = next
	f.version++
	return nil
}

// Version counts the changes made to the value since the file opened, so
// that what a reader made of the value can be kept until it changes.
func (f *File[T]) Version() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.version
}

// Set replaces the value and saves it.
func (f *File[T]) Set(v T) error {
	return f.Update(func(p *T) error { *p = v; return nil })
}

// clone deep-copies src into dst through JSON, so that fn in Update may
// change slices and maps without touching the value readers hold.
func clone[T any](dst *T, src T) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// writeJSON writes v to path atomically: into a temporary file of the same
// directory, flushed to disk, then renamed over path.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, append(data, '\n'))
}

// WriteFile writes data to path atomically, creating its directory.
func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
