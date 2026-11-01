package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/spotify"
)

// errNoStore is the credential store's error where there is none.
var errNoStore = fmt.Errorf("mygo: secrets: %w", errors.ErrUnsupported)

// errRefused is the credential store refusing, for a reason that isn't
// that it isn't available.
var errRefused = errors.New("mygo: secrets: credential manager: write: The parameter is incorrect.")

// fakeKeeper is the credential store: getErr, setErr and deleteErr make
// it fail.
type fakeKeeper struct {
	mu                        sync.Mutex
	kept                      map[string][]byte
	getErr, setErr, deleteErr error
	gets, sets                int
}

func newKeeper() *fakeKeeper { return &fakeKeeper{kept: map[string][]byte{}} }

func (k *fakeKeeper) Get(key string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.gets++
	if k.getErr != nil {
		return nil, k.getErr
	}
	v, ok := k.kept[key]
	if !ok {
		return nil, mygo.ErrSecretNotFound
	}
	return bytes.Clone(v), nil
}

func (k *fakeKeeper) Set(key string, v []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.setErr != nil {
		return k.setErr
	}
	k.sets++
	k.kept[key] = bytes.Clone(v)
	return nil
}

func (k *fakeKeeper) Delete(key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.deleteErr != nil {
		return k.deleteErr
	}
	delete(k.kept, key)
	return nil
}

// unavailableKeeper is a credential store that isn't there.
func unavailableKeeper() *fakeKeeper {
	k := newKeeper()
	k.getErr, k.setErr, k.deleteErr = errNoStore, errNoStore, errNoStore
	return k
}

// fakeSealer seals with the package's AES-GCM and a fixed key, the purpose
// inside: another purpose doesn't open it. fail makes it fail.
type fakeSealer struct{ fail error }

var fakeSealerKey = bytes.Repeat([]byte{3}, 32)

func (f fakeSealer) protect(data, purpose []byte) ([]byte, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	return seal(fakeSealerKey, append(bytes.Clone(purpose), data...))
}

func (f fakeSealer) unprotect(sealed, purpose []byte) ([]byte, error) {
	data, err := open(fakeSealerKey, sealed)
	if err != nil {
		return nil, err
	}
	rest, ok := bytes.CutPrefix(data, purpose)
	if !ok {
		return nil, errors.New("sealed for another purpose")
	}
	return rest, nil
}

// testSecret is a secretStore of the Last.fm connection in a folder of its
// own, with a sealer when seal isn't nil.
func testSecret(t *testing.T, keep keeper, seal sealer) *secretStore {
	t.Helper()
	dir := t.TempDir()
	s := &secretStore{key: lastfmSecret, plain: filepath.Join(dir, "lastfm.json"), keep: keep, seal: seal}
	if seal != nil {
		s.sealed = filepath.Join(dir, "lastfm.dpapi")
	}
	return s
}

func exists(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

func write(t *testing.T, f string, data []byte) {
	t.Helper()
	if err := os.WriteFile(f, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

var secret = []byte(`{"apiKey":"a","secret":"b","session":{"name":"test","key":"c"}}`)

func load(t *testing.T, s *secretStore) []byte {
	t.Helper()
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return got
}

// TestSecretInTheStore keeps the secret in the credential store, and
// removes the files left by runs without it.
func TestSecretInTheStore(t *testing.T) {
	k := newKeeper()
	s := testSecret(t, k, fakeSealer{})
	write(t, s.plain, []byte("old"))
	write(t, s.sealed, []byte("old"))
	if err := s.Save(secret); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k.kept[lastfmSecret], secret) || exists(s.plain) || exists(s.sealed) {
		t.Errorf("kept %q, the files left: plain %v, sealed %v", k.kept[lastfmSecret], exists(s.plain), exists(s.sealed))
	}
	if got := load(t, s); !bytes.Equal(got, secret) {
		t.Errorf("loaded %q", got)
	}
	// A plain file left beside the store goes.
	write(t, s.plain, []byte("old"))
	if got := load(t, s); !bytes.Equal(got, secret) || exists(s.plain) {
		t.Errorf("loaded %q, the plain file left: %v", got, exists(s.plain))
	}
}

// TestSecretWithoutAStore keeps the secret in a plain file readable by the
// user alone, where there is no store and no sealer, as on Linux without a
// Secret Service, and reads it from there without moving it.
func TestSecretWithoutAStore(t *testing.T) {
	k := unavailableKeeper()
	s := testSecret(t, k, nil)
	if _, err := s.Load(); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("nothing kept: %v", err)
	}
	if err := s.Save(secret); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.plain)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("the file's mode is %v", info.Mode().Perm())
	}
	if got := load(t, s); !bytes.Equal(got, secret) || !exists(s.plain) {
		t.Errorf("loaded %q, the file kept: %v", got, exists(s.plain))
	}
}

