package tags

import (
	"encoding/binary"
	"testing"
)

// TestGenres reads a genre of ID3v2 by its name, by the number of a genre
// of ID3v1, alone or refined by a name, and the ISRC.
func TestGenres(t *testing.T) {
	for _, c := range []struct{ tcon, want string }{
		{"Dream Pop", "Dream Pop"},
		{"(17)", "Rock"},
		{"17", "Rock"},
		{"(9)Black Metal", "Black Metal"},
		{"(RX)", "Remix"},
		{"((Unknown) Folk", "(Unknown) Folk"},
		{"(999)", "(999)"},
		{"2000s", "2000s"},
	} {
		body := cat(frame4("TCON", 0, append([]byte{3}, c.tcon...)), frame4("TSRC", 0, append([]byte{3}, "QZES82100001"...)))
		got := read(t, cat(id3(4, 0, body), mp3))
		if got.Genre != c.want || got.ISRC != "QZES82100001" {
			t.Errorf("TCON %q reads %q, ISRC %q", c.tcon, got.Genre, got.ISRC)
		}
	}
	if n, ok := GenreNumber("rock"); !ok || n != 17 {
		t.Errorf("GenreNumber(rock) = %d, %v", n, ok)
	}
	if _, ok := GenreNumber("Dream Pop"); ok {
		t.Error("Dream Pop has a number")
	}
}

// TestID3v23Values reads the values of a text frame of ID3v2.3 between
// NULs, as programs write several artists.
func TestID3v23Values(t *testing.T) {
	body := frame3("TPE1", 0, append([]byte{1}, cat(utf16le("Mira Holt"), []byte{0, 0}, utf16le("Orin Vale"))...))
	got := read(t, cat(id3(3, 0, body), mp3))
	if len(got.Artists) != 2 || got.Artists[0] != "Mira Holt" || got.Artists[1] != "Orin Vale" {
		t.Errorf("artists %q", got.Artists)
	}
}

// TestMP4Genres reads an MP4 file's genre of text, or of a number, its
// ISRC of a freeform item, and the values of an item of several data
// boxes.
func TestMP4Genres(t *testing.T) {
	box := func(kind string, data ...[]byte) []byte {
		b := cat(data...)
		return cat(be32(8+len(b)), []byte(kind), b)
	}
	text := func(v string) []byte { return box("data", be32(1), be32(0), []byte(v)) }
	gnre := box("gnre", box("data", be32(0), be32(0), binary.BigEndian.AppendUint16(nil, 18)))
	isrc := box("----", box("mean", be32(0), []byte("com.apple.iTunes")), box("name", be32(0), []byte("ISRC")), text("QZES82100001"))
	file := func(items ...[]byte) []byte {
		ilst := box("ilst", items...)
		meta := box("meta", be32(0), box("hdlr", make([]byte, 25)), ilst)
		return cat(box("ftyp", []byte("M4A \x00\x00\x00\x00")), box("moov", box("udta", meta)), box("mdat"))
	}
	got := read(t, file(gnre, isrc, box("\xa9ART", text("Ada Field"), text("Orin Vale"))))
	if got.Genre != "Rock" || got.ISRC != "QZES82100001" || len(got.Artists) != 2 || got.Artists[1] != "Orin Vale" {
		t.Errorf("read %+v", got)
	}
	got = read(t, file(box("\xa9gen", text("Ambient")), gnre))
	if got.Genre != "Ambient" {
		t.Errorf("the genre of text reads %q", got.Genre)
	}
}
