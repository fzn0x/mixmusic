package tags

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// The lyrics here are made up, as are the songs they're of.

// utf16be is s in UTF-16, big-endian after its byte order mark.
func utf16be(s string) []byte {
	b := []byte{0xFE, 0xFF}
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.BigEndian.AppendUint16(b, u)
	}
	return b
}

// syltFrame is the data of a SYLT frame of encoding enc, times in unit, of
// content, after its description: pieces, each a string ended as enc
// ends them, and its time.
func syltFrame(enc, unit, content byte, desc []byte, pieces ...any) []byte {
	b := cat([]byte{enc}, []byte("eng"), []byte{unit, content}, desc)
	for _, p := range pieces {
		switch p := p.(type) {
		case []byte:
			b = cat(b, p)
		case int:
			b = cat(b, be32(p))
		}
	}
	return b
}

// z16 ends a string of UTF-16, z8 one of ISO-8859-1 or UTF-8.
var z16, z8 = []byte{0, 0}, []byte{0}

func checkLyrics(t *testing.T, got Tags, plain, synced string) {
	t.Helper()
	if got.Lyrics != plain {
		t.Errorf("lyrics %q, want %q", got.Lyrics, plain)
	}
	if got.SyncedLyrics != synced {
		t.Errorf("synced lyrics %q, want %q", got.SyncedLyrics, synced)
	}
}

// TestLyricsID3v24 reads the lyrics of an ID3v2.4 tag: a USLT frame in
// UTF-8, keeping its line breaks, and a SYLT frame in UTF-16 with a byte
// order mark, timed in milliseconds, whose syllables join into lines
// where a line break starts one.
func TestLyricsID3v24(t *testing.T) {
	sylt := syltFrame(1, 2, 1, cat(utf16le("verses"), z16),
		cat(utf16le("\nPaper"), z16), 1000,
		cat(utf16le(" lan"), z16), 1250,
		cat(utf16le("terns"), z16), 1500,
		cat(utf16le("\nOver the bay"), z16), 4000,
		cat(utf16le("\n"), z16), 9000,
		cat(utf16le("Café"), z16), 9000,
		cat(utf16le(" lights"), z16), 9600,
	)
	body := cat(
		frame4("TIT2", 0, append([]byte{3}, "Paper Lanterns"...)),
		frame4("USLT", 0, cat([]byte{3}, []byte("eng"), []byte("a description\x00"), []byte("\r\nPaper lanterns\r\nOver the bay\r\n\r\nCafé lights\x01\r\n"))),
		frame4("SYLT", 0, sylt),
		frame4("TXXX", 0, cat([]byte{3}, []byte("LYRICS\x00"), []byte("A second text, after the USLT's"))),
	)
	got := read(t, cat(id3(4, 0, body), mp3))
	checkLyrics(t, got, "Paper lanterns\nOver the bay\n\nCafé lights",
		"[00:01.000]<00:01.000>Paper<00:01.250> lan<00:01.500>terns\n"+
			"[00:04.000]Over the bay\n"+
			"[00:09.000]<00:09.000><00:09.000>Café<00:09.600> lights")
	if got.Title != "Paper Lanterns" {
		t.Errorf("title %q", got.Title)
	}
}

// TestLyricsID3v23 reads the lyrics of an ID3v2.3 tag, unsynchronised:
// USLT in UTF-16 big-endian after its byte order mark, and SYLT in
// ISO-8859-1, a line each, timed in frames of the MPEG audio after the
// tag, 26.12 ms each at 44.1 kHz.
func TestLyricsID3v23(t *testing.T) {
	sylt := syltFrame(0, 1, 1, z8,
		cat([]byte("Caf\xe9 au lait"), z8), 38,
		cat([]byte("Noir"), z8), 153,
	)
	body := cat(
		frame3("USLT", 0, cat([]byte{1}, []byte("eng"), utf16be("d"), z16, utf16be("Ünder the pier\nTwo lines"))),
		frame3("SYLT", 0, sylt),
	)
	tag := id3(3, 0x80, bytes.ReplaceAll(body, []byte{0xFF}, []byte{0xFF, 0x00}))
	checkLyrics(t, read(t, cat(tag, mp3)), "Ünder the pier\nTwo lines", "[00:00.993]Café au lait\n[00:03.997]Noir")
}

// TestLyricsID3v22 reads the lyrics of an ID3v2.2 tag, ULT and SLT frames
// in ISO-8859-1.
func TestLyricsID3v22(t *testing.T) {
	body := cat(
		frame2("ULT", cat([]byte{0}, []byte("eng"), z8, []byte("Slow tide\nM\xe9t\xe9o"))),
		frame2("SLT", syltFrame(0, 2, 1, z8, cat([]byte("Slow tide"), z8), 61_500, cat([]byte("M\xe9t\xe9o"), z8), 3_725_250)),
	)
	checkLyrics(t, read(t, cat(id3(2, 0, body), mp3)), "Slow tide\nMétéo", "[01:01.500]Slow tide\n[62:05.250]Météo")
}

