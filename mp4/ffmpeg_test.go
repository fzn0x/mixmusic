package mp4

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// ffmpegFiles are the files of testdata: 3 seconds of sine waves in AAC-LC,
// as ffmpeg 7.0 makes them, with what ffprobe reports of them in the .json
// of the same name:
//
//   - progressive, with its moov after the mdat
//   - faststart, mono at 48 kHz, with its moov first
//   - fragmented, in 6 fragments, without a sidx or an edit list
//   - sidx, fragmented as YouTube's files are: an edit list for the
//     priming, and a sidx after the moov that indexes every fragment
//   - dash, fragmented by ffmpeg's DASH muxer, with a sidx before each
//     fragment, of it alone
//   - video, mono after an MPEG-4 video track, in 16 chunks
//
// They were made with
//
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=44100:d=3" -c:a aac -b:a 64k -flags:a +bitexact progressive.m4a
//	ffmpeg -fflags +bitexact -f lavfi -i sine=frequency=440:duration=3:sample_rate=48000 -c:a aac -b:a 48k -flags:a +bitexact -movflags +faststart faststart.m4a
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=44100:d=3" -c:a aac -b:a 64k -flags:a +bitexact -movflags frag_keyframe+empty_moov+default_base_moof -frag_duration 500000 fragmented.m4a
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=44100:d=3" -c:a aac -b:a 64k -flags:a +bitexact -movflags frag_keyframe+default_base_moof+delay_moov+global_sidx -frag_duration 500000 sidx.m4a
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=44100:d=3" -c:a aac -b:a 64k -flags:a +bitexact -f dash -single_file 1 -single_file_name dash.mp4 -seg_duration 0.5 dash.mpd
//	ffmpeg -fflags +bitexact -f lavfi -i testsrc=size=32x32:rate=5:duration=3 -f lavfi -i sine=frequency=440:duration=3:sample_rate=44100 -c:v mpeg4 -b:v 20k -g 5 -c:a aac -b:a 32k -flags +bitexact video.mp4
//
// and the 5.1 surround of
//
//   - surround-ac3, a second of AC-3, as YouTube's format 380, with its
//     moov after the mdat
//   - surround-eac3, of E-AC-3, as format 328, fragmented with a sidx as
//     YouTube's files are
//   - surround-ac3-44k, half a second of AC-3 at 44.1 kHz, with its moov
//     first
//
// whose channels are sine waves of 440, 550, 660, 60, 770 and 880 Hz, in
// the order of 5.1(side): left, right, center, LFE, left and right
// surround. They were made with
//
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.25*sin(2*PI*440*t)|0.25*sin(2*PI*550*t)|0.25*sin(2*PI*660*t)|0.25*sin(2*PI*60*t)|0.25*sin(2*PI*770*t)|0.25*sin(2*PI*880*t):s=48000:d=1:c=5.1(side)" -c:a ac3 -b:a 192k -flags:a +bitexact surround-ac3.mp4
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.25*sin(2*PI*440*t)|0.25*sin(2*PI*550*t)|0.25*sin(2*PI*660*t)|0.25*sin(2*PI*60*t)|0.25*sin(2*PI*770*t)|0.25*sin(2*PI*880*t):s=48000:d=1:c=5.1(side)" -c:a eac3 -b:a 192k -flags:a +bitexact -movflags frag_keyframe+default_base_moof+delay_moov+global_sidx -frag_duration 250000 surround-eac3.mp4
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.25*sin(2*PI*440*t)|0.25*sin(2*PI*550*t)|0.25*sin(2*PI*660*t)|0.25*sin(2*PI*60*t)|0.25*sin(2*PI*770*t)|0.25*sin(2*PI*880*t):s=44100:d=0.5:c=5.1(side)" -c:a ac3 -b:a 192k -flags:a +bitexact -movflags +faststart surround-ac3-44k.mp4
//
// The .json files are what
//
//	ffprobe -v error -select_streams a:0 -show_entries packet=pts,duration,size:stream=codec_name,profile,sample_rate,channels,time_base,extradata:format=duration -show_data -of json=compact=1 NAME > NAME.json
//
// reports of them.
var ffmpegFiles = []string{"progressive.m4a", "faststart.m4a", "fragmented.m4a", "sidx.m4a", "dash.mp4", "video.mp4"}

