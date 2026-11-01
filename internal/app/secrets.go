package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/store"
)

// keeper is the system's credential store, mygo.Secrets: tests fake it.
type keeper interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

// sealer encrypts a secret for the user, for a file to keep it where the
// credential store isn't available: DPAPI on Windows. purpose ties what it
// seals to one secret; it is no secret itself.
type sealer interface {
	protect(data, purpose []byte) ([]byte, error)
	unprotect(sealed, purpose []byte) ([]byte, error)
}

// secretStore keeps a secret in the system's credential store. Where the
// store isn't available, it keeps it in a file: sealed for the user where
// the system can (DPAPI on Windows, userSealer), else readable by the user
// alone, as on Linux without a Secret Service. A secret it finds in a file
// moves into the store once the store can keep it, and a save that
// reaches the store removes the files left (#104).
type secretStore struct {
	// key is the secret's key in the credential store; plain the file of
	// a system with no store and no sealer, "" for none; sealed the file
	// seal seals it into, "" without a sealer.
	key, plain, sealed string
	keep               keeper
	seal               sealer

	mu sync.Mutex
	// told keeps what was logged, once a run.
	told map[string]bool
}

// newSecretStore keeps the secret of key, with its files in dir: plain,
// the plain file, "" for none, and sealed, the file sealed for the user
// where the system has a sealer.
func newSecretStore(dir, key, plain, sealed string) *secretStore {
	s := &secretStore{key: key, keep: mygo.Secrets, seal: userSealer}
	if plain != "" {
		s.plain = filepath.Join(dir, plain)
	}
	if userSealer != nil {
		s.sealed = filepath.Join(dir, sealed)
	}
	return s
}

// purpose is the sealer's purpose for the secret.
func (s *secretStore) purpose() []byte { return []byte("MixMusic " + s.key) }

// Load returns the secret, or mygo.ErrSecretNotFound. One found in a file
// moves into the credential store when the store answers that it has
// none: kept by a version, or a system, without a store. With a sealer, a
// plain file moves into the sealed one where the store isn't available.
// The sealed file, written only while the store wasn't available and
// removed once a save reaches it, is newer than what the store holds.
// Without a sealer, where the store isn't available and nothing is kept,
// it fails with the store's error.
func (s *secretStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept, err := s.keep.Get(s.key)
	notFound := errors.Is(err, mygo.ErrSecretNotFound)
	if err != nil && !notFound && !unavailable(err) {
		return nil, err
	}
	if data, ok := s.readSealed(); ok {
		if err == nil || notFound {
			// The store can keep it again.
			if serr := s.keep.Set(s.key, data); serr == nil {
				removeFile(s.sealed)
			}
		}
		removeFile(s.plain)
		return data, nil
	}
	if err == nil {
		removeFile(s.plain) // left by a run without the store
		return kept, nil
	}
	if s.plain != "" {
		data, rerr := os.ReadFile(s.plain)
		switch {
		case rerr == nil:
			if s.seal != nil || notFound {
				if serr := s.save(data); serr != nil {
					log.Printf("mixmusic: moving %s: %v", filepath.Base(s.plain), serr)
				} else {
					log.Printf("mixmusic: moved %s out of %s, in plain text", s.key, filepath.Base(s.plain))
				}
			}
			return data, nil
		case !errors.Is(rerr, os.ErrNotExist):
			return nil, rerr
		}
	}
	if s.seal != nil {
		// The sealed file stands in for the store, and has nothing.
		return nil, mygo.ErrSecretNotFound
	}
	return nil, err
}

// Save keeps the secret in the credential store, or, where the store
// isn't available, in the sealed file, or in the plain one without a
// sealer.
func (s *secretStore) Save(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(data)
}

func (s *secretStore) save(data []byte) error {
	err := s.keep.Set(s.key, data)
	switch {
	case err == nil:
		removeFile(s.sealed)
		removeFile(s.plain)
		return nil
	case !unavailable(err):
		return err
	case s.seal != nil:
		if serr := s.writeSealed(data); serr != nil {
			log.Printf("mixmusic: the Credential Manager can't keep %s: %v", s.key, err)
			return fmt.Errorf("Windows did not let MixMusic keep your sign-in: %w", serr)
		}
		removeFile(s.plain)
		s.tell("store", "mixmusic: the Credential Manager can't keep %s (%v): keeping it encrypted for the Windows user in %s", s.key, err, filepath.Base(s.sealed))
		return nil
	case s.plain != "":
		// os.CreateTemp makes it readable by the user alone.
		return store.WriteFile(s.plain, data)
	}
	return err
}

// Clear forgets the secret, in the store and in its files.
func (s *secretStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.keep.Delete(s.key)
	if errors.Is(err, mygo.ErrSecretNotFound) || unavailable(err) {
		err = nil
	}
	for _, f := range []string{s.sealed, s.plain} {
		if f == "" {
			continue
		}
		if rmErr := os.Remove(f); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}
	return err
}

// readSealed reads the sealed file. One that doesn't open, damaged or
// sealed for another account or computer, is as good as none: it stays
// until a save replaces it.
func (s *secretStore) readSealed() ([]byte, bool) {
	if s.seal == nil {
		return nil, false
	}
	sealed, err := os.ReadFile(s.sealed)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.tell("read", "mixmusic: reading %s: %v", filepath.Base(s.sealed), err)
		}
		return nil, false
	}
	data, err := s.seal.unprotect(sealed, s.purpose())
	if err != nil {
		s.tell("open", "mixmusic: %s doesn't open, as if signed out: %v", filepath.Base(s.sealed), err)
		return nil, false
	}
	return data, true
}

func (s *secretStore) writeSealed(data []byte) error {
	sealed, err := s.seal.protect(data, s.purpose())
	if err != nil {
		return err
	}
	return store.WriteFile(s.sealed, sealed)
}

// removeFile removes a file a secret was kept in, if any.
func removeFile(f string) {
	if f == "" {
		return
	}
	if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("mixmusic: removing %s: %v", filepath.Base(f), err)
	}
}

// tell logs what, once a run.
func (s *secretStore) tell(what, format string, args ...any) {
	if s.told[what] {
		return
	}
	if s.told == nil {
		s.told = map[string]bool{}
	}
	s.told[what] = true
	log.Printf(format, args...)
}

// jsonSecret is a secretStore of a value kept as JSON.
type jsonSecret[T any] struct{ secret *secretStore }

func (j jsonSecret[T]) Load() (T, error) {
	var v T
	data, err := j.secret.Load()
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal(data, &v)
}

func (j jsonSecret[T]) Save(v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return j.secret.Save(data)
}

func (j jsonSecret[T]) Clear() error { return j.secret.Clear() }
