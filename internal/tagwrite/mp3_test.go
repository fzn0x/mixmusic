package tagwrite

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

// mp3Audio is n frames of MPEG-1 Layer III at 128 kbit/s and 44.1 kHz, of
// made-up bytes.
func mp3Audio(n int) []byte {
	var b []byte
	for i := range n {
		f := make([]byte, 417)
		copy(f, []byte{0xFF, 0xFB, 0x90, 0x64})
		for j := 4; j < len(f); j++ {
			f[j] = byte(i*31 + j*7)
		}
		b = append(b, f...)
	}
	return b
}

// mp3File is an MP3 file: tag, n frames, and an ID3v1 tag if v1.
func mp3File(tag []byte, n int, v1 bool) []byte {
	b := append(bytes.Clone(tag), mp3Audio(n)...)
	if v1 {
		b = append(b, id3v1("Old Title", "Old Artist", "Old Album", "1999", 7, 17)...)
	}
	return b
}

// id3v1 is an ID3v1.1 tag.
func id3v1(title, artist, album, year string, track, genre byte) []byte {
	b := make([]byte, 128)
	copy(b, "TAG")
	copy(b[3:33], title)
	copy(b[33:63], artist)
	copy(b[63:93], album)
	copy(b[93:97], year)
	b[126], b[127] = track, genre
	return b
}

func id3Of(version byte, frames ...[]byte) []byte {
	body := bytes.Join(append(frames, make([]byte, 32)), nil)
	return append([]byte{'I', 'D', '3', version, 0, 0}, append(safe(len(body)), body...)...)
}

func id3Tag4(frames ...[]byte) []byte { return id3Of(4, frames...) }

func frame4(id string, data []byte) []byte {
	return bytes.Join([][]byte{[]byte(id), safe(len(data)), {0, 0}, data}, nil)
}

func frame3(id string, data []byte) []byte {
	return bytes.Join([][]byte{[]byte(id), binary.BigEndian.AppendUint32(nil, uint32(len(data))), {0, 0}, data}, nil)
}

func frame2(id string, data []byte) []byte {
	n := len(data)
	return bytes.Join([][]byte{[]byte(id), {byte(n >> 16), byte(n >> 8), byte(n)}, data}, nil)
}

func textFrame4(id string, values ...string) []byte {
	return frame4(id, append([]byte{3}, strings.Join(values, "\x00")...))
}

