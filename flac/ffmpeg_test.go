package flac

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// haveFFmpeg skips t without ffmpeg on the PATH.
func haveFFmpeg(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg isn't on the PATH")
	}
}

// ffmpeg runs ffmpeg with args.
func ffmpeg(t testing.TB, args ...string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, stderr.Bytes())
	}
}

// wav returns a WAV file of samples, interleaved, of bps bits: 16, or 24
// in 32-bit containers as ffmpeg reads them.
func wav(samples []int32, channels, rate, bps int) []byte {
	size := 2
	if bps > 16 {
		size = 3
	}
	var data bytes.Buffer
	for _, s := range samples {
		for k := range size {
			data.WriteByte(byte(s >> (8 * k)))
		}
	}
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	le(uint32(36 + data.Len()))
	b.WriteString("WAVEfmt ")
	le(uint32(16))
	le(uint16(1))
	le(uint16(channels))
	le(uint32(rate))
	le(uint32(rate * channels * size))
	le(uint16(channels * size))
	le(uint16(size * 8))
	b.WriteString("data")
	le(uint32(data.Len()))
	b.Write(data.Bytes())
	return b.Bytes()
}

// music makes seconds of sound the encoder must work at: tones that come
// and go, a random walk, and moments of silence and of noise at full
// scale, of bps bits.
func music(channels, rate, bps int, seconds float64) []int32 {
	n := int(seconds * float64(rate))
	rng := rand.New(rand.NewPCG(1, 2))
	peak := float64(int64(1)<<(bps-1) - 1)
	out := make([]int32, 0, n*channels)
	walk := make([]float64, channels)
	for i := range n {
		t := float64(i) / float64(rate)
		for c := range channels {
			var v float64
			switch phase := int(t * 4); phase % 4 {
			case 0:
				v = 0.6*math.Sin(2*math.Pi*float64(220*(c+1))*t) + 0.2*math.Sin(2*math.Pi*3000*t)
			case 1:
				walk[c] = max(-0.9, min(0.9, walk[c]+0.01*(rng.Float64()-0.5)))
				v = walk[c]
			case 2:
				if i%5000 < 2500 {
					v = 0
				} else {
					v = rng.Float64()*2 - 1
				}
			case 3:
				v = 0.3 * math.Sin(2*math.Pi*440*t) * math.Sin(2*math.Pi*3*t)
			}
			out = append(out, int32(math.Round(v*peak)))
		}
	}
	return out
}

// TestFFmpegLossless encodes sound with ffmpeg's FLAC encoder, at each of
// its levels and ways of coding stereo, and decodes it back as it was,
// sample for sample.
func TestFFmpegLossless(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	for _, c := range []struct {
		name                string
		channels, rate, bps int
		args                []string
	}{
		{"level0", 2, 44100, 16, []string{"-compression_level", "0"}},
		{"level5", 2, 44100, 16, []string{"-compression_level", "5"}},
		{"level12", 2, 48000, 16, []string{"-compression_level", "12"}},
		{"left-side", 2, 44100, 16, []string{"-ch_mode", "left_side"}},
		{"right-side", 2, 44100, 16, []string{"-ch_mode", "right_side"}},
		{"mid-side", 2, 44100, 16, []string{"-ch_mode", "mid_side"}},
		{"fixed", 1, 22050, 16, []string{"-lpc_type", "fixed"}},
		{"verbatim", 2, 32000, 16, []string{"-lpc_type", "none", "-compression_level", "0"}},
		{"exact-rice", 2, 44100, 16, []string{"-exact_rice_parameters", "1", "-compression_level", "8"}},
		{"24-bit", 2, 96000, 24, []string{"-sample_fmt", "s32", "-compression_level", "8"}},
		{"24-bit-mid-side", 2, 88200, 24, []string{"-sample_fmt", "s32", "-ch_mode", "mid_side"}},
		{"surround", 6, 48000, 16, nil},
		{"small-frames", 2, 44100, 16, []string{"-frame_size", "300"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := music(c.channels, c.rate, c.bps, 1.5)
			src, out := filepath.Join(dir, c.name+".wav"), filepath.Join(dir, c.name+".flac")
			if err := os.WriteFile(src, wav(in, c.channels, c.rate, c.bps), 0o644); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"-i", src, "-c:a", "flac"}, c.args...)
			if c.bps == 24 {
				args = append(args, "-bits_per_raw_sample", "24")
			}
			ffmpeg(t, append(args, out)...)
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			r := open(t, data)
			if i := r.Info(); i.BitsPerSample != c.bps || i.Channels != c.channels || i.SampleRate != c.rate {
				t.Fatalf("info %+v", i)
			}
			got := decodeAll(t, r)
			if len(got) != len(in) {
				t.Fatalf("%d samples, want %d", len(got), len(in))
			}
			for i := range in {
				if got[i] != in[i] {
					t.Fatalf("sample %d of channel %d is %d, want %d", i/c.channels, i%c.channels, got[i], in[i])
				}
			}
			if sum(got, c.bps) != r.Info().MD5 {
				t.Error("the MD5 of STREAMINFO differs")
			}
		})
	}
}
