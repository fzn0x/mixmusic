package tags

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"strings"
	"unicode/utf16"
)

// The frames of ID3v2.3 and 2.4 the tags take, and those of 2.2.
var id3Frames = map[string]string{
	"TIT2": "title", "TPE1": "artist", "TALB": "album", "TPE2": "albumartist",
	"TRCK": "track", "TPOS": "disc", "TYER": "year", "TDRC": "year", "TORY": "year", "TDOR": "year",
	"TT2": "title", "TP1": "artist", "TAL": "album", "TP2": "albumartist",
	"TRK": "track", "TPA": "disc", "TYE": "year", "TOR": "year",
}

// id3v2 reads the ID3v2 tag at offset at into t, of version 2.2, 2.3 or
// 2.4, and returns the offset after it. A tag it can't read is passed
// over.
func (f *file) id3v2(at int64, t *Tags) (int64, error) {
	var h [10]byte
	if _, err := f.readAt(at, h[:]); err != nil {
		return at, nil
	}
	if string(h[:3]) != "ID3" || h[6]|h[7]|h[8]|h[9] >= 0x80 {
		return at, nil
	}
	size := syncsafe(h[6:10])
	end := at + 10 + size
	if h[5]&0x10 != 0 {
		end += 10 // a footer
	}
	version, flags := h[3], h[5]
	if version < 2 || version > 4 {
		return end, nil
	}
	b, err := f.bytesAt(at+10, size, maxTag)
	if err != nil || b == nil {
		return end, err
	}
	if flags&0x80 != 0 && version < 4 {
		// The whole tag is unsynchronised; in 2.4 each frame says so.
		b = resync(b)
	}
	if flags&0x40 != 0 && version > 2 {
		// An extended header.
		if len(b) < 4 {
			return end, nil
		}
		n := int(binary.BigEndian.Uint32(b)) + 4
		if version == 4 {
			n = int(syncsafe(b[:4]))
		}
		if n < 0 || n > len(b) {
			return end, nil
		}
		b = b[n:]
	}
	readFrames(b, version, flags&0x80 != 0, t)
	return end, nil
}

// syncsafe reads a number of 4 bytes of 7 bits each.
func syncsafe(b []byte) int64 {
	return int64(b[0])<<21 | int64(b[1])<<14 | int64(b[2])<<7 | int64(b[3])
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

// readFrames reads the frames of a tag's body into t.
func readFrames(b []byte, version byte, unsynced bool, t *Tags) {
	idLen, headLen := 4, 10
	if version == 2 {
		idLen, headLen = 3, 6
	}
	for len(b) >= headLen {
		id := string(b[:idLen])
		if b[0] == 0 {
			return // the padding
		}
		var size int
		var flags uint16
		switch version {
		case 2:
			size = int(b[3])<<16 | int(b[4])<<8 | int(b[5])
		case 3:
			size = int(binary.BigEndian.Uint32(b[4:]))
			flags = binary.BigEndian.Uint16(b[8:])
		default:
			size = int(syncsafe(b[4:8]))
			flags = binary.BigEndian.Uint16(b[8:])
		}
		b = b[headLen:]
		if size < 0 || size > len(b) {
			return
		}
		data := b[:size]
		b = b[size:]
		data, ok := frameData(data, version, flags, unsynced)
		if !ok {
			continue
		}
		switch {
		case id == "APIC":
			t.setPicture(apic(data))
		case id == "PIC":
			t.setPicture(pic(data))
		case id3Frames[id] != "":
			t.set(id3Frames[id], textFrame(data, version)...)
		}
	}
}

// frameData returns the data of a frame, as its flags tell: of 2.3 and
// 2.4, decompressed, and of 2.4, resynchronised. An encrypted frame is
// passed over.
func frameData(data []byte, version byte, flags uint16, unsynced bool) ([]byte, bool) {
	var compressed, encrypted, grouped, resyncd, length bool
	switch version {
	case 3:
		compressed, encrypted, grouped = flags&0x0080 != 0, flags&0x0040 != 0, flags&0x0020 != 0
		length = compressed
	case 4:
		grouped, compressed, encrypted = flags&0x0040 != 0, flags&0x0008 != 0, flags&0x0004 != 0
		resyncd, length = flags&0x0002 != 0 || unsynced, flags&0x0001 != 0
	}
	if encrypted {
		return nil, false
	}
	if version == 3 {
		// 2.3 puts the decompressed size before the group.
		if length {
			if len(data) < 4 {
				return nil, false
			}
			data = data[4:]
		}
		if grouped {
			if len(data) < 1 {
				return nil, false
			}
			data = data[1:]
		}
	} else {
		if grouped {
			if len(data) < 1 {
				return nil, false
			}
			data = data[1:]
		}
		if length {
			if len(data) < 4 {
				return nil, false
			}
			data = data[4:]
		}
	}
	if resyncd {
		data = resync(data)
	}
	if compressed {
		z, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, false
		}
		out, err := io.ReadAll(io.LimitReader(z, maxTag))
		if err != nil {
			return nil, false
		}
		data = out
	}
	return data, true
}

