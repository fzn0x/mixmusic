package aac

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

// fixtures are the files of testdata: half a second or so of synthesized
// signals, as ffmpeg 7.0's aac encoder encodes them, which exercise the
// tools of AAC-LC between them. The one of M/S in every band leaves PNS
// off: ffmpeg's decoder gives each channel noise of its own where the
// standard has them share it, as TestCorrelatedNoise checks. They were
// made with
//
//	ffmpeg -f lavfi -i "aevalsrc=0.2*sin(2*PI*330*t)+0.8*lt(mod(t\,0.2)\,0.002)*sin(2*PI*3000*t)|0.2*sin(2*PI*495*t)+0.7*lt(mod(t+0.1\,0.2)\,0.002):s=44100:d=0.6" -c:a aac -b:a 128k -f adts stereo-44k-clicks.aac
//	ffmpeg -f lavfi -i "aevalsrc=0.6*(random(0)*2-1)*gt(mod(t\,0.25)\,0.15)+0.1*sin(2*PI*220*t):s=48000:d=0.6" -c:a aac -b:a 96k -f adts mono-48k-bursts.aac
//	ffmpeg -f lavfi -i "aevalsrc=0.4*sin(2*PI*440*t)+0.3*lt(mod(t\,0.2)\,0.002)+0.05*(random(0)*2-1)|0.38*sin(2*PI*440*t)+0.3*lt(mod(t\,0.2)\,0.002)+0.05*(random(0)*2-1):s=48000:d=0.6" -c:a aac -b:a 96k -aac_ms 1 -aac_pns 0 -f adts stereo-48k-ms.aac
//	ffmpeg -f lavfi -i "aevalsrc=(0.5*sin(2*PI*1000*t)+0.3*sin(2*PI*2500*t))*exp(-30*mod(t\,0.25))|(0.4*sin(2*PI*800*t))*exp(-40*mod(t+0.1\,0.25)):s=44100:d=2" -c:a aac -b:a 128k -aac_tns 1 -f adts stereo-44k-tns.aac
//	ffmpeg -f lavfi -i "aevalsrc=0.3*(random(0)*2-1)+0.2*sin(2*PI*300*t)|-0.15*(random(0)*2-1)+0.2*sin(2*PI*450*t):s=44100:d=0.6" -c:a aac -b:a 48k -aac_is 1 -f adts stereo-44k-is.aac
//	ffmpeg -f lavfi -i "aevalsrc=0.4*sin(2*PI*300*t)+0.3*lt(mod(t\,0.3)\,0.003):s=22050:d=0.6" -c:a aac -b:a 48k -f adts mono-22k.aac
//	ffmpeg -f lavfi -i "aevalsrc=0.3*sin(2*PI*500*t)+0.2*lt(mod(t\,0.3)\,0.003)|0.3*sin(2*PI*700*t):s=32000:d=0.6" -c:a aac -b:a 96k -f adts stereo-32k.aac
//	ffmpeg -f lavfi -i "aevalsrc=1.4*sin(2*PI*200*t)+0.3*sin(2*PI*2000*t)|0.001*sin(2*PI*880*t):s=44100:d=0.4" -c:a aac -b:a 192k -f adts stereo-44k-loud-quiet.aac
//	ffmpeg -f lavfi -i "aevalsrc=0.4*sin(2*PI*440*t)|0.3*sin(2*PI*660*t):s=44100:d=0.3" -c:a aac -b:a 128k -aac_pce 1 -f adts stereo-44k-pce.aac
//
// fixtures.json holds, for each, what TestFixtures compares the decode
// with, which go test -run TestFixtures -update writes again.
var fixtures = []encoding{
	{"stereo-44k-clicks", 2, "aevalsrc=0.2*sin(2*PI*330*t)+0.8*lt(mod(t\\,0.2)\\,0.002)*sin(2*PI*3000*t)|0.2*sin(2*PI*495*t)+0.7*lt(mod(t+0.1\\,0.2)\\,0.002):s=44100:d=0.6", []string{"-b:a", "128k"}},
	{"mono-48k-bursts", 1, "aevalsrc=0.6*(random(0)*2-1)*gt(mod(t\\,0.25)\\,0.15)+0.1*sin(2*PI*220*t):s=48000:d=0.6", []string{"-b:a", "96k"}},
	{"stereo-48k-ms", 2, "aevalsrc=0.4*sin(2*PI*440*t)+0.3*lt(mod(t\\,0.2)\\,0.002)+0.05*(random(0)*2-1)|0.38*sin(2*PI*440*t)+0.3*lt(mod(t\\,0.2)\\,0.002)+0.05*(random(0)*2-1):s=48000:d=0.6", []string{"-b:a", "96k", "-aac_ms", "1", "-aac_pns", "0"}},
	{"stereo-44k-tns", 2, "aevalsrc=(0.5*sin(2*PI*1000*t)+0.3*sin(2*PI*2500*t))*exp(-30*mod(t\\,0.25))|(0.4*sin(2*PI*800*t))*exp(-40*mod(t+0.1\\,0.25)):s=44100:d=2", []string{"-b:a", "128k", "-aac_tns", "1"}},
	{"stereo-44k-is", 2, "aevalsrc=0.3*(random(0)*2-1)+0.2*sin(2*PI*300*t)|-0.15*(random(0)*2-1)+0.2*sin(2*PI*450*t):s=44100:d=0.6", []string{"-b:a", "48k", "-aac_is", "1"}},
	{"mono-22k", 1, "aevalsrc=0.4*sin(2*PI*300*t)+0.3*lt(mod(t\\,0.3)\\,0.003):s=22050:d=0.6", []string{"-b:a", "48k"}},
	{"stereo-32k", 2, "aevalsrc=0.3*sin(2*PI*500*t)+0.2*lt(mod(t\\,0.3)\\,0.003)|0.3*sin(2*PI*700*t):s=32000:d=0.6", []string{"-b:a", "96k"}},
	{"stereo-44k-loud-quiet", 2, "aevalsrc=1.4*sin(2*PI*200*t)+0.3*sin(2*PI*2000*t)|0.001*sin(2*PI*880*t):s=44100:d=0.4", []string{"-b:a", "192k"}},
	{"stereo-44k-pce", 2, "aevalsrc=0.4*sin(2*PI*440*t)|0.3*sin(2*PI*660*t):s=44100:d=0.3", []string{"-b:a", "128k", "-aac_pce", "1"}},
}

