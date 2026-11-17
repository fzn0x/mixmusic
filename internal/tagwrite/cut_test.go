package tagwrite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/ogg"
	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// remuxCut remuxes file leaving out what leave returns, and fails the test
// on an error.
func remuxCut(t *testing.T, file []byte, leave func(time.Duration) []Cut) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := RemuxWebMWith(&out, bytes.NewReader(file), Tags{Title: "Harbour Tone"}, Title, Remux{Leave: leave}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// preSkipOf is the pre-skip of the OpusHead of an Ogg Opus file.
func preSkipOf(t *testing.T, b []byte) int64 {
	t.Helper()
	head := oggPackets(t, b)[0]
	if !bytes.HasPrefix(head, []byte("OpusHead")) {
		t.Fatalf("its first packet is %q", head[:min(len(head), 8)])
	}
	return int64(binary.LittleEndian.Uint16(head[10:]))
}

// decodeOgg decodes an Ogg Opus file whole, as players do: every packet
// from the first, the pre-skip dropped from the start, and the samples
// past the last page's granule position from the end. It returns the
// samples, stereo, and fails the test on a granule position that isn't
// where the packets it ends are.
func decodeOgg(t *testing.T, b []byte) []float32 {
	t.Helper()
	r, err := ogg.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	preSkip := int(binary.LittleEndian.Uint16(head.Data[10:]))
	if _, err := r.ReadPacket(); err != nil {
		t.Fatal(err)
	}
	dec, err := opus.NewDecoder(opusRate, 2)
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]float32, 2*5760)
	var out []float32
	at := int64(0)
	for {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n, err := dec.Decode(p.Data, pcm, false)
		if err != nil {
			t.Fatalf("a packet at %d doesn't decode: %v", at, err)
		}
		to := n
		switch {
		case p.Last:
			if p.Granule > at+int64(n) || p.Granule <= at {
				t.Fatalf("the last page ends at granule %d, of a last packet from %d to %d", p.Granule, at, at+int64(n))
			}
			to = int(p.Granule - at)
		case p.Granule >= 0 && p.Granule != at+int64(n):
			t.Fatalf("a page ends at granule %d, where its last packet ends at %d", p.Granule, at+int64(n))
		}
		out = append(out, pcm[:2*to]...)
		at += int64(n)
	}
	if len(out) < 2*preSkip {
		t.Fatalf("%d samples, fewer than the pre-skip's %d", len(out)/2, preSkip)
	}
	return out[2*preSkip:]
}

// TestRemuxWebMCuts leaves stretches out of a remux, by whole packets: one
// from the start by the pre-skip, from the packet 200 ms before its end, one
// between by the packets whose middle is in it, and one to the end by the
// last page's granule position. The pages count the samples kept, in turn,
// with their checksums.
func TestRemuxWebMCuts(t *testing.T) {
	// 200 packets of 20 ms, but the one at 0.96 s, of 60 ms: 4.04 s, of
	// which the last block drops 400 samples.
	var packets [][]byte
	for i := range 200 {
		frames := 1
		if i == 50 {
			frames = 3
		}
		packets = append(packets, opusPacket(i, frames, 120+i%40))
	}
	file, _ := webmFile(t, "A_OPUS", packets, 400)
	var told time.Duration
	out := remuxCut(t, file, func(length time.Duration) []Cut {
		told = length
		// Out of order, as Leave may give them.
		return []Cut{{3500 * time.Millisecond, length}, {0, time.Second}, {2 * time.Second, 2500 * time.Millisecond}}
	})
	if told != 4040*time.Millisecond {
		t.Errorf("Leave was told a length of %v", told)
	}
	pages := oggPages(t, out)
	if last := pages[len(pages)-1]; last.flags&pageLast == 0 {
		t.Error("the last page isn't flagged so")
	}

	// The audio starts at 1 s, the sample 48312 of the stream with its
	// pre-skip of 312: the copy starts at the packet that holds the sample
	// 200 ms before, 40 at 38400, and drops the 9912 samples between.
	if got := preSkipOf(t, out); got != 48312-38400 {
		t.Errorf("a pre-skip of %d", got)
	}
	// From 2 s to 2.5 s, the samples 96312 to 120312: the packets whose
	// middle is there, 98 at 96000 to 122 at 119040, go. The stretch to the
	// end starts at 3.5 s, the sample 168312: packet 173, from 168000, is
	// the last kept, and the audio ends at that sample, 105912 in the
	// copy, which drops 62400 samples before it.
	want := append(append([][]byte{}, packets[40:98]...), packets[123:174]...)
	got := oggPackets(t, out)[2:]
	if len(got) != len(want) {
		t.Fatalf("%d packets, want %d", len(got), len(want))
	}
	for i := range want {
		// oggPackets puts each packet's granule position after it.
		if g := got[i]; !bytes.Equal(g[:len(g)-8], want[i]) {
			t.Fatalf("packet %d of the copy isn't the one due", i)
		}
	}
	r, err := ogg.NewReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	total, end := int64(0), int64(-1)
	for i := 0; ; i++ {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			continue
		}
		n, _ := opus.PacketSamples(p.Data, opusRate)
		total += int64(n)
		switch {
		case p.Last:
			end = p.Granule
		case p.Granule >= 0 && p.Granule != total:
			t.Errorf("packet %d ends a page at granule %d, after %d samples", i, p.Granule, total)
		}
	}
	if end != 168312-62400 {
		t.Errorf("the copy ends at granule %d, want %d", end, 168312-62400)
	}
	// 4.04 s less 1 s, 0.5 s and 0.54 s.
	if got := end - preSkipOf(t, out); got != 2*opusRate {
		t.Errorf("the copy lasts %d samples, want %d", got, 2*opusRate)
	}
}