// textFrame reads the values of a text frame: one, or of 2.4 any number,
// between NULs.
func textFrame(data []byte, version byte) []string {
	if len(data) < 1 {
		return nil
	}
	s := decode(data[0], data[1:])
	values := strings.Split(strings.TrimRight(s, "\x00"), "\x00")
	if version < 4 {
		values = values[:1]
	}
	return values
}

// decode decodes text of an ID3v2 encoding: 0 ISO-8859-1, 1 UTF-16 with a
// byte order mark, 2 UTF-16BE, 3 UTF-8.
func decode(enc byte, b []byte) string {
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
				continue // the byte order mark, as read
			case 0xFFFE:
				big = !big // the mark the other way round
				continue
			}
			u = append(u, v)
		}
		return string(utf16.Decode(u))
	case 3:
		return string(b)
	}
	// ISO-8859-1, which some programs write UTF-8 as.
	return utf8OrLatin1(b)
}

// cut splits b at the end of a string of encoding enc: a NUL, or two at an
// even offset of UTF-16.
func cut(enc byte, b []byte) (string, []byte) {
	if enc == 1 || enc == 2 {
		for i := 0; i+1 < len(b); i += 2 {
			if b[i] == 0 && b[i+1] == 0 {
				return decode(enc, b[:i]), b[i+2:]
			}
		}
		return decode(enc, b), nil
	}
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return decode(enc, b[:i]), b[i+1:]
	}
	return decode(enc, b), nil
}

// apic reads an attached picture of 2.3 and 2.4.
func apic(data []byte) *Picture {
	if len(data) < 4 {
		return nil
	}
	enc := data[0]
	i := bytes.IndexByte(data[1:], 0)
	if i < 0 || 2+i >= len(data) {
		return nil
	}
	mime := strings.ToLower(latin1(data[1 : 1+i]))
	kind := data[2+i]
	_, rest := cut(enc, data[3+i:])
	switch mime {
	case "jpg", "jpeg", "image/jpg":
		mime = "image/jpeg"
	case "png":
		mime = "image/png"
	}
	return &Picture{MIME: mime, Data: rest, front: kind == 3}
}

// pic reads an attached picture of 2.2.
func pic(data []byte) *Picture {
	if len(data) < 6 {
		return nil
	}
	enc, format, kind := data[0], strings.ToUpper(string(data[1:4])), data[4]
	_, rest := cut(enc, data[5:])
	mime := ""
	switch format {
	case "JPG":
		mime = "image/jpeg"
	case "PNG":
		mime = "image/png"
	}
	return &Picture{MIME: mime, Data: rest, front: kind == 3}
}

// id3v1 reads the ID3v1 tag at the end of the file into t, for the fields
// its ID3v2 tag left empty.
func (f *file) id3v1(t *Tags) error {
	if f.size < 128 {
		return nil
	}
	var b [128]byte
	if _, err := f.readAt(f.size-128, b[:]); err != nil {
		return err
	}
	if string(b[:3]) != "TAG" {
		return nil
	}
	field := func(b []byte) string {
		if i := bytes.IndexByte(b, 0); i >= 0 {
			b = b[:i]
		}
		return utf8OrLatin1(b)
	}
	t.set("title", field(b[3:33]))
	t.set("artist", field(b[33:63]))
	t.set("album", field(b[63:93]))
	t.set("year", field(b[93:97]))
	if b[125] == 0 && b[126] != 0 {
		// ID3v1.1: the comment's last byte is the track.
		if t.Track == 0 {
			t.Track = int(b[126])
		}
	}
	return nil
}
