// Package store keeps values in JSON files. Writes are atomic: a file is
// either the old value or the new one, never half of each, even when the
// app is killed while it saves.
//
// A file opened with OpenWith may be edited by hand while the app runs:
// Reload takes in what was saved to it, and every write reads it first, so
// that it never saves over an edit it hasn't read. While the file doesn't
// decode, it stays as it was saved, and writes wait until it does.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// File holds a value of type T, kept in a JSON file.
type File[T any] struct {
	path string
	// decode reads the file of a File OpenWith opened; nil for Open's.
	decode func([]byte) (T, error)
	mu     sync.RWMutex
	value  T
	// Of a File OpenWith opened: disk is the file's bytes as this process
	// last read or wrote them, nil when it had none; synced is what they
	// last decoded to, the base that edits are merged from; held is why
	// the file doesn't decode, while writes wait for it to; aside is why
	// OpenWith kept the file aside.
	disk   []byte
	synced T
	held   error
	aside  error
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

// OpenWith loads the value kept at path as decode reads it, or def when
// the file does not exist yet, for a file that may be edited while the app
// runs. A file decode can't read is kept aside as path + ".broken", and
// def is saved in its place; Aside tells why. A path that is a symbolic
// link stays one: writes go to the file it links to.
func OpenWith[T any](path string, def T, decode func([]byte) (T, error)) (*File[T], error) {
	f := &File[T]{path: path, decode: decode, value: def, synced: def}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return f, nil
	case err != nil:
		return nil, err
	}
	v, err := decode(data)
	if err != nil {
		f.aside = err
		if err := f.moveAside(data); err != nil {
			return nil, err
		}
		return f, nil
	}
	f.value, f.synced, f.disk = v, v, data
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

// Held returns why the file doesn't decode, while the writes of a File
// OpenWith opened wait for it to; nil otherwise.
func (f *File[T]) Held() error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.held
}

// Aside returns why OpenWith kept the file aside as path + ".broken", or
// nil when it didn't.
func (f *File[T]) Aside() error { return f.aside }

// Update changes the value with fn and saves it. When fn returns an error,
// nothing changes and the error is returned.
//
// Of a File OpenWith opened, it reads the file first: an edit saved to it
// since it was last read is taken in, under what fn changed, before the
// value is saved. While the file doesn't decode, the value changes but
// nothing is saved, and Held tells why.
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
	if f.decode != nil {
		from := f.value
		data, err := os.ReadFile(f.path)
		if err == nil && !f.unchanged(data) {
			if _, err := f.take(data); err != nil {
				return err
			}
			// What fn changed goes over the edit.
			if next, err = Merge(from, f.value, next); err != nil {
				return err
			}
		}
		// A file gone is saved again; one that doesn't decode, or that
		// can't be read for a moment while it doesn't, waits.
		if f.held != nil && !errors.Is(err, fs.ErrNotExist) {
			f.value = next
			return nil
		}
	}
	return f.save(next)
}

// Set replaces the value and saves it.
func (f *File[T]) Set(v T) error {
	return f.Update(func(p *T) error { *p = v; return nil })
}

// Change is what Reload found in the file.
type Change[T any] struct {
	// Prev is the value before, and Value the value now.
	Prev, Value T
	// Data is the file's bytes, when Reload took them in; nil when the
	// file holds what this process last read or wrote, or is missing.
	Data []byte
	// Changed tells whether the value changed.
	Changed bool
}

// Reload takes in an edit of the file of a File OpenWith opened: its own
// writes, which the file still holds, are no edit. What the value changed
// since the file last decoded, while it didn't, is merged in under the
// edit, and saved with it. A file that doesn't decode stays as it is, and
// its error is returned, as Held returns it, until it decodes. A file gone
// changes nothing; the next write saves it again.
func (f *File[T]) Reload() (Change[T], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := Change[T]{Prev: f.value, Value: f.value}
	if f.decode == nil {
		return c, nil
	}
	data, err := os.ReadFile(f.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		f.disk, f.held = nil, nil
		return c, nil
	case err != nil:
		return c, err
	case f.unchanged(data):
		return c, f.held
	}
	unsaved, err := f.take(data)
	if err == nil && f.held != nil {
		return c, f.held
	}
	if err == nil && unsaved {
		err = f.save(f.value)
	}
	c.Value, c.Data = f.value, data
	c.Changed = !same(c.Prev, f.value)
	return c, err
}

// unchanged tells whether data, the file's bytes, are those this process
// last read or wrote: no edit. An empty file where there was none is one.
func (f *File[T]) unchanged(data []byte) bool {
	return f.disk != nil && bytes.Equal(data, f.disk)
}

