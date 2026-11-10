package tagwrite

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/fzn0x/mixmusic/internal/tags"
)

// An MP3 file's tags are an ID3v2 tag at its start, and maybe an ID3v1
// tag of 128 bytes at its end. The new ID3v2 tag is of the version of the
// old one, 2.3 or 2.4, or 2.4 when the file had none, or one of 2.2,
// whose frames it takes as 2.4 names them. Its frames are the new ones,
// then those of the old tag the write doesn't change, as they were, then
// padding. An ID3v1 tag takes the new title, artists, album, year, track
// and genre as far as it can hold them, so that it never tells what the
// ID3v2 tag no longer does.

// id3Padding is the padding after the frames of a new tag, which lets
// programs that change tags in place change them without moving the
// audio.
const id3Padding = 1024

// The frames of each field, of ID3v2.3 and 2.4.
var id3Fields = []struct {
	field Fields
	ids   []string
}{
	{Title, []string{"TIT2"}},
	{Artists, []string{"TPE1"}},
	{Album, []string{"TALB"}},
	{AlbumArtists, []string{"TPE2"}},
	{Track, []string{"TRCK"}},
	{Disc, []string{"TPOS"}},
	{Year, []string{"TYER", "TDRC", "TDAT", "TIME", "TRDA"}},
	{Genre, []string{"TCON"}},
	{ISRC, []string{"TSRC"}},
	{Cover, []string{"APIC"}},
}

// id3v22 are the frames of ID3v2.2 a tag of 2.4 takes, by their names in
// 2.4; those it doesn't name go.
var id3v22 = map[string]string{
	"TT1": "TIT1", "TT2": "TIT2", "TT3": "TIT3", "TP1": "TPE1", "TP2": "TPE2", "TP3": "TPE3", "TP4": "TPE4",
	"TCM": "TCOM", "TXT": "TEXT", "TLA": "TLAN", "TCO": "TCON", "TAL": "TALB", "TPA": "TPOS", "TRK": "TRCK",
	"TRC": "TSRC", "TYE": "TDRC", "TOR": "TDOR", "TBP": "TBPM", "TMT": "TMED", "TFT": "TFLT", "TCR": "TCOP",
	"TPB": "TPUB", "TEN": "TENC", "TSS": "TSSE", "TOF": "TOFN", "TLE": "TLEN", "TKE": "TKEY", "TOT": "TOAL",
	"TOA": "TOPE", "TOL": "TOLY", "TXX": "TXXX", "COM": "COMM", "ULT": "USLT", "UFI": "UFID", "WXX": "WXXX",
	"WAR": "WOAR", "WAS": "WOAS", "WAF": "WOAF", "WCM": "WCOM", "WCP": "WCOP", "WPB": "WPUB", "PIC": "APIC",
}

// id3Tag is an ID3v2 tag as read: its version, and its frames.
type id3Tag struct {
	version byte
	frames  []id3Frame
}

// id3Frame is a frame of a tag, as the tag of its version holds it, but
// for unsynchronisation, which is undone.
type id3Frame struct {
	id    string
	flags uint16
	data  []byte
}

// id3End returns where the ID3v2 tag at the file's start ends, its footer
// included: 0 without one, and false for one that ends past the file.
func (f *file) id3End() (int64, bool) {
	var h [10]byte
	if n, _ := f.readAt(0, h[:]); n < 10 || string(h[:3]) != "ID3" {
		return 0, true
	}
	if h[6]|h[7]|h[8]|h[9] >= 0x80 {
		return 0, false
	}
	end := 10 + syncsafe(h[6:10])
	if h[5]&0x10 != 0 {
		end += 10 // a footer
	}
	return end, end <= f.size
}

