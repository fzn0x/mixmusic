package app

import (
	"bytes"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSealOpen(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	data := []byte(`[{"name":"SAPISID","value":"x"}]`)
	sealed, err := seal(key, data)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("SAPISID")) {
		t.Fatal("the session is readable in the file")
	}
	got, err := open(key, sealed)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("open = %q, %v", got, err)
	}
	if _, err := open(bytes.Repeat([]byte{8}, 32), sealed); err == nil {
		t.Error("another key opened the session")
	}
	if _, err := open(key, sealed[:4]); err == nil {
		t.Error("a damaged file opened")
	}
}

// testYouTube is a youtubeStore in a folder of its own.
func testYouTube(t *testing.T, keep keeper, seal sealer) *youtubeStore {
	t.Helper()
	dir := t.TempDir()
	key := &secretStore{key: youtubeKeySecret, keep: keep, seal: seal}
	if seal != nil {
		key.sealed = filepath.Join(dir, "youtube-session-key.dpapi")
	}
	return &youtubeStore{dir: dir, key: key}
}

var testCookies = []*http.Cookie{{Name: "SAPISID", Value: "x", Domain: ".youtube.com", Path: "/", Secure: true}}

func loadCookies(t *testing.T, s *youtubeStore) []*http.Cookie {
	t.Helper()
	session, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := session.cookies
	if len(got) != 1 || got[0].Name != "SAPISID" || got[0].Value != "x" {
		t.Fatalf("loaded %v", got)
	}
	return got
}

// TestYouTubeStore keeps the session in youtube-session.bin, with its key
// in the store, or sealed where the store isn't available, and in a plain
// file only where there's neither.
func TestYouTubeStore(t *testing.T) {
	t.Run("store", func(t *testing.T) {
		k := newKeeper()
		s := testYouTube(t, k, fakeSealer{})
		if err := s.Save(youtubeSession{cookies: testCookies}); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(youtubeSession{cookies: testCookies}); err != nil {
			t.Fatal(err)
		}
		if len(k.kept[youtubeKeySecret]) != 32 || !exists(s.sealed()) || exists(s.plain()) {
			t.Errorf("the key has %d bytes, the session files: sealed %v, plain %v", len(k.kept[youtubeKeySecret]), exists(s.sealed()), exists(s.plain()))
		}
		if k.gets != 1 {
			t.Errorf("the key was read %d times for two saves", k.gets)
		}
		s.cached = nil // the next start
		loadCookies(t, s)
	})
	t.Run("sealed key", func(t *testing.T) {
		s := testYouTube(t, unavailableKeeper(), fakeSealer{})
		if err := s.Save(youtubeSession{cookies: testCookies}); err != nil {
			t.Fatal(err)
		}
		if !exists(s.key.sealed) || !exists(s.sealed()) || exists(s.plain()) {
			t.Errorf("sealed key %v, session sealed %v, plain %v", exists(s.key.sealed), exists(s.sealed()), exists(s.plain()))
		}
		s.cached = nil
		loadCookies(t, s)
	})
	t.Run("no store", func(t *testing.T) {
		s := testYouTube(t, unavailableKeeper(), nil)
		if err := s.Save(youtubeSession{cookies: testCookies}); err != nil {
			t.Fatal(err)
		}
		if !exists(s.plain()) || exists(s.sealed()) {
			t.Errorf("plain %v, sealed %v", exists(s.plain()), exists(s.sealed()))
		}
		loadCookies(t, s)
		if !exists(s.plain()) {
			t.Error("the plain file went with no store to move to")
		}
	})
	t.Run("moves", func(t *testing.T) {
		for _, c := range []struct {
			name string
			keep *fakeKeeper
			seal sealer
		}{{"into the store", newKeeper(), nil}, {"into the sealed key", unavailableKeeper(), fakeSealer{}}} {
			s := testYouTube(t, c.keep, c.seal)
			write(t, s.plain(), []byte(`[{"name":"SAPISID","value":"x","domain":".youtube.com","path":"/","secure":true,"httpOnly":false}]`))
			loadCookies(t, s)
			if exists(s.plain()) || !exists(s.sealed()) {
				t.Errorf("%s: plain %v, sealed %v", c.name, exists(s.plain()), exists(s.sealed()))
			}
			s.cached = nil
			loadCookies(t, s)
		}
	})
	// The account the session uses stays beside its cookies; the first
	// account's session is kept as its cookies alone, as before.
	t.Run("account", func(t *testing.T) {
		s := testYouTube(t, newKeeper(), nil)
		if err := s.Save(youtubeSession{cookies: testCookies, authUser: 2}); err != nil {
			t.Fatal(err)
		}
		s.cached = nil
		if got, err := s.Load(); err != nil || got.authUser != 2 {
			t.Fatalf("loaded the account %d, %v", got.authUser, err)
		}
		loadCookies(t, s)
		s.Save(youtubeSession{cookies: testCookies})
		data, err := s.load()
		if err != nil || !bytes.HasPrefix(data, []byte(`[{"name":"SAPISID"`)) {
			t.Errorf("the first account's session is kept as %s, %v", data, err)
		}
		if got := loadCookies(t, s); got[0].Domain != ".youtube.com" {
			t.Errorf("loaded %+v", got[0])
		}
	})
	t.Run("clear", func(t *testing.T) {
		k := newKeeper()
		s := testYouTube(t, k, fakeSealer{})
		if err := s.Save(youtubeSession{cookies: testCookies}); err != nil {
			t.Fatal(err)
		}
		if err := s.Clear(); err != nil {
			t.Fatal(err)
		}
		if len(k.kept) != 0 || exists(s.sealed()) || s.cached != nil {
			t.Errorf("left %d secrets, sealed %v, a cached key %v", len(k.kept), exists(s.sealed()), s.cached != nil)
		}
		if _, err := s.Load(); err == nil {
			t.Error("a session was loaded after clearing")
		}
	})
}
