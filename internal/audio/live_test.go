package audio

import (
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/aac"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/mp4"
)

// liveSongs are the songs TestLivePlays plays: two that YouTube has only
// in AAC, as issue #120 found them, and one it has in Opus.
var liveSongs = []struct {
	videoID, title, codec string
}{
	{"dpgUTslQkFs", "The Life", "mp4a.40.2"},
	{"PGi4kp_sXZ0", "LA Monster", "mp4a.40.2"},
	{"3IUqoyTxEU8", "an Opus song", "opus"},
}

// TestLivePlays finds the streams of liveSongs as the app does, through
// YouTube's player API, else yt-dlp, and plays a few seconds of each on a
// deck, from the proxy: from the start, and after a seek. It finds them
// with yt-dlp alone too, in both qualities, when the yt-dlp MixMusic
// installed, or one on the PATH, is there. With ffmpeg on the PATH, it
// checks what the deck played against ffmpeg's decode of the same file. It
// runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/audio -run Live -v
//
// Neither the streams' URLs nor their files leave the test: the files go
// to its temporary folder.
func TestLivePlays(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	config, _ := os.UserConfigDir()
	tm := tools.New(filepath.Join(config, "MixMusic", "bin"))
	_, noYTDLP := tm.YTDLP()

	for _, song := range liveSongs {
		t.Run(song.videoID, func(t *testing.T) {
			r := stream.NewResolver(tm, t.TempDir())
			r.Direct = stream.NewDirect()
			s, err := r.Resolve(ctx, song.videoID)
			if err != nil {
				t.Fatalf("%s: %v", song.title, err)
			}
			// Never the URL: it carries this machine's IP address.
			t.Logf("%s: %s, %d bytes at %.0f kbit/s, %v, %.2f LKFS (measured %v)", song.title, s.MimeType, s.Size, s.Bitrate,
				s.Duration, s.Loudness, s.Measured)
			if s.Codec != song.codec {
				t.Errorf("a stream of %s, want %s", s.Codec, song.codec)
			}
			// The loudness the app asks of a stream yt-dlp found, for Even
			// loudness: the video's, which YouTube may round apart from its
			// formats' by a hundredth of a dB.
			if v, err := stream.NewDirect().Loudness(ctx, song.videoID); err != nil || math.Abs(v-s.Loudness) > 0.05 {
				t.Errorf("loudness: %v, %v; the stream said %v", v, err, s.Loudness)
			}
			file := livePlay(t, ctx, stream.NewProxy(r), song.videoID, s.Duration)

			if noYTDLP != nil {
				t.Logf("yt-dlp: %v", noYTDLP)
				return
			}
			for _, q := range []stream.Quality{stream.QualityBest, stream.QualitySaver} {
				y := stream.NewResolver(tm, t.TempDir())
				y.SetQuality(q)
				s, err := y.Resolve(ctx, song.videoID)
				if err != nil {
					t.Fatalf("yt-dlp, %s: %v", q, err)
				}
				t.Logf("yt-dlp, %s: %s, %d bytes at %.0f kbit/s", q, s.MimeType, s.Size, s.Bitrate)
				if s.Codec != song.codec {
					t.Errorf("yt-dlp, %s: a stream of %s, want %s", q, s.Codec, song.codec)
				}
				if q == stream.QualityBest {
					again := livePlay(t, ctx, stream.NewProxy(y), song.videoID, s.Duration)
					t.Logf("yt-dlp's file is the player API's: %v", bytes.Equal(again, file))
				}
			}
		})
	}
}

