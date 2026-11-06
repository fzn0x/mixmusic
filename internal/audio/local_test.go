package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The files kept on a computer that the tests play without ffmpeg: FLAC
// from flac's testdata, and Opus and Vorbis in Ogg from ogg's, where the
// commands that made them are, and half a second of a 440 Hz sine wave in
// MP3, mono at 22.05 kHz, made with
//
//	ffmpeg -fflags +bitexact -f lavfi -i sine=frequency=440:duration=0.5:sample_rate=22050 -c:a libmp3lame -b:a 32k -flags:a +bitexact testdata/tone.mp3
var (
	flacStereo   = "../../flac/testdata/stereo-16.flac"
	flacMono24   = "../../flac/testdata/mono-24.flac"
	flacSurround = "../../flac/testdata/surround.flac"
	oggOpus      = "../../ogg/testdata/tone.opus"
	oggVorbis    = "../../ogg/testdata/tone.ogg"
	mp3Tone      = "testdata/tone.mp3"
	localFiles   = []string{flacStereo, flacMono24, flacSurround, oggOpus, oggVorbis, mp3Tone}
)

// makeLocal makes a file of each kind ffmpeg writes that a deck plays,
// of a second of a sine wave in stereo at 44.1 kHz, as music kept on a
// computer is, and returns their paths, by name.
func makeLocal(t *testing.T) map[string]string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on the PATH")
	}
	dir := t.TempDir()
	src := "aevalsrc=0.4*sin(2*PI*440*t)|0.3*sin(2*PI*660*t):s=44100:d=1"
	files := map[string][]string{
		"mp3-cbr.mp3":   {"-c:a", "libmp3lame", "-b:a", "192k"},
		"mp3-vbr.mp3":   {"-c:a", "libmp3lame", "-q:a", "2", "-id3v2_version", "3", "-metadata", "title=A made-up title"},
		"mp3-mono.mp3":  {"-ac", "1", "-ar", "22050", "-c:a", "libmp3lame", "-b:a", "64k"},
		"wav-16.wav":    {"-c:a", "pcm_s16le"},
		"wav-24.wav":    {"-c:a", "pcm_s24le", "-ar", "96000"},
		"wav-float.wav": {"-c:a", "pcm_f32le", "-ac", "1"},
		"wav-8.wav":     {"-c:a", "pcm_u8", "-ar", "8000"},
		"flac.flac":     {"-c:a", "flac", "-sample_fmt", "s16"},
		"m4a.m4a":       {"-c:a", "aac", "-b:a", "128k"},
		"opus.opus":     {"-c:a", "libopus", "-b:a", "96k", "-ar", "48000"},
		"vorbis.ogg":    {"-c:a", "libvorbis", "-q:a", "5"},
	}
	out := map[string]string{}
	for name, args := range files {
		path := filepath.Join(dir, name)
		cmd := exec.Command("ffmpeg", append(append([]string{"-v", "error", "-nostdin", "-f", "lavfi", "-i", src}, args...), path)...)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %s: %v\n%s", name, err, b)
		}
		out[name] = path
	}
	return out
}

// ffmpegDecodeWith decodes a file with ffmpeg as ffmpegDecode does, with
// the decoder it names, or ffmpeg's own when "", a mono file's in both
// channels at its level.
func ffmpegDecodeWith(t *testing.T, path, decoder string, mono bool) []float32 {
	t.Helper()
	args := []string{"-v", "error"}
	if decoder != "" {
		args = append(args, "-c:a", decoder)
	}
	filter := "aresample=48000:filter_type=kaiser:kaiser_beta=12:filter_size=256:cutoff=0.95:phase_shift=14"
	if mono {
		filter = "pan=stereo|c0=c0|c1=c0," + filter
	}
	args = append(args, "-i", path, "-af", filter, "-f", "f32le", "-")
	out, err := exec.Command("ffmpeg", args...).Output()
	if err != nil {
		t.Fatalf("ffmpeg: %v", err)
	}
	pcm := make([]float32, len(out)/4)
	binary.Read(bytes.NewReader(out), binary.LittleEndian, pcm)
	return pcm
}

