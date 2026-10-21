package addons

import (
	"errors"
	"path/filepath"
	"sync"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
)

// maxStorage bounds what one plugin keeps, in bytes.
const maxStorage = 1 << 20

// ErrStorageFull is returned when a plugin would keep more than it may.
var ErrStorageFull = errors.New("addons: the plugin's storage is full (1 MB)")

// Storage keeps the values of plugins, a file of strings per plugin, so
// that what a plugin keeps survives reinstalls.
type Storage struct {
	Dir string

	mu    sync.Mutex
	files map[string]*store.File[map[string]string]
}

// NewStorage returns the storage of plugins kept in dir.
func NewStorage(dir string) *Storage {
	return &Storage{Dir: dir, files: map[string]*store.File[map[string]string]{}}
}

func (s *Storage) file(plugin string) (*store.File[map[string]string], error) {
	if !settings.ValidID(plugin) {
		return nil, errors.New("addons: bad plugin id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f := s.files[plugin]; f != nil {
		return f, nil
	}
	f, err := store.Open(filepath.Join(s.Dir, plugin+".json"), map[string]string{})
	if err != nil {
		return nil, err
	}
	s.files[plugin] = f
	return f, nil
}

// All returns every value a plugin keeps.
func (s *Storage) All(plugin string) (map[string]string, error) {
	f, err := s.file(plugin)
	if err != nil {
		return nil, err
	}
	values := f.Get()
	if values == nil {
		values = map[string]string{}
	}
	return values, nil
}

// Set keeps a value of a plugin.
func (s *Storage) Set(plugin, key, value string) error {
	f, err := s.file(plugin)
	if err != nil {
		return err
	}
	return f.Update(func(values *map[string]string) error {
		if *values == nil {
			*values = map[string]string{}
		}
		(*values)[key] = value
		size := 0
		for k, v := range *values {
			size += len(k) + len(v)
		}
		if size > maxStorage {
			return ErrStorageFull
		}
		return nil
	})
}

// Delete removes a value of a plugin.
func (s *Storage) Delete(plugin, key string) error {
	f, err := s.file(plugin)
	if err != nil {
		return err
	}
	return f.Update(func(values *map[string]string) error {
		delete(*values, key)
		return nil
	})
}
