package stream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestLiveDirect finds the stream of a song through YouTube's player API
// itself, and reads all of it as the proxy does, after a seek: YouTube
// changes what it answers from time to time. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/stream -run Live -v
func TestLiveDirect(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	songs, err := youtube.New().Search(ctx, "daft punk", youtube.FilterSongs)
	if err != nil || len(songs) == 0 || songs[0].Track == nil {
		t.Fatalf("search: %d songs, %v", len(songs), err)
	}
	_, videoID, _ := media.ParseID(songs[0].Track.ID)

	d := NewDirect()
	var s0 Stream
	for _, q := range []Quality{QualityBest, QualitySaver} {
		start := time.Now()
		s, err := d.Stream(ctx, videoID, q)
		if err != nil {
			t.Fatalf("%s of %s: %v", q, videoID, err)
		}
		// Never the URL: it carries this machine's IP address.
		t.Logf("%s of %s (%s): found in %s, %d bytes at %.0f kbit/s, %.2f LKFS", q, videoID, songs[0].Track.Title,
			time.Since(start).Round(time.Millisecond), s.Size, s.Bitrate, s.Loudness)
		if s.Size <= 0 || s.Codec != "opus" || !s.Measured {
			t.Fatalf("stream of %d bytes in %s, measured %v", s.Size, s.Codec, s.Measured)
		}

		start = time.Now()
		seek := s.Size * 3 / 4
		if n := readRange(t, s, seek, min(seek+chunkSize, s.Size)-1); n != min(chunkSize, s.Size-seek) {
			t.Fatalf("%d bytes after the seek", n)
		}
		var read int64
		for off := int64(0); off < s.Size; off += chunkSize {
			read += readRange(t, s, off, min(off+chunkSize, s.Size)-1)
		}
		if read != s.Size {
			t.Fatalf("read %d of %d bytes", read, s.Size)
		}
		t.Logf("read whole in %s", time.Since(start).Round(time.Millisecond))
		if q == QualityBest {
			s0 = s
		}
	}

	// YouTube gives the route no stream of a video made for children, which
	// yt-dlp finds then.
	if _, err := d.Stream(ctx, "XqZsoesa55w", QualityBest); err == nil {
		t.Log("YouTube's player API now gives the streams of videos made for children")
	} else {
		t.Logf("made for children: %v", err)
	}

	// The loudness of a stream yt-dlp found, from a route that has no
	// visitor ID yet. It is the video's, which YouTube may round apart from
	// its formats' by a hundredth of a dB.
	v, err := NewDirect().Loudness(ctx, videoID)
	if err != nil || math.Abs(v-s0.Loudness) > 0.05 {
		t.Errorf("loudness of %s: %v, %v; the stream said %v", videoID, v, err, s0.Loudness)
	}
}

// readRange reads bytes start to end of a stream as the proxy fetches
// them, and checks that the first are of WebM with Opus.
func readRange(t *testing.T, s Stream, start, end int64) int64 {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	if err != nil {
		t.Fatal("a stream URL that doesn't parse")
	}
	for k, v := range s.Header {
		req.Header[k] = v
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("bytes %d-%d: the network failed", start, end)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusPartialContent {
		t.Fatalf("bytes %d-%d: HTTP %d", start, end, res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("bytes %d-%d: read %d before the network failed", start, end, len(b))
	}
	if start == 0 && (!bytes.HasPrefix(b, []byte{0x1a, 0x45, 0xdf, 0xa3}) || !bytes.Contains(b[:min(len(b), 4096)], []byte("A_OPUS"))) {
		t.Fatal("the stream is not WebM with Opus")
	}
	return int64(len(b))
}

// TestLivePicture finds the picture of a music video as the now playing
// page does, through YouTube's player API and through yt-dlp when there is
// one, reads its index, and fetches a part of it from the middle through
// the proxy, as the page fetches them. It runs with MIXMUSIC_LIVE=1.
func TestLivePicture(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := NewResolver(nil, "")
	r.Direct = NewDirect()
	p := NewProxy(r)
	for _, families := range [][]string{{"vp9", "avc1"}, {"avc1"}} {
		pic, err := p.Picture(ctx, musicVideoID, func(vs []Video) (Video, bool) {
			return PickVideo(vs, families, 630, 354, false)
		})
		if err != nil {
			t.Fatalf("%v: %v", families, err)
		}
		mid := pic.Index.Segments[len(pic.Index.Segments)/2]
		t.Logf("%v: %s, %s, %d × %d, %d bytes, init %d, %d segments, %s", families, pic.Itag, pic.MimeType, pic.Width, pic.Height,
			pic.Size, pic.Index.Init, len(pic.Index.Segments), pic.Duration)
		rec := httptest.NewRecorder()
		p.ServeVideo(rec, httptest.NewRequest("GET", fmt.Sprintf("%s&range=%d-%d", VideoPath(musicVideoID, pic.Itag), mid.Start, mid.End), nil))
		if rec.Code != http.StatusPartialContent || int64(rec.Body.Len()) != mid.End-mid.Start+1 {
			t.Fatalf("%v: the middle part: %d, %d bytes", families, rec.Code, rec.Body.Len())
		}
	}
}
