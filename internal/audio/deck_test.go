package audio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// Test files: Opus in WebM, as ffmpeg and libopus make them. stereo-cues-
// front is laid out as YouTube's files are; fec-stereo-40 is speech coded
// with SILK, with in-band FEC; mono-stream is mono, without Cues.
var (
	cuesFront  = "../../webm/testdata/stereo-cues-front.webm"
	speech     = "../../opus/testdata/fec-stereo-40.webm"
	monoStream = "../../webm/testdata/mono-stream.webm"
)

// memSource is a Source of bytes all there.
type memSource struct{ *bytes.Reader }

func (memSource) Wake()        {}
func (memSource) Close() error { return nil }

// fileOpener opens the files whose URL is their path.
func fileOpener(ctx context.Context, url string, interrupted func() bool) (Source, error) {
	b, err := os.ReadFile(url)
	if err != nil {
		return nil, err
	}
	return memSource{bytes.NewReader(b)}, nil
}

// reference decodes a file whole, as a deck must play it: in stereo,
// without the pre-skip and the padding of the last packet.
func reference(t *testing.T, path string) []float32 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := webm.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	head, err := parseOpusHead(r.Track().CodecPrivate)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := opus.NewDecoder(Rate, 2)
	if err != nil {
		t.Fatal(err)
	}
	dec.SetGain(head.gain)
	pcm := make([]float32, 2*5760)
	var out []float32
	var pad time.Duration
	for {
		p, err := r.ReadPacket()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n, err := dec.Decode(p.Data, pcm, false)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, pcm[:2*n]...)
		pad = p.DiscardPadding
	}
	delay := frames(r.Track().CodecDelay)
	return out[2*delay : len(out)-2*int(frames(pad))]
}

// drain reads a deck's ring to the end of its track.
func drain(t *testing.T, d *deck) []float32 {
	t.Helper()
	var out []float32
	buf := make([]float32, 2*1024)
	deadline := time.Now().Add(10 * time.Second)
	for {
		r := d.ring.Load()
		n := r.readTo(buf)
		out = append(out, buf[:2*n]...)
		if n == 0 {
			if r.done.Load() && r.readable() == 0 {
				return out
			}
			if time.Now().After(deadline) {
				t.Fatalf("the deck gave %d frames, then nothing", len(out)/2)
			}
			time.Sleep(time.Millisecond)
		}
	}
}

// checkFrom checks got, a deck's frames from start, against want, the
// whole decode. WebM gives times in milliseconds, rounded, so the deck
// starts within 1 ms of start; its end is exact: the frames of got end
// where want does, which tells where they start.
func checkFrom(t *testing.T, got, want []float32, start int64, what string) {
	t.Helper()
	extra := int64(len(got)/2) - (int64(len(want)/2) - start)
	if extra < -Rate/1000 || extra > Rate/1000 {
		t.Errorf("%s: %d frames, want %d, within 1 ms", what, len(got)/2, int64(len(want)/2)-start)
		return
	}
	// The decoder starts 80 ms before, which leaves it close to, not
	// exactly at, where decoding from the start leaves it.
	if s := snr(got, want[2*(start-extra):]); s < 40 {
		t.Errorf("%s: SNR %.1f dB against the whole decode", what, s)
	}
}

// snr compares got with want, in dB.
func snr(got, want []float32) float64 {
	var sig, noise float64
	for i := range min(len(got), len(want)) {
		d := float64(got[i]) - float64(want[i])
		sig += float64(want[i]) * float64(want[i])
		noise += d * d
	}
	return 10 * math.Log10(sig/max(noise, 1e-30))
}