// TestDeckPlaysLocalFiles plays a file of each kind as ffmpeg decodes it:
// the two agree within what their decoders and converters differ by.
func TestDeckPlaysLocalFiles(t *testing.T) {
	files := makeLocal(t)
	for _, c := range []struct {
		name    string
		decoder string
		mono    bool
		least   float64 // in dB
	}{
		{"flac.flac", "", false, 100},
		{"wav-16.wav", "", false, 100},
		{"wav-24.wav", "", false, 100},
		{"wav-float.wav", "", true, 100},
		// From 8 kHz, the converters' filters differ more.
		{"wav-8.wav", "", false, 60},
		{"m4a.m4a", "", false, 90},
		{"opus.opus", "libopus", false, 80},
		{"vorbis.ogg", "", false, 80},
		{"mp3-cbr.mp3", "", false, 60},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := playAll(t, files[c.name])
			want := ffmpegDecodeWith(t, files[c.name], c.decoder, c.mono)
			// ffmpeg keeps an AAC file's padding, and cuts a Vorbis file's
			// last packet short of where its last page says.
			if d := len(got)/2 - len(want)/2; d < -1100 || d > 160 {
				t.Errorf("%d frames, ffmpeg's %d", len(got)/2, len(want)/2)
			}
			n := min(len(got), len(want))
			edge := 2 * 2048
			s := snr(got[edge:n-edge], want[edge:n-edge])
			t.Logf("%.1f dB from ffmpeg's decode", s)
			if s < c.least {
				t.Errorf("%.1f dB from ffmpeg's decode", s)
			}
		})
	}
}

// TestMP3AgainstFFmpeg decodes MP3 files with go-mp3, as their tracks do,
// at their own rate, as ffmpeg decodes them to 16 bits: the two agree,
// once go-mp3's start, which keeps the encoder's delay, and the frame of
// LAME's tag, as silence, lines up with ffmpeg's, which drops them, as
// the tracks' delay does, and the tracks end where ffmpeg's decode does.
func TestMP3AgainstFFmpeg(t *testing.T) {
	files := makeLocal(t)
	for _, c := range []struct {
		name string
		mono bool
	}{{"mp3-cbr.mp3", false}, {"mp3-vbr.mp3", false}, {"mp3-mono.mp3", true}} {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(files[c.name])
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			start, end := mp3Gap(f)
			tr, tm, err := openMP3(f)
			if err != nil {
				t.Fatal(err)
			}
			if start <= 0 || end <= start || tm.delay <= 0 {
				t.Fatalf("no gap told: from %d to %d, a delay of %v", start, end, tm.delay)
			}
			codec := tr.(*pcmTrack).codec
			var got []float32
			for {
				pcm, err := codec.read()
				if err != nil {
					break
				}
				got = append(got, pcm...)
			}
			args := []string{"-v", "error", "-i", files[c.name]}
			if c.mono {
				args = append(args, "-af", "pan=stereo|c0=c0|c1=c0")
			}
			out, err := exec.Command("ffmpeg", append(args, "-f", "s16le", "-")...).Output()
			if err != nil {
				t.Fatal(err)
			}
			want := make([]float32, len(out)/2)
			for i := range want {
				want[i] = float32(int16(binary.LittleEndian.Uint16(out[2*i:]))) / 32768
			}
			best, lag := math.Inf(-1), 0
			for l := 0; l < 4000 && 2*l < len(got); l++ {
				if s := snr(got[2*l:min(len(got), 2*l+20000)], want); s > best {
					best, lag = s, l
				}
			}
			if int64(lag) != start || len(got)/2 != int(end) {
				t.Errorf("%d frames, ffmpeg's from %d: want %d, from %d", len(got)/2, lag, end, start)
			}
			got = got[2*lag:]
			if len(got) != len(want) {
				t.Errorf("%d frames of audio, ffmpeg's %d", len(got)/2, len(want)/2)
			}
			n := min(len(got), len(want))
			s := snr(got[:n], want[:n])
			t.Logf("%.1f dB from ffmpeg's decode, %d frames later", s, lag)
			if s < 60 {
				t.Errorf("%.1f dB from ffmpeg's decode, %d frames later", s, lag)
			}
		})
	}
}

// TestDeckSeeksLocalFiles starts each file kind a deck plays from a
// computer at a moment in it, which plays as it does decoded from the
// start.
func TestDeckSeeksLocalFiles(t *testing.T) {
	paths := localFiles
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		files := makeLocal(t)
		paths = append(paths, files["mp3-cbr.mp3"], files["wav-24.wav"], files["vorbis.ogg"])
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			whole := playAll(t, path)
			for _, at := range []time.Duration{50 * time.Millisecond, 150 * time.Millisecond} {
				d := newDeck(path, at, fileOpener, func() {})
				got := drain(t, d)
				d.close()
				if err := d.failure(); err != nil {
					t.Fatal(err)
				}
				from := 2 * int(frames(at))
				if from >= len(whole) {
					continue
				}
				want := whole[from:]
				// The first frames settle as the decoder and converter do.
				settle := 2 * 4800
				if len(got) < settle+2 || len(want) < settle+2 {
					continue
				}
				n := min(len(got), len(want))
				if s := snr(got[settle:n], want[settle:n]); s < 60 {
					t.Errorf("from %v: %.1f dB from the whole decode, %d frames of %d", at, s, len(got)/2, len(want)/2)
				}
			}
		})
	}
}

