package tags

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

// The tags here are made up, as are the songs they name.

// png is the start of a PNG image, as a cover's data.
var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR made-up cover")

// jpeg is the start of a JPEG image.
var jpeg = []byte("\xff\xd8\xff\xe0 another made-up cover")

func be32(n int) []byte { return binary.BigEndian.AppendUint32(nil, uint32(n)) }
func le32(n int) []byte { return binary.LittleEndian.AppendUint32(nil, uint32(n)) }
func safe(n int) []byte {
	return []byte{byte(n >> 21 & 127), byte(n >> 14 & 127), byte(n >> 7 & 127), byte(n & 127)}
}
func cat(b ...[]byte) []byte { return bytes.Join(b, nil) }

// id3 is an ID3v2 tag of version v and flags, of body.
func id3(v, flags byte, body []byte) []byte {
	return cat([]byte{'I', 'D', '3', v, 0, flags}, safe(len(body)), body)
}

// frame4 is a frame of ID3v2.4.
func frame4(id string, flags uint16, data []byte) []byte {
	return cat([]byte(id), safe(len(data)), binary.BigEndian.AppendUint16(nil, flags), data)
}

// frame3 is a frame of ID3v2.3.
func frame3(id string, flags uint16, data []byte) []byte {
	return cat([]byte(id), be32(len(data)), binary.BigEndian.AppendUint16(nil, flags), data)
}

// frame2 is a frame of ID3v2.2.
func frame2(id string, data []byte) []byte {
	n := len(data)
	return cat([]byte(id), []byte{byte(n >> 16), byte(n >> 8), byte(n)}, data)
}