func utf16Text(s string) []byte {
	b := []byte{1, 0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

// mp3AudioOf is what follows a file's ID3v2 tag, up to its ID3v1 tag.
func mp3AudioOf(t *testing.T, b []byte) []byte {
	t.Helper()
	if string(b[:3]) == "ID3" {
		b = b[10+syncsafe(b[6:10]):]
	}
	if len(b) >= 128 && string(b[len(b)-128:len(b)-125]) == "TAG" {
		b = b[:len(b)-128]
	}
	return b
}

// frameIDs are the frames of the ID3v2 tag at the start of b, by their
// names, and its version.
func frameIDs(t *testing.T, b []byte) ([]string, byte) {
	t.Helper()
	f, _ := newFile(bytes.NewReader(b))
	tag, err := f.readID3()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, fr := range tag.frames {
		ids = append(ids, fr.id)
	}
	return ids, tag.version
}

// TestMP3 writes every field of an ID3v2.4 tag, keeps the frames it
// doesn't change, a comment and a user's frame, takes the old pictures'
// place with the new cover, and brings the ID3v1 tag along; the audio is
// as it was.
func TestMP3(t *testing.T) {
	old := id3Tag4(
		textFrame4("TIT2", "Old Title"), textFrame4("TPE1", "Old Artist"), textFrame4("TCON", "(17)"),
		frame4("COMM", append([]byte{3}, "eng\x00Recorded live"...)),
		frame4("TXXX", append([]byte{3}, "MOOD\x00calm"...)),
		frame4("APIC", append(append([]byte{0}, "image/jpeg\x00\x04\x00"...), jpegCover(t, 8, 8)...)), // a back cover
	)
	data := mp3File(old, 40, true)
	if got := readTags(t, save(t, "old.mp3", data)); got.Genre != "Rock" {
		t.Errorf("the old genre reads %q", got.Genre)
	}
	path := save(t, "song.mp3", data)
	tg := song(t)
	b := write(t, path, tg, All)
	matches(t, path, tg)
	if !bytes.Equal(mp3AudioOf(t, b), mp3AudioOf(t, data)) {
		t.Error("the audio changed")
	}
	ids, version := frameIDs(t, b)
	want := []string{"TIT2", "TPE1", "TALB", "TPE2", "TRCK", "TPOS", "TDRC", "TCON", "TSRC", "APIC", "COMM", "TXXX"}
	if version != 4 || strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Errorf("ID3v2.%d of %q, want %q", version, ids, want)
	}
	// ID3v1 holds ISO-8859-1, of which the title and the artists have
	// characters it doesn't.
	v1 := b[len(b)-128:]
	if string(v1[:3]) != "TAG" || strings.Trim(string(v1[3:63]), "\x00") != "" {
		t.Errorf("the ID3v1 title and artist are %q", v1[3:63])
	}
	if string(bytes.TrimRight(v1[63:93], "\x00")) != "Caf\xe9 Nights" || string(v1[93:97]) != "2021" || v1[126] != 3 || v1[127] != 255 {
		t.Errorf("the ID3v1 tag: %q", v1)
	}

	// Taking fields away takes them out of both tags.
	b = write(t, path, Tags{}, Album|Year|Cover|Genre)
	tg.Album, tg.Year, tg.Cover, tg.Genre = "", 0, nil, ""
	matches(t, path, tg)
	v1 = b[len(b)-128:]
	if strings.Trim(string(v1[63:97]), "\x00") != "" {
		t.Errorf("the ID3v1 tag keeps %q", v1[63:97])
	}
	if !bytes.Equal(mp3AudioOf(t, b), mp3AudioOf(t, data)) {
		t.Error("the audio changed")
	}
}

// TestMP3v23 keeps an ID3v2.3 tag of 2.3, unsynchronised whole, writing
// text that ISO-8859-1 doesn't hold in UTF-16, two artists between NULs,
// and the year as TYER; it grows with a large cover, then shrinks.
func TestMP3v23(t *testing.T) {
	body := bytes.Join([][]byte{
		frame3("TIT2", utf16Text("Ünder the Pier")),
		frame3("TYER", append([]byte{0}, "1999"...)),
		frame3("TRCK", append([]byte{0}, "7/9"...)),
		frame3("TDAT", append([]byte{0}, "0504"...)),
		frame3("PRIV", []byte("maker\x00\xff\x00\xff")),
	}, nil)
	unsynced := bytes.ReplaceAll(body, []byte{0xFF}, []byte{0xFF, 0x00})
	tag := append([]byte{'I', 'D', '3', 3, 0, 0x80}, append(safe(len(unsynced)), unsynced...)...)
	data := mp3File(tag, 30, false)
	path := save(t, "pier.mp3", data)
	if got := readTags(t, path); got.Title != "Ünder the Pier" || got.Year != 1999 || got.Track != 7 {
		t.Fatalf("the old tags read %+v", got)
	}
	tg := Tags{Title: "Ünder the Pier", Artists: []string{"Mira Holt", "ミラ・ホルト"}, Track: 8, Year: 2001,
		Cover: &Picture{Data: pngCover(t, 300, 300, 3)}}
	b := write(t, path, tg, Artists|Track|Year|Cover)
	matches(t, path, tg)
	ids, version := frameIDs(t, b)
	if version != 3 || strings.Join(ids, " ") != "TPE1 TRCK TYER APIC TIT2 PRIV" {
		t.Errorf("ID3v2.%d of %q", version, ids)
	}
	f, _ := newFile(bytes.NewReader(b))
	tag2, _ := f.readID3()
	if v := tag2.value("TRCK"); v != "8/9" {
		t.Errorf("the track's frame holds %q, want its count kept", v)
	}
	for _, fr := range tag2.frames {
		if fr.id == "PRIV" && string(fr.data) != "maker\x00\xff\x00\xff" {
			t.Errorf("PRIV holds %q", fr.data)
		}
	}
	big := len(b)
	if !bytes.Equal(mp3AudioOf(t, b), mp3AudioOf(t, data)) {
		t.Error("the audio changed")
	}

	cover := len(tg.Cover.Data)
	tg.Cover = nil
	b = write(t, path, tg, Cover)
	if big-len(b) < cover {
		t.Errorf("%d bytes without the cover, %d with one of %d", len(b), big, cover)
	}
	matches(t, path, tg)
	if !bytes.Equal(mp3AudioOf(t, b), mp3AudioOf(t, data)) {
		t.Error("the audio changed")
	}
}

// TestMP3v22 makes an ID3v2.2 tag one of 2.4, keeping the frames 2.4
// has, and the picture.
func TestMP3v22(t *testing.T) {
	cover := pngCover(t, 4, 4, 9)
	tag := id3Of(2,
		frame2("TT2", append([]byte{0}, "Old Song"...)),
		frame2("TCM", append([]byte{0}, "Composer Name"...)),
		frame2("TRK", append([]byte{0}, "2/10"...)),
		frame2("PIC", append(append([]byte{0}, "PNG\x03\x00"...), cover...)),
		frame2("XYZ", []byte("a frame 2.4 has no name for")),
	)
	data := mp3File(tag, 10, false)
	path := save(t, "old.mp3", data)
	b := write(t, path, Tags{Title: "New Song", Track: 3}, Title|Track)
	ids, version := frameIDs(t, b)
	if version != 4 || strings.Join(ids, " ") != "TIT2 TRCK TCOM APIC" {
		t.Errorf("ID3v2.%d of %q", version, ids)
	}
	got := readTags(t, path)
	if got.Title != "New Song" || got.Track != 3 || got.Picture == nil || !bytes.Equal(got.Picture.Data, cover) {
		t.Errorf("read %+v", got)
	}
	f, _ := newFile(bytes.NewReader(b))
	tag4, _ := f.readID3()
	if v := tag4.value("TRCK"); v != "3/10" {
		t.Errorf("TRCK holds %q", v)
	}
	if !bytes.Equal(mp3AudioOf(t, b), mp3AudioOf(t, data)) {
		t.Error("the audio changed")
	}
}

// TestMP3WithoutTags gives an MP3 without tags an ID3v2.4 tag.
func TestMP3WithoutTags(t *testing.T) {
	data := mp3File(nil, 12, false)
	path := save(t, "bare.mp3", data)
	tg := song(t)
	b := write(t, path, tg, All)
	matches(t, path, tg)
	if !bytes.Equal(mp3AudioOf(t, b), data) {
		t.Error("the audio changed")
	}
	if _, version := frameIDs(t, b); version != 4 {
		t.Errorf("ID3v2.%d", version)
	}
}

// TestGenreOfParens writes a genre that starts with "(", which ID3 would
// read as a genre's number.
func TestGenreOfParens(t *testing.T) {
	path := save(t, "song.mp3", mp3File(nil, 3, false))
	write(t, path, Tags{Genre: "(Unknown) Folk"}, Genre)
	if got := readTags(t, path); got.Genre != "(Unknown) Folk" {
		t.Errorf("the genre reads %q", got.Genre)
	}
}
