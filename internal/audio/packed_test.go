package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/aac"
	"github.com/fzn0x/mixmusic/resample"
)

// ADTS files of AAC-LC, half a second or so each of signals made up, as
// ffmpeg's encoder encodes them: the aac package's fixtures, whose
// fixtures_test.go has the commands that made them.
var (
	adtsClicks = "../../aac/testdata/stereo-44k-clicks.aac"
	adtsMono22 = "../../aac/testdata/mono-22k.aac"
	adtsMS48   = "../../aac/testdata/stereo-48k-ms.aac"
	adtsPCE    = "../../aac/testdata/stereo-44k-pce.aac"
)

// adtsFrames splits an ADTS file into its frames, headers and all.
func adtsFrames(t testing.TB, path string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	for len(b) > 0 {
		h, err := aac.ParseADTS(b)
		if err != nil || h.Length > len(b) {
			t.Fatalf("%s: %v", path, err)
		}
		out = append(out, b[:h.Length])
		b = b[h.Length:]
	}
	return out
}

// id3Tag is the ID3 tag that starts a segment of HLS packed audio: a
// PRIV frame of the timestamp of its first frame, in 90 kHz ticks, as RFC
// 8216 section 3.4 has it.
func id3Tag(ticks uint64) []byte {
	priv := []byte("com.apple.streaming.transportStreamTimestamp\x00")
	priv = binary.BigEndian.AppendUint64(priv, ticks&(1<<33-1))
	frame := append([]byte("PRIV"), syncsafe(len(priv))...)
	frame = append(append(frame, 0, 0), priv...)
	tag := append([]byte("ID3\x04\x00\x00"), syncsafe(len(frame))...)
	return append(tag, frame...)
}

func syncsafe(n int) []byte {
	return []byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)}
}

// packedSegments are the frames of the ADTS files at paths as the segments
// of a live stream are: per segments frames, each after an ID3 tag.
func packedSegments(t testing.TB, per int, paths ...string) [][]byte {
	t.Helper()
	var segs [][]byte
	var ticks uint64
	for _, path := range paths {
		fs := adtsFrames(t, path)
		rate := uint64(cfgOf(t, fs[0]).SampleRate)
		for len(fs) > 0 {
			n := min(per, len(fs))
			seg := id3Tag(ticks)
			for _, f := range fs[:n] {
				seg = append(seg, f...)
			}
			segs = append(segs, seg)
			ticks += uint64(n) * aac.FrameLength * 90000 / rate
			fs = fs[n:]
		}
	}
	return segs
}

func cfgOf(t testing.TB, frame []byte) aac.Config {
	t.Helper()
	h, err := aac.ParseADTS(frame)
	if err != nil {
		t.Fatal(err)
	}
	return h.Config
}

// packedReference decodes the frames of the ADTS files at paths, one file
// after the other, as a live track plays them: each file whole, from
// silence, converted to stereo at Rate.
func packedReference(t testing.TB, paths ...string) []float32 {
	t.Helper()
	var out []float32
	for _, path := range paths {
		fs := adtsFrames(t, path)
		cfg := cfgOf(t, fs[0])
		dec, err := aac.NewDecoder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		pcm := make([]float32, 2*aac.FrameLength)
		var in []float32
		for _, f := range fs {
			n, err := dec.Decode(f[7:], pcm)
			if err != nil {
				t.Fatal(err)
			}
			in = append(in, pcm[:2*n]...)
		}
		c, err := resample.New(cfg.SampleRate, Rate, 2)
		if err != nil {
			t.Fatal(err)
		}
		conv := make([]float32, 2*(c.MaxOutput(len(in)/2)+c.Latency()))
		_, n := c.Process(in, conv)
		n += c.Flush(conv[2*n:])
		out = append(out, conv[:2*n]...)
	}
	return out
}

// playPacked plays the bytes of packed audio on a deck, to their end.
func playPacked(t *testing.T, b []byte) ([]float32, *deck) {
	t.Helper()
	open := func(context.Context, string, func() bool) (Source, error) { return memSource{bytes.NewReader(b)}, nil }
	d := newDeck("live", 0, open, func() {})
	t.Cleanup(d.close)
	return drain(t, d), d
}

// TestDeckPlaysPackedAudio plays a live stream's packed audio: its frames,
// past the ID3 tags of its segments, and whatever else comes between them
// that is no frame, as the decoder and the converter make them, frame for
// frame, with no length.
func TestDeckPlaysPackedAudio(t *testing.T) {
	want := packedReference(t, adtsClicks)
	segs := packedSegments(t, 8, adtsClicks)
	if len(segs) < 3 {
		t.Fatalf("%d segments", len(segs))
	}
	got, d := playPacked(t, slices.Concat(segs...))
	if err := d.failure(); err != nil {
		t.Fatal(err)
	}
	if total, ok := d.info(); total != 0 || !ok || !d.live.Load() {
		t.Errorf("a duration of %v, read %v, live %v", total, ok, d.live.Load())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%d frames, %.1f dB from the decode of %d", len(got)/2, snr(got, want), len(want)/2)
	}

	// An ID3 tag of version 3, one of version 4 with a footer, and bytes
	// that are no frame, as a segment cut short would leave, among them a
	// syncword whose header would swallow the next segment.
	v3 := []byte("ID3\x03\x00\x00\x00\x00\x00\x05hello")
	v4 := []byte("ID3\x04\x00\x10\x00\x00\x00\x03abc3DI\x04\x00\x10\x00\x00\x00\x03")
	junk := []byte("\x00\x00\xff\xf1junk\x00\x00\x00")
	if h, err := aac.ParseADTS(junk[2:]); err != nil || h.Length <= len(junk)-2 {
		t.Fatalf("the syncword of the junk is a frame of %d bytes, %v", h.Length, err)
	}
	b := slices.Concat(segs[0], v3, segs[1], v4, junk, segs[2])
	for _, s := range segs[3:] {
		b = append(b, s...)
	}
	if got, d := playPacked(t, b); d.failure() != nil || !slices.Equal(got, want) {
		t.Errorf("with tags and junk: %d frames, %.1f dB from the decode, %v", len(got)/2, snr(got, want), d.failure())
	}
}

