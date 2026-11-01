package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/aac"
	"github.com/fzn0x/mixmusic/internal/audio/resample"
	"github.com/fzn0x/mixmusic/internal/mp4"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// Test files: AAC-LC in MP4, as ffmpeg makes it, from internal/mp4's
// testdata, where the commands that made them are. Each holds 3 seconds of
// sine waves. sidx is laid out as YouTube's files are: fragments, after a
// sidx and a moov with an edit list; progressive has the same audio, and
// its moov after it. Both are 440 Hz on the left and 660 Hz on the right,
// at full scale, at 44.1 kHz. faststart is a mono 440 Hz at 48 kHz, at 1/8
// of full scale, which plays unconverted.
var (
	aacFragments   = "../mp4/testdata/sidx.m4a"
	aacProgressive = "../mp4/testdata/progressive.m4a"
	aacMono48      = "../mp4/testdata/faststart.m4a"
	aacFiles       = []string{aacFragments, aacProgressive, aacMono48}
)

// aacReference decodes a file whole, as a deck must play it: the frames of
// each packet its duration tells, without the codec delay the edit list
// skips, converted to stereo at Rate. It converts what plays alone, from
// silence, where a deck converts the delay too: the two differ in their
// first frames.
func aacReference(t *testing.T, path string) []float32 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := mp4.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := aac.ParseConfig(r.Track().Config)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := aac.NewDecoder(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rate := int64(cfg.SampleRate)
	pcm := make([]float32, 2*aac.FrameLength)
	var in []float32
	for {
		p, err := r.ReadPacket()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n, err := dec.Decode(p.Data, pcm)
		if err != nil {
			t.Fatal(err)
		}
		in = append(in, pcm[:2*min(int64(n), framesAt(p.Duration, rate))]...)
	}
	in = in[2*framesAt(r.Track().CodecDelay, rate):]
	c, err := resample.New(cfg.SampleRate, Rate, 2)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float32, 2*(c.MaxOutput(len(in)/2)+c.Latency()))
	_, n := c.Process(in, out)
	n += c.Flush(out[2*n:])
	return out[:2*n]
}

// tone returns the amplitude and the phase, in frames, of the tone of hz
// in channel c of pcm, interleaved stereo at Rate, as a sine from the
// start: by least squares, away from the ends.
func tone(pcm []float32, c int, hz float64) (amplitude, phase float64) {
	var ss, cc, xs, xc float64
	for i := Rate / 10; i < len(pcm)/2-Rate/10; i++ {
		w := 2 * math.Pi * hz * float64(i) / Rate
		s, co, x := math.Sin(w), math.Cos(w), float64(pcm[2*i+c])
		xs, xc, ss, cc = xs+x*s, xc+x*co, ss+s*s, cc+co*co
	}
	a, b := xs/ss, xc/cc
	return math.Hypot(a, b), math.Atan2(b, a) / (2 * math.Pi * hz) * Rate
}

func TestDeckPlaysAAC(t *testing.T) {
	for _, path := range aacFiles {
		t.Run(path, func(t *testing.T) {
			want := aacReference(t, path)
			d := newDeck(path, 0, fileOpener, func() {})
			defer d.close()
			got := drain(t, d)
			if err := d.failure(); err != nil {
				t.Fatal(err)
			}
			// 3 seconds at 48 kHz, as the file tells.
			if total, ok := d.info(); !ok || total != 3*time.Second || len(got)/2 != 3*Rate {
				t.Errorf("%d frames, duration %v, %v", len(got)/2, total, ok)
			}
			if len(got) != len(want) {
				t.Fatalf("%d frames, want %d", len(got)/2, len(want)/2)
			}
			// Past the converter's window of the first frame, frame for frame.
			const settle = 64
			for i := 2 * settle; i < len(got); i++ {
				if got[i] != want[i] {
					t.Fatalf("frame %d differs: %v, want %v", i/2, got[i], want[i])
				}
			}
			if s := snr(got[:2*settle], want[:2*settle]); s < 60 {
				t.Errorf("the first %d frames are %.1f dB from the reference", settle, s)
			}
			// The sines start with the track, at their pitch: the codec delay
			// is gone, and the rate is Rate.
			tones := []struct {
				c      int
				hz, at float64
			}{{0, 440, 1}, {1, 660, 1}}
			if path == aacMono48 {
				tones = []struct {
					c      int
					hz, at float64
				}{{0, 440, 0.125}, {1, 440, 0.125}}
			}
			for _, w := range tones {
				a, ph := tone(got, w.c, w.hz)
				if math.Abs(a/w.at-1) > 0.02 || math.Abs(ph) > 0.25 {
					t.Errorf("channel %d: %v Hz at %.4f, %.3f frames late; want %v, on time", w.c, w.hz, a, ph, w.at)
				}
			}
		})
	}
}

