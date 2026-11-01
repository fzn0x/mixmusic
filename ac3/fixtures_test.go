package ac3

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "encode the fixtures of testdata again with ffmpeg, and rewrite fixtures.json")

// fixture is a stream of testdata: of an encoding of ffmpeg 7.0's AC-3
// encoder, its dithflags cleared, as undither does, then edited by edits,
// in turn, to use the tools the encoder doesn't.
type fixture struct {
	encoding
	edits []string
}

// fixtures are the streams of testdata, a fifth of a second or so of
// synthesized tones and noise each, which exercise every audio coding
// mode, sample rate and tool between them. go test -run TestFixtures
// -update makes them again, of the encodings and edits below.
var fixtures = func() []fixture {
	six := strings.Join([]string{tone(0.4, 440, 0.1, 0), tone(0.4, 550, 0.12, 0.02), tone(0.4, 660, 0.14, 0.04),
		"0.5*sin(2*PI*50*t)", tone(0.3, 770, 0.08, 0.06), tone(0.3, 880, 0.16, 0.03)}, "|")
	three := tone(0.4, 440, 0.1, 0) + "|" + tone(0.4, 550, 0.07, 0.02) + "|" + tone(0.4, 990, 0.09, 0.05)
	four := three + "|" + tone(0.3, 660, 0.11, 0.03)
	noisy := "0.1*(random(0)*2-1)"
	return []fixture{
		{encoding{"5.1-48k-cpl", "aevalsrc=" + six + ":s=48000:d=0.2", "5.1", []string{"-b:a", "192k", "-channel_coupling", "1"}}, nil},
		{encoding{"5.1-48k-edited", "aevalsrc=" + six + ":s=48000:d=0.2", "5.1", []string{"-b:a", "384k"}}, []string{"delta", "short", "dynrng"}},
		{encoding{"5.1-44k-loro", "aevalsrc=" + six + ":s=44100:d=0.12", "5.1", []string{"-b:a", "448k", "-dmix_mode", "loro", "-loro_cmixlev", "0.841", "-loro_surmixlev", "0.5"}}, nil},
		{encoding{"5.0-32k", "aevalsrc=" + strings.Join([]string{tone(0.4, 440, 0.1, 0), noisy, tone(0.3, 300, 0.12, 0), noisy, tone(0.2, 900, 0.1, 0.05)}, "|") + ":s=32000:d=0.2", "5.0", []string{"-b:a", "256k"}}, nil},
		{encoding{"stereo-48k-remat", "aevalsrc=0.4*sin(2*PI*440*t)+0.05*(random(0)*2-1)|0.38*sin(2*PI*440*t)+0.05*(random(0)*2-1):s=48000:d=0.2", "stereo", []string{"-b:a", "192k"}}, []string{"delta"}},
		{encoding{"stereo-44k-phase", "aevalsrc=" + tone(0.4, 500, 0.1, 0) + "+" + noisy + "|" + tone(0.3, 700, 0.08, 0.03) + "+" + noisy + ":s=44100:d=0.25", "stereo", []string{"-b:a", "96k", "-channel_coupling", "1"}}, []string{"phase"}},
		{encoding{"mono-32k", "aevalsrc=" + tone(0.5, 330, 0.1, 0) + "+" + noisy + ":s=32000:d=0.25", "mono", []string{"-b:a", "64k"}}, []string{"delta", "short"}},
		{encoding{"3.0-48k", "aevalsrc=" + three + ":s=48000:d=0.15", "3.0", []string{"-b:a", "192k"}}, nil},
		{encoding{"2.1-lfe-32k", "aevalsrc=" + tone(0.4, 440, 0.1, 0) + "|" + tone(0.4, 550, 0.07, 0.02) + "|0.5*sin(2*PI*40*t)|" + tone(0.3, 990, 0.09, 0.05) + ":s=32000:d=0.2", "FL+FR+LFE+BC", []string{"-b:a", "192k"}}, nil},
		{encoding{"3.1-44k", "aevalsrc=" + four + ":s=44100:d=0.15", "4.0", []string{"-b:a", "256k"}}, nil},
		{encoding{"quad-48k", "aevalsrc=" + four + ":s=48000:d=0.15", "quad", []string{"-b:a", "256k"}}, nil},
		{encoding{"mono-lfe-44k", "aevalsrc=" + tone(0.4, 440, 0.1, 0) + "|0.5*sin(2*PI*40*t):s=44100:d=0.15", "FC+LFE", []string{"-b:a", "128k"}}, nil},
	}
}()