func TestFFmpegFiles(t *testing.T) {
	ms := time.Millisecond
	for _, c := range []struct {
		name       string
		id         uint32
		channels   int
		rate       int
		delay      time.Duration // the priming, which the edit list skips
		duration   time.Duration // 0 when the file gives none
		table      uint32        // the samples of the moov
		fragmented bool
		indexed    int // the fragments of the sidx
		fragments  int // the fragments known at the end
	}{
		{"progressive.m4a", 1, 2, 44100, 1024 * time.Second / 44100, 133324 * time.Second / 44100, 131, false, 0, 0},
		{"faststart.m4a", 1, 1, 48000, 1024 * time.Second / 48000, 145024 * time.Second / 48000, 142, false, 0, 0},
		{"fragmented.m4a", 1, 2, 44100, 0, 0, 0, true, 0, 6},
		{"sidx.m4a", 1, 2, 44100, 1024 * time.Second / 44100, 133324 * time.Second / 44100, 0, true, 6, 6},
		{"dash.mp4", 1, 2, 44100, 1024 * time.Second / 44100, 0, 0, true, 1, 6},
		{"video.mp4", 2, 1, 44100, 1024 * time.Second / 44100, 133324 * time.Second / 44100, 131, false, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := readProbe(t, c.name)
			f, err := os.Open(filepath.Join("testdata", c.name))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r, err := NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			tr := r.Track()
			if tr.ID != c.id || tr.Channels != c.channels || tr.SampleRate != c.rate || tr.Timescale != uint32(c.rate) || tr.CodecDelay != c.delay {
				t.Errorf("track %+v", tr)
			}
			if r.Duration() != c.duration || r.table.count != c.table || r.fragmented != c.fragmented || r.indexed != c.indexed {
				t.Errorf("duration %v, %d samples in the moov, fragmented %v, %d fragments indexed", r.Duration(), r.table.count, r.fragmented, r.indexed)
			}
			checkProbe(t, r, p)

			want := p.want(r)
			var times []time.Duration
			for at := -100 * ms; at <= 3300*ms; at += 50 * ms {
				times = append(times, at)
			}
			checkSeeks(t, r, want, times, 0)
			// To each packet, and about it, in an order that goes back and on.
			times = times[:0]
			for i := range want {
				w := want[(i*37)%len(want)]
				times = append(times, w.time, w.time-1, w.time+1)
			}
			checkSeeks(t, r, want, times, 2)
			if len(r.frags) != c.fragments {
				t.Errorf("%d fragments known", len(r.frags))
			}
		})
	}
}