// TestLyricsTXXX reads lyrics of TXXX frames, plain and synced, by their
// descriptions in any case, as taggers write fields of their own.
func TestLyricsTXXX(t *testing.T) {
	body := cat(
		frame4("TXXX", 0, cat([]byte{3}, []byte("Lyrics\x00"), []byte("One line\nAnother"))),
		frame4("TXXX", 0, cat([]byte{1}, utf16le("SYNCEDLYRICS"), z16, utf16le("[00:01.00]One line\n[00:02.00]Another"))),
		frame4("TXXX", 0, cat([]byte{3}, []byte("ITUNESADVISORY\x00"), []byte("1"))),
	)
	got := read(t, cat(id3(4, 0, body), mp3))
	checkLyrics(t, got, "One line\nAnother", "[00:01.00]One line\n[00:02.00]Another")
	if !got.Explicit {
		t.Error("the advisory beside the lyrics was lost")
	}
}

// TestLyricsSYLTLeftOut passes over SYLT frames that aren't sung lyrics,
// as chords, and those timed in frames of audio that isn't MPEG, as a WAV
// file's, while its USLT frame is read.
func TestLyricsSYLTLeftOut(t *testing.T) {
	chords := frame3("SYLT", 0, syltFrame(0, 2, 5, z8, cat([]byte("Am"), z8), 1000, cat([]byte("G"), z8), 2000))
	frames := frame3("SYLT", 0, syltFrame(0, 1, 1, z8, cat([]byte("Field"), z8), 40, cat([]byte("Recording"), z8), 80))
	uslt := frame3("USLT", 0, cat([]byte{0}, []byte("eng"), z8, []byte("Field recording")))
	if got := read(t, cat(id3(3, 0, cat(chords, uslt)), mp3)); got.SyncedLyrics != "" || got.Lyrics != "Field recording" {
		t.Errorf("chords: %q, %q", got.SyncedLyrics, got.Lyrics)
	}
	v2 := id3(3, 0, cat(frames, uslt))
	chunk := cat([]byte("id3 "), le32(len(v2)), v2)
	if len(v2)%2 != 0 {
		chunk = append(chunk, 0)
	}
	body := cat([]byte("WAVE"), []byte("fmt "), le32(16), make([]byte, 16), []byte("data"), le32(4), make([]byte, 4), chunk)
	checkLyrics(t, read(t, cat([]byte("RIFF"), le32(len(body)), body)), "Field recording", "")
}

// TestLyricsMPEGFrames tells how long a frame of MPEG audio lasts, by its
// header: its version, layer and sample rate.
func TestLyricsMPEGFrames(t *testing.T) {
	for _, c := range []struct {
		name   string
		header string
		ms     float64
	}{
		{"MPEG-1 Layer III, 48 kHz", "\xff\xfb\x94\x64", 24},
		{"MPEG-2 Layer III, 24 kHz", "\xff\xf3\x84\x64", 24},
		{"MPEG-2.5 Layer III, 8 kHz", "\xff\xe3\x88\x64", 72},
		{"MPEG-1 Layer II, 32 kHz", "\xff\xfd\x88\x64", 36},
		{"MPEG-1 Layer I, 48 kHz", "\xff\xff\x84\x64", 8},
		{"a reserved version", "\xff\xeb\x90\x64", 0},
		{"a reserved rate", "\xff\xfb\x9c\x64", 0},
		{"no header", "fLaC", 0},
		{"the end of the file", "\xff", 0},
	} {
		f := &file{r: strings.NewReader(c.header), size: int64(len(c.header))}
		if got := f.mpegFrameMs(0); got != c.ms {
			t.Errorf("%s: %v ms, want %v", c.name, got, c.ms)
		}
	}
}

// TestLyricsVorbis reads the lyrics of Vorbis comments, of keys in any
// case, of a FLAC file and an Opus file: synced lyrics of LRC, of a
// SYNCEDLYRICS or SYNCLYRICS comment, and plain lyrics, of the best of
// LYRICS, UNSYNCEDLYRICS and UNSYNCED LYRICS, their line breaks kept.
func TestLyricsVorbis(t *testing.T) {
	flac := flacFile([]byte{4}, vcomments("TITLE=Glass Rivers", "unsynced lyrics=A second text",
		"lyrics=Glass rivers\r\nRun clear", "SyncedLyrics=[00:01.00]Glass rivers\n[00:03.50]Run clear"))
	checkLyrics(t, read(t, flac), "Glass rivers\nRun clear", "[00:01.00]Glass rivers\n[00:03.50]Run clear")
	flac = flacFile([]byte{4}, vcomments("LYRICS=Glass rivers\r\nRun clear", "SYNCLYRICS=[00:01.00]Glass"))
	checkLyrics(t, read(t, flac), "Glass rivers\nRun clear", "[00:01.00]Glass")
	opus := cat(
		oggPage(2, 0, cat([]byte("OpusHead"), []byte{1, 2, 0x38, 1, 0x80, 0xBB, 0, 0, 0, 0, 0})),
		oggPage(0, 1, cat([]byte("OpusTags"), vcomments("TITLE=Small Hours", "UNSYNCEDLYRICS=Small hours\n\nThe end"))),
	)
	checkLyrics(t, read(t, opus), "Small hours\n\nThe end", "")
}