// TestSecretRefused gives the store's error back when it refuses for
// another reason than not being available, and keeps no file.
func TestSecretRefused(t *testing.T) {
	for _, seal := range []sealer{nil, fakeSealer{}} {
		k := newKeeper()
		k.setErr = errRefused
		s := testSecret(t, k, seal)
		if err := s.Save(secret); !errors.Is(err, errRefused) {
			t.Errorf("sealer %v: saving gave %v", seal != nil, err)
		}
		if exists(s.plain) || (s.sealed != "" && exists(s.sealed)) {
			t.Errorf("sealer %v: a file was written", seal != nil)
		}
		// Nor does Load read a file then.
		write(t, s.plain, secret)
		k.getErr = errRefused
		if _, err := s.Load(); !errors.Is(err, errRefused) {
			t.Errorf("sealer %v: loading gave %v", seal != nil, err)
		}
		if !exists(s.plain) {
			t.Errorf("sealer %v: the plain file went", seal != nil)
		}
	}
}

// TestSecretSealed seals the secret for the user where the store isn't
// available.
func TestSecretSealed(t *testing.T) {
	k := unavailableKeeper()
	s := testSecret(t, k, fakeSealer{})
	if _, err := s.Load(); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Errorf("nothing kept: %v", err)
	}
	write(t, s.plain, []byte("old"))
	if err := s.Save(secret); err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(s.sealed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("apiKey")) || exists(s.plain) {
		t.Errorf("the secret is readable: sealed %q, the plain file left: %v", sealed, exists(s.plain))
	}
	if got := load(t, s); !bytes.Equal(got, secret) {
		t.Errorf("loaded %q", got)
	}

	// Where sealing fails too, the user hears why in words of their own.
	k = unavailableKeeper()
	s = testSecret(t, k, fakeSealer{fail: errors.New("dpapi: Access is denied.")})
	if err := s.Save(secret); err == nil || !strings.HasPrefix(err.Error(), "Windows did not let MixMusic keep your sign-in: ") {
		t.Errorf("saving gave %v", err)
	}
}

// TestSecretMoves moves a secret found in a plain file into the store, or
// into the sealed file where the store isn't available; without a sealer,
// only once a store is there.
func TestSecretMoves(t *testing.T) {
	t.Run("into the store", func(t *testing.T) {
		for _, seal := range []sealer{nil, fakeSealer{}} {
			k := newKeeper()
			s := testSecret(t, k, seal)
			write(t, s.plain, secret)
			if got := load(t, s); !bytes.Equal(got, secret) {
				t.Errorf("loaded %q", got)
			}
			if !bytes.Equal(k.kept[lastfmSecret], secret) || exists(s.plain) {
				t.Errorf("sealer %v: the store kept %q, the plain file left: %v", seal != nil, k.kept[lastfmSecret], exists(s.plain))
			}
		}
	})
	t.Run("into the sealed file", func(t *testing.T) {
		s := testSecret(t, unavailableKeeper(), fakeSealer{})
		write(t, s.plain, secret)
		if got := load(t, s); !bytes.Equal(got, secret) || exists(s.plain) || !exists(s.sealed) {
			t.Errorf("loaded %q, plain %v, sealed %v", got, exists(s.plain), exists(s.sealed))
		}
		if got := load(t, s); !bytes.Equal(got, secret) {
			t.Errorf("loaded %q from the sealed file", got)
		}
	})
	t.Run("sealing fails", func(t *testing.T) {
		s := testSecret(t, unavailableKeeper(), fakeSealer{fail: errors.New("dpapi: Access is denied.")})
		write(t, s.plain, secret)
		if got := load(t, s); !bytes.Equal(got, secret) || !exists(s.plain) {
			t.Errorf("loaded %q, the plain file kept: %v", got, exists(s.plain))
		}
	})
	t.Run("no store", func(t *testing.T) {
		k := unavailableKeeper()
		s := testSecret(t, k, nil)
		write(t, s.plain, secret)
		if got := load(t, s); !bytes.Equal(got, secret) || !exists(s.plain) {
			t.Errorf("loaded %q, the plain file kept: %v", got, exists(s.plain))
		}
	})
	t.Run("from the sealed file", func(t *testing.T) {
		// The sealed file was kept while the store wasn't available: once
		// it is, the secret goes there, over the older one it kept.
		k := unavailableKeeper()
		s := testSecret(t, k, fakeSealer{})
		if err := s.Save(secret); err != nil {
			t.Fatal(err)
		}
		k.getErr, k.setErr, k.deleteErr = nil, nil, nil
		k.kept[lastfmSecret] = []byte("older")
		if got := load(t, s); !bytes.Equal(got, secret) {
			t.Errorf("loaded %q", got)
		}
		if !bytes.Equal(k.kept[lastfmSecret], secret) || exists(s.sealed) {
			t.Errorf("the store kept %q, the sealed file left: %v", k.kept[lastfmSecret], exists(s.sealed))
		}
	})
	t.Run("sealed, the store still refusing", func(t *testing.T) {
		k := unavailableKeeper()
		s := testSecret(t, k, fakeSealer{})
		if err := s.Save(secret); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(s.sealed)
		load(t, s)
		after, _ := os.ReadFile(s.sealed)
		if !bytes.Equal(before, after) || k.sets != 0 {
			t.Errorf("the sealed file was written again, the store asked %d times", k.sets)
		}
	})
}

