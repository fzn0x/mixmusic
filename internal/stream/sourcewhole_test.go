package stream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// gatedUpstream serves size bytes of audio with range requests, holding
// the first request from byte from until release is closed; holding closes
// once it holds it.
func gatedUpstream(t *testing.T, size int, from int64) (srv *httptest.Server, data []byte, holding, release chan struct{}) {
	t.Helper()
	data = make([]byte, size)
	for i := range data {
		data[i] = byte(i * 13)
	}
	holding, release = make(chan struct{}), make(chan struct{})
	var held atomic.Bool
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Range"), fmt.Sprintf("bytes=%d-", from)) && held.CompareAndSwap(false, true) {
			close(holding)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv, data, holding, release
}

// TestSourceKeptWholeGoesToTheCache reads far ahead of the download, as
// the engine does past a part it passes over, with the source kept whole:
// the read is served at once by a download from there, while the download
// from the start goes on to the end, and the track goes to the cache.
func TestSourceKeptWholeGoesToTheCache(t *testing.T) {
	size := 3*chunkSize + 5000
	srv, data, holding, release := gatedUpstream(t, size, chunkSize)
	p := testProxy(srv, size)
	p.Cache = NewCache(t.TempDir(), 1<<30)
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.KeepWhole()
	select {
	case <-holding:
	case <-time.After(5 * time.Second):
		t.Fatal("the download didn't ask for its second chunk")
	}
	at := int64(2*chunkSize + 100)
	if _, err := s.Seek(at, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1000)
	within(t, "the read far ahead", func() error {
		if _, err := io.ReadFull(s, buf); err != nil {
			return err
		}
		if !bytes.Equal(buf, data[at:at+1000]) {
			return fmt.Errorf("other bytes")
		}
		return nil
	})
	close(release)
	deadline := time.Now().Add(10 * time.Second)
	for !p.Cache.Has(testID) {
		if time.Now().After(deadline) {
			t.Fatal("the track is not in the cache: the download from the start stopped")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := s.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	within(t, "reading it whole", func() error {
		got, err := io.ReadAll(s)
		if err == nil && !bytes.Equal(got, data) {
			err = fmt.Errorf("%d other bytes of %d", len(got), size)
		}
		return err
	})
}
