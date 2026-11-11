package stream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// TestSourceTellsLocal tells, of the audio a Source reads, whether it
// comes from this computer: from what is kept, as a download, or the
// cache, rather than from YouTube.
func TestSourceTellsLocal(t *testing.T) {
	var minGen atomic.Int32
	size := 5000
	srv, _ := upstream(t, size, &minGen)
	read := func(t *testing.T, p *Proxy) *Source {
		t.Helper()
		s, err := p.Open(context.Background(), testID, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		if _, err := io.ReadAll(s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	t.Run("streamed", func(t *testing.T) {
		if read(t, testProxy(srv, size)).Local() {
			t.Error("a stream from YouTube is local")
		}
	})
	t.Run("kept", func(t *testing.T) {
		p := testProxy(srv, size)
		p.Kept = keptStore{testID: "kept audio"}
		if !read(t, p).Local() {
			t.Error("a download isn't local")
		}
	})
	t.Run("cached", func(t *testing.T) {
		p := testProxy(srv, size)
		p.Cache = NewCache(t.TempDir(), 1<<20)
		read(t, p)
		deadline := time.Now().Add(5 * time.Second)
		for !p.Cache.Has(testID) {
			if time.Now().After(deadline) {
				t.Fatal("the track read whole is not in the cache")
			}
			time.Sleep(10 * time.Millisecond)
		}
		// Another program, as an antivirus, may hold a file a moment after it
		// came into the cache, which then can't serve it: the track streams
		// meanwhile. Once the cache serves it, it is local.
		deadline = time.Now().Add(5 * time.Second)
		for !read(t, p).Local() {
			if time.Now().After(deadline) {
				t.Fatal("a track from the cache isn't local")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	t.Run("kept no longer", func(t *testing.T) {
		// A download whose file is gone streams instead.
		p := testProxy(srv, size)
		p.Kept = goneStore{}
		if read(t, p).Local() {
			t.Error("a stream in place of a download gone is local")
		}
	})
}

// goneStore keeps the audio of every video, whose file is gone.
type goneStore struct{}

func (goneStore) Serve(http.ResponseWriter, *http.Request, string) bool { return false }

// TestSourceKeptBreaksOff fails a read of a download whose file breaks off
// as the proxy serves it, and tells it's local, as the engine words it.
func TestSourceKeptBreaksOff(t *testing.T) {
	p := NewProxy(NewResolver(nil, ""))
	p.Resolver.Run = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("no network") }
	p.Kept = brokenStore{}
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := io.ReadAll(s); err == nil {
		t.Fatal("a download cut short read whole")
	}
	if !s.Local() {
		t.Error("a download cut short isn't local")
	}
}

// brokenStore keeps audio of 1000 bytes, of which it serves 100: its file
// can't be read past them.
type brokenStore struct{}

func (brokenStore) Serve(w http.ResponseWriter, _ *http.Request, _ string) bool {
	w.Header().Set("Content-Length", "1000")
	w.WriteHeader(http.StatusOK)
	w.Write(make([]byte, 100))
	return true
}