// TestSecretSealedElsewhere takes a sealed file that doesn't open, as one
// from another account, as no secret, and keeps it until a save replaces
// it.
func TestSecretSealedElsewhere(t *testing.T) {
	s := testSecret(t, unavailableKeeper(), fakeSealer{})
	write(t, s.sealed, []byte("sealed for someone else"))
	if _, err := s.Load(); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Errorf("loading gave %v", err)
	}
	if !exists(s.sealed) {
		t.Error("the file went")
	}
	if err := s.Save(secret); err != nil {
		t.Fatal(err)
	}
	if got := load(t, s); !bytes.Equal(got, secret) {
		t.Errorf("loaded %q", got)
	}
}

// TestSecretClear forgets the secret everywhere it was kept.
func TestSecretClear(t *testing.T) {
	k := newKeeper()
	s := testSecret(t, k, fakeSealer{})
	k.kept[lastfmSecret] = secret
	write(t, s.plain, secret)
	write(t, s.sealed, secret)
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if len(k.kept) != 0 || exists(s.plain) || exists(s.sealed) {
		t.Errorf("left %d secrets, plain %v, sealed %v", len(k.kept), exists(s.plain), exists(s.sealed))
	}
	if err := s.Clear(); err != nil {
		t.Errorf("clearing again: %v", err)
	}
	if err := testSecret(t, unavailableKeeper(), fakeSealer{}).Clear(); err != nil {
		t.Errorf("clearing without a store: %v", err)
	}
}

// TestJSONSecret keeps the Spotify token and the Last.fm connection as
// JSON.
func TestJSONSecret(t *testing.T) {
	tokens := tokenStore{testSecret(t, newKeeper(), nil)}
	token := spotify.Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Unix(1700000000, 0).UTC()}
	if err := tokens.Save(token); err != nil {
		t.Fatal(err)
	}
	if got, err := tokens.Load(); err != nil || got.AccessToken != "a" || got.RefreshToken != "r" || !got.Expiry.Equal(token.Expiry) {
		t.Errorf("loaded %+v, %v", got, err)
	}
	lastfm := lastfmStore{testSecret(t, unavailableKeeper(), fakeSealer{})}
	c := lastfmCredentials{APIKey: "a", Secret: "b"}
	c.Session.Name, c.Session.Key = "test", "c"
	if err := lastfm.Save(c); err != nil {
		t.Fatal(err)
	}
	if got, err := lastfm.Load(); err != nil || got != c {
		t.Errorf("loaded %+v, %v", got, err)
	}
	if err := lastfm.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := lastfm.Load(); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Errorf("loaded after clearing: %v", err)
	}
}
