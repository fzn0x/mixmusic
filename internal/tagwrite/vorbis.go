package tagwrite

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// Vorbis comments are the tags of FLAC, Opus and Vorbis files: a vendor,
// then "KEY=value" pairs, a key given any number of times, of any case.
// The new comments come first, then those of the old the write doesn't
// change, in their order.

// The keys of each field, in upper case: the first is the one written.
var commentKeys = []struct {
	field Fields
	keys  []string
}{
	{Title, []string{"TITLE"}},
	{Artists, []string{"ARTIST"}},
	{Album, []string{"ALBUM"}},
	{AlbumArtists, []string{"ALBUMARTIST", "ALBUM ARTIST", "ALBUM_ARTIST"}},
	{Track, []string{"TRACKNUMBER"}},
	{Disc, []string{"DISCNUMBER"}},
	{Year, []string{"DATE", "YEAR"}},
	{Genre, []string{"GENRE"}},
	{ISRC, []string{"ISRC"}},
	{Cover, []string{"METADATA_BLOCK_PICTURE", "COVERART", "COVERARTMIME"}},
}

// comments are Vorbis comments: the vendor, and each comment as it was.
type comments struct {
	vendor []byte
	list   [][]byte
}

// parseComments reads Vorbis comments, and returns what follows them, as
// Vorbis's framing bit.
func parseComments(b []byte) (comments, []byte, error) {
	bad := fmt.Errorf("%w: its comments are broken", ErrFormat)
	var c comments
	if len(b) < 8 {
		return c, nil, bad
	}
	n := int64(binary.LittleEndian.Uint32(b))
	if n > int64(len(b))-8 {
		return c, nil, bad
	}
	c.vendor, b = b[4:4+n], b[4+n:]
	count := binary.LittleEndian.Uint32(b)
	b = b[4:]
	for ; count > 0; count-- {
		if len(b) < 4 {
			return c, nil, bad
		}
		l := int64(binary.LittleEndian.Uint32(b))
		if l > int64(len(b))-4 {
			return c, nil, bad
		}
		c.list, b = append(c.list, b[4:4+l]), b[4+l:]
	}
	return c, b, nil
}

// bytes are the comments as a block or a packet holds them, before what
// follows them.
func (c comments) bytes() []byte {
	var b bytes.Buffer
	b.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(c.vendor))))
	b.Write(c.vendor)
	b.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(c.list))))
	for _, v := range c.list {
		b.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(v))))
		b.Write(v)
	}
	return b.Bytes()
}

// key is the key of a comment, in upper case, and false for one without
// "=".
func key(comment []byte) (string, bool) {
	k, _, ok := bytes.Cut(comment, []byte("="))
	return strings.ToUpper(string(k)), ok
}

// value is the first value of a key, "" for none.
func (c comments) value(k string) string {
	for _, v := range c.list {
		if kk, ok := key(v); ok && kk == k {
			_, val, _ := bytes.Cut(v, []byte("="))
			return string(val)
		}
	}
	return ""
}

// has reports whether the comments hold a key of field.
func (c comments) has(field Fields) bool {
	for _, f := range commentKeys {
		if f.field&field == 0 {
			continue
		}
		for _, k := range f.keys {
			if c.value(k) != "" {
				return true
			}
		}
	}
	return false
}

// edit changes the comments of the fields written to t's: the new ones
// first. picture, if set, is the cover as a FLAC PICTURE block, which Ogg
// files hold in a comment; FLAC files hold it in a block of its own.
func (c *comments) edit(t Tags, fields Fields, picture []byte) {
	drop := map[string]bool{}
	for _, f := range commentKeys {
		if fields&f.field != 0 {
			for _, k := range f.keys {
				drop[k] = true
			}
		}
	}
	var list [][]byte
	add := func(k string, values ...string) {
		for _, v := range values {
			if v != "" {
				list = append(list, []byte(k+"="+v))
			}
		}
	}
	if fields&Title != 0 {
		add("TITLE", t.Title)
	}
	if fields&Artists != 0 {
		add("ARTIST", t.Artists...)
	}
	if fields&Album != 0 {
		add("ALBUM", t.Album)
	}
	if fields&AlbumArtists != 0 {
		add("ALBUMARTIST", t.AlbumArtists...)
	}
	if fields&Track != 0 && t.Track > 0 {
		add("TRACKNUMBER", withCount(t.Track, c.value("TRACKNUMBER")))
	}
	if fields&Disc != 0 && t.Disc > 0 {
		add("DISCNUMBER", withCount(t.Disc, c.value("DISCNUMBER")))
	}
	if fields&Year != 0 && t.Year > 0 {
		add("DATE", strconv.Itoa(t.Year))
	}
	if fields&Genre != 0 {
		add("GENRE", t.Genre)
	}
	if fields&ISRC != 0 {
		add("ISRC", t.ISRC)
	}
	if fields&Cover != 0 && picture != nil {
		add("METADATA_BLOCK_PICTURE", base64.StdEncoding.EncodeToString(picture))
	}
	for _, v := range c.list {
		if k, ok := key(v); ok && drop[k] {
			continue
		}
		list = append(list, v)
	}
	c.list = list
}
