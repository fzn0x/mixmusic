package audio

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestInspect tells what the files of each kind a deck plays hold, as
// ffprobe tells it of them.
func TestInspect(t *testing.T) {
	for _, c := range []struct {
		path string
		want Format
	}{
		{flacStereo, Format{Container: "FLAC", Codec: "FLAC", SampleRate: 44100, BitDepth: 16, Channels: 2, Duration: 200 * time.Millisecond}},
		{flacMono24, Format{Container: "FLAC", Codec: "FLAC", SampleRate: 96000, BitDepth: 24, Channels: 1, Duration: 100 * time.Millisecond}},
		{flacSurround, Format{Container: "FLAC", Codec: "FLAC", SampleRate: 8000, BitDepth: 16, Channels: 6, Duration: 200 * time.Millisecond}},
		{oggOpus, Format{Container: "Ogg", Codec: "Opus", SampleRate: 48000, Channels: 2, Duration: 3 * time.Second}},
		{oggVorbis, Format{Container: "Ogg", Codec: "Vorbis", SampleRate: 44100, Channels: 2, Duration: 2 * time.Second}},
		{mp3Tone, Format{Container: "MP3", Codec: "MP3", SampleRate: 22050, Channels: 1, Duration: 500 * time.Millisecond}},
		{aacProgressive, Format{Container: "MP4", Codec: "AAC", SampleRate: 44100, Channels: 2, Duration: 3 * time.Second}},
		{aacMono48, Format{Container: "MP4", Codec: "AAC", SampleRate: 48000, Channels: 1, Duration: 3 * time.Second}},
		{surroundAC3, Format{Container: "MP4", Codec: "AC-3", SampleRate: 48000, Channels: 6, Duration: 994 * time.Millisecond}},
		{surroundEAC3, Format{Container: "MP4", Codec: "E-AC-3", SampleRate: 48000, Channels: 6, Duration: 994 * time.Millisecond}},
		{surroundAC3At4, Format{Container: "MP4", Codec: "AC-3", SampleRate: 44100, Channels: 6, Duration: 494 * time.Millisecond}},
		{cuesFront, Format{Container: "WebM", Codec: "Opus", SampleRate: 48000, Channels: 2, Duration: 3 * time.Second}},
		{monoStream, Format{Container: "WebM", Codec: "Opus", SampleRate: 48000, Channels: 1}},
	} {
		b, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Inspect(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(c.path), err)
			continue
		}
		if d := got.Duration - c.want.Duration; d < -5*time.Millisecond || d > 5*time.Millisecond {
			t.Errorf("%s: %v long, want %v", filepath.Base(c.path), got.Duration, c.want.Duration)
		}
		got.Duration = c.want.Duration
		if got != c.want {
			t.Errorf("%s: %+v, want %+v", filepath.Base(c.path), got, c.want)
		}
	}
}