// TestLocalFilesPlayWithoutFFmpeg plays the files of the decoders'
// testdata whole: as many frames as they hold, at Rate, of their tones.
func TestLocalFilesPlayWithoutFFmpeg(t *testing.T) {
	for _, c := range []struct {
		path string
		dur  time.Duration
	}{
		{flacStereo, 200 * time.Millisecond},
		{flacMono24, 100 * time.Millisecond},
		{flacSurround, 200 * time.Millisecond},
		{oggOpus, 3 * time.Second},
		{oggVorbis, 2 * time.Second},
		{mp3Tone, 500 * time.Millisecond},
	} {
		t.Run(filepath.Base(c.path), func(t *testing.T) {
			got := playAll(t, c.path)
			want := int(frames(c.dur))
			if n := len(got) / 2; n < want-48 || n > want+48 {
				t.Errorf("%d frames, want %d", n, want)
			}
			var peak float64
			for _, v := range got {
				peak = max(peak, math.Abs(float64(v)))
			}
			if peak < 0.1 || peak > 1 {
				t.Errorf("a peak of %.3f", peak)
			}
		})
	}
}

// TestSniffLocalFiles tells the kinds of the files kept on a computer
// apart, MP3 and FLAC after an ID3 tag too, from HLS packed audio.
func TestSniffLocalFiles(t *testing.T) {
	// A tag of 8 KB, and what follows it.
	big := func(after string) string {
		return "ID3\x03\x00\x00\x00\x00\x40\x00" + strings.Repeat("\x00", 8192) + after
	}
	for _, c := range []struct {
		head string
		want container
	}{
		{"fLaC\x00\x00\x00\x22", flacFile},
		{"OggS\x00\x02\x00\x00", oggFile},
		{"RIFF\x24\x00\x00\x00WAVEfmt ", wavFile},
		{"RIFF\x24\x00\x00\x00AVI LIST", webmFile},
		{"\xff\xfb\x90\x64\x00\x00\x00\x00", mp3File},
		{"\xff\xf3\x48\xc4\x00\x00\x00\x00", mp3File},
		{"\xff\xfb\xf0\x64\x00\x00\x00\x00", webmFile}, // a reserved bitrate
		{"ID3\x04\x00\x00\x00\x00\x00\x02xx\xff\xfb\x90\x64", mp3File},
		{"ID3\x04\x00\x00\x00\x00\x00\x02xxfLaC", flacFile},
		{"ID3\x04\x00\x00\x00\x00\x00\x02xx\xff\xf1\x50\x80", packedAudio},
		{"ID3\x04\x00\x00\x00\x00\x00\x00\xff\xfb\x90\x64", mp3File},
		{big("\xff\xfb\x90\x64"), mp3File},
		{big("fLaC"), flacFile},
		{big(""), packedAudio},
	} {
		r := bytes.NewReader([]byte(c.head))
		got, err := sniff(r)
		if pos, _ := r.Seek(0, 1); got != c.want || err != nil || pos != 0 {
			t.Errorf("%q: %v, %v, at %d", c.head[:min(len(c.head), 16)], got, err, pos)
		}
	}
}

// TestDeckLocalFailures fails a broken file of each kind with the words of
// the player.
func TestDeckLocalFailures(t *testing.T) {
	flacData, _ := os.ReadFile(flacStereo)
	opusData, _ := os.ReadFile(oggOpus)
	for _, c := range []struct {
		name string
		data []byte
		want error
	}{
		{"FLAC of no frames", flacData[:100], nil},
		{"Ogg of another codec", append([]byte("OggS"), opusData[4:28]...), errUnsupported},
		{"WAV of no audio", []byte("RIFF\x04\x00\x00\x00WAVE"), errUnsupported},
		{"WAV of ADPCM", []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x02\x00\x02\x00\x44\xac\x00\x00\x10\xb1\x02\x00\x04\x00\x10\x00data\x00\x00\x00\x00"), errUnsupported},
		{"MP3 of no frames", []byte("\xff\xfb\x90\x64" + strings.Repeat("\x00", 40)), errUnsupported},
	} {
		t.Run(c.name, func(t *testing.T) {
			open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
				return memSource{bytes.NewReader(c.data)}, nil
			}
			d := newDeck("x", 0, open, func() {})
			defer d.close()
			drain(t, d)
			err := d.failure()
			if c.want == nil {
				if err != nil && !errors.Is(err, errUnsupported) && !errors.Is(err, errDecode) {
					t.Errorf("failure %v", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Errorf("failure %v, want %v", err, c.want)
			}
		})
	}
}