// sketches is the number of sketches of a decode.
const sketches = 32

// fixture is what fixtures.json holds of a fixture: what ffmpeg's decoder
// decodes it to, as its energy and its sketches, and what its frames use.
type fixture struct {
	Frames int `json:"frames"`
	// Energy is the sum of the squares of ffmpeg's samples.
	Energy float64 `json:"energy"`
	// Sketches are the sums of ffmpeg's samples, each multiplied by a
	// pseudo-random sign, of a sequence of signs per sketch.
	Sketches [sketches]float64 `json:"sketches"`
	// SNR is the decoder's against ffmpeg's decode, when it was made.
	SNR   float64 `json:"snr"`
	Tools toolUse `json:"tools"`
}

// sketch computes the sketches of samples, interleaved, of channels of
// pcm, interleaved stereo when stereo is set.
func sketch(pcm []float32, channels int, stereo bool) [sketches]float64 {
	var s [sketches]float64
	stride := channels
	if stereo {
		stride = 2
	}
	for k := range s {
		seed := uint32(k+1) * 0x9E3779B9
		var sum float64
		for i := 0; i+channels <= len(pcm); i += stride {
			for c := range channels {
				seed = seed*1664525 + 1013904223
				if seed&0x80000000 != 0 {
					sum -= float64(pcm[i+c])
				} else {
					sum += float64(pcm[i+c])
				}
			}
		}
		s[k] = sum
	}
	return s
}

// TestFixtures decodes the fixtures and measures the SNR of their decodes
// against ffmpeg's, without ffmpeg: as the sketches are linear, the
// difference of a decode's sketches and ffmpeg's are sketches of the
// difference of the decodes, and the mean of their squares estimates its
// energy, within a dB or two.
func TestFixtures(t *testing.T) {
	path := filepath.Join("testdata", "fixtures.json")
	want := map[string]*fixture{}
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
	for _, e := range fixtures {
		t.Run(e.name, func(t *testing.T) {
			file := filepath.Join("testdata", e.name+".aac")
			if *update {
				args := append([]string{"-y", "-f", "lavfi", "-i", e.src, "-c:a", "aac"}, e.args...)
				ffmpeg(t, append(args, "-f", "adts", file)...)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			ours, use, shared := decodeFile(t, data, e.channels)
			if *update {
				ref := ffmpegDecode(t, file)
				var energy float64
				for _, v := range ref {
					energy += float64(v) * float64(v)
				}
				snr, _, _ := compare(ref, e.channels, ours, shared)
				want[e.name] = &fixture{len(ours) / 2 / FrameLength, energy, sketch(ref, e.channels, false), math.Round(snr*10) / 10, use}
			}
			w := want[e.name]
			if w == nil {
				t.Fatalf("%s isn't in %s", e.name, path)
			}
			if frames := len(ours) / 2 / FrameLength; frames != w.Frames {
				t.Errorf("%d frames, want %d", frames, w.Frames)
			}
			for _, k := range tools {
				if use[k] != w.Tools[k] {
					t.Errorf("%d frames use %s, want %d", use[k], k, w.Tools[k])
				}
			}
			got := sketch(ours, e.channels, true)
			var noise float64
			for k := range got {
				d := got[k] - w.Sketches[k]
				noise += d * d / sketches
			}
			snr := 10 * math.Log10(w.Energy/noise)
			t.Logf("SNR about %.1f dB (%.1f dB when made); %v", snr, w.SNR, use)
			if snr < 110 {
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

// fixtureFrames returns the frames of the fixture name, and its config.
func fixtureFrames(t testing.TB, name string) (Config, [][]byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".aac"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, frames, err := readADTS(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Channels == 0 {
		cfg.Channels = 2 // stereo-44k-pce
	}
	if !strings.HasPrefix(name, "mono") && cfg.Channels != 2 {
		t.Fatalf("%s: %d channels", name, cfg.Channels)
	}
	return cfg, frames
}