// TestDeckPlaysPackedAudioOfAnotherConfig plays a stream whose config
// changes, as after a discontinuity of its playlist: each part decodes
// anew, at its rate, without a frame lost.
func TestDeckPlaysPackedAudioOfAnotherConfig(t *testing.T) {
	paths := []string{adtsClicks, adtsMono22, adtsMS48}
	want := packedReference(t, paths...)
	got, d := playPacked(t, slices.Concat(packedSegments(t, 10, paths...)...))
	if err := d.failure(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%d frames, %.1f dB from the decode of %d", len(got)/2, snr(got, want), len(want)/2)
	}
}

// TestDeckDropsBadPackedFrames plays on past a frame the decoder can't
// decode, which plays as if it never came, but fails once maxBadFrames in
// a row went by.
func TestDeckDropsBadPackedFrames(t *testing.T) {
	// A channel pair element cut short.
	bad := append(header(4, 2, 8), 0x20)
	dec, _ := aac.NewDecoder(aac.Config{ObjectType: aac.ObjectTypeLC, SampleRate: 44100, Channels: 2})
	if _, err := dec.Decode(bad[7:], make([]float32, 2*aac.FrameLength)); !errors.Is(err, aac.ErrFormat) {
		t.Fatalf("the bad frame decodes: %v", err)
	}
	want := packedReference(t, adtsClicks)
	segs := packedSegments(t, 8, adtsClicks)
	b := slices.Concat(segs[0], bad, segs[1])
	for _, s := range segs[2:] {
		b = append(b, s...)
	}
	if got, d := playPacked(t, b); d.failure() != nil || !slices.Equal(got, want) {
		t.Errorf("past a bad frame: %d frames, %.1f dB from the decode, %v", len(got)/2, snr(got, want), d.failure())
	}

	b = slices.Concat(segs[0], bytes.Repeat(bad, maxBadFrames+1), segs[1])
	if _, d := playPacked(t, b); !errors.Is(d.failure(), errDecode) {
		t.Errorf("past %d bad frames: %v", maxBadFrames+1, d.failure())
	}
}

// header makes the header of an unprotected ADTS frame of AAC-LC of
// sampling frequency index rate and channel configuration channels, of
// length bytes.
func header(rate, channels, length int) []byte {
	return []byte{0xFF, 0xF1, byte(1<<6 | rate<<2 | channels>>2), byte(channels&3<<6 | length>>11),
		byte(length >> 3), byte(length&7<<5 | 0x1F), 0xFC}
}

func TestDeckPackedAudioFailures(t *testing.T) {
	clicks := adtsFrames(t, adtsClicks)
	main := slices.Clone(clicks[0])
	main[2] &^= 0xC0 // the profile of AAC Main
	for _, c := range []struct {
		name string
		data []byte
		says string
	}{
		{"no frames after the tag", append(id3Tag(0), bytes.Repeat([]byte{0x47}, maxJunk+100)...), "no ADTS frame"},
		{"too few bytes for a frame", append(id3Tag(0), 0x47, 0x47), "no ADTS frame"},
		{"AAC Main", append(id3Tag(0), main...), "object type 1"},
		// A program config element in the frames of a stream of channel
		// configuration 0.
		{"channel configuration 0", slices.Concat(adtsFrames(t, adtsPCE)...), "channel configuration 0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, d := playPacked(t, c.data)
			if err := d.failure(); !errors.Is(err, errUnsupported) || !strings.Contains(err.Error(), c.says) {
				t.Errorf("failure %v, want %v saying %q", err, errUnsupported, c.says)
			}
		})
	}
	// A stream cut in the middle of a frame ends there.
	b := slices.Concat(packedSegments(t, 8, adtsClicks)...)
	if _, d := playPacked(t, b[:len(b)-100]); d.failure() != nil {
		t.Errorf("cut short: %v", d.failure())
	}
}

// TestEngineDoesNotSeekLive leaves a live stream where it plays when asked
// to seek: it has no time to seek to.
func TestEngineDoesNotSeekLive(t *testing.T) {
	want := packedReference(t, adtsClicks)
	path := filepath.Join(t.TempDir(), "live.aac")
	if err := os.WriteFile(path, slices.Concat(packedSegments(t, 8, adtsClicks)...), 0o600); err != nil {
		t.Fatal(err)
	}
	e, dev, rec := newTestEngine(t)
	e.Load(path, 0, true)
	buffered(t, e, 0, 4800)
	got := dev.play(t, 4800)
	e.Seek(2 * time.Second)
	if p := e.Position(); p > time.Second {
		t.Errorf("position %v after a seek", p)
	}
	got = append(got, dev.play(t, len(want)/2-4800)...)
	for i := 2 * fadeFrames; i < len(want); i++ {
		if got[i] != want[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], want[i])
		}
	}
	rec.waitFor(t, "ended")
}