func (f *file) writeMP3(w io.Writer, t Tags, fields Fields) error {
	end, ok := f.id3End()
	if !ok {
		return fmt.Errorf("%w: its ID3 tag is longer than the file", ErrFormat)
	}
	var old id3Tag
	if end > 0 {
		var err error
		if old, err = f.readID3(); err != nil {
			return err
		}
	}
	version := byte(4)
	if old.version == 3 {
		version = 3
	}
	var body bytes.Buffer
	for _, fr := range newID3Frames(version, t, fields, old) {
		writeFrame(&body, version, fr)
	}
	for _, fr := range old.kept(version, fields) {
		writeFrame(&body, version, fr)
	}
	body.Write(make([]byte, id3Padding))
	if body.Len() >= 1<<28 {
		return fmt.Errorf("%w: its tags would be larger than an ID3 tag holds", ErrFormat)
	}
	head := []byte{'I', 'D', '3', version, 0, 0}
	head = append(head, safe(body.Len())...)
	if _, err := w.Write(head); err != nil {
		return err
	}
	if _, err := w.Write(body.Bytes()); err != nil {
		return err
	}
	audioEnd := f.size
	var v1 [128]byte
	hasV1 := false
	if f.size-end >= 128 {
		if _, err := f.readAt(f.size-128, v1[:]); err == nil && string(v1[:3]) == "TAG" {
			hasV1, audioEnd = true, f.size-128
		}
	}
	if err := f.copyTo(w, end, audioEnd); err != nil {
		return err
	}
	if hasV1 {
		v1 = updateV1(v1, t, fields)
		_, err := w.Write(v1[:])
		return err
	}
	return nil
}

// readID3 reads the ID3v2 tag at the file's start: of a version it
// doesn't know, or compressed whole, as 2.2 lets a tag be, none of its
// frames.
func (f *file) readID3() (id3Tag, error) {
	var h [10]byte
	if _, err := f.readAt(0, h[:]); err != nil {
		return id3Tag{}, err
	}
	version, flags := h[3], h[5]
	b, err := f.bytesAt(10, syncsafe(h[6:10]))
	if err != nil {
		return id3Tag{}, err
	}
	tag := id3Tag{version: version}
	if version < 2 || version > 4 || version == 2 && flags&0x40 != 0 {
		return id3Tag{}, nil
	}
	unsynced := flags&0x80 != 0
	if unsynced && version < 4 {
		b = resync(b)
	}
	if flags&0x40 != 0 {
		// An extended header, which goes.
		if len(b) < 4 {
			return tag, nil
		}
		n := int(binary.BigEndian.Uint32(b)) + 4
		if version == 4 {
			n = int(syncsafe(b[:4]))
		}
		if n < 0 || n > len(b) {
			return tag, nil
		}
		b = b[n:]
	}
	idLen, headLen := 4, 10
	if version == 2 {
		idLen, headLen = 3, 6
	}
	for len(b) >= headLen && b[0] != 0 {
		fr := id3Frame{id: string(b[:idLen])}
		var size int
		switch version {
		case 2:
			size = int(b[3])<<16 | int(b[4])<<8 | int(b[5])
		case 3:
			size = int(binary.BigEndian.Uint32(b[4:]))
			fr.flags = binary.BigEndian.Uint16(b[8:])
		default:
			size = int(syncsafe(b[4:8]))
			fr.flags = binary.BigEndian.Uint16(b[8:])
		}
		b = b[headLen:]
		if size < 0 || size > len(b) {
			break
		}
		fr.data = b[:size:size]
		b = b[size:]
		if version == 4 && (unsynced || fr.flags&0x0002 != 0) {
			// Unsynchronised in 2.4 frame by frame, which the new tag
			// isn't.
			fr.data, fr.flags = resync(fr.data), fr.flags&^0x0002
		}
		tag.frames = append(tag.frames, fr)
	}
	return tag, nil
}

// kept are the frames of the old tag the new one of version keeps: all
// but those of the fields written, and those that ask to go once the tag
// changes. Those of 2.2 take their names in 2.4, or go.
func (old id3Tag) kept(version byte, fields Fields) []id3Frame {
	drop := map[string]bool{}
	for _, f := range id3Fields {
		if fields&f.field != 0 {
			for _, id := range f.ids {
				drop[id] = true
			}
		}
	}
	var out []id3Frame
	for _, fr := range old.frames {
		switch old.version {
		case 2:
			id, ok := id3v22[fr.id]
			if !ok {
				continue
			}
			if fr.id == "PIC" {
				fr.data = apicOf22(fr.data)
			}
			fr.id = id
		case 3:
			if fr.flags&0x8000 != 0 {
				continue // tag alter preservation: discard the frame
			}
		case 4:
			if fr.flags&0x4000 != 0 {
				continue
			}
		}
		if !drop[fr.id] && fr.data != nil {
			out = append(out, fr)
		}
	}
	return out
}

