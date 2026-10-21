package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSourceReadsWhole(t *testing.T) {
	var minGen atomic.Int32
	size := chunkSize + 54321
	srv, data := upstream(t, size, &minGen)
	p := testProxy(srv, size)
	p.Cache = NewCache(t.TempDir(), 1<<30)
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := io.ReadAll(s)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("%d bytes of %d, %v", len(got), size, err)
	}
	// The proxy keeps the track once it sent the end, which the reader may
	// read first.
	deadline := time.Now().Add(5 * time.Second)
	for !p.Cache.Has(testID) {
		if time.Now().After(deadline) {
			t.Fatal("the track read whole is not in the cache")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// From the cache, the same.
	s2, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if got, err := io.ReadAll(s2); err != nil || !bytes.Equal(got, data) {
		t.Errorf("from the cache: %d bytes, %v", len(got), err)
	}
}

func TestSourceSeeks(t *testing.T) {
	var minGen atomic.Int32
	size := 100000
	srv, data := upstream(t, size, &minGen)
	s, err := testProxy(srv, size).Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	buf := make([]byte, 1000)
	for _, at := range []int64{90000, 10, 50000, 0, 99000} {
		if pos, err := s.Seek(at, io.SeekStart); pos != at || err != nil {
			t.Fatalf("Seek(%d) = %d, %v", at, pos, err)
		}
		if _, err := io.ReadFull(s, buf); err != nil || !bytes.Equal(buf, data[at:at+1000]) {
			t.Fatalf("at %d: %v", at, err)
		}
	}
	if pos, err := s.Seek(-500, io.SeekEnd); pos != int64(size)-500 || err != nil {
		t.Errorf("Seek from the end = %d, %v", pos, err)
	}
	if _, err := s.Seek(-1, io.SeekStart); err == nil {
		t.Error("a negative position seeked")
	}
}

func TestSourceRefused(t *testing.T) {
	var minGen atomic.Int32
	minGen.Store(1 << 30) // every URL expired
	srv, _ := upstream(t, 1000, &minGen)
	_, err := testProxy(srv, 1000).Open(context.Background(), testID, nil)
	if err == nil {
		t.Fatal("a refused stream opened")
	}
	if strings.HasPrefix(err.Error(), "stream:") || err.Error() == errLoad.Error() {
		t.Errorf("the error %q doesn't say why in plain words", err)
	}
}

// stallingUpstream serves its first chunk, then holds the next ones until
// release closes.
func stallingUpstream(t *testing.T, size int, release chan struct{}) *httptest.Server {
	t.Helper()
	data := make([]byte, size)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Range"), "bytes=0-") {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSourceInterruptsAWaitingRead(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	size := chunkSize + 1000
	srv := stallingUpstream(t, size, release)
	var interrupt atomic.Bool
	s, err := testProxy(srv, size).Open(context.Background(), testID, interrupt.Load)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Seek(chunkSize+10, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	read := make(chan error, 1)
	go func() {
		_, err := s.Read(make([]byte, 10))
		read <- err
	}()
	select {
	case err := <-read:
		t.Fatalf("the read beyond what came returned %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	interrupt.Store(true)
	s.Wake()
	select {
	case err := <-read:
		if !errors.Is(err, ErrInterrupted) {
			t.Errorf("the interrupted read returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the interrupted read still waits")
	}
}

func TestSourceCloseStopsTheDownload(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	size := chunkSize + 1000
	var canceled atomic.Bool
	inner := stallingUpstream(t, size, release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, inner.URL+r.URL.String(), nil)
		req.Header = r.Header
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			canceled.Store(r.Context().Err() != nil)
			return
		}
		defer resp.Body.Close()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}))
	defer srv.Close()
	s, err := testProxy(srv, size).Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // the second chunk waits upstream
	s.Close()
	if _, err := s.Read(make([]byte, 10)); err == nil {
		t.Error("read after Close")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !canceled.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the download goes on after Close")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// heldUpstream serves size bytes of audio with range requests, holding the
// first request from byte from until the test ends; holding closes once it
// holds it.
func heldUpstream(t *testing.T, size int, from int64) (srv *httptest.Server, data []byte, holding chan struct{}) {
	t.Helper()
	data = make([]byte, size)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	release := make(chan struct{})
	holding = make(chan struct{})
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
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	return srv, data, holding
}

// within fails the test unless read returns within a few seconds.
func within(t *testing.T, what string, read func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- read() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%s waits for the download", what)
	}
}

func TestSourceJumpsAhead(t *testing.T) {
	size := 3*chunkSize + 5000
	// The download's second chunk never comes.
	srv, data, holding := heldUpstream(t, size, chunkSize)
	s, err := testProxy(srv, size).Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	select {
	case <-holding:
	case <-time.After(5 * time.Second):
		t.Fatal("the download didn't ask for its second chunk")
	}
	readAt := func(at int64) {
		t.Helper()
		if _, err := s.Seek(at, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 1000)
		within(t, fmt.Sprint("the read at ", at), func() error {
			if _, err := io.ReadFull(s, buf); err != nil {
				return err
			}
			if !bytes.Equal(buf, data[at:at+1000]) {
				return errors.New("other bytes")
			}
			return nil
		})
	}
	// Far ahead, the download starts again there, and goes on to the end.
	readAt(2*chunkSize + 100)
	readAt(int64(size) - 1000)
	// The first download stopped: what it left out comes when read.
	readAt(chunkSize + 10)
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

func TestSourcePutKeepsSpansApart(t *testing.T) {
	const size = 1000
	rng := rand.New(rand.NewPCG(3, 4))
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	for round := range 500 {
		s := &Source{}
		have := make([]bool, size)
		for range 1 + rng.IntN(12) {
			off := rng.IntN(size)
			n := rng.IntN(min(size-off, 200) + 1)
			s.put(int64(off), data[off:off+n])
			for i := off; i < off+n; i++ {
				have[i] = true
			}
		}
		got := make([]bool, size)
		for i, sp := range s.spans {
			if len(sp.data) == 0 || (i > 0 && s.spans[i-1].end() >= sp.off) {
				t.Fatalf("round %d: span %d at %d of %d bytes, after one ending at %d", round, i, sp.off, len(sp.data), s.spans[max(i-1, 0)].end())
			}
			if !bytes.Equal(sp.data, data[sp.off:sp.end()]) {
				t.Fatalf("round %d: span %d holds other bytes", round, i)
			}
			for j := sp.off; j < sp.end(); j++ {
				got[j] = true
			}
		}
		if !slices.Equal(got, have) {
			t.Fatalf("round %d: the spans hold other bytes than those put", round)
		}
		for off := range size {
			end := off
			for end < size && have[end] {
				end++
			}
			if r := s.reach(int64(off)); r != int64(end) {
				t.Fatalf("round %d: reach(%d) = %d, want %d", round, off, r, end)
			}
		}
	}
}
