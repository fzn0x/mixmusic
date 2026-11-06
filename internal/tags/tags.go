// Package tags reads the tags of music files kept on a computer: the
// title, artists, album, album artists, track and disc numbers, year and
// cover a file tells of itself. It reads ID3v2 and ID3v1 tags, as MP3
// files have them, and WAV files in a chunk, the items of an MP4 file's
// ilst, Vorbis comments and pictures, as FLAC, Opus and Vorbis files have
// them, and the INFO list of a WAV file.
//
// Files on disk can be broken, or made to break what reads them: the
// sizes it reads are bounded, so that a corrupt one cannot allocate much
// memory, and malformed tags are passed over, never panics. A file
// without tags, or of tags it can't read, has none.
package tags

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrUnknown is returned by Read for a file of a kind it doesn't know.
var ErrUnknown = errors.New("tags: not a music file of a kind with tags")

const (
	// maxPicture bounds the bytes of a cover: larger ones are passed over.
	maxPicture = 16 << 20
	// maxText bounds the bytes of a tag's text.
	maxText = 64 << 10
	// maxTag bounds the bytes of a tag read whole, as an ID3v2 tag or a
	// block of Vorbis comments, a cover in it included.
	maxTag = maxPicture + 1<<20
)

// Tags are what a file tells of itself. A field it doesn't tell is empty,
// or 0.
type Tags struct {
	Title   string
	Artists []string
	Album   string
	// AlbumArtists are the artists of the album, when they differ from
	// the track's, as a compilation's.
	AlbumArtists []string
	// Track and Disc are the track's numbers on its album, from 1.
	Track, Disc int
	Year        int
	// Picture is the cover, the front cover when the file tells which.
	Picture *Picture
}

// Picture is an image a file holds.
type Picture struct {
	// MIME is its type, as image/jpeg or image/png.
	MIME string
	Data []byte
	// front is set for the front cover, which a file's other pictures
	// give way to.
	front bool
}

// Read reads the tags of a file, of the kind its first bytes tell: ID3v2
// at its start, as MP3 files and some FLAC files have, MP4, FLAC, Ogg or
// WAV, and ID3v1 at its end. Its errors are those of r, or ErrUnknown.
func Read(r io.ReadSeeker) (Tags, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return Tags{}, err
	}
	f := &file{r: r, size: size}
	var head [12]byte
	n, err := f.readAt(0, head[:])
	if err != nil && n < 4 {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return Tags{}, ErrUnknown
		}
		return Tags{}, err
	}
	var t Tags
	at := int64(0)
	known := false
	if string(head[:3]) == "ID3" {
		end, err := f.id3v2(0, &t)
		if err != nil {
			return Tags{}, err
		}
		at, known = end, true
		n, _ = f.readAt(at, head[:])
	}
	switch {
	case n >= 4 && string(head[:4]) == "fLaC":
		err = f.flac(at+4, &t)
	case n >= 4 && string(head[:4]) == "OggS":
		err = f.ogg(&t)
	case n >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		err = f.wav(&t)
	case n >= 8 && mp4Box(string(head[4:8])):
		err = f.mp4(&t)
	case known, n >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		// MP3, which may have an ID3v1 tag at its end too.
		err = f.id3v1(&t)
	default:
		return Tags{}, ErrUnknown
	}
	if err != nil {
		return Tags{}, err
	}
	t.clean()
	return t, nil
}

// mp4Box reports whether an MP4 file may start with a box of this type.
func mp4Box(kind string) bool {
	switch kind {
	case "ftyp", "moov", "mdat", "free", "skip", "wide", "pdin", "uuid":
		return true
	}
	return false
}

// file reads the parts of a file its tags are in.
type file struct {
	r    io.ReadSeeker
	size int64
}

// readAt reads len(b) bytes at offset off.
func (f *file) readAt(off int64, b []byte) (int, error) {
	if _, err := f.r.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(f.r, b)
}

// bytesAt reads n bytes at offset off, up to the end of the file: none
// when n is more than limit.
func (f *file) bytesAt(off, n, limit int64) ([]byte, error) {
	n = min(n, f.size-off)
	if n <= 0 || n > limit {
		return nil, nil
	}
	b := make([]byte, n)
	k, err := f.readAt(off, b)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	return b[:k], err
}

// set fills the fields of t that are still empty with the values a tag
// gives for key, a field's name: title, artist, album, albumartist, track,
// disc or year. A key that names no field changes nothing.
func (t *Tags) set(key string, values ...string) {
	var vs []string
	for _, v := range values {
		if v = text(v); v != "" {
			vs = append(vs, v)
		}
	}
	if len(vs) == 0 {
		return
	}
	switch key {
	case "title":
		if t.Title == "" {
			t.Title = vs[0]
		}
	case "artist":
		if len(t.Artists) == 0 {
			t.Artists = vs
		}
	case "album":
		if t.Album == "" {
			t.Album = vs[0]
		}
	case "albumartist":
		if len(t.AlbumArtists) == 0 {
			t.AlbumArtists = vs
		}
	case "track":
		if t.Track == 0 {
			t.Track = number(vs[0])
		}
	case "disc":
		if t.Disc == 0 {
			t.Disc = number(vs[0])
		}
	case "year":
		if t.Year == 0 {
			t.Year = year(vs[0])
		}
	}
}

// setPicture keeps p as the cover, unless the cover found before is the
// front cover, or p is too large.
func (t *Tags) setPicture(p *Picture) {
	if p == nil || len(p.Data) == 0 || len(p.Data) > maxPicture {
		return
	}
	if p.MIME == "" || !strings.HasPrefix(p.MIME, "image/") {
		p.MIME = sniffImage(p.Data)
	}
	if p.MIME == "" {
		return
	}
	if t.Picture == nil || p.front && !t.Picture.front {
		t.Picture = p
	}
}

// clean drops the album artists that are the track's artists.
func (t *Tags) clean() {
	if len(t.AlbumArtists) == len(t.Artists) {
		same := true
		for i := range t.Artists {
			if !strings.EqualFold(t.Artists[i], t.AlbumArtists[i]) {
				same = false
			}
		}
		if same {
			t.AlbumArtists = nil
		}
	}
}

// text is s as a tag's value: valid UTF-8, without control characters,
// trimmed, and cut at maxText bytes.
func text(s string) string {
	if len(s) > maxText {
		s = s[:maxText]
	}
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// number reads a track's or a disc's number, as "3" or "3/12": 0 when it
// is none.
func number(s string) int {
	s, _, _ = strings.Cut(s, "/")
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 9999 {
		return 0
	}
	return n
}

// year reads the year of a date, as "2024", "2024-05-01" or
// "2024-05-01T10:00:00Z": 0 when it is none.
func year(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	n, err := strconv.Atoi(s[:4])
	if err != nil || n < 1000 || n > 9999 {
		return 0
	}
	return n
}

// sniffImage tells the type of an image by its first bytes: "" for one
// that isn't JPEG, PNG, GIF or WebP.
func sniffImage(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\xFF\xD8\xFF")):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}

// latin1 decodes ISO-8859-1.
func latin1(b []byte) string {
	if !bytes.ContainsFunc(b, func(r rune) bool { return r >= utf8.RuneSelf }) {
		return string(b) // ASCII
	}
	rs := make([]rune, len(b))
	for i, c := range b {
		rs[i] = rune(c)
	}
	return string(rs)
}

// utf8OrLatin1 decodes text that may be UTF-8, as most programs write
// now, or ISO-8859-1, as the older did.
func utf8OrLatin1(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return latin1(b)
}