// make encodes the fixture into testdata, with ffmpeg, and edits it.
func (f fixture) make(t testing.TB, path string) {
	t.Helper()
	data, err := os.ReadFile(f.encode(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	data = undither(t, data)
	for _, e := range f.edits {
		n := 0
		switch e {
		case "short":
			data = edit(t, data, shortBlocks)
			n = 1
		case "dynrng":
			data = edit(t, data, dynamicRanges(&n))
		case "phase":
			data = edit(t, data, phaseFlags(&n))
		case "delta":
			data = edit(t, data, func(s bitstring, m blockMarks) bitstring {
				if s[len(s)-18] == 0 {
					if r := reencode(t, s.bytes()); r != nil {
						n++
						return r
					}
				}
				return s
			})
		default:
			t.Fatalf("no edit %s", e)
		}
		if n == 0 {
			t.Fatalf("%s: no frame has room for %s", f.name, e)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// sketches is the number of sketches of a decode.
const sketches = 32

// reference is what fixtures.json holds of a fixture: what ffmpeg's
// decoder decodes it to, as its energy and its sketches, and what its
// frames use.
type reference struct {
	Frames int `json:"frames"`
	// Energy is the sum of the squares of ffmpeg's samples.
	Energy float64 `json:"energy"`
	// Sketches are the sums of ffmpeg's samples, as Decode's six channels
	// would have them, each multiplied by a pseudo-random sign, of a
	// sequence of signs per sketch.
	Sketches [sketches]float64 `json:"sketches"`
	// SNR is the decoder's against ffmpeg's decode, when it was made.
	SNR   float64 `json:"snr"`
	Tools toolUse `json:"tools"`
}

// sketch computes the sketches of samples of six channels, interleaved.
func sketch(pcm []float32) [sketches]float64 {
	var s [sketches]float64
	for k := range s {
		seed := uint32(k+1) * 0x9E3779B9
		var sum float64
		for _, v := range pcm {
			seed = seed*1664525 + 1013904223
			if seed&0x80000000 != 0 {
				sum -= float64(v)
			} else {
				sum += float64(v)
			}
		}
		s[k] = sum
	}
	return s
}

// asDecoded returns ffmpeg's decode of a stream of acmod and lfeon as
// Decode writes it, of six channels.
func asDecoded(ref []float32, acmod int, lfeon bool) []float32 {
	order := ffmpegOrder(acmod, lfeon)
	n := len(order)
	out := make([]float32, len(ref)/n*MaxChannels)
	for f := range len(ref) / n {
		for i, c := range order {
			v := ref[f*n+i]
			if c < 0 {
				out[f*MaxChannels+LeftSurround] = v * minus3dB
				out[f*MaxChannels+RightSurround] = v * minus3dB
			} else {
				out[f*MaxChannels+c] = v
			}
		}
	}
	return out
}

// TestFixtures decodes the fixtures and measures the SNR of their decodes
// against ffmpeg's, without ffmpeg: as the sketches are linear, the
// difference of a decode's sketches and ffmpeg's are sketches of the
// difference of the decodes, and the mean of their squares estimates its
// energy, within a dB or two.
func TestFixtures(t *testing.T) {
	path := filepath.Join("testdata", "fixtures.json")
	want := map[string]*reference{}
	if *update {
		haveFFmpeg(t)
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &want); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			file := filepath.Join("testdata", f.name+".ac3")
			if *update {
				f.make(t, file)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			ours, d, use := decodeAll(t, data)
			if *update {
				ref := asDecoded(ffmpegDecode(t, file), d.h.acmod, d.h.lfeon)
				var energy, noise float64
				for i, v := range ref {
					energy += float64(v) * float64(v)
					e := float64(ours[i] - v)
					noise += e * e
				}
				snr := 10 * math.Log10(energy/noise)
				want[f.name] = &reference{len(ours) / MaxChannels / FrameLength, energy, sketch(ref), math.Round(snr*10) / 10, use}
			}
			w := want[f.name]
			if w == nil {
				t.Fatalf("%s isn't in %s", f.name, path)
			}
			if frames := len(ours) / MaxChannels / FrameLength; frames != w.Frames {
				t.Errorf("%d frames, want %d", frames, w.Frames)
			}
			for _, k := range tools {
				if use[k] != w.Tools[k] {
					t.Errorf("%d blocks use %s, want %d", use[k], k, w.Tools[k])
				}
			}
			got := sketch(ours)
			var noise float64
			for k := range got {
				e := got[k] - w.Sketches[k]
				noise += e * e / sketches
			}
			snr := 10 * math.Log10(w.Energy/noise)
			t.Logf("SNR about %.1f dB (%.1f dB when made); %v", snr, w.SNR, use)
			if snr < 95 {
				t.Errorf("SNR about %.1f dB", snr)
			}
		})
	}
	if *update {
		b, err := json.MarshalIndent(want, "", "\t")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t testing.TB, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
