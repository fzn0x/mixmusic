package audio

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestProbe tells how long the files kept on a computer play, as a deck
// plays them, and that it can.
func TestProbe(t *testing.T) {
	for _, c := range []struct {
		path string
		want time.Duration
	}{
		{flacStereo, 200 * time.Millisecond},
		{flacMono24, 100 * time.Millisecond},
		{flacSurround, 200 * time.Millisecond},
		{oggOpus, 3 * time.Second},
		{oggVorbis, 2 * time.Second},
		{mp3Tone, 500 * time.Millisecond},
		{aacProgressive, 3 * time.Second},
	} {
		b, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Probe(bytes.NewReader(b))
		if err != nil || got < c.want-30*time.Millisecond || got > c.want+30*time.Millisecond {
			t.Errorf("%s: %v, %v; want %v", filepath.Base(c.path), got, err, c.want)
		}
	}
}

// TestProbeFFmpeg tells the length of the files of each kind ffmpeg makes,
// a second each, MP3 of a constant bitrate from its size too.
func TestProbeFFmpeg(t *testing.T) {
	for name, path := range makeLocal(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Probe(bytes.NewReader(b))
		if err != nil || got < 950*time.Millisecond || got > 1100*time.Millisecond {
			t.Errorf("%s: %v, %v", name, got, err)
		}
		if filepath.Ext(name) != ".mp3" {
			continue
		}
		// Without its LAME tag, a file of a constant bitrate tells its
		// length by its size.
		if got := mp3Length(bytes.NewReader(b)); name == "mp3-cbr.mp3" && (got < 950*time.Millisecond || got > 1100*time.Millisecond) {
			t.Errorf("%s by its size: %v", name, got)
		}
	}
}

// TestProbeRefuses tells why a deck can't play broken files, and files of
// other kinds.
func TestProbeRefuses(t *testing.T) {
	flacData, _ := os.ReadFile(flacStereo)
	webm, _ := os.ReadFile(cuesFront)
	for name, b := range map[string][]byte{
		"empty":          nil,
		"text":           []byte("a shopping list, not a song"),
		"FLAC cut short": flacData[:100],
		"WAV of ADPCM":   []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x02\x00\x02\x00\x44\xac\x00\x00\x10\xb1\x02\x00\x04\x00\x10\x00data\x00\x00\x00\x00"),
		"MP3 of nothing": append([]byte("\xff\xfb\x90\x64"), make([]byte, 40)...),
		"WebM":           webm,
	} {
		if _, err := Probe(bytes.NewReader(b)); !errors.Is(err, ErrCantPlay) {
			t.Errorf("%s: %v, want ErrCantPlay", name, err)
		}
	}
}