// TestInspectFFmpeg tells what the files of each kind ffmpeg makes hold:
// WAV of each sample size, MP3 in mono, and the rates they were made at.
func TestInspectFFmpeg(t *testing.T) {
	want := map[string]Format{
		"mp3-cbr.mp3":   {Container: "MP3", Codec: "MP3", SampleRate: 44100, Channels: 2},
		"mp3-vbr.mp3":   {Container: "MP3", Codec: "MP3", SampleRate: 44100, Channels: 2},
		"mp3-mono.mp3":  {Container: "MP3", Codec: "MP3", SampleRate: 22050, Channels: 1},
		"wav-16.wav":    {Container: "WAV", Codec: "PCM", SampleRate: 44100, BitDepth: 16, Channels: 2},
		"wav-24.wav":    {Container: "WAV", Codec: "PCM", SampleRate: 96000, BitDepth: 24, Channels: 2},
		"wav-float.wav": {Container: "WAV", Codec: "PCM", SampleRate: 44100, BitDepth: 32, Float: true, Channels: 1},
		"wav-8.wav":     {Container: "WAV", Codec: "PCM", SampleRate: 8000, BitDepth: 8, Channels: 2},
		"flac.flac":     {Container: "FLAC", Codec: "FLAC", SampleRate: 44100, BitDepth: 16, Channels: 2},
		"m4a.m4a":       {Container: "MP4", Codec: "AAC", SampleRate: 44100, Channels: 2},
		"opus.opus":     {Container: "Ogg", Codec: "Opus", SampleRate: 48000, Channels: 2},
		"vorbis.ogg":    {Container: "Ogg", Codec: "Vorbis", SampleRate: 44100, Channels: 2},
	}
	for name, path := range makeLocal(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Inspect(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got.Duration < 950*time.Millisecond || got.Duration > 1100*time.Millisecond {
			t.Errorf("%s: %v long", name, got.Duration)
		}
		got.Duration = 0
		if got != want[name] {
			t.Errorf("%s: %+v, want %+v", name, got, want[name])
		}
	}
}

// TestInspectRefuses tells a file a deck can't play, as Probe does.
func TestInspectRefuses(t *testing.T) {
	for name, b := range map[string][]byte{
		"text":  []byte("not a file of audio, but some text that goes on for a while"),
		"empty": nil,
	} {
		if f, err := Inspect(bytes.NewReader(b)); !errors.Is(err, ErrCantPlay) {
			t.Errorf("%s: %+v, %v", name, f, err)
		}
	}
}

// loadedOf waits until the engine's active deck read its track's headers.
func loadedOf(t *testing.T, e *Engine) Loaded {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		l, ok := e.Loaded()
		if ok && l.Ready {
			return l
		}
		if time.Now().After(deadline) {
			t.Fatalf("the deck didn't read its headers: %+v, %v", l, ok)
		}
	}
}

// TestEngineLoaded tells what the engine plays: nothing before a track
// loads, then the track's file as its deck read it.
func TestEngineLoaded(t *testing.T) {
	e, _, _ := newTestEngine(t)
	if l, ok := e.Loaded(); ok {
		t.Errorf("before a track: %+v", l)
	}
	e.Load(surroundAC3, Cue{})
	l := loadedOf(t, e)
	want := Format{Container: "MP4", Codec: "AC-3", SampleRate: 48000, Channels: 6, Duration: l.Format.Duration}
	if l.URL != surroundAC3 || l.Format != want || l.Measured || l.Trim != 0 {
		t.Errorf("loaded %+v, want %+v", l, want)
	}
	// Its decoder gives an MP3 in stereo, whatever the file holds.
	e.Load(mp3Tone, Cue{})
	if l := loadedOf(t, e); l.URL != mp3Tone || l.Format.Codec != "MP3" || l.Format.SampleRate != 22050 || l.Format.Channels != 0 {
		t.Errorf("an MP3: %+v", l)
	}
}

// TestEngineLoadedLoudness tells how loud the engine plays a track: as its
// source measured it, turned down by Even loudness while it's on.
func TestEngineLoadedLoudness(t *testing.T) {
	e, _ := newMeasuredEngine(t)
	e.Load(cuesFront+"#-8", Cue{})
	if l := loadedOf(t, e); !l.Measured || l.LKFS != -8 || !l.EvenLoudness || l.Trim != -6 || l.Levelling {
		t.Errorf("a loud track: %+v", l)
	}
	e.SetLoudness(false)
	e.SetNormalize(true)
	if l := loadedOf(t, e); !l.Measured || l.EvenLoudness || l.Trim != 0 || !l.Levelling {
		t.Errorf("with Even loudness off: %+v", l)
	}
	e.Load(cuesFront, Cue{})
	if l := loadedOf(t, e); l.Measured || l.Trim != 0 {
		t.Errorf("a track of no known loudness: %+v", l)
	}
	if got := LoudnessTrim(-20); got != 0 {
		t.Errorf("a quiet track's trim: %v", got)
	}
}
