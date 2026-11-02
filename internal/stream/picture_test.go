package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// pictureStream is a stream of a video's picture of size bytes, its first
// bytes head, which it reads without holding it.
type pictureStream struct {
	head []byte
	size int64
	at   int64
}

// byteAt is the byte at off of a pictureStream past its head.
func byteAt(off int64) byte { return byte(off * 7) }

func (s *pictureStream) Read(p []byte) (int, error) {
	if s.at >= s.size {
		return 0, io.EOF
	}
	n := min(int64(len(p)), s.size-s.at)
	for i := range n {
		if off := s.at + i; off < int64(len(s.head)) {
			p[i] = s.head[off]
		} else {
			p[i] = byteAt(off)
		}
	}
	s.at += n
	return int(n), nil
}

func (s *pictureStream) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.at = off
	case io.SeekCurrent:
		s.at += off
	case io.SeekEnd:
		s.at = s.size + off
	}
	return s.at, nil
}

// pictureTube is YouTube serving the audio and two streams of the picture
// of musicVideoID, the WebM and MP4 streams of the head files, refusing
// URLs whose generation is below minGen as it refuses expired ones; status,
// when set, answers every request of the picture.
type pictureTube struct {
	srv            *httptest.Server
	minGen, status atomic.Int32
	asks           atomic.Int32 // requests of the picture
}

func newPictureTube(t *testing.T) *pictureTube {
	t.Helper()
	heads := map[string]*pictureStream{
		"243": {head: readHead(t, "video-243-head.webm"), size: size243},
		"134": {head: readHead(t, "video-134-head.mp4"), size: size134},
	}
	pt := &pictureTube{}
	pt.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gen int32
		fmt.Sscanf(r.URL.Query().Get("gen"), "%d", &gen)
		itag := r.URL.Query().Get("itag")
		if itag != "251" {
			pt.asks.Add(1)
		}
		switch {
		case gen < pt.minGen.Load():
			w.WriteHeader(http.StatusForbidden)
			return
		case pt.status.Load() != 0:
			w.WriteHeader(int(pt.status.Load()))
			return
		case r.Header.Get("Range") == "":
			t.Error("a request of the picture without a range")
		}
		s := heads[itag]
		if s == nil {
			http.ServeContent(w, r, "", time.Time{}, strings.NewReader("audio"))
			return
		}
		http.ServeContent(w, r, "", time.Time{}, &pictureStream{head: s.head, size: s.size})
	}))
	t.Cleanup(pt.srv.Close)
	return pt
}

// proxy returns a proxy whose yt-dlp finds the streams of pt, the
// generation of their URLs increasing at each run.
func (pt *pictureTube) proxy() *Proxy {
	var gen atomic.Int32
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		g := gen.Add(1)
		u := func(itag string) string {
			return fmt.Sprintf("%s/v?gen=%d&itag=%s&expire=%d", pt.srv.URL, g, itag, time.Now().Add(6*time.Hour).Unix())
		}
		return fmt.Appendf(nil, `{"url": %q, "ext": "webm", "acodec": "opus", "filesize": 5, "formats": [
			{"format_id": "243", "url": %q, "ext": "webm", "protocol": "https", "vcodec": "vp9", "acodec": "none",
			 "width": 640, "height": 360, "fps": 30, "filesize": %d},
			{"format_id": "134", "url": %q, "ext": "mp4", "protocol": "https", "vcodec": "avc1.4D401E", "acodec": "none",
			 "width": 640, "height": 360, "fps": 30}]}`, u("251"), u("243"), size243, u("134")), nil
	}
	return NewProxy(r)
}

func getVideo(t *testing.T, p *Proxy, method, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/video/"+musicVideoID+"?"+query, nil)
	rec := httptest.NewRecorder()
	p.ServeVideo(rec, req)
	return rec
}

// expected returns bytes from to to of the stream 243 of pictureTube.
func expected(t *testing.T, from, to int64) []byte {
	s := &pictureStream{head: readHead(t, "video-243-head.webm"), size: size243}
	s.Seek(from, io.SeekStart)
	b := make([]byte, to-from+1)
	io.ReadFull(s, b)
	return b
}

func TestPictureReadsTheIndex(t *testing.T) {
	pt := newPictureTube(t)
	p := pt.proxy()
	for _, c := range []struct {
		family, itag string
		init         int64
	}{{"vp9", "243", 220}, {"avc1", "134", 741}} {
		pic, err := p.Picture(context.Background(), musicVideoID, func(vs []Video) (Video, bool) {
			return PickVideo(vs, []string{c.family}, 630, 354, false)
		})
		if err != nil {
			t.Fatalf("%s: %v", c.family, err)
		}
		if pic.Itag != c.itag || pic.Index.Init != c.init || len(pic.Index.Segments) < 90 {
			t.Errorf("%s: %s, init %d, %d segments", c.family, pic.Itag, pic.Index.Init, len(pic.Index.Segments))
		}
		// 134's size wasn't given: YouTube tells it.
		if pic.Size == 0 || pic.Index.Segments[len(pic.Index.Segments)-1].End != pic.Size-1 {
			t.Errorf("%s: %d bytes", c.family, pic.Size)
		}
		if d := pic.Duration - 474316*time.Millisecond; d < -time.Second || d > time.Second {
			t.Errorf("%s lasts %s", c.family, pic.Duration)
		}
	}
	_, err := p.Picture(context.Background(), musicVideoID, func(vs []Video) (Video, bool) {
		return PickVideo(vs, []string{"hev1"}, 630, 354, false)
	})
	if !errors.Is(err, errNoVideo) {
		t.Errorf("no codec the page plays: %v", err)
	}
	// YouTube gone: the error names no stream URL.
	url := pt.srv.URL
	pt.srv.Close()
	p.HTTP.CloseIdleConnections()
	_, err = p.Picture(context.Background(), musicVideoID, func(vs []Video) (Video, bool) {
		return PickVideo(vs, []string{"vp9"}, 630, 354, false)
	})
	if err == nil || strings.Contains(err.Error(), url) || strings.Contains(err.Error(), "videoplayback") || strings.Contains(err.Error(), "/v?") {
		t.Errorf("YouTube gone: %v", err)
	}
}