func utf16le(s string) []byte {
	b := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

// mp3 is an MP3 frame's start, after a tag.
var mp3 = []byte("\xff\xfb\x90\x64" + strings.Repeat("\x00", 200))

func read(t *testing.T, b []byte) Tags {
	t.Helper()
	got, err := Read(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func check(t *testing.T, got, want Tags) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags\n%+v\nwant\n%+v", got, want)
	}
}

// TestID3v24 reads an ID3v2.4 tag: text in UTF-8 and ISO-8859-1, two
// artists, numbers of a count, a date, and the front cover of two
// pictures, unsynchronised.
func TestID3v24(t *testing.T) {
	unsynced := bytes.ReplaceAll(png, []byte{0xFF}, []byte{0xFF, 0x00})
	body := cat(
		frame4("TIT2", 0, append([]byte{3}, "Lanterns over the Bay"...)),
		frame4("TPE1", 0, append([]byte{3}, "Ada Field\x00Orin Vale"...)),
		frame4("TALB", 0, append([]byte{0}, "Caf\xe9 Nights"...)),
		frame4("TPE2", 0, append([]byte{3}, "Various Artists"...)),
		frame4("TRCK", 0, append([]byte{3}, "3/12"...)),
		frame4("TPOS", 0, append([]byte{3}, "2/2"...)),
		frame4("TDRC", 0, append([]byte{3}, "2021-04-05"...)),
		frame4("APIC", 0, cat([]byte{3}, []byte("image/jpeg\x00"), []byte{0}, []byte("back\x00"), jpeg)),
		frame4("APIC", 2, cat([]byte{0}, []byte("image/png\x00"), []byte{3}, []byte("front\x00"), unsynced)),
		make([]byte, 64), // padding
	)
	check(t, read(t, cat(id3(4, 0, body), mp3)), Tags{
		Title: "Lanterns over the Bay", Artists: []string{"Ada Field", "Orin Vale"}, Album: "Café Nights",
		AlbumArtists: []string{"Various Artists"}, Track: 3, Disc: 2, Year: 2021,
		Picture: &Picture{MIME: "image/png", Data: png, front: true},
	})
}

// TestID3v23 reads an ID3v2.3 tag, unsynchronised whole after an extended
// header: text in UTF-16, a compressed frame, and a year.
func TestID3v23(t *testing.T) {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(append([]byte{0}, "Paper Moons"...))
	w.Close()
	body := cat(
		be32(6), make([]byte, 6), // an extended header
		frame3("TIT2", 0, append([]byte{1}, utf16le("Ünder the Pier")...)),
		frame3("TPE1", 0, append([]byte{1}, append(utf16le("Mira Holt"), 0, 0)...)),
		frame3("TALB", 0x0080, cat(be32(12), z.Bytes())),
		frame3("TYER", 0, append([]byte{0}, "1999"...)),
		frame3("TRCK", 0, append([]byte{0}, "7"...)),
		frame3("APIC", 0, cat([]byte{0}, []byte("image/png\x00"), []byte{3}, []byte("\x00"), png)),
	)
	tag := id3(3, 0xC0, bytes.ReplaceAll(body, []byte{0xFF}, []byte{0xFF, 0x00}))
	check(t, read(t, cat(tag, mp3)), Tags{
		Title: "Ünder the Pier", Artists: []string{"Mira Holt"}, Album: "Paper Moons", Track: 7, Year: 1999,
		Picture: &Picture{MIME: "image/png", Data: png, front: true},
	})
}

// TestID3v22 reads an ID3v2.2 tag, of three-letter frames.
func TestID3v22(t *testing.T) {
	body := cat(
		frame2("TT2", append([]byte{0}, "Slow Tide"...)),
		frame2("TP1", append([]byte{0}, "The Harbour Lights"...)),
		frame2("TAL", append([]byte{0}, "Shore"...)),
		frame2("TRK", append([]byte{0}, "01"...)),
		frame2("PIC", cat([]byte{0}, []byte("JPG"), []byte{3}, []byte("\x00"), jpeg)),
	)
	check(t, read(t, cat(id3(2, 0, body), mp3)), Tags{
		Title: "Slow Tide", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 1,
		Picture: &Picture{MIME: "image/jpeg", Data: jpeg, front: true},
	})
}

// id3v1 is an ID3v1.1 tag.
func id3v1(title, artist, album, year string, track byte) []byte {
	field := func(s string, n int) []byte { return append([]byte(s), make([]byte, n-len(s))...) }
	return cat([]byte("TAG"), field(title, 30), field(artist, 30), field(album, 30), field(year, 4), make([]byte, 29), []byte{track, 0})
}

// TestID3v1 reads the ID3v1 tag at the end of an MP3 file, alone, and
// for what its ID3v2 tag leaves out.
func TestID3v1(t *testing.T) {
	check(t, read(t, cat(mp3, id3v1("Night Bus", "Juno Reyes", "Late", "2008", 4))), Tags{
		Title: "Night Bus", Artists: []string{"Juno Reyes"}, Album: "Late", Year: 2008, Track: 4,
	})
	v2 := id3(4, 0, frame4("TIT2", 0, append([]byte{3}, "Night Bus (Live)"...)))
	check(t, read(t, cat(v2, mp3, id3v1("Night Bus", "Juno Reyes", "", "", 0))), Tags{
		Title: "Night Bus (Live)", Artists: []string{"Juno Reyes"},
	})
}

// vcomments is a block of Vorbis comments.
func vcomments(pairs ...string) []byte {
	b := cat(le32(6), []byte("vendor"), le32(len(pairs)))
	for _, p := range pairs {
		b = cat(b, le32(len(p)), []byte(p))
	}
	return b
}

// flacPicture is a FLAC PICTURE block's body.
func flacPictureBlock(kind int, mime string, data []byte) []byte {
	return cat(be32(kind), be32(len(mime)), []byte(mime), be32(4), []byte("desc"), make([]byte, 16), be32(len(data)), data)
}

// flacFile is a FLAC file of a STREAMINFO and blocks of a type each.
func flacFile(blocks ...[]byte) []byte {
	b := cat([]byte("fLaC"), []byte{0, 0, 0, 34}, make([]byte, 34))
	for i := 0; i < len(blocks); i += 2 {
		kind := blocks[i][0]
		if i == len(blocks)-2 {
			kind |= 0x80
		}
		n := len(blocks[i+1])
		b = cat(b, []byte{kind, byte(n >> 16), byte(n >> 8), byte(n)}, blocks[i+1])
	}
	return append(b, 0xFF, 0xF8)
}

// TestFLAC reads a FLAC file's Vorbis comments, of keys in any case and
// given twice, and its PICTURE block; its album artists, its artists,
// are not told apart.
func TestFLAC(t *testing.T) {
	b := flacFile(
		[]byte{4}, vcomments("title=Glass Rivers", "ARTIST=Ada Field", "Artist=Orin Vale", "ALBUM=Field Notes",
			"ALBUMARTIST=Ada Field", "albumartist=Orin Vale", "TRACKNUMBER=05", "DISCNUMBER=1/1", "DATE=2017-09", "COMMENT=x"),
		[]byte{6}, flacPictureBlock(0, "image/jpeg", jpeg),
		[]byte{6}, flacPictureBlock(3, "image/png", png),
	)
	check(t, read(t, b), Tags{
		Title: "Glass Rivers", Artists: []string{"Ada Field", "Orin Vale"}, Album: "Field Notes", Track: 5, Disc: 1, Year: 2017,
		Picture: &Picture{MIME: "image/png", Data: png, front: true},
	})
	// An ID3v2 tag before it, as some programs write.
	v2 := id3(3, 0, frame3("TALB", 0, append([]byte{0}, "From the ID3 tag"...)))
	if got := read(t, cat(v2, b)); got.Album != "From the ID3 tag" || got.Title != "Glass Rivers" {
		t.Errorf("after an ID3v2 tag: %+v", got)
	}
}

// oggCRC is the checksum of Ogg's pages.
func oggCRC(b []byte) uint32 {
	var c uint32
	for _, v := range b {
		c ^= uint32(v) << 24
		for range 8 {
			if c&0x80000000 != 0 {
				c = c<<1 ^ 0x04C11DB7
			} else {
				c <<= 1
			}
		}
	}
	return c
}

// oggPage is an Ogg page of one packet, of fewer than 255·255 bytes.
func oggPage(flags byte, seq int, packet []byte) []byte {
	var lacing []byte
	n := len(packet)
	for ; n >= 255; n -= 255 {
		lacing = append(lacing, 255)
	}
	lacing = append(lacing, byte(n))
	h := cat([]byte("OggS"), []byte{0, flags}, make([]byte, 8), le32(7), le32(seq), make([]byte, 4), []byte{byte(len(lacing))}, lacing, packet)
	binary.LittleEndian.PutUint32(h[22:], oggCRC(h))
	return h
}

// TestOgg reads the comment headers of Opus and Vorbis files, with a
// cover as METADATA_BLOCK_PICTURE, and in the older COVERART.
func TestOgg(t *testing.T) {
	pic := base64.StdEncoding.EncodeToString(flacPictureBlock(3, "image/png", png))
	opus := cat(
		oggPage(2, 0, cat([]byte("OpusHead"), []byte{1, 2, 0x38, 1, 0x80, 0xBB, 0, 0, 0, 0, 0})),
		oggPage(0, 1, cat([]byte("OpusTags"), vcomments("TITLE=Small Hours", "ARTIST=Mira Holt", "METADATA_BLOCK_PICTURE="+pic))),
	)
	check(t, read(t, opus), Tags{Title: "Small Hours", Artists: []string{"Mira Holt"}, Picture: &Picture{MIME: "image/png", Data: png, front: true}})
	vorbis := cat(
		oggPage(2, 0, cat([]byte("\x01vorbis"), make([]byte, 23))),
		oggPage(0, 1, cat([]byte("\x03vorbis"), vcomments("TITLE=Copper", "COVERART="+base64.StdEncoding.EncodeToString(jpeg), "COVERARTMIME=image/jpeg"), []byte{1})),
	)
	check(t, read(t, vorbis), Tags{Title: "Copper", Picture: &Picture{MIME: "image/jpeg", Data: jpeg}})
}

// mbox is an MP4 box.
func mbox(kind string, parts ...[]byte) []byte {
	body := cat(parts...)
	return cat(be32(8+len(body)), []byte(kind), body)
}

// item is an item of an ilst, of a data box of a type.
func item(kind string, typ int, value []byte) []byte {
	return mbox(kind, mbox("data", be32(typ), make([]byte, 4), value))
}

// TestMP4 reads the items of an MP4 file's ilst, its moov after its
// audio, as some programs write it.
func TestMP4(t *testing.T) {
	ilst := mbox("ilst",
		item("\xa9nam", 1, []byte("Paper Lanterns")),
		item("\xa9ART", 1, []byte("Juno Reyes")),
		item("\xa9alb", 1, []byte("Kites")),
		item("aART", 1, []byte("Juno Reyes & Friends")),
		item("\xa9day", 1, []byte("2012-03-01T00:00:00Z")),
		item("trkn", 0, []byte{0, 0, 0, 9, 0, 12, 0, 0}),
		item("disk", 0, []byte{0, 0, 0, 2, 0, 2}),
		item("covr", 13, jpeg),
	)
	meta := mbox("meta", make([]byte, 4), mbox("hdlr", make([]byte, 25)), ilst)
	file := cat(mbox("ftyp", []byte("M4A "), be32(0)), mbox("mdat", make([]byte, 300)), mbox("moov", mbox("trak", make([]byte, 40)), mbox("udta", meta)))
	check(t, read(t, file), Tags{
		Title: "Paper Lanterns", Artists: []string{"Juno Reyes"}, Album: "Kites", AlbumArtists: []string{"Juno Reyes & Friends"},
		Track: 9, Disc: 2, Year: 2012, Picture: &Picture{MIME: "image/jpeg", Data: jpeg, front: true},
	})
}

// TestWAV reads a WAV file's INFO list, and an ID3v2 tag in a chunk of
// its own, which comes first.
func TestWAV(t *testing.T) {
	sub := func(id, v string) []byte {
		b := cat([]byte(id), le32(len(v)+1), []byte(v), []byte{0})
		if len(b)%2 != 0 {
			b = append(b, 0)
		}
		return b
	}
	info := cat([]byte("INFO"), sub("INAM", "Field Recording"), sub("IART", "Orin Vale"), sub("ICRD", "2003"))
	v2 := id3(3, 0, frame3("TALB", 0, append([]byte{0}, "Outdoors"...)))
	chunk := func(id string, b []byte) []byte {
		c := cat([]byte(id), le32(len(b)), b)
		if len(b)%2 != 0 {
			c = append(c, 0)
		}
		return c
	}
	body := cat([]byte("WAVE"), chunk("fmt ", make([]byte, 16)), chunk("data", make([]byte, 7)), chunk("LIST", info), chunk("id3 ", v2))
	check(t, read(t, cat([]byte("RIFF"), le32(len(body)), body)), Tags{
		Title: "Field Recording", Artists: []string{"Orin Vale"}, Album: "Outdoors", Year: 2003,
	})
}

// TestBroken reads files of broken tags as they are: what can be read of
// them, and never more than they hold.
func TestBroken(t *testing.T) {
	good := id3(4, 0, frame4("TIT2", 0, append([]byte{3}, "Kept"...)))
	for _, c := range []struct {
		name string
		data []byte
		want Tags
	}{
		{"a frame past the tag", cat(id3(4, 0, cat(frame4("TIT2", 0, append([]byte{3}, "Kept"...)), []byte("TPE1\x00\x00\x7f\x7f\x00\x00x"))), mp3), Tags{Title: "Kept"}},
		{"a tag past the file", cat(good[:6], safe(1<<20), good[10:]), Tags{Title: "Kept"}},
		{"a picture of no type", cat(id3(4, 0, frame4("APIC", 0, []byte{3, 0, 3, 0, 1, 2, 3})), mp3), Tags{}},
		{"comments past their block", flacFile([]byte{4}, cat(le32(1000), []byte("vendor"))), Tags{}},
		{"a picture past its block", flacFile([]byte{6}, cat(be32(3), be32(9), []byte("image/png"), be32(0), make([]byte, 16), be32(1<<30))), Tags{}},
		{"an MP4 box past its parent", cat(mbox("ftyp", []byte("M4A ")), be32(1<<30), []byte("moov")), Tags{}},
		{"numbers that aren't", flacFile([]byte{4}, vcomments("TRACKNUMBER=one", "DISCNUMBER=-1", "DATE=soon")), Tags{}},
	} {
		t.Run(c.name, func(t *testing.T) { check(t, read(t, c.data), c.want) })
	}
	for _, b := range [][]byte{nil, []byte("hello, I am not music"), []byte("\x1a\x45\xdf\xa3 a WebM file")} {
		if _, err := Read(bytes.NewReader(b)); err != ErrUnknown {
			t.Errorf("%q: %v, want ErrUnknown", b, err)
		}
	}
}

// FuzzRead reads any bytes as a file's tags: it never panics, and reads
// valid UTF-8, of numbers in range, and a picture of an image's type.
func FuzzRead(f *testing.F) {
	f.Add(cat(id3(4, 0, frame4("TIT2", 0, append([]byte{3}, "Kept"...))), mp3))
	f.Add(cat(id3(3, 0x80, frame3("APIC", 0, cat([]byte{1}, []byte("image/png\x00"), []byte{3}, utf16le("d"), []byte{0, 0}, png))), mp3))
	f.Add(cat(id3(2, 0, frame2("TT2", append([]byte{0}, "x"...))), mp3))
	f.Add(cat(mp3, id3v1("a", "b", "c", "2000", 1)))
	f.Add(flacFile([]byte{4}, vcomments("TITLE=x", "ARTIST=y"), []byte{6}, flacPictureBlock(3, "image/png", png)))
	f.Add(cat(oggPage(2, 0, cat([]byte("OpusHead"), make([]byte, 11))), oggPage(0, 1, cat([]byte("OpusTags"), vcomments("TITLE=x")))))
	f.Add(cat(mbox("ftyp", []byte("M4A ")), mbox("moov", mbox("udta", mbox("meta", make([]byte, 4), mbox("ilst", item("\xa9nam", 1, []byte("x"))))))))
	f.Add(cat([]byte("RIFF"), le32(30), []byte("WAVE"), []byte("LIST"), le32(18), []byte("INFOINAM"), le32(5), []byte("abcd\x00\x00")))
	f.Fuzz(func(t *testing.T, b []byte) {
		got, err := Read(bytes.NewReader(b))
		if err != nil {
			if err != ErrUnknown {
				t.Fatalf("an error of no reader: %v", err)
			}
			return
		}
		for _, s := range append(append([]string{got.Title, got.Album}, got.Artists...), got.AlbumArtists...) {
			if strings.ToValidUTF8(s, "") != s || strings.TrimSpace(s) != s || s == "" && len(got.Artists) > 0 && s == got.Artists[0] {
				t.Fatalf("text %q", s)
			}
		}
		if got.Track < 0 || got.Track > 9999 || got.Disc < 0 || got.Disc > 9999 || got.Year != 0 && (got.Year < 1000 || got.Year > 9999) {
			t.Fatalf("numbers %+v", got)
		}
		if p := got.Picture; p != nil && (!strings.HasPrefix(p.MIME, "image/") || len(p.Data) == 0) {
			t.Fatalf("a picture of %q, %d bytes", p.MIME, len(p.Data))
		}
	})
}