// TestLyricsMP4 reads the lyrics of an MP4 file's ©lyr item, its old
// Mac line breaks made "\n", or else of iTunes' own LYRICS item.
func TestLyricsMP4(t *testing.T) {
	m4a := func(items ...[]byte) []byte {
		meta := mbox("meta", make([]byte, 4), mbox("hdlr", make([]byte, 25)), mbox("ilst", items...))
		return cat(mbox("ftyp", []byte("M4A "), be32(0)), mbox("moov", mbox("udta", meta)), mbox("mdat", make([]byte, 30)))
	}
	itunes := func(name, value string) []byte {
		return mbox("----", mbox("mean", be32(0), []byte("com.apple.iTunes")), mbox("name", be32(0), []byte(name)),
			mbox("data", be32(1), be32(0), []byte(value)))
	}
	got := read(t, m4a(item("\xa9nam", 1, []byte("Kites")), item("\xa9lyr", 1, []byte("Kites\rIn the wind\r")), itunes("LYRICS", "Not this one")))
	checkLyrics(t, got, "Kites\nIn the wind", "")
	if got.Title != "Kites" {
		t.Errorf("title %q", got.Title)
	}
	checkLyrics(t, read(t, m4a(itunes("ISRC", "XX0000000001"), itunes("lyrics", "From iTunes"))), "From iTunes", "")
}

// TestLyricsBroken reads frames of lyrics cut short as far as they go,
// and passes over a time past what a song lasts.
func TestLyricsBroken(t *testing.T) {
	for _, c := range []struct {
		name          string
		frames        []byte
		plain, synced string
	}{
		{"a USLT of no text", frame4("USLT", 0, []byte{3, 'e', 'n'}), "", ""},
		{"a SYLT of no pieces", frame4("SYLT", 0, []byte{3, 'e', 'n', 'g', 2}), "", ""},
		{"a SYLT cut in a time", frame4("SYLT", 0, cat(syltFrame(3, 2, 1, z8, cat([]byte("One"), z8), 1000), []byte("Two\x00\x00\x00"))), "", "[00:01.000]One"},
		{"a SYLT of a time no song lasts", frame4("SYLT", 0, syltFrame(0, 1, 1, z8, cat([]byte("Late"), z8), 0xFFFFFFFF, cat([]byte("On time"), z8), 0)), "", "[00:00.000]On time"},
		{"a SYLT in an unknown unit", frame4("SYLT", 0, syltFrame(0, 3, 1, z8, cat([]byte("One"), z8), 1000)), "", ""},
	} {
		t.Run(c.name, func(t *testing.T) { checkLyrics(t, read(t, cat(id3(4, 0, c.frames), mp3)), c.plain, c.synced) })
	}
}

// FuzzReadLyrics reads any bytes as a file's tags: the lyrics it reads are
// valid UTF-8 of maxLyrics bytes at most, trimmed, without control
// characters but line breaks and tabs.
func FuzzReadLyrics(f *testing.F) {
	f.Add(cat(id3(4, 0, frame4("USLT", 0, cat([]byte{3}, []byte("eng"), z8, []byte("A\r\nB")))), mp3))
	f.Add(cat(id3(3, 0, frame3("SYLT", 0, syltFrame(1, 1, 1, z16, cat(utf16le("\nA"), z16), 10, cat(utf16le("b"), z16), 20))), mp3))
	f.Add(cat(id3(2, 0, frame2("SLT", syltFrame(0, 2, 0, z8, cat([]byte("x"), z8), 5))), mp3))
	f.Add(flacFile([]byte{4}, vcomments("LYRICS=x\ny", "SYNCEDLYRICS=[00:01.00]x")))
	f.Add(cat(mbox("ftyp", []byte("M4A ")), mbox("moov", mbox("udta", mbox("meta", make([]byte, 4), mbox("ilst", item("\xa9lyr", 1, []byte("x\ry"))))))))
	f.Fuzz(func(t *testing.T, b []byte) {
		got, err := Read(bytes.NewReader(b))
		if err != nil {
			return
		}
		for _, s := range []string{got.Lyrics, got.SyncedLyrics} {
			if !utf8.ValidString(s) || len(s) > maxLyrics || strings.TrimSpace(s) != s ||
				strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) {
				t.Fatalf("lyrics %q", s)
			}
		}
	})
}
