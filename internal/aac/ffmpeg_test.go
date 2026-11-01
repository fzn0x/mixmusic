package aac

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hasFFmpeg reports whether ffmpeg is on the PATH.
func hasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// haveFFmpeg skips t without ffmpeg on the PATH.
func haveFFmpeg(t testing.TB) {
	t.Helper()
	if !hasFFmpeg() {
		t.Skip("ffmpeg isn't on the PATH")
	}
}

// ffmpeg runs ffmpeg with args, and returns what it writes.
func ffmpeg(t testing.TB, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-hide_banner", "-loglevel", "error", "-nostdin"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, stderr.Bytes())
	}
	return out
}

// ffmpegDecode decodes the ADTS file at path with ffmpeg, into float32
// samples of its channels, interleaved.
func ffmpegDecode(t testing.TB, path string) []float32 {
	t.Helper()
	b := ffmpeg(t, "-i", path, "-f", "f32le", "-c:a", "pcm_f32le", "pipe:1")
	pcm := make([]float32, len(b)/4)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return pcm
}

// toolUse counts the frames that use each tool of the decoder.
type toolUse map[string]int

// tools are the names toolUse counts.
var tools = []string{
	"short", "start", "stop", "sine", "kbd", "ms", "ms-all", "is", "is-out", "is-ms",
	"pns", "pns-ms", "tns", "tns-long", "tns-short", "pulse", "escape",
}

func (u toolUse) String() string {
	var parts []string
	for _, k := range tools {
		if u[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, u[k]))
		}
	}
	return strings.Join(parts, ", ")
}

// note counts the tools the frame d decoded last used, and reports whether
// its channels shared their noise, as ffmpeg's decoder doesn't.
func (u toolUse) note(d *Decoder) bool {
	seen := map[string]bool{}
	for c := range d.cfg.Channels {
		s := &d.ch[c].ics
		switch s.info.seq {
		case eightShortSequence:
			seen["short"] = true
		case longStartSequence:
			seen["start"] = true
		case longStopSequence:
			seen["stop"] = true
		}
		if s.info.shape == 1 {
			seen["kbd"] = true
		} else {
			seen["sine"] = true
		}
		for i, cb := range s.bandType[:s.info.groups*s.info.maxSFB] {
			ms := d.cfg.Channels == 2 && d.msMask != 0 && d.msUsed[i]
			switch cb {
			case noiseBook:
				seen["pns"] = true
				if ms && d.ch[0].ics.bandType[i] == noiseBook && d.ch[1].ics.bandType[i] == noiseBook {
					seen["pns-ms"] = true
				}
			case intensityBook, intensityBook2:
				seen["is"] = true
				if cb == intensityBook2 {
					seen["is-out"] = true
				}
				if ms {
					seen["is-ms"] = true
				}
			case escapeBook:
				seen["escape"] = true
			}
		}
		if s.tns.present {
			seen["tns"] = true
			if s.info.seq == eightShortSequence {
				seen["tns-short"] = true
			} else {
				seen["tns-long"] = true
			}
		}
		if s.pulse.present {
			seen["pulse"] = true
		}
	}
	if d.cfg.Channels == 2 && d.commonWindow && d.msMask != 0 {
		seen["ms"] = true
		if d.msMask == 2 {
			seen["ms-all"] = true
		}
	}
	for k := range seen {
		u[k]++
	}
	return seen["pns-ms"]
}