// value is the first value of the text frame of id the tag holds, "" for
// none.
func (old id3Tag) value(id string) string {
	for _, fr := range old.frames {
		name := fr.id
		if old.version == 2 {
			name = id3v22[name]
		}
		if name == id && len(fr.data) > 0 {
			s := decodeID3(fr.data[0], fr.data[1:])
			s, _, _ = strings.Cut(s, "\x00")
			return s
		}
	}
	return ""
}

// pictured reports whether the tag holds a picture whose image is data.
func (old id3Tag) pictured(data []byte) bool {
	for _, fr := range old.frames {
		if (fr.id == "APIC" || fr.id == "PIC") && bytes.HasSuffix(fr.data, data) {
			return true
		}
	}
	return false
}

// newID3Frames are the frames of the fields written of t, in a tag of
// version, in the order of id3Fields.
func newID3Frames(version byte, t Tags, fields Fields, old id3Tag) []id3Frame {
	var out []id3Frame
	text := func(id string, values ...string) {
		if len(values) == 0 || values[0] == "" {
			return
		}
		out = append(out, id3Frame{id: id, data: textData(version, values)})
	}
	if fields&Title != 0 {
		text("TIT2", t.Title)
	}
	if fields&Artists != 0 {
		text("TPE1", t.Artists...)
	}
	if fields&Album != 0 {
		text("TALB", t.Album)
	}
	if fields&AlbumArtists != 0 {
		text("TPE2", t.AlbumArtists...)
	}
	if fields&Track != 0 && t.Track > 0 {
		text("TRCK", withCount(t.Track, old.value("TRCK")))
	}
	if fields&Disc != 0 && t.Disc > 0 {
		text("TPOS", withCount(t.Disc, old.value("TPOS")))
	}
	if fields&Year != 0 && t.Year > 0 {
		id := "TDRC"
		if version == 3 {
			id = "TYER"
		}
		text(id, strconv.Itoa(t.Year))
	}
	if fields&Genre != 0 {
		g := t.Genre
		if strings.HasPrefix(g, "(") {
			g = "(" + g // a genre's own "(", which would read as a number's
		}
		text("TCON", g)
	}
	if fields&ISRC != 0 {
		text("TSRC", t.ISRC)
	}
	if fields&Cover != 0 && t.Cover != nil {
		var b bytes.Buffer
		b.WriteByte(0) // ISO-8859-1, of the empty description
		b.WriteString(t.Cover.MIME)
		b.WriteByte(0)
		b.WriteByte(3) // the front cover
		b.WriteByte(0) // the description, empty
		b.Write(t.Cover.Data)
		out = append(out, id3Frame{id: "APIC", data: b.Bytes()})
	}
	return out
}

// withCount is the number n as a track's or a disc's frame holds it, with
// the count the old one held, as "4/12".
func withCount(n int, old string) string {
	s := strconv.Itoa(n)
	if _, count, ok := strings.Cut(old, "/"); ok && strings.TrimSpace(count) != "" {
		s += "/" + strings.TrimSpace(count)
	}
	return s
}

