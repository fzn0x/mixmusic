package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeLive is the media server of a live stream, and yt-dlp, which finds
// it: a playlist of a window of segments, which the test moves on, and the
// segments, each of bytes of its own. Its playlist's URL expires, in its
// path, as YouTube's do.
type fakeLive struct {
	t   *testing.T
	srv *httptest.Server
	// runs counts yt-dlp's runs, each of which gives a playlist URL of
	// another expire.
	runs atomic.Int32

	mu sync.Mutex
	// The playlist holds the segments first to end-1, and ends there when
	// ended.
	first, end int64
	ended      bool
	// expire is that of the playlist URL yt-dlp found last.
	expire int64
	// refused are the expires of the playlist URLs the media server
	// refuses, as once they expired.
	refused map[int64]bool
	// segment, when set, serves a segment in place of the fake.
	segment func(w http.ResponseWriter, r *http.Request, seq int64) bool
	// playlist, when set, serves the playlist in place of the fake.
	playlist func(w http.ResponseWriter, r *http.Request) bool
	loads    int     // of the playlist
	fetched  []int64 // the segments served, in order
	notLive  bool    // yt-dlp finds the video no longer live
}

func newFakeLive(t *testing.T, first, end int64) *fakeLive {
	f := &fakeLive{t: t, first: first, end: end, refused: map[int64]bool{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// segmentBytes are the bytes of segment seq: its number, and a few kB.
func segmentBytes(seq int64) []byte {
	return fmt.Appendf(bytes.Repeat([]byte{'.'}, 3000), "|segment %d|", seq)
}

// proxy returns a proxy of the stream, as found by yt-dlp, which waits
// and gives up quickly, and reloads the playlist every 20 ms.
func (f *fakeLive) proxy() *Proxy {
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		f.runs.Add(1)
		f.mu.Lock()
		f.expire = max(f.expire+1, time.Now().Add(6*time.Hour).Unix())
		expire, notLive := f.expire, f.notLive
		f.mu.Unlock()
		if notLive {
			return fmt.Appendf(nil, `{"url": "%s/videoplayback?expire=%d", "ext": "webm", "acodec": "opus", "filesize": 100}`,
				f.srv.URL, expire), nil
		}
		return fmt.Appendf(nil, `{"url": "%s/hls_playlist/expire/%d/index.m3u8", "protocol": "m3u8_native", "live_status": "is_live",
			"ext": "mp4", "http_headers": {"User-Agent": "test"}}`, f.srv.URL, expire), nil
	}
	p := NewProxy(r)
	p.stall, p.reconnect, p.backoff = 300*time.Millisecond, time.Second, 10*time.Millisecond
	p.liveReload = 20 * time.Millisecond
	return p
}

func (f *fakeLive) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("User-Agent") != "test" {
		f.t.Errorf("%s asked for without yt-dlp's headers", r.URL.Path)
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/hls_playlist/expire/"):
		expire, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/hls_playlist/expire/"), "/index.m3u8"), 10, 64)
		f.mu.Lock()
		f.loads++
		refused, playlist := f.refused[expire], f.playlist
		text := f.playlistText()
		f.mu.Unlock()
		switch {
		case refused:
			w.WriteHeader(http.StatusForbidden)
		case playlist != nil && playlist(w, r):
		default:
			fmt.Fprint(w, text)
		}
	case strings.HasPrefix(r.URL.Path, "/seg/"):
		seq, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/seg/"), 10, 64)
		f.mu.Lock()
		segment := f.segment
		listed := seq >= f.first && seq < f.end
		f.mu.Unlock()
		if segment != nil && segment(w, r, seq) {
			return
		}
		if !listed {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		f.fetched = append(f.fetched, seq)
		f.mu.Unlock()
		w.Write(segmentBytes(seq))
	default:
		http.NotFound(w, r)
	}
}

// playlistText is the playlist now, of segments at absolute URLs, as
// YouTube's, after a discontinuity and an ad's cues. Its caller holds mu.
func (f *fakeLive) playlistText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:5\n#EXT-X-MEDIA-SEQUENCE:%d\n#EXT-X-DISCONTINUITY-SEQUENCE:3\n", f.first)
	for seq := f.first; seq < f.end; seq++ {
		if seq == f.first+1 {
			b.WriteString("#EXT-X-DISCONTINUITY\n" + `#EXT-X-DATERANGE:CLASS="CUEPOINT-AD",DURATION=60.0,ID="1-AD-START",START-DATE="2026-11-01T20:41:10Z"` + "\n")
		}
		fmt.Fprintf(&b, "#EXT-X-PROGRAM-DATE-TIME:2026-11-01T20:39:55.047+00:00\n#EXTINF:5.0,\n%s/seg/%d\n", f.srv.URL, seq)
	}
	if f.ended {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	return b.String()
}

// move has the playlist hold segments first to end-1.
func (f *fakeLive) move(first, end int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.first, f.end = first, end
}

// served returns the segments served so far.
func (f *fakeLive) served() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.fetched)
}

