package resample

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// ffmpegResampler is the filter FFmpeg compares with: the resampler of
// libswresample with a long Kaiser-windowed filter, which converts below
// its cutoff as exactly as this package does.
const ffmpegResampler = "filter_type=kaiser:kaiser_beta=12:filter_size=256:cutoff=0.95:phase_shift=14"

// agreement returns how far apart in dB, as a signal-to-noise ratio, the
// frames of got and want are, both interleaved stereo, away from their
// ends, which each converter fills in its own way.
func agreement(got, want []float32) float64 {
	const margin = 2 * 1024
	var sig, diff float64
	for i := margin; i < min(len(got), len(want))-margin; i++ {
		d := float64(got[i]) - float64(want[i])
		sig += float64(want[i]) * float64(want[i])
		diff += d * d
	}
	return 10 * math.Log10(sig/diff)
}

// readFloats reads a file of little-endian floats.
func readFloats(t *testing.T, name string) []float32 {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	f := make([]float32, len(b)/4)
	binary.Read(bytes.NewReader(b), binary.LittleEndian, f)
	return f
}

// TestFFmpegFiles converts the chord of testdata, a quarter of a second at
// 44.1 kHz, as FFmpeg 7.0 converted it to 48 kHz: the two agree, frame for
// frame, in time and level. They were made with
//
//	ffmpeg -f lavfi -i "aevalsrc=0.2*sin(2*PI*110*t)+0.2*sin(2*PI*1234*t+1)+0.2*sin(2*PI*5000*t+2)+0.15*sin(2*PI*11000*t+3)+0.1*sin(2*PI*16000*t+4)|0.2*sin(2*PI*220*t)+0.2*sin(2*PI*3000*t+1)+0.2*sin(2*PI*7000*t+2)+0.15*sin(2*PI*13000*t+3)+0.1*sin(2*PI*18000*t+4):s=44100:d=0.25" -f f32le chord-44100.f32
//	ffmpeg -f f32le -ar 44100 -ac 2 -i chord-44100.f32 -af aresample=48000:filter_type=kaiser:kaiser_beta=12:filter_size=256:cutoff=0.95:phase_shift=14 -f f32le chord-48000.f32
func TestFFmpegFiles(t *testing.T) {
	in, want := readFloats(t, "chord-44100.f32"), readFloats(t, "chord-48000.f32")
	c, err := New(44100, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := convert(t, c, in)
	snr := agreement(got, want)
	t.Logf("%d frames here, %d from ffmpeg, %.1f dB apart", len(got)/2, len(want)/2, snr)
	if len(got) != len(want) || snr < 100 {
		t.Errorf("%d frames, %.1f dB from FFmpeg's %d", len(got)/2, snr, len(want)/2)
	}
}

// chord returns frames of interleaved stereo at rate: 24 tones on each
// channel, from 40 Hz to 0.68 of the Nyquist frequency, 15 kHz at 44.1
// kHz, below where the passband of any good converter ends, of random
// phases, the same for a seed.
func chord(seed uint32, rate, frames int) []float32 {
	type tone struct{ hz, phase float64 }
	var chans [2][24]tone
	top := 0.68 * float64(rate) / 2
	for c := range chans {
		for i := range chans[c] {
			seed = seed*1664525 + 1013904223
			chans[c][i] = tone{40 * math.Pow(top/40, float64(i)/23), 2 * math.Pi * float64(seed) / (1 << 32)}
		}
	}
	out := make([]float32, 2*frames)
	for i := range frames {
		t := float64(i) / float64(rate)
		for c, tones := range chans {
			var v float64
			for _, tn := range tones {
				v += 0.03 * math.Sin(2*math.Pi*tn.hz*t+tn.phase)
			}
			out[2*i+c] = float32(v)
		}
	}
	return out
}

// TestFFmpeg converts a chord from each rate AAC has to 48 kHz here and
// with FFmpeg: the two agree, frame for frame, in time and level. It needs
// ffmpeg, and skips without.
func TestFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("no ffmpeg to compare with")
	}
	const to = 48000
	for _, from := range []int{44100, 32000, 24000, 22050} {
		in := chord(uint32(from), from, 2*from)
		var raw, stdout, stderr bytes.Buffer
		binary.Write(&raw, binary.LittleEndian, in)
		cmd := exec.Command(ffmpeg, "-v", "error", "-f", "f32le", "-ar", strconv.Itoa(from), "-ac", "2", "-i", "pipe:0",
			"-af", "aresample="+strconv.Itoa(to)+":"+ffmpegResampler, "-f", "f32le", "pipe:1")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = &raw, &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %s", err, stderr.Bytes())
		}
		want := make([]float32, stdout.Len()/4)
		binary.Read(&stdout, binary.LittleEndian, want)

		c, err := New(from, to, 2)
		if err != nil {
			t.Fatal(err)
		}
		got := convert(t, c, in)
		snr := agreement(got, want)
		t.Logf("%d Hz: %d frames here, %d from ffmpeg, %.1f dB apart", from, len(got)/2, len(want)/2, snr)
		if len(got) != len(want) || snr < 100 {
			t.Errorf("%d Hz: %d frames, %.1f dB from FFmpeg's %d", from, len(got)/2, snr, len(want)/2)
		}
	}
}