// readProbe reads what ffprobe reports of a file of testdata.
func readProbe(t *testing.T, name string) probe {
	t.Helper()
	ext := filepath.Ext(name)
	b, err := os.ReadFile(filepath.Join("testdata", name[:len(name)-len(ext)]+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var p probe
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestFFmpegLayouts checks the Reader against ffprobe on more layouts than
// testdata holds, which ffmpeg makes when it is on the PATH: longer files,
// trafs after those of a video track, base offsets in the tfhd rather than
// from the moof, and QuickTime's sound descriptions.
func TestFFmpegLayouts(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on the PATH")
	}
	stereo := []string{"-f", "lavfi", "-i", "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=44100:d=30"}
	video := []string{"-f", "lavfi", "-i", "testsrc=size=32x32:rate=10:duration=10", "-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=32000",
		"-c:v", "mpeg4", "-b:v", "20k", "-g", "10"}
	aac := []string{"-c:a", "aac", "-b:a", "64k"}
	for _, c := range []struct {
		name string
		args []string
		dash bool // whether ffmpeg writes an MPD, naming the file in its args
	}{
		{"long-sidx.m4a", slices.Concat(stereo, aac, []string{"-movflags", "frag_keyframe+default_base_moof+delay_moov+global_sidx", "-frag_duration", "2000000"}), false},
		{"long-fragments.m4a", slices.Concat(stereo, aac, []string{"-movflags", "frag_keyframe+empty_moov+default_base_moof", "-frag_duration", "1000000"}), false},
		{"long-dash.mp4", slices.Concat(stereo, aac, []string{"-f", "dash", "-single_file", "1", "-single_file_name", "long-dash.mp4", "-seg_duration", "2"}), true},
		{"base-offsets.m4a", slices.Concat(stereo, aac, []string{"-movflags", "frag_keyframe+empty_moov", "-frag_duration", "1000000"}), false},
		{"video-fragments.mp4", slices.Concat(video, aac, []string{"-movflags", "frag_keyframe+empty_moov+default_base_moof"}), false},
		{"video-base-offsets.mp4", slices.Concat(video, aac, []string{"-movflags", "frag_keyframe+empty_moov"}), false},
		{"video-faststart.mp4", slices.Concat(video, aac, []string{"-movflags", "+faststart"}), false},
		{"quicktime.mov", slices.Concat(stereo, aac), false},
		{"mono-22050.m4a", []string{"-f", "lavfi", "-i", "sine=frequency=300:duration=7:sample_rate=22050", "-c:a", "aac", "-b:a", "32k"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			// ffmpeg runs in the temporary folder, as its DASH muxer finds the
			// folder of the MPD by its slashes, which Windows paths lack.
			dir := t.TempDir()
			args := slices.Concat([]string{"-v", "error", "-y"}, c.args, []string{c.name})
			if c.dash {
				args[len(args)-1] = "out.mpd"
			}
			cmd := exec.Command("ffmpeg", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg: %v\n%s", err, out)
			}
			path := filepath.Join(dir, c.name)
			p := runProbe(t, path)
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r, err := NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			checkProbe(t, r, p)
			want := p.want(r)
			var times []time.Duration
			for i := range want {
				w := want[(i*61)%len(want)]
				times = append(times, w.time, w.time-1, w.time+time.Millisecond)
			}
			times = append(times, -time.Second, time.Hour, 0)
			checkSeeks(t, r, want, times, 2)
		})
	}
}

// TestSurround reads the AC-3 and E-AC-3 files of testdata: their dac3 or
// dec3, whose acmod and lfeon give the channels, and their packets, each
// a syncframe of 1536 samples, as ffprobe reports them.
func TestSurround(t *testing.T) {
	for _, c := range []struct {
		name   string
		codec  string
		config []byte // the payload of the dac3 or dec3
		rate   int
		frag   bool
	}{
		// fscod 0, bsid 8, bsmod 0, acmod 7, lfeon 1, bit_rate_code 10.
		{"surround-ac3.mp4", "ac-3", []byte{0x10, 0x3D, 0x40}, 48000, false},
		// data_rate 192, num_ind_sub 0; fscod 0, bsid 16, asvc 0, bsmod 0,
		// acmod 7, lfeon 1, num_dep_sub 0.
		{"surround-eac3.mp4", "ec-3", []byte{0x06, 0x00, 0x20, 0x0F, 0x00, 0x00}, 48000, true},
		{"surround-ac3-44k.mp4", "ac-3", []byte{0x50, 0x3D, 0x40}, 44100, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := readProbe(t, c.name)
			f, err := os.Open(filepath.Join("testdata", c.name))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r, err := NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			tr := r.Track()
			// ffmpeg's encoders prime with a block of 256 samples.
			delay := 256 * time.Second / time.Duration(c.rate)
			if tr.Codec != c.codec || !slices.Equal(tr.Config, c.config) || tr.Channels != 6 || tr.SampleRate != c.rate || tr.CodecDelay != delay || r.fragmented != c.frag {
				t.Errorf("track %+v, fragmented %v", tr, r.fragmented)
			}
			checkProbe(t, r, p)
			want := p.want(r)
			for _, w := range want[:len(want)-1] {
				if w.duration != 1536*time.Second/time.Duration(c.rate) {
					t.Fatalf("ffprobe reports a packet of %v", w.duration)
				}
			}
			var times []time.Duration
			for i := range want {
				w := want[(i*7)%len(want)]
				times = append(times, w.time, w.time-1, w.time+time.Millisecond)
			}
			checkSeeks(t, r, want, times, 2)
		})
	}
}