func TestDeckStartsAtAAC(t *testing.T) {
	for _, path := range []string{aacFragments, aacMono48} {
		want := aacReference(t, path)
		for _, at := range []time.Duration{0, 7 * time.Millisecond, 500 * time.Millisecond, 1234 * time.Millisecond, 2900 * time.Millisecond} {
			d := newDeck(path, at, fileOpener, func() {})
			got := drain(t, d)
			d.close()
			checkFrom(t, got, want, frames(at), path+" at "+at.String())
		}
	}
}

func TestDeckSeeksAAC(t *testing.T) {
	want := aacReference(t, aacFragments)
	d := newDeck(aacFragments, 0, fileOpener, func() {})
	defer d.close()
	buf := make([]float32, 2*4800)
	// Read some, seek back and forth, across fragments, then read to the
	// end.
	for _, at := range []time.Duration{2 * time.Second, 300 * time.Millisecond, 1500 * time.Millisecond} {
		r := d.ring.Load()
		for r.readable() < 4800 && !r.done.Load() {
			time.Sleep(time.Millisecond)
		}
		r.readTo(buf)
		d.seek(at)
		if got := d.position(); got != frames(at) {
			t.Errorf("position %d after the seek to %v, want %d", got, at, frames(at))
		}
	}
	got := drain(t, d)
	checkFrom(t, got, want, frames(1500*time.Millisecond), "after the seeks")
	// After the end, a seek plays again; past the end, nothing plays.
	d.seek(0)
	if got = drain(t, d); len(got) != len(want) {
		t.Errorf("%d frames from the start again, want %d", len(got)/2, len(want)/2)
	}
	d.seek(4 * time.Second)
	if got = drain(t, d); len(got) != 0 || d.failure() != nil {
		t.Errorf("%d frames past the end, %v", len(got)/2, d.failure())
	}
}

// TestDeckEndsAnAACStreamCutInItsLastMoment ends a track whose stream
// breaks off within endSlack of its end, as at its end, as an Opus one
// does. The file must tell its duration: faststart's moov does.
func TestDeckEndsAnAACStreamCutInItsLastMoment(t *testing.T) {
	want := aacReference(t, aacMono48)
	// The file's last 300 bytes are its last 56 ms.
	d := newDeck("x", 0, cutOpener(t, aacMono48, 300, errors.New("the network failed while loading the audio")), func() {})
	defer d.close()
	got := drain(t, d)
	if err := d.failure(); err != nil {
		t.Fatalf("the deck failed: %v", err)
	}
	if lost := len(want)/2 - len(got)/2; lost <= 0 || int64(lost) > frames(endSlack) {
		t.Errorf("%d frames short of the end", lost)
	}
	// What came plays as the file does, but for the converter's last
	// frames, which the end of the input changes.
	if s := snr(got, want); s < 60 {
		t.Errorf("%.1f dB from the whole decode", s)
	}
}