func TestServeVideoRanges(t *testing.T) {
	pt := newPictureTube(t)
	p := pt.proxy()
	rec := getVideo(t, p, "GET", "itag=243&range=1000000-1099999")
	if rec.Code != http.StatusPartialContent || rec.Header().Get("Content-Range") != fmt.Sprintf("bytes 1000000-1099999/%d", size243) {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Content-Range"))
	}
	if !bytes.Equal(rec.Body.Bytes(), expected(t, 1000000, 1099999)) {
		t.Error("other bytes")
	}
	if rec.Header().Get("Content-Type") != `video/webm; codecs="vp9"` || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("headers %v", rec.Header())
	}
	// A range longer than a chunk gets a chunk; one past the end, its end.
	rec = getVideo(t, p, "GET", "itag=243&range=0-99999999")
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != chunkSize ||
		rec.Header().Get("Content-Range") != fmt.Sprintf("bytes 0-%d/%d", chunkSize-1, size243) {
		t.Errorf("a long range: %d, %d bytes, %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Range"))
	}
	end := fmt.Sprintf("itag=243&range=%d-%d", size243-10, size243+10)
	if rec = getVideo(t, p, "GET", end); rec.Code != http.StatusPartialContent || rec.Body.Len() != 10 {
		t.Errorf("past the end: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	asks := pt.asks.Load()
	if rec = getVideo(t, p, "HEAD", "itag=243&range=0-99"); rec.Code != http.StatusPartialContent || rec.Header().Get("Content-Length") != "100" {
		t.Errorf("HEAD: %d %v", rec.Code, rec.Header())
	}
	if pt.asks.Load() != asks {
		t.Error("HEAD asked YouTube for the picture")
	}
	for query, code := range map[string]int{
		"itag=999&range=0-99": http.StatusNotFound,
		"itag=243":            http.StatusBadRequest,
		"itag=243&range=99-0": http.StatusBadRequest,
		"itag=243&range=-5":   http.StatusBadRequest,
		"range=0-99":          http.StatusBadRequest,
		fmt.Sprintf("itag=243&range=%d-%d", size243, size243+5): http.StatusRequestedRangeNotSatisfiable,
	} {
		if rec := getVideo(t, p, "GET", query); rec.Code != code {
			t.Errorf("%s: %d, want %d", query, rec.Code, code)
		}
	}
	for _, path := range []string{"/video/nope", "/yt/" + musicVideoID} {
		rec := httptest.NewRecorder()
		p.ServeVideo(rec, httptest.NewRequest("GET", path+"?itag=243&range=0-9", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

func TestServeVideoResolvesExpiredURLsAgain(t *testing.T) {
	pt := newPictureTube(t)
	p := pt.proxy()
	if rec := getVideo(t, p, "GET", "itag=243&range=0-99"); rec.Code != http.StatusPartialContent {
		t.Fatalf("first: %d", rec.Code)
	}
	pt.minGen.Store(2) // YouTube now refuses the first URLs
	rec := getVideo(t, p, "GET", "itag=243&range=5000-5099")
	if rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), expected(t, 5000, 5099)) {
		t.Errorf("after expiry: %d", rec.Code)
	}
}

// TestServeVideoLeavesTheAudioAlone keeps the picture out of the audio's
// cache, and its failures out of the audio's, which the player reads as why
// the track couldn't play.
func TestServeVideoLeavesTheAudioAlone(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	pt := newPictureTube(t)
	p := pt.proxy()
	p.Cache = NewCache(t.TempDir(), 1<<30)
	if rec := getVideo(t, p, "GET", fmt.Sprintf("itag=243&range=0-%d", chunkSize-1)); rec.Code != http.StatusPartialContent {
		t.Fatalf("%d", rec.Code)
	}
	if p.Cache.Has(musicVideoID) {
		t.Error("the picture went to the audio's cache")
	}
	pt.status.Store(http.StatusInternalServerError)
	if rec := getVideo(t, p, "GET", "itag=243&range=0-99"); rec.Code != http.StatusBadGateway {
		t.Errorf("YouTube failing: %d", rec.Code)
	}
	if why := p.Failure(musicVideoID); why != "" {
		t.Errorf("the audio failed: %s", why)
	}
	if !strings.Contains(logs.String(), "the picture of "+musicVideoID) || strings.Contains(logs.String(), pt.srv.URL) {
		t.Errorf("the log: %s", logs.String())
	}
}