// livePlay plays 5 seconds of a video from its start, and 3 from a minute
// in, from p, checks them against ffmpeg's decode of its file, when ffmpeg
// is on the PATH, and returns the file.
func livePlay(t *testing.T, ctx context.Context, p *stream.Proxy, videoID string, length time.Duration) []byte {
	t.Helper()
	open := func(ctx context.Context, videoID string, interrupted func() bool) (Source, error) {
		return p.Open(ctx, videoID, interrupted)
	}
	start := time.Now()
	d := newDeck(videoID, 0, open, func() {})
	defer d.close()
	head := take(t, d, 5*Rate)
	t.Logf("5 s from the start in %s", time.Since(start).Round(time.Millisecond))
	total, _ := d.info()
	if total <= 0 || length > 0 && (total-length).Abs() > time.Second {
		t.Errorf("a duration of %v; YouTube tells %v", total, length)
	}
	const at = time.Minute
	d.seek(at)
	later := take(t, d, 3*Rate)

	src, err := p.Open(ctx, videoID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	file, err := io.ReadAll(src)
	if err != nil {
		t.Fatal(err)
	}
	kind, _ := sniff(bytes.NewReader(file))
	isMP4 := kind == mp4File
	t.Logf("a file of %d bytes, MP4 %v", len(file), isMP4)
	if isMP4 {
		// YouTube's edit list skips the encoder's priming: the deck needs
		// no priming of its own.
		r, err := mp4.NewReader(bytes.NewReader(file))
		if err != nil {
			t.Fatal(err)
		}
		tr := r.Track()
		t.Logf("%s at %d Hz in %d channels, a codec delay of %v, %v long", tr.Codec, tr.SampleRate, tr.Channels, tr.CodecDelay, r.Duration())
		if tr.CodecDelay <= 0 {
			t.Error("no edit list skips the encoder's priming")
		}
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Log("ffmpeg is not on the PATH: no decode to compare with")
		return file
	}
	path := filepath.Join(t.TempDir(), videoID)
	if err := os.WriteFile(path, file, 0o600); err != nil {
		t.Fatal(err)
	}
	want := ffmpegDecode(t, path, false) // the songs are in stereo
	// AAC plays at its frames after a seek, Opus within the millisecond of
	// WebM's times.
	slack := int64(0)
	if !isMP4 {
		slack = Rate / 1000
	}
	compareLive(t, "the start", head, want, 0, 0)
	compareLive(t, "after the seek", later, want, frames(at), slack)
	return file
}

// take reads n frames from a deck's ring, as they come.
func take(t *testing.T, d *deck, n int) []float32 {
	t.Helper()
	out := make([]float32, 0, 2*n)
	buf := make([]float32, 2*1024)
	deadline := time.Now().Add(time.Minute)
	for len(out) < 2*n {
		r := d.ring.Load()
		k := r.readTo(buf[:min(len(buf), 2*n-len(out))])
		out = append(out, buf[:2*k]...)
		if k == 0 {
			if err := d.failure(); err != nil {
				t.Fatalf("after %d frames: %v", len(out)/2, err)
			}
			if r.done.Load() || time.Now().After(deadline) {
				t.Fatalf("the deck gave %d frames of %d", len(out)/2, n)
			}
			time.Sleep(time.Millisecond)
		}
	}
	return out
}

// compareLive compares got, frames a deck played from frame at of the
// track, with want, ffmpeg's decode of all of it: their levels, in dBFS,
// and their difference, at the lag where it is least, which should be
// within slack frames. The frames of the start, which ffmpeg converts from
// silence before the track, differ, and are left out.
func compareLive(t *testing.T, what string, got, want []float32, at, slack int64) {
	t.Helper()
	const margin, search = 1024, 2 * Rate / 1000
	got = got[2*margin:]
	at += margin
	if int64(len(want)) < 2*(at+search)+int64(len(got)) {
		t.Fatalf("%s: ffmpeg decoded %d frames", what, len(want)/2)
	}
	best, lag := math.Inf(-1), int64(0)
	for l := int64(-search); l <= search; l++ {
		if at+l < 0 {
			continue
		}
		if s := snr(got, want[2*(at+l):]); s > best {
			best, lag = s, l
		}
	}
	level := func(x []float32) float64 {
		var e float64
		for _, v := range x {
			e += float64(v) * float64(v)
		}
		return 10 * math.Log10(e/float64(len(x)))
	}
	w := want[2*(at+lag):][:len(got)]
	t.Logf("%s: %.2f dBFS, ffmpeg's %.2f dBFS; %.1f dB apart, %d frames late", what, level(got), level(w), best, lag)
	if lag < -slack || lag > slack || best < 60 {
		t.Errorf("%s: %.1f dB from ffmpeg's decode, %d frames late", what, best, lag)
	}
}

// TestLiveStreamPlays plays a few seconds of a live stream, a 24/7 radio
// YouTube Music finds, as the app does: from the HLS playlist the player
// API gives, else yt-dlp, near its live edge, and on past the segments
// that were there as it started, which a reload of the playlist brings.
// It finds the stream with yt-dlp alone too, in both qualities, when the
// yt-dlp MixMusic installed is there. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/audio -run LiveStream -v
//
// With ffmpeg on the PATH, it checks what the deck played of format 234
// against ffmpeg's decode of the same packed audio. It plays into no
// device, and tells neither the playlist's URL nor its segments'.
func TestLiveStreamPlays(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	videos, err := youtube.New().Search(ctx, "lofi hip hop radio beats to relax study to", youtube.FilterVideos)
	if err != nil {
		t.Fatal(err)
	}
	var videoID, title string
	for _, v := range videos {
		if v.Track != nil && v.Track.DurationMs == 0 {
			_, videoID, _ = media.ParseID(v.Track.ID)
			title = v.Track.Title
			break
		}
	}
	if videoID == "" {
		t.Skip("YouTube Music found no live video")
	}
	t.Logf("%s (%s)", title, videoID)
	itag := regexp.MustCompile(`/itag/(\d+)/`)

	play := func(t *testing.T, r *stream.Resolver) {
		start := time.Now()
		s, err := r.Resolve(ctx, videoID)
		if err != nil {
			t.Fatal(err)
		}
		format := ""
		if m := itag.FindStringSubmatch(s.URL); m != nil {
			format = m[1]
		}
		t.Logf("found in %s: live %v, format %s, %s, expires in %s", time.Since(start).Round(time.Millisecond), s.Live, format,
			s.Codec, time.Until(s.Expires).Round(time.Minute))
		if !s.Live {
			t.Fatal("the stream of a live video isn't live")
		}
		p := stream.NewProxy(r)
		var tee *teeSource
		open := func(ctx context.Context, videoID string, interrupted func() bool) (Source, error) {
			l, err := p.OpenLive(ctx, videoID, interrupted)
			if err != nil {
				return nil, err
			}
			tee = &teeSource{Live: l}
			return tee, nil
		}
		start = time.Now()
		d := newDeck(videoID, 0, open, func() {})
		defer d.close()
		first := take(t, d, Rate/10)
		t.Logf("the first sound in %s", time.Since(start).Round(time.Millisecond))
		// Past the three segments it starts with, of 5 seconds each.
		rest := take(t, d, 20*Rate)
		t.Logf("20 s more in %s", time.Since(start).Round(time.Millisecond))
		level := func(x []float32) float64 {
			var e float64
			for _, v := range x {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatalf("a sample of %v", v)
				}
				e += float64(v) * float64(v)
			}
			return 10 * math.Log10(e/float64(len(x)))
		}
		t.Logf("%.1f dBFS, then %.1f dBFS", level(first), level(rest))
		if level(rest) < -60 {
			t.Error("the live stream played silence")
		}
		if total, _ := d.info(); total != 0 || !d.live.Load() {
			t.Errorf("a duration of %v, live %v", total, d.live.Load())
		}
		if _, err := exec.LookPath("ffmpeg"); err != nil || format != "234" {
			return // 233 is HE-AAC, which ffmpeg decodes with its SBR
		}
		d.close()
		path := filepath.Join(t.TempDir(), "live.aac")
		if err := os.WriteFile(path, tee.frames(), 0o600); err != nil {
			t.Fatal(err)
		}
		compareLive(t, "the live stream", append(first, rest...), ffmpegDecode(t, path, false), 0, 0)
	}

	t.Run("the player API", func(t *testing.T) {
		r := stream.NewResolver(nil, t.TempDir())
		r.Direct = stream.NewDirect()
		r.Run = func(context.Context, ...string) ([]byte, error) { return nil, tools.ErrNoYTDLP }
		play(t, r)
	})
	config, _ := os.UserConfigDir()
	tm := tools.New(filepath.Join(config, "MixMusic", "bin"))
	if _, err := tm.YTDLP(); err != nil {
		t.Logf("yt-dlp: %v", err)
		return
	}
	for _, q := range []stream.Quality{stream.QualityBest, stream.QualitySaver} {
		t.Run("yt-dlp, "+string(q), func(t *testing.T) {
			r := stream.NewResolver(tm, t.TempDir())
			r.SetQuality(q)
			play(t, r)
		})
	}
}

// teeSource is a live stream, which keeps what was read of it.
type teeSource struct {
	*stream.Live
	mu  sync.Mutex
	got []byte
}

func (s *teeSource) Read(b []byte) (int, error) {
	n, err := s.Live.Read(b)
	s.mu.Lock()
	s.got = append(s.got, b[:n]...)
	s.mu.Unlock()
	return n, err
}

func (s *teeSource) Seek(offset int64, whence int) (int64, error) {
	pos, err := s.Live.Seek(offset, whence)
	if err == nil {
		s.mu.Lock()
		s.got = s.got[:min(int64(len(s.got)), pos)]
		s.mu.Unlock()
	}
	return pos, err
}

// frames are the ADTS frames of what was read, without the ID3 tags of
// the segments, as ffmpeg reads them.
func (s *teeSource) frames() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []byte
	for b := s.got; len(b) > 0; {
		if n, ok := id3Size(b); ok {
			b = b[min(n, len(b)):]
			continue
		}
		h, err := aac.ParseADTS(b)
		if err != nil || h.Length > len(b) {
			break
		}
		out = append(out, b[:h.Length]...)
		b = b[h.Length:]
	}
	return out
}