// textData is the data of a text frame of values: of 2.4 in UTF-8, of 2.3
// in ISO-8859-1 when it holds them, else in UTF-16, each with its byte
// order mark; values apart by a NUL.
func textData(version byte, values []string) []byte {
	if version == 4 {
		return append([]byte{3}, strings.Join(values, "\x00")...)
	}
	if b, ok := latin1(strings.Join(values, "\x00")); ok {
		return append([]byte{0}, b...)
	}
	b := []byte{1}
	for i, v := range values {
		if i > 0 {
			b = append(b, 0, 0)
		}
		b = append(b, 0xFF, 0xFE)
		for _, u := range utf16.Encode([]rune(v)) {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
	}
	return b
}

// writeFrame writes a frame of a tag of version.
func writeFrame(w *bytes.Buffer, version byte, fr id3Frame) {
	w.WriteString(fr.id)
	if version == 4 {
		w.Write(safe(len(fr.data)))
	} else {
		w.Write(binary.BigEndian.AppendUint32(nil, uint32(len(fr.data))))
	}
	w.Write(binary.BigEndian.AppendUint16(nil, fr.flags))
	w.Write(fr.data)
}

// apicOf22 is a picture of 2.2, of a format as "JPG", as one of 2.3 and
// 2.4, of a MIME type.
func apicOf22(data []byte) []byte {
	if len(data) < 5 {
		return nil
	}
	mime := "image/" + strings.ToLower(string(data[1:4]))
	if mime == "image/jpg" {
		mime = "image/jpeg"
	}
	out := []byte{data[0]}
	out = append(out, mime...)
	out = append(out, 0)
	return append(out, data[4:]...)
}

// updateV1 is an ID3v1 tag with the fields written of t, as far as it
// holds them: text of ISO-8859-1, up to 30 bytes, else none; the track of
// ID3v1.1, and the genre by its number.
func updateV1(b [128]byte, t Tags, fields Fields) [128]byte {
	put := func(at, n int, s string) {
		v, ok := latin1(s)
		if !ok {
			v = nil
		}
		field := b[at : at+n]
		clear(field)
		copy(field, v)
	}
	if fields&Title != 0 {
		put(3, 30, t.Title)
	}
	if fields&Artists != 0 {
		put(33, 30, strings.Join(t.Artists, ", "))
	}
	if fields&Album != 0 {
		put(63, 30, t.Album)
	}
	if fields&Year != 0 {
		y := ""
		if t.Year > 0 {
			y = strconv.Itoa(t.Year)
		}
		put(93, 4, y)
	}
	if fields&Track != 0 {
		b[125], b[126] = 0, 0
		if t.Track > 0 && t.Track < 256 {
			b[126] = byte(t.Track)
		}
	}
	if fields&Genre != 0 {
		b[127] = 255
		if n, ok := tags.GenreNumber(t.Genre); ok && n < 255 {
			b[127] = byte(n)
		}
	}
	return b
}

// latin1 encodes s in ISO-8859-1: false when it holds a character that has
// none.
func latin1(s string) ([]byte, bool) {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			return nil, false
		}
		b = append(b, byte(r))
	}
	return b, true
}

// decodeID3 decodes text of an ID3v2 encoding: 0 ISO-8859-1, 1 UTF-16
// with a byte order mark, 2 UTF-16BE, 3 UTF-8.
func decodeID3(enc byte, b []byte) string {
	switch enc {
	case 1, 2:
		big := enc == 2
		u := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			v := uint16(b[i]) | uint16(b[i+1])<<8
			if big {
				v = uint16(b[i])<<8 | uint16(b[i+1])
			}
			switch v {
			case 0xFEFF:
				continue
			case 0xFFFE:
				big = !big
				continue
			}
			u = append(u, v)
		}
		return string(utf16.Decode(u))
	case 3:
		return string(b)
	}
	rs := make([]rune, len(b))
	for i, c := range b {
		rs[i] = rune(c)
	}
	return string(rs)
}

// syncsafe reads a number of 4 bytes of 7 bits each, and safe writes one.
func syncsafe(b []byte) int64 {
	return int64(b[0])<<21 | int64(b[1])<<14 | int64(b[2])<<7 | int64(b[3])
}

func safe(n int) []byte {
	return []byte{byte(n >> 21 & 127), byte(n >> 14 & 127), byte(n >> 7 & 127), byte(n & 127)}
}

// resync undoes unsynchronisation: the zero after each 0xFF goes.
func resync(b []byte) []byte {
	if !bytes.Contains(b, []byte{0xFF, 0x00}) {
		return b
	}
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		out = append(out, b[i])
		if b[i] == 0xFF && i+1 < len(b) && b[i+1] == 0 {
			i++
		}
	}
	return out
}