// decodeFile decodes the frames of an ADTS file with a new Decoder into
// interleaved stereo, counts the tools they use, and tells those whose
// channels shared their noise.
func decodeFile(t testing.TB, data []byte, channels int) ([]float32, toolUse, []bool) {
	t.Helper()
	cfg, frames, err := readADTS(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Channels == 0 {
		cfg.Channels = channels
	}
	d, err := NewDecoder(cfg)
	if err != nil {
		t.Fatal(err)
	}
	use := toolUse{}
	pcm := make([]float32, 2*FrameLength*len(frames))
	shared := make([]bool, len(frames))
	for i, f := range frames {
		if _, err := d.Decode(f, pcm[2*FrameLength*i:]); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		shared[i] = use.note(d)
	}
	return pcm, use, shared
}

// compare returns the SNR, in dB, of ours, interleaved stereo, against
// ref, of channels interleaved, and the frame whose own SNR is the worst.
// It leaves out the frames that skip tells, those whose channels share
// their noise where ffmpeg's decoder gives each noise of its own, and the
// frame after each, which the second half of their windows reaches.
func compare(ref []float32, channels int, ours []float32, skip []bool) (snr float64, worst int, worstSNR float64) {
	n := min(len(ref)/channels, len(ours)/2)
	var sig, noise float64
	worstSNR = math.Inf(1)
	for f := 0; f*FrameLength < n; f++ {
		if f < len(skip) && skip[f] || f > 0 && f <= len(skip) && skip[f-1] {
			continue
		}
		var fs, fn float64
		for i := f * FrameLength; i < min(n, (f+1)*FrameLength); i++ {
			for c := range channels {
				r, o := float64(ref[i*channels+c]), float64(ours[2*i+c])
				fs += r * r
				fn += (r - o) * (r - o)
			}
		}
		sig += fs
		noise += fn
		if s := 10 * math.Log10(fs/fn); fs > 1e-12 && s < worstSNR {
			worst, worstSNR = f, s
		}
	}
	return 10 * math.Log10(sig/noise), worst, worstSNR
}

// TestFiles decodes the ADTS files AAC_FILES lists, as ffmpeg -c copy -f
// adts extracts them from an MP4 file, and checks that the decoder
// decodes them as ffmpeg's decoder does.
func TestFiles(t *testing.T) {
	list := os.Getenv("AAC_FILES")
	if list == "" {
		t.Skip("AAC_FILES lists ADTS files to compare with ffmpeg's decode")
	}
	haveFFmpeg(t)
	for _, path := range filepath.SplitList(list) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg, _, err := readADTS(data)
			if err != nil {
				t.Fatal(err)
			}
			ref := ffmpegDecode(t, path)
			ours, use, shared := decodeFile(t, data, cfg.Channels)
			if len(ref)/cfg.Channels != len(ours)/2 {
				t.Errorf("ffmpeg decodes %d samples, the decoder %d", len(ref)/cfg.Channels, len(ours)/2)
			}
			snr, worst, worstSNR := compare(ref, cfg.Channels, ours, shared)
			t.Logf("%+v: SNR %.1f dB, worst frame %d at %.1f dB; %d frames; %v", cfg, snr, worst, worstSNR, len(ours)/2/FrameLength, use)
			if snr < 90 {
				t.Errorf("SNR %.1f dB", snr)
			}
		})
	}
}

// encoding is a signal ffmpeg's aac encoder encodes, and how.
type encoding struct {
	name     string
	channels int
	src      string   // a lavfi source
	args     []string // the encoder's options
}