// withConfig returns the bytes of an MP4 file whose AudioSpecificConfig
// edit changed.
func withConfig(t *testing.T, path string, edit func(asc []byte)) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := mp4.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	asc := r.Track().Config
	at := bytes.Index(b, []byte("esds"))
	i := bytes.Index(b[at:], asc)
	if at < 0 || i < 0 {
		t.Fatal("no AudioSpecificConfig in the esds")
	}
	edit(b[at+i : at+i+len(asc)])
	return b
}

func TestDeckAACFailures(t *testing.T) {
	b, err := os.ReadFile(aacFragments)
	if err != nil {
		t.Fatal(err)
	}
	ftyp := b[:binary.BigEndian.Uint32(b)]
	for _, c := range []struct {
		name string
		data []byte
		want error
		says string
	}{
		// YouTube's itag 139.
		{"HE-AAC", withConfig(t, aacFragments, func(asc []byte) { asc[0] = 5<<3 | asc[0]&7 }), errUnsupported, "mp4a.40.5"},
		{"frames of 960 samples", withConfig(t, aacFragments, func(asc []byte) { asc[1] |= 4 }), errUnsupported, "960"},
		{"no moov", append(ftyp[:len(ftyp):len(ftyp)], "\x00\x00\x00\x10mdat0123456789"...), errUnsupported, "mp4"},
		{"cut short", b[:len(b)/2], errNetwork, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
				return memSource{bytes.NewReader(c.data)}, nil
			}
			d := newDeck("x", 0, open, func() {})
			defer d.close()
			drain(t, d)
			if err := d.failure(); !errors.Is(err, c.want) || !strings.Contains(err.Error(), c.says) {
				t.Errorf("failure %v, want %v saying %q", err, c.want, c.says)
			}
		})
	}
}

func TestSniffMP4(t *testing.T) {
	for _, c := range []struct {
		head string
		want bool
	}{
		{"\x00\x00\x00\x18ftypdash", true},
		{"\x00\x00\x00\x18stypmsdh", true},
		{"\x00\x00\x02\xdfmoov", true},
		{"\x1a\x45\xdf\xa3\x9f\x42\x86\x81", false},
		{"\x00\x00\x00\x18ftyp"[:7], false},
		{"", false},
		{"hello, this is not a file of audio", false},
	} {
		r := bytes.NewReader([]byte(c.head))
		got, err := sniffMP4(r)
		if pos, _ := r.Seek(0, io.SeekCurrent); got != c.want || err != nil || pos != 0 {
			t.Errorf("%q: %v, %v, at %d", c.head, got, err, pos)
		}
	}
	why := errors.New("the network failed")
	b, _ := os.ReadFile(aacFragments)
	if _, err := sniffMP4(&brokenSource{Reader: bytes.NewReader(b), n: 6, why: why}); err != why {
		t.Errorf("a read that fails: %v", err)
	}
}

// TestAACConverterRoom checks that the room an AAC track gives its
// converter takes a frame in one Process, whatever was held back, and the
// end in one Flush, at each sampling frequency of AAC.
func TestAACConverterRoom(t *testing.T) {
	for _, rate := range []int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350} {
		c, err := resample.New(rate, Rate, 2)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]float32, 2*(c.MaxOutput(aac.FrameLength)+c.Latency()))
		in := make([]float32, 2*aac.FrameLength)
		seed := uint32(rate)
		for range 50 {
			for i := range in {
				seed = seed*1664525 + 1013904223
				in[i] = float32(seed)/(1<<32) - 0.5
			}
			if read, _ := c.Process(in, out); read != aac.FrameLength {
				t.Fatalf("%d Hz: Process read %d frames of %d", rate, read, aac.FrameLength)
			}
		}
		if n := c.Flush(out); n >= len(out)/2 || c.Flush(out) != 0 {
			t.Errorf("%d Hz: Flush wrote %d frames of the %d it had room for", rate, n, len(out)/2)
		}
	}
}

