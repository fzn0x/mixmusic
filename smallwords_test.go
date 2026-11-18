package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/localfiles"
)

// TestSmallWordsInTheLanguage says the explicit badge and the units of
// sizes, bitrates, bit depths and sample rates in en-XA, which brackets
// each message of the catalogs, with the numbers inside.
func TestSmallWordsInTheLanguage(t *testing.T) {
	usePseudoLocale(t)
	flac := localfiles.Copy{Facts: localfiles.Facts{Codec: "FLAC", SampleRate: 96000, BitDepth: 24}}
	mp3 := localfiles.Copy{Facts: localfiles.Facts{Codec: "MP3", SampleRate: 44100}, Bitrate: 320}
	for what, txt := range map[string]string{
		"512 MB": byteSize(512 << 20), "1.5 KB": byteSize(1536), "44.1 kHz": kilohertz(44100),
		"FLAC": copyFormat(flac), "MP3": copyFormat(mp3),
	} {
		if !strings.Contains(txt, "[") {
			t.Errorf("%s says %q, which no catalog translates", what, txt)
		}
	}
	for what, want := range map[string]string{byteSize(512 << 20): "512", kilohertz(44100): "44.1", copyFormat(mp3): "320"} {
		if !strings.Contains(what, want) {
			t.Errorf("%q lost its number %s", what, want)
		}
	}
}

// TestSmallWordsInEnglish keeps the units as they were in English.
func TestSmallWordsInEnglish(t *testing.T) {
	mp3 := localfiles.Copy{Facts: localfiles.Facts{Codec: "MP3", SampleRate: 44100}, Bitrate: 320}
	for got, want := range map[string]string{byteSize(512 << 20): "512 MB", byteSize(1536): "1.5 KB", byteSize(12): "12 B",
		kilohertz(44100): "44.1 kHz", copyFormat(mp3): "MP3 · 320 kbit/s · 44.1 kHz"} {
		if got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
}