// take takes in data, an edit of the file: when it decodes, what it
// changed goes over the value, and unsaved tells whether the value differs
// from it then, with changes of its own made while the file didn't
// decode; when it doesn't decode, writes are held.
func (f *File[T]) take(data []byte) (unsaved bool, err error) {
	theirs, err := f.decode(data)
	if err != nil {
		f.disk, f.held = data, err
		return false, nil
	}
	merged, err := Merge(f.synced, f.value, theirs)
	if err != nil {
		return false, err
	}
	f.disk, f.held, f.synced, f.value = data, nil, theirs, merged
	return !same(merged, theirs), nil
}

// Create saves the value of a File OpenWith opened when its file is
// missing, as it is until the first write.
func (f *File[T]) Create() error {
	if f.decode == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := os.Stat(f.path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return f.save(f.value)
}

// Flush saves the value of a File OpenWith opened where its file holds
// none: when it is missing, and when it doesn't decode, whose text is kept
// aside as path + ".broken" first. An edit not taken in yet is, first, as
// Reload takes it. Otherwise it writes nothing. It is meant for when the
// app quits.
func (f *File[T]) Flush() error {
	if f.decode == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := os.ReadFile(f.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return f.save(f.value)
	case err != nil:
		return err
	case !f.unchanged(data):
		unsaved, err := f.take(data)
		switch {
		case err != nil:
			return err
		case unsaved && f.held == nil:
			return f.save(f.value)
		}
	}
	if f.held == nil {
		return nil
	}
	return f.moveAside(data)
}

// Replace saves v over what the file of a File OpenWith opened holds,
// without taking an edit in: a file that doesn't decode is kept aside as
// path + ".broken" first. Of a File Open opened, it is Set.
func (f *File[T]) Replace(v T) error {
	if f.decode == nil {
		return f.Set(v)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if data, err := os.ReadFile(f.path); err == nil {
		if _, err := f.decode(data); err != nil {
			f.value = v
			return f.moveAside(data)
		}
	}
	return f.save(v)
}

// moveAside keeps data, the text of a file that doesn't decode, as path +
// ".broken", and saves the value in its place, through a link that path
// may be.
func (f *File[T]) moveAside(data []byte) error {
	if err := WriteFile(f.path+".broken", data); err != nil {
		return fmt.Errorf("store: %s is corrupt and cannot be moved aside: %w", f.path, err)
	}
	return f.save(f.value)
}

// save writes v and makes it the value, which the file then holds.
func (f *File[T]) save(v T) error {
	data, err := marshal(v)
	if err != nil {
		return err
	}
	if err := f.write(data); err != nil {
		return err
	}
	f.value = v
	if f.decode != nil {
		f.disk, f.synced, f.held = data, v, nil
	}
	return nil
}

// write writes data to the file. Of a File OpenWith opened, a path that is
// a symbolic link stays one: data goes to the file it links to, unless
// that can't be written, as in a folder that is read-only, where data
// takes the link's place.
func (f *File[T]) write(data []byte) error {
	if f.decode != nil {
		if target, err := filepath.EvalSymlinks(f.path); err == nil && target != filepath.Clean(f.path) {
			err := WriteFile(target, data)
			if err == nil {
				return nil
			}
			log.Printf("mixmusic: %s links to %s, which can't be written (%v): saving over the link", f.path, target, err)
		}
	}
	return WriteFile(f.path, data)
}

// Merge returns a, with what b changed of base: a merge of the JSON
// objects the values marshal to, by their top-level members, where b
// wins. A member a and b both changed takes b's whole value, as a map of
// b's in place of a's.
func Merge[T any](base, a, b T) (T, error) {
	var out T
	baseMembers, err := members(base)
	if err != nil {
		return out, err
	}
	aMembers, err := members(a)
	if err != nil {
		return out, err
	}
	bMembers, err := members(b)
	if err != nil {
		return out, err
	}
	for key, v := range bMembers {
		if was, ok := baseMembers[key]; !ok || !bytes.Equal(was, v) {
			aMembers[key] = v
		}
	}
	for key := range baseMembers {
		if _, ok := bMembers[key]; !ok {
			delete(aMembers, key)
		}
	}
	data, err := json.Marshal(aMembers)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(data, &out)
	return out, err
}

// members are the top-level members of the JSON object v marshals to.
func members(v any) (map[string]json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("store: merging a value that isn't a JSON object: %w", err)
	}
	return m, nil
}

// same tells whether a and b marshal to the same JSON.
func same[T any](a, b T) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && bytes.Equal(x, y)
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

// marshal is v as the file keeps it.
func marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
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