func TestDeckPlaysWhole(t *testing.T) {
	for _, path := range []string{cuesFront, speech, monoStream} {
		t.Run(path, func(t *testing.T) {
			want := reference(t, path)
			d := newDeck(path, 0, fileOpener, func() {})
			defer d.close()
			got := drain(t, d)
			if err := d.failure(); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("%d frames, want %d", len(got)/2, len(want)/2)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("frame %d differs: %v, want %v", i/2, got[i], want[i])
				}
			}
			// A file written to a pipe has no duration.
			if total, ok := d.info(); !ok || total <= 0 && path != monoStream {
				t.Errorf("duration %v, %v", total, ok)
			}
		})
	}
}

func TestDeckStartsAt(t *testing.T) {
	want := reference(t, cuesFront)
	for _, at := range []time.Duration{0, 7 * time.Millisecond, 500 * time.Millisecond, 1234 * time.Millisecond, 2900 * time.Millisecond} {
		d := newDeck(cuesFront, at, fileOpener, func() {})
		got := drain(t, d)
		d.close()
		checkFrom(t, got, want, frames(at), "at "+at.String())
	}
}

func TestDeckSeeks(t *testing.T) {
	want := reference(t, cuesFront)
	d := newDeck(cuesFront, 0, fileOpener, func() {})
	defer d.close()
	buf := make([]float32, 2*4800)
	// Read some, seek back and forth, then read to the end.
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
	// After the end, a seek plays again.
	d.seek(0)
	got = drain(t, d)
	if len(got) != len(want) {
		t.Errorf("%d frames from the start again, want %d", len(got)/2, len(want)/2)
	}
}

// stalledSource gives the first n bytes of a file, then waits, as a
// stalled download does, until its reads are interrupted.
type stalledSource struct {
	*bytes.Reader
	n           int64
	interrupted func() bool
	mu          sync.Mutex
	cond        *sync.Cond
	waits       int
}

func (s *stalledSource) Read(p []byte) (int, error) {
	pos, _ := s.Seek(0, io.SeekCurrent)
	if pos >= s.n {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.waits++
		for !s.interrupted() {
			s.cond.Wait()
		}
		return 0, errors.New("interrupted")
	}
	return s.Reader.Read(p[:min(int64(len(p)), s.n-pos)])
}

func (s *stalledSource) Wake() {
	s.mu.Lock()
	s.cond.Broadcast()
	s.mu.Unlock()
}

func (s *stalledSource) Close() error { return nil }

func TestDeckSeekInterruptsAStalledRead(t *testing.T) {
	b, err := os.ReadFile(cuesFront)
	if err != nil {
		t.Fatal(err)
	}
	var src *stalledSource
	opened := make(chan struct{})
	open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
		src = &stalledSource{Reader: bytes.NewReader(b), n: int64(len(b) / 2), interrupted: interrupted}
		src.cond = sync.NewCond(&src.mu)
		close(opened)
		return src, nil
	}
	d := newDeck(cuesFront, 0, open, func() {})
	<-opened
	// Wait for the read that stalls.
	deadline := time.Now().Add(5 * time.Second)
	for {
		src.mu.Lock()
		w := src.waits
		src.mu.Unlock()
		if w > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the deck never waited")
		}
		// Make room in the ring, for the deck to read on.
		d.ring.Load().readTo(make([]float32, 2*4800))
		time.Sleep(time.Millisecond)
	}
	start := time.Now()
	d.seek(100 * time.Millisecond)
	for d.ring.Load().readable() == 0 {
		if time.Since(start) > 5*time.Second {
			t.Fatal("the seek waited for the stalled read")
		}
		time.Sleep(time.Millisecond)
	}
	if err := d.failure(); err != nil {
		t.Errorf("the interrupted read failed the deck: %v", err)
	}
	closed := make(chan struct{})
	go func() {
		d.close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("close waited for a stalled read")
	}
}