// TestRemuxWebMCutsJoin joins the stretches that overlap or touch, leaves
// out nothing for no stretch, and refuses to leave out all of the audio.
func TestRemuxWebMCutsJoin(t *testing.T) {
	var packets [][]byte
	for i := range 100 {
		packets = append(packets, opusPacket(i, 1, 100))
	}
	file, _ := webmFile(t, "A_OPUS", packets, 0)
	plain := remuxCut(t, file, nil)
	if got := remuxCut(t, file, func(time.Duration) []Cut { return nil }); !bytes.Equal(got, plain) {
		t.Error("leaving out nothing changes the copy")
	}
	if got := remuxCut(t, file, func(time.Duration) []Cut { return []Cut{{time.Second, time.Second}, {-time.Second, 0}} }); !bytes.Equal(got, plain) {
		t.Error("empty stretches change the copy")
	}

	// 0.5 s to 0.8 s, and 0.7 s to 1 s, are 0.5 s to 1 s: the packets 25 to
	// 49 go, whose middles are from 0.51 s to 0.99 s.
	out := remuxCut(t, file, func(time.Duration) []Cut {
		return []Cut{{700 * time.Millisecond, time.Second}, {500 * time.Millisecond, 800 * time.Millisecond}}
	})
	got := oggPackets(t, out)
	data := func(i int) []byte { return got[i][:len(got[i])-8] }
	if len(got) != 2+75 || !bytes.Equal(data(2+24), packets[24]) || !bytes.Equal(data(2+25), packets[50]) {
		t.Errorf("%d packets", len(got)-2)
	}
	if preSkipOf(t, out) != 312 {
		t.Error("a stretch between changed the pre-skip")
	}

	all := []Cut{{0, time.Second}, {time.Second, 2 * time.Second}}
	if err := RemuxWebMWith(io.Discard, bytes.NewReader(file), Tags{}, 0, Remux{Leave: func(time.Duration) []Cut { return all }}); !errors.Is(err, errAllCut) {
		t.Errorf("leaving out all of it: %v", err)
	}
}

