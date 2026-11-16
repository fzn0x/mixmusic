package stream

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// countingUpstream serves size bytes of audio with range requests, as
// upstream does, counting the requests and the bytes it sends.
func countingUpstream(t *testing.T, size int) (srv *httptest.Server, data []byte, asks, sent *atomic.Int64) {
	t.Helper()
	data = make([]byte, size)
	for i := range data {
		data[i] = byte(i * 13)
	}
	asks, sent = new(atomic.Int64), new(atomic.Int64)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks.Add(1)
		http.ServeContent(countingWriter{w, sent}, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv, data, asks, sent
}

type countingWriter struct {
	http.ResponseWriter
	n *atomic.Int64
}

func (w countingWriter) Write(b []byte) (int, error) {
	w.n.Add(int64(len(b)))
	return w.ResponseWriter.Write(b)
}

// TestRangesReadOnlyWhatIsRead reads a stretch far into the audio with
// two blocks of it, the first and that of the stretch, and keeps none of
// it in the cache.
func TestRangesReadOnlyWhatIsRead(t *testing.T) {
	size := 3 << 20
	srv, data, _, sent := countingUpstream(t, size)
	p := testProxy(srv, size)
	p.Cache = NewCache(t.TempDir(), 1<<30)
	r, err := p.OpenRanges(context.Background(), testID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if end, err := r.Seek(0, io.SeekEnd); err != nil || end != int64(size) {
		t.Fatalf("size = %d, %v", end, err)
	}
	if _, err := r.Seek(2_000_000, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 1000)
	if _, err := io.ReadFull(r, got); err != nil || !bytes.Equal(got, data[2_000_000:2_001_000]) {
		t.Fatalf("read %v", err)
	}
	if n := sent.Load(); n > 2*rangeFirst {
		t.Errorf("YouTube sent %d bytes for 1000", n)
	}
	if p.Cache.Has(testID) {
		t.Error("a stretch went to the cache")
	}
}

// TestRangesReadOnInLargerBlocks reads the audio through in blocks that
// grow as the reads go on, not a request a block of the first size.
func TestRangesReadOnInLargerBlocks(t *testing.T) {
	size := 3 << 20
	srv, data, asks, _ := countingUpstream(t, size)
	p := testProxy(srv, size)
	r, err := p.OpenRanges(context.Background(), testID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("read %d bytes of %d, %v", len(got), size, err)
	}
	if n := asks.Load(); n > 8 {
		t.Errorf("%d requests for %d bytes", n, size)
	}
	// Reading back a little reads what came.
	before := asks.Load()
	if _, err := r.Seek(int64(size-100), io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if b, err := io.ReadAll(r); err != nil || !bytes.Equal(b, data[size-100:]) || asks.Load() != before {
		t.Errorf("read back: %d bytes, %v, %d more requests", len(b), err, asks.Load()-before)
	}
}

// TestRangesTellWhyTheyFail opens with why the proxy couldn't, in plain
// words, and stops reading once closed.
func TestRangesTellWhyTheyFail(t *testing.T) {
	var minGen atomic.Int32
	minGen.Store(1 << 30) // every URL expired
	srv, _ := upstream(t, 1000, &minGen)
	_, err := testProxy(srv, 1000).OpenRanges(context.Background(), testID)
	if err == nil || !strings.HasPrefix(err.Error(), "YouTube refused to send its audio") {
		t.Fatalf("refused: %v", err)
	}

	minGen.Store(0)
	r, err := testProxy(srv, 1000).OpenRanges(context.Background(), testID)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	if _, err := r.Read(make([]byte, 10)); err == nil {
		t.Error("read once closed")
	}
}