func TestDeckFailures(t *testing.T) {
	b, err := os.ReadFile(cuesFront)
	if err != nil {
		t.Fatal(err)
	}
	mono, err := os.ReadFile(monoStream)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		data []byte
		want error
	}{
		{"not WebM", []byte("hello, this is not a WebM file at all"), errUnsupported},
		// About 1.5 s before its end: farther than endSlack.
		{"cut short", b[:len(b)/2], errNetwork},
		// A file that doesn't tell its duration has no end to be near.
		{"cut short without a duration", mono[:len(mono)-300], errNetwork},
	} {
		t.Run(c.name, func(t *testing.T) {
			open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
				return memSource{bytes.NewReader(c.data)}, nil
			}
			d := newDeck("x", 0, open, func() {})
			defer d.close()
			drain(t, d)
			if err := d.failure(); !errors.Is(err, c.want) {
				t.Errorf("failure %v, want %v", err, c.want)
			}
		})
	}
	t.Run("not opened", func(t *testing.T) {
		why := errors.New("YouTube refused the stream")
		open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
			return nil, why
		}
		d := newDeck("x", 0, open, func() {})
		defer d.close()
		drain(t, d)
		if err := d.failure(); err != why {
			t.Errorf("failure %v, want %v", err, why)
		}
	})
}

// brokenSource is the bytes of a file up to n, past which it fails with
// why, as a stream.Source whose download broke off does; at io.EOF when why
// is nil, as a file cut short.
type brokenSource struct {
	*bytes.Reader
	n   int64
	why error
}

func (s *brokenSource) Read(p []byte) (int, error) {
	pos, _ := s.Seek(0, io.SeekCurrent)
	if pos >= s.n {
		if s.why == nil {
			return 0, io.EOF
		}
		return 0, s.why
	}
	return s.Reader.Read(p[:min(int64(len(p)), s.n-pos)])
}

func (*brokenSource) Wake()        {}
func (*brokenSource) Close() error { return nil }

// cutOpener opens a file whose last cut bytes never come.
func cutOpener(t *testing.T, path string, cut int, why error) Opener {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
		return &brokenSource{Reader: bytes.NewReader(b), n: int64(len(b) - cut), why: why}, nil
	}
}

// TestDeckEndsAStreamCutInItsLastMoment ends a track whose stream breaks
// off within endSlack of its end, as at its end: loading it again would
// cost more than what is left.
func TestDeckEndsAStreamCutInItsLastMoment(t *testing.T) {
	want := reference(t, cuesFront)
	for _, why := range []error{nil, errors.New("the network failed while loading the audio")} {
		// The file's last 1000 bytes are its last 66.5 ms.
		d := newDeck("x", 0, cutOpener(t, cuesFront, 1000, why), func() {})
		defer d.close()
		got := drain(t, d)
		if err := d.failure(); err != nil {
			t.Fatalf("%v: the deck failed: %v", why, err)
		}
		if lost := len(want)/2 - len(got)/2; lost <= 0 || int64(lost) > frames(endSlack) {
			t.Errorf("%v: %d frames short of the end", why, lost)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%v: frame %d differs", why, i/2)
			}
		}
		// It waits at its end, as a track that ended does: a seek plays it
		// again.
		d.seek(0)
		if again := drain(t, d); len(again) != len(got) || d.failure() != nil {
			t.Errorf("%v: %d frames from the start again, want %d; %v", why, len(again)/2, len(got)/2, d.failure())
		}
	}
}

func TestParseOpusHead(t *testing.T) {
	head := []byte("OpusHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\xfe\x00")
	h, err := parseOpusHead(head)
	if err != nil || h.channels != 2 || h.preSkip != 312 || h.gain != -512 {
		t.Errorf("%+v, %v", h, err)
	}
	surround := append([]byte(nil), head...)
	surround[18] = 1
	if _, err := parseOpusHead(surround); !errors.Is(err, errUnsupported) {
		t.Errorf("mapping family 1: %v", err)
	}
	if _, err := parseOpusHead(head[:18]); !errors.Is(err, errUnsupported) {
		t.Errorf("short: %v", err)
	}
}
