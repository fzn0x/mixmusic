package tags

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"strings"

	"github.com/fzn0x/mixmusic/ogg"
)

// The keys of Vorbis comments the tags take, in upper case.
var vorbisKeys = map[string]string{
	"TITLE": "title", "ARTIST": "artist", "ALBUM": "album",
	"ALBUMARTIST": "albumartist", "ALBUM ARTIST": "albumartist", "ALBUM_ARTIST": "albumartist",
	"TRACKNUMBER": "track", "DISCNUMBER": "disc", "DATE": "year", "YEAR": "year", "ORIGINALDATE": "year",
}

// flac reads the metadata blocks of a FLAC file from offset at, after its
// "fLaC": its VORBIS_COMMENT and PICTURE blocks.
func (f *file) flac(at int64, t *Tags) error {
	for range 1024 {
		var h [4]byte
		if _, err := f.readAt(at, h[:]); err != nil {
			return nil
		}
		last, kind := h[0]&0x80 != 0, h[0]&0x7F
		size := int64(h[1])<<16 | int64(h[2])<<8 | int64(h[3])
		switch kind {
		case 4, 6:
			b, err := f.bytesAt(at+4, size, maxTag)
			if err != nil {
				return err
			}
			if kind == 4 {
				comments(b, t)
			} else {
				t.setPicture(flacPicture(b))
			}
		}
		at += 4 + size
		if last || kind == 127 {
			break
		}
	}
	return nil
}

// ogg reads the comment header of an Ogg file's stream, of Opus or Vorbis,
// its second packet.
func (f *file) ogg(t *Tags) error {
	if _, err := f.r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r, err := ogg.NewReader(f.r)
	if err != nil {
		return nil
	}
	head, err := r.ReadPacket()
	if err != nil {
		return nil
	}
	opus := bytes.HasPrefix(head.Data, []byte("OpusHead"))
	if !opus && !bytes.HasPrefix(head.Data, []byte("\x01vorbis")) {
		return nil
	}
	p, err := r.ReadPacket()
	if err != nil {
		return nil
	}
	switch {
	case opus && bytes.HasPrefix(p.Data, []byte("OpusTags")):
		comments(p.Data[8:], t)
	case !opus && bytes.HasPrefix(p.Data, []byte("\x03vorbis")):
		comments(p.Data[7:], t)
	}
	return nil
}

// comments reads Vorbis comments: their vendor, then "KEY=value" pairs,
// each after its length, a key given any number of times.
func comments(b []byte, t *Tags) {
	if len(b) < 8 {
		return
	}
	vendor := int64(binary.LittleEndian.Uint32(b))
	if vendor > int64(len(b))-8 {
		return
	}
	b = b[4+vendor:]
	n := binary.LittleEndian.Uint32(b)
	b = b[4:]
	values := map[string][]string{}
	var order []string
	for ; n > 0 && len(b) >= 4; n-- {
		l := int64(binary.LittleEndian.Uint32(b))
		if l > int64(len(b))-4 {
			break
		}
		c := string(b[4 : 4+l])
		b = b[4+l:]
		key, value, ok := strings.Cut(c, "=")
		if !ok {
			continue
		}
		key = strings.ToUpper(key)
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = append(values[key], value)
	}
	for _, key := range order {
		switch {
		case vorbisKeys[key] != "":
			t.set(vorbisKeys[key], values[key]...)
		case key == "METADATA_BLOCK_PICTURE":
			for _, v := range values[key] {
				if raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v)); err == nil {
					t.setPicture(flacPicture(raw))
				}
			}
		case key == "COVERART":
			// The older way: an image in base64, its type in COVERARTMIME.
			if raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(values[key][0])); err == nil {
				mime := ""
				if m := values["COVERARTMIME"]; len(m) > 0 {
					mime = m[0]
				}
				t.setPicture(&Picture{MIME: mime, Data: raw})
			}
		}
	}
}

// flacPicture reads a FLAC PICTURE block: its type, its MIME type and
// description after their lengths, four numbers of its size and colours,
// then its data after its length.
func flacPicture(b []byte) *Picture {
	read := func() ([]byte, bool) {
		if len(b) < 4 {
			return nil, false
		}
		n := int64(binary.BigEndian.Uint32(b))
		if n > int64(len(b))-4 {
			return nil, false
		}
		v := b[4 : 4+n]
		b = b[4+n:]
		return v, true
	}
	if len(b) < 4 {
		return nil
	}
	kind := binary.BigEndian.Uint32(b)
	b = b[4:]
	mime, ok := read()
	if !ok {
		return nil
	}
	if _, ok := read(); !ok { // the description
		return nil
	}
	if len(b) < 16 {
		return nil
	}
	b = b[16:]
	data, ok := read()
	if !ok || string(mime) == "-->" {
		return nil // a link to an image, not one
	}
	return &Picture{MIME: strings.ToLower(string(mime)), Data: data, front: kind == 3}
}