// TestDeckAACAgainstFFmpeg plays the test files as ffmpeg decodes them and
// converts them to 48 kHz, with the long filter internal/audio/resample
// compares with: the two agree, frame for frame, in time and level. It
// skips without ffmpeg.
func TestDeckAACAgainstFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on the PATH")
	}
	for _, path := range aacFiles {
		got := playAll(t, path)
		want := ffmpegDecode(t, path)
		// ffmpeg keeps the padding of the last packet.
		if len(want) < len(got) {
			t.Fatalf("%s: %d frames, ffmpeg's %d", path, len(got)/2, len(want)/2)
		}
		s := snr(got[2*1024:len(got)-2*1024], want[2*1024:len(got)-2*1024])
		t.Logf("%s: %.1f dB from ffmpeg's decode", path, s)
		if s < 100 {
			t.Errorf("%s: %.1f dB from ffmpeg's decode", path, s)
		}
	}
}

// playAll plays a file whole on a deck.
func playAll(t *testing.T, path string) []float32 {
	t.Helper()
	d := newDeck(path, 0, fileOpener, func() {})
	defer d.close()
	got := drain(t, d)
	if err := d.failure(); err != nil {
		t.Fatal(err)
	}
	return got
}

// ffmpegDecode decodes a file with ffmpeg into stereo at Rate, a mono
// file's in both channels.
func ffmpegDecode(t *testing.T, path string) []float32 {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", path,
		"-af", "aresample=48000:filter_type=kaiser:kaiser_beta=12:filter_size=256:cutoff=0.95:phase_shift=14",
		"-f", "f32le", "-").Output()
	if err != nil {
		t.Fatalf("ffmpeg: %v", err)
	}
	pcm := make([]float32, len(out)/4)
	binary.Read(bytes.NewReader(out), binary.LittleEndian, pcm)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := mp4.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if r.Track().Channels == 1 {
		stereo := make([]float32, 2*len(pcm))
		for i, v := range pcm {
			stereo[2*i], stereo[2*i+1] = v, v
		}
		pcm = stereo
	}
	return pcm
}

// TestDeckPlaysAACThroughTheProxy plays an MP4 file as the app does: from
// the stream package's proxy, which yt-dlp found it for, and which keeps it
// in its cache, from which it plays again with the network gone.
func TestDeckPlaysAACThroughTheProxy(t *testing.T) {
	b, err := os.ReadFile(aacFragments)
	if err != nil {
		t.Fatal(err)
	}
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
	}))
	defer media.Close()
	r := stream.NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": "%s/a?expire=%d", "ext": "m4a", "acodec": "mp4a.40.2", "filesize": %d}`,
			media.URL, time.Now().Add(6*time.Hour).Unix(), len(b)), nil
	}
	p := stream.NewProxy(r)
	p.Cache = stream.NewCache(t.TempDir(), 1<<20)
	open := func(ctx context.Context, videoID string, interrupted func() bool) (Source, error) {
		return p.Open(ctx, videoID, interrupted)
	}
	const videoID = "dpgUTslQkFs"
	want := aacReference(t, aacFragments)
	for _, from := range []string{"YouTube", "the cache"} {
		d := newDeck(videoID, 0, open, func() {})
		got := drain(t, d)
		d.close()
		// As the file plays, past the converter's window of its first frame.
		if err := d.failure(); err != nil || len(got) != len(want) || !slices.Equal(got[2*64:], want[2*64:]) {
			t.Fatalf("from %s: %d frames of %d, %.1f dB from the whole decode, %v", from, len(got)/2, len(want)/2, snr(got, want), err)
		}
		for deadline := time.Now().Add(5 * time.Second); !p.Cache.Has(videoID); {
			if time.Now().After(deadline) {
				t.Fatal("the stream played whole is not in the cache")
			}
			time.Sleep(time.Millisecond)
		}
		media.Close()
	}
}
