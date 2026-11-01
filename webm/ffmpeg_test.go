package webm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ffmpegFiles are the files of testdata: 3 seconds of sine waves in Opus
// at 64 kb/s, as ffmpeg 7.0 and libopus make them, with what ffprobe
// reports of them in the .json of the same name:
//
//   - mono-stream, written to a pipe: a Segment of unknown size, without
//     Duration or Cues
//   - stereo-cues-end, with Cues after the clusters, found through the
//     SeekHead
//   - stereo-cues-front, with Cues before the clusters, as YouTube's files
//   - video, mono after a VP8 video track, which alone has cue points
//
// They were made with
//
//	ffmpeg -fflags +bitexact -f lavfi -i sine=frequency=440:duration=3:sample_rate=48000 -c:a libopus -b:a 64k -flags:a +bitexact -cluster_time_limit 500 -f webm pipe:1 > mono-stream.webm
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=48000:d=3" -c:a libopus -b:a 64k -flags:a +bitexact -cluster_time_limit 500 stereo-cues-end.webm
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=sin(440*2*PI*t)|sin(660*2*PI*t):s=48000:d=3" -c:a libopus -b:a 64k -flags:a +bitexact -cluster_time_limit 500 -cues_to_front 1 stereo-cues-front.webm
//	ffmpeg -fflags +bitexact -f lavfi -i testsrc=size=32x32:rate=5:duration=3 -f lavfi -i sine=frequency=440:duration=3:sample_rate=48000 -c:v libvpx -b:v 20k -g 5 -c:a libopus -b:a 64k -flags +bitexact video.webm
//	ffprobe -v error -select_streams a:0 -show_entries packet=pts,size,flags:stream=codec_name,sample_rate,channels,initial_padding:format=duration -of json=compact=1 NAME.webm > NAME.json
var ffmpegFiles = []string{"mono-stream", "stereo-cues-end", "stereo-cues-front", "video"}

func TestFFmpegFiles(t *testing.T) {
	for _, c := range []struct {
		name     string
		channels int
		number   uint64
		cues     int  // the cue points Seek uses
		inline   bool // whether they come before the clusters
		streamed bool // whether the Segment's size and the Duration are unknown
	}{
		{"mono-stream", 1, 1, 0, false, true},
		{"stereo-cues-end", 2, 1, 7, false, false},
		{"stereo-cues-front", 2, 1, 7, true, false},
		{"video", 1, 2, 3, false, false}, // the cue points of the video track
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", c.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var p probe
			if err := json.Unmarshal(b, &p); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(filepath.Join("testdata", c.name+".webm"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r, err := NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			tr := r.Track()
			if tr.Number != c.number || tr.Channels != c.channels || tr.SeekPreRoll != 80*time.Millisecond {
				t.Errorf("track %+v", tr)
			}
			if (r.segEnd < 0) != c.streamed || (r.Duration() == 0) != c.streamed || r.cuesRead != c.inline {
				t.Errorf("Segment's end %d, duration %v, Cues read %v", r.segEnd, r.Duration(), r.cuesRead)
			}
			checkProbe(t, r, p)

			// ffmpeg pads the last packet to 20 ms, and says by how much.
			r.Seek(0)
			packets := readAll(t, r)
			for i, pkt := range packets {
				pad := time.Duration(0)
				if i == len(packets)-1 {
					pad = time.Duration(len(packets)*960-312-3*48000) * time.Second / 48000
				}
				if pkt.DiscardPadding != pad || pkt.Duration != 0 {
					t.Errorf("packet %d: DiscardPadding %v, Duration %v; want %v and 0", i, pkt.DiscardPadding, pkt.Duration, pad)
				}
			}

			var times []time.Duration
			for ms := -100; ms <= 3300; ms += 200 {
				times = append(times, time.Duration(ms)*time.Millisecond)
			}
			checkSeeks(t, r, p.want(r), times)
			if len(r.cues) != c.cues || c.cues == 0 && len(r.clusters) != 7 {
				t.Errorf("%d cue points, %d clusters scanned", len(r.cues), len(r.clusters))
			}
		})
	}
}