var encodings = []encoding{
	{"stereo-44k-tones", 2, "aevalsrc=0.4*sin(2*PI*440*t)+0.2*sin(2*PI*1250*t)|0.3*sin(2*PI*660*t)+0.2*sin(2*PI*3100*t):s=44100:d=2", []string{"-b:a", "128k"}},
	{"mono-48k-tones", 1, "aevalsrc=0.5*sin(2*PI*440*t)+0.1*sin(2*PI*5000*t):s=48000:d=2", []string{"-b:a", "96k"}},
	{"stereo-44k-clicks", 2, "aevalsrc=0.2*sin(2*PI*330*t)+0.8*lt(mod(t\\,0.2)\\,0.002)*sin(2*PI*3000*t)|0.2*sin(2*PI*495*t)+0.7*lt(mod(t+0.1\\,0.2)\\,0.002):s=44100:d=2", []string{"-b:a", "128k"}},
	{"mono-44k-bursts", 1, "aevalsrc=0.6*(random(0)*2-1)*gt(mod(t\\,0.5)\\,0.4)+0.1*sin(2*PI*220*t):s=44100:d=2", []string{"-b:a", "128k"}},
	{"stereo-48k-ms", 2, "aevalsrc=0.4*sin(2*PI*440*t)+0.05*sin(2*PI*7000*t)|0.38*sin(2*PI*440*t)+0.05*sin(2*PI*7000*t+0.3):s=48000:d=2", []string{"-b:a", "128k", "-aac_ms", "1"}},
	{"stereo-44k-is", 2, "aevalsrc=0.3*sin(2*PI*300*t)+0.2*(random(0)*2-1)|0.3*sin(2*PI*450*t)+0.1*(random(0)*2-1):s=44100:d=2", []string{"-b:a", "64k", "-aac_is", "1"}},
	{"stereo-44k-pns", 2, "anoisesrc=color=pink:amplitude=0.3:sample_rate=44100:duration=2:seed=7", []string{"-ac", "2", "-b:a", "64k", "-aac_pns", "1"}},
	{"mono-44k-pns", 1, "anoisesrc=color=white:amplitude=0.2:sample_rate=44100:duration=2:seed=3", []string{"-b:a", "48k", "-aac_pns", "1"}},
	{"stereo-44k-tns", 2, "aevalsrc=(0.5*sin(2*PI*1000*t)+0.3*sin(2*PI*2500*t))*exp(-30*mod(t\\,0.25))|(0.4*sin(2*PI*800*t))*exp(-40*mod(t+0.1\\,0.25)):s=44100:d=2", []string{"-b:a", "128k", "-aac_tns", "1"}},
	{"stereo-32k", 2, "aevalsrc=0.3*sin(2*PI*500*t)+0.2*lt(mod(t\\,0.3)\\,0.003)|0.3*sin(2*PI*700*t):s=32000:d=2", []string{"-b:a", "96k"}},
	{"mono-22k", 1, "aevalsrc=0.4*sin(2*PI*300*t)+0.3*lt(mod(t\\,0.3)\\,0.003):s=22050:d=2", []string{"-b:a", "48k"}},
	{"stereo-44k-quiet", 2, "aevalsrc=0.001*sin(2*PI*440*t)|0.0005*sin(2*PI*880*t):s=44100:d=2", []string{"-b:a", "128k"}},
	{"stereo-44k-loud", 2, "aevalsrc=1.4*sin(2*PI*200*t)+0.3*sin(2*PI*2000*t)|0.99*sgn(sin(2*PI*150*t)):s=44100:d=2", []string{"-b:a", "192k"}},
	{"stereo-44k-pce", 2, "aevalsrc=0.4*sin(2*PI*440*t)|0.3*sin(2*PI*660*t):s=44100:d=1", []string{"-b:a", "128k", "-aac_pce", "1"}},
	{"stereo-44k-is-out", 2, "aevalsrc=0.3*sin(2*PI*300*t)+0.2*(random(0)*2-1)|0.3*sin(2*PI*450*t)-0.1*(random(0)*2-1):s=44100:d=2", []string{"-b:a", "64k", "-aac_is", "1"}},
	{"stereo-96k", 2, "aevalsrc=0.3*sin(2*PI*500*t)+0.3*lt(mod(t\\,0.3)\\,0.002)|0.3*sin(2*PI*20000*t):s=96000:d=1", []string{"-b:a", "192k"}},
	{"mono-88k", 1, "aevalsrc=0.3*sin(2*PI*500*t)+0.2*(random(0)*2-1):s=88200:d=1", []string{"-b:a", "128k"}},
	{"stereo-64k", 2, "aevalsrc=0.3*sin(2*PI*500*t)+0.3*lt(mod(t\\,0.3)\\,0.002)|0.3*sin(2*PI*9000*t):s=64000:d=1", []string{"-b:a", "160k"}},
	{"stereo-24k", 2, "aevalsrc=0.3*sin(2*PI*500*t)+0.3*lt(mod(t\\,0.3)\\,0.002)|0.3*sin(2*PI*4000*t):s=24000:d=1", []string{"-b:a", "64k"}},
	{"mono-16k", 1, "aevalsrc=0.3*sin(2*PI*500*t)+0.3*lt(mod(t\\,0.3)\\,0.002):s=16000:d=1", []string{"-b:a", "32k"}},
	{"mono-12k", 1, "aevalsrc=0.3*sin(2*PI*500*t)+0.1*(random(0)*2-1):s=12000:d=1", []string{"-b:a", "24k"}},
	{"mono-11k", 1, "aevalsrc=0.3*sin(2*PI*700*t)+0.3*lt(mod(t\\,0.3)\\,0.002):s=11025:d=1", []string{"-b:a", "24k"}},
	{"stereo-8k", 2, "aevalsrc=0.3*sin(2*PI*500*t)+0.3*lt(mod(t\\,0.3)\\,0.002)|0.3*sin(2*PI*1500*t):s=8000:d=1", []string{"-b:a", "24k"}},
	{"mono-7k", 1, "aevalsrc=0.3*sin(2*PI*500*t)+0.3*lt(mod(t\\,0.3)\\,0.002):s=7350:d=1", []string{"-b:a", "16k"}},
}

// TestFFmpeg encodes synthesized signals with ffmpeg's aac encoder, which
// uses all the tools of AAC-LC but pulse data, and checks that the decoder
// decodes them as ffmpeg's decoder does.
func TestFFmpeg(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	for _, e := range encodings {
		t.Run(e.name, func(t *testing.T) {
			path := filepath.Join(dir, e.name+".aac")
			args := append([]string{"-f", "lavfi", "-i", e.src, "-c:a", "aac"}, e.args...)
			ffmpeg(t, append(args, "-f", "adts", path)...)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ref := ffmpegDecode(t, path)
			ours, use, shared := decodeFile(t, data, e.channels)
			if len(ref)/e.channels != len(ours)/2 {
				t.Errorf("ffmpeg decodes %d samples, the decoder %d", len(ref)/e.channels, len(ours)/2)
			}
			snr, worst, worstSNR := compare(ref, e.channels, ours, shared)
			t.Logf("SNR %.1f dB, worst frame %d at %.1f dB; %d bytes; %v", snr, worst, worstSNR, len(data), use)
			if snr < 90 {
				t.Errorf("SNR %.1f dB", snr)
			}
		})
	}
}