// waitServed waits for the segments served to be want.
func (f *fakeLive) waitServed(want ...int64) {
	f.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !slices.Equal(f.served(), want); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			f.t.Fatalf("served %v, want %v", f.served(), want)
		}
	}
}

// readSegments reads the segments of l that follow, failing the test
// unless they come within a few seconds.
func readSegments(t *testing.T, l *Live, seqs ...int64) {
	t.Helper()
	for _, seq := range seqs {
		want := segmentBytes(seq)
		got := make([]byte, len(want))
		done := make(chan error, 1)
		go func() {
			_, err := io.ReadFull(l, got)
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("segment %d: %q, %v", seq, got[max(0, len(got)-20):], err)
			}
		case <-time.After(5 * time.Second):
			l.Close()
			t.Fatalf("segment %d didn't come", seq)
		}
	}
}

// TestLiveStartsNearItsEdge plays a live stream from three segments before
// the end of its playlist, which it loads again as it goes, past the
// tags of discontinuities and ads, and fetches each new segment once.
func TestLiveStartsNearItsEdge(t *testing.T) {
	f := newFakeLive(t, 100, 110)
	l, err := f.proxy().OpenLive(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// What a deck reads first, to tell the kind of the audio, it reads
	// again.
	head := make([]byte, 8)
	if _, err := io.ReadFull(l, head); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	readSegments(t, l, 107, 108, 109)
	for end := int64(111); end <= 115; end++ {
		f.move(end-10, end)
		readSegments(t, l, end-1)
	}
	f.waitServed(107, 108, 109, 110, 111, 112, 113, 114)
	f.mu.Lock()
	loads := f.loads
	f.mu.Unlock()
	if loads < 6 || f.runs.Load() != 1 {
		t.Errorf("%d loads of the playlist, %d runs of yt-dlp", loads, f.runs.Load())
	}
	if _, err := l.Seek(0, io.SeekStart); err == nil {
		t.Error("a live stream seeked back to segments it read")
	}
}

// TestLiveWaitsWhilePaused fetches no more than liveAhead segments ahead
// of a reader that doesn't read, and goes on from there, or from near the
// live edge once the playlist left them out.
func TestLiveWaitsWhilePaused(t *testing.T) {
	f := newFakeLive(t, 100, 110)
	l, err := f.proxy().OpenLive(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	f.move(100, 120)
	f.waitServed(107, 108, 109, 110)
	time.Sleep(100 * time.Millisecond)
	if got := f.served(); len(got) != liveAhead {
		t.Errorf("served %v to a reader that didn't read", got)
	}
	// It goes on where it was.
	readSegments(t, l, 107)
	f.waitServed(107, 108, 109, 110, 111)

	// After a while, the next segment is gone: the stream goes on from
	// near the live edge, past those it had.
	f.move(190, 200)
	readSegments(t, l, 108, 109, 110, 111, 197, 198, 199)
}

// TestLiveEnds reads the last segments of a stream whose broadcast ended,
// and the end.
func TestLiveEnds(t *testing.T) {
	f := newFakeLive(t, 100, 110)
	l, err := f.proxy().OpenLive(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	readSegments(t, l, 107, 108, 109)
	f.mu.Lock()
	f.end, f.ended = 112, true
	f.mu.Unlock()
	readSegments(t, l, 110, 111)
	if n, err := l.Read(make([]byte, 10)); n != 0 || err != io.EOF {
		t.Errorf("after the end: %d bytes, %v", n, err)
	}
}

// TestLiveFindsItsStreamAgain finds the stream of the video again when the
// media server refuses its playlist, as once its URL expired, or a
// segment, and goes on with the segments that follow, each fetched once.
// A video no longer live ended.
func TestLiveFindsItsStreamAgain(t *testing.T) {
	f := newFakeLive(t, 100, 110)
	l, err := f.proxy().OpenLive(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	readSegments(t, l, 107, 108, 109)
	f.mu.Lock()
	f.refused[f.expire] = true
	f.end = 112
	f.mu.Unlock()
	readSegments(t, l, 110, 111)
	if f.runs.Load() != 2 {
		t.Errorf("%d runs of yt-dlp", f.runs.Load())
	}

	var refusedOnce atomic.Bool
	f.mu.Lock()
	f.segment = func(w http.ResponseWriter, r *http.Request, seq int64) bool {
		if seq == 112 && !refusedOnce.Swap(true) {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		return false
	}
	f.end = 113
	f.mu.Unlock()
	readSegments(t, l, 112)
	if f.runs.Load() != 3 {
		t.Errorf("%d runs of yt-dlp after a segment was refused", f.runs.Load())
	}
	f.waitServed(107, 108, 109, 110, 111, 112)

	// The broadcast ended, and its playlist expired: the stream ends.
	f.mu.Lock()
	f.refused[f.expire] = true
	f.notLive = true
	f.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(l)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the end of a broadcast: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream of a broadcast that ended didn't end")
	}
}

// TestLiveGetsPastBreaks connects again when a segment breaks off, or the
// playlist stalls, and passes over a segment gone from the media server
// as the playlist moves on, once it stays gone. The log never tells a
// URL.
func TestLiveGetsPastBreaks(t *testing.T) {
	logs := captureLog(t)
	f := newFakeLive(t, 100, 110)
	var broke, stalled atomic.Bool
	var gone atomic.Int32
	f.mu.Lock()
	f.segment = func(w http.ResponseWriter, r *http.Request, seq int64) bool {
		switch {
		case seq == 108 && !broke.Swap(true):
			// Half the segment, and the connection drops.
			w.Header().Set("Content-Length", "3020")
			w.Write(segmentBytes(seq)[:1000])
			panic(http.ErrAbortHandler)
		case seq == 111:
			gone.Add(1)
			http.NotFound(w, r) // gone, as the playlist moves on
			return true
		}
		return false
	}
	f.playlist = func(w http.ResponseWriter, r *http.Request) bool {
		f.mu.Lock()
		stall := f.end == 111 && !stalled.Swap(true)
		f.mu.Unlock()
		if stall {
			time.Sleep(500 * time.Millisecond) // longer than the stall timeout
		}
		return false
	}
	f.mu.Unlock()
	l, err := f.proxy().OpenLive(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	readSegments(t, l, 107, 108, 109)
	f.move(101, 111)
	readSegments(t, l, 110)
	f.move(102, 112)
	for deadline := time.Now().Add(5 * time.Second); gone.Load() < maxMisses; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("segment 111 asked for %d times", gone.Load())
		}
	}
	f.move(102, 114)
	readSegments(t, l, 112, 113)
	if !broke.Load() || !stalled.Load() {
		t.Error("the stream didn't break")
	}
	got := logs.String()
	if !strings.Contains(got, "broke off") || !strings.Contains(got, "comes again") || !strings.Contains(got, "segment 111") ||
		strings.Contains(got, "http") ||
		strings.Contains(got, "127.0.0.1") {
		t.Errorf("the log:\n%s", got)
	}
}

// TestLiveFailures tells why a live stream can't play, in plain words, as
// it opens, or once it broke for good.
func TestLiveFailures(t *testing.T) {
	for name, c := range map[string]struct {
		set  func(f *fakeLive)
		says string
	}{
		"no longer live": {func(f *fakeLive) { f.notLive = true }, "no longer live"},
		"not a playlist": {func(f *fakeLive) {
			f.playlist = func(w http.ResponseWriter, r *http.Request) bool { fmt.Fprint(w, "<html>"); return true }
		}, "HLS playlist"},
		"refused": {func(f *fakeLive) {
			f.playlist = func(w http.ResponseWriter, r *http.Request) bool { w.WriteHeader(http.StatusForbidden); return true }
		}, "YouTube refused to send its audio"},
		"HTTP 500": {func(f *fakeLive) {
			f.segment = func(w http.ResponseWriter, r *http.Request, seq int64) bool { w.WriteHeader(500); return true }
		}, "YouTube answered HTTP 500"},
		// The playlist plays, its segments are refused, also once the stream
		// was found again.
		"refused segments": {func(f *fakeLive) {
			f.segment = func(w http.ResponseWriter, r *http.Request, seq int64) bool {
				w.WriteHeader(http.StatusForbidden)
				return true
			}
		}, "YouTube refused to send its audio"},
	} {
		f := newFakeLive(t, 100, 110)
		f.mu.Lock()
		c.set(f)
		f.mu.Unlock()
		p := f.proxy()
		_, err := p.OpenLive(context.Background(), testID, nil)
		if err == nil || !strings.Contains(err.Error(), c.says) || p.Failure(testID) != err.Error() {
			t.Errorf("%s: %v, failure %q; want one saying %q", name, err, p.Failure(testID), c.says)
		}
	}

	// A stream that broke off for good, after it played.
	f := newFakeLive(t, 100, 110)
	p := f.proxy()
	l, err := p.OpenLive(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	readSegments(t, l, 107, 108, 109)
	f.mu.Lock()
	f.playlist = func(w http.ResponseWriter, r *http.Request) bool { panic(http.ErrAbortHandler) }
	f.mu.Unlock()
	_, err = io.ReadAll(l)
	if err == nil || !strings.Contains(err.Error(), "stopped sending the audio") || p.Failure(testID) != err.Error() {
		t.Errorf("broken off: %v, failure %q", err, p.Failure(testID))
	}

	// A read waiting for a segment stops once interrupted.
	f = newFakeLive(t, 100, 110)
	var stop atomic.Bool
	l, err = f.proxy().OpenLive(context.Background(), testID, stop.Load)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	readSegments(t, l, 107, 108, 109)
	done := make(chan error, 1)
	go func() {
		_, err := l.Read(make([]byte, 10))
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	stop.Store(true)
	l.Wake()
	if err := <-done; !errors.Is(err, ErrInterrupted) {
		t.Errorf("an interrupted read: %v", err)
	}
}