// TestRemuxWebMCutDecodes leaves stretches out of the remux of a recorded
// WebM file of Opus, as YouTube's are, decodes the copy whole and checks
// it lasts what it should: what the stretches leave of it, to the sample
// at its start and end, and within half a packet on either side of one
// between. What is left is the audio of the WebM file, there.
func TestRemuxWebMCutDecodes(t *testing.T) {
	file := fixture(t, filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	r, err := webm.NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	length := r.Duration()
	whole := decodeOgg(t, remuxCut(t, file, nil))
	// The WebM file tells a length 8 ms longer than its audio: its Duration
	// counts the codec's delay.
	size := int64(len(whole) / 2)
	if d := size - samplesOf(length); d > 0 || d < -960 {
		t.Fatalf("the whole decodes to %d samples, of %v", size, length)
	}

	start, from, to, tail := 500*time.Millisecond, 1200*time.Millisecond, 1700*time.Millisecond, 2500*time.Millisecond
	out := remuxCut(t, file, func(l time.Duration) []Cut {
		return []Cut{{0, start}, {from, to}, {tail, l}}
	})
	oggPages(t, out)
	cut := decodeOgg(t, out)
	want := samplesOf(tail - start - (to - from))
	if got := int64(len(cut) / 2); got < want-960 || got > want+960 {
		t.Fatalf("the copy decodes to %d samples, want %d within a packet", got, want)
	}

	// like reports whether the copy's samples from at, for d, are the
	// whole's from wholeAt, within 1 % of the whole's loudness.
	like := func(at, wholeAt int64, d time.Duration) (float64, bool) {
		n := 2 * samplesOf(d)
		a, b := cut[2*at:2*at+n], whole[2*wholeAt:2*wholeAt+n]
		var diff, power float64
		for i := range a {
			diff += float64(a[i]-b[i]) * float64(a[i]-b[i])
			power += float64(b[i]) * float64(b[i])
		}
		ratio := math.Sqrt(diff / max(power, 1e-9))
		return ratio, power > 0 && ratio < 0.01
	}
	// The copy starts on the sample where the start's stretch ends.
	if off, ok := like(0, samplesOf(start), 600*time.Millisecond); !ok {
		t.Errorf("the start of the copy is %.3f off the WebM's from %v", off, start)
	}
	// After the stretch between, the audio is the WebM's from where the
	// packets kept start again, once the decoder settled.
	before := samplesOf(from - start)
	gone := samplesOf(tail-start) - int64(len(cut)/2)
	settled := samplesOf(100 * time.Millisecond)
	if off, ok := like(before+settled, samplesOf(start)+before+gone+settled, 600*time.Millisecond); !ok {
		t.Errorf("the copy after the stretch between is %.3f off the WebM's", off)
	}
	if gone%960 != 0 {
		t.Errorf("%d samples left out between, not whole packets of 20 ms", gone)
	}
	// It ends on the sample where the end's stretch starts.
	end := int64(len(cut) / 2)
	if off, ok := like(end-samplesOf(200*time.Millisecond), samplesOf(tail-200*time.Millisecond), 200*time.Millisecond); !ok {
		t.Errorf("the end of the copy is %.3f off the WebM's before %v", off, tail)
	}
}

// TestFFmpegRemuxWebMCut has ffmpeg make a WebM file of Opus, as YouTube's
// are, remuxes it leaving out a stretch at its start, one between and one
// at its end, and has ffmpeg decode the copy: it lasts what is left.
func TestFFmpegRemuxWebMCut(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg isn't on the PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "tone.webm")
	out, err := exec.Command("ffmpeg", "-v", "error", "-nostdin", "-f", "lavfi", "-i", "sine=frequency=330:duration=6.37",
		"-ac", "2", "-c:a", "libopus", "-b:a", "96k", "-f", "webm", src).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg can't make it: %v\n%s", err, out)
	}
	cut := remuxCut(t, fixture(t, src), func(length time.Duration) []Cut {
		return []Cut{{0, 1500 * time.Millisecond}, {3 * time.Second, 4 * time.Second}, {5 * time.Second, length}}
	})
	dst := filepath.Join(dir, "tone.opus")
	if err := os.WriteFile(dst, cut, 0o644); err != nil {
		t.Fatal(err)
	}
	pcm, err := exec.Command("ffmpeg", "-v", "error", "-nostdin", "-i", dst, "-f", "s16le", "-ac", "2", "-ar", "48000", "-").Output()
	if err != nil {
		t.Fatalf("ffmpeg decodes the copy: %v", err)
	}
	// 1.5 s, 1 s and 1.37 s left out of 6.37 s: 2.5 s, within half a
	// packet on either side of the stretch between, of two channels of 16
	// bits.
	got, want := len(pcm)/4, 25*opusRate/10
	if got < want-960 || got > want+960 {
		t.Errorf("ffmpeg decodes the copy to %d samples, want %d within a packet", got, want)
	}
}
