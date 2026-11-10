package tagwrite

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fzn0x/mixmusic/internal/tags"
)

// A FLAC file's tags are its VORBIS_COMMENT block, and its covers PICTURE
// blocks, among the metadata blocks after "fLaC". The new metadata keeps
// STREAMINFO first and the other blocks in their order, the comments in
// the place of the old ones, or after STREAMINFO, and the cover after
// them. Padding fills the room the old metadata took when the new fits in
// it, so that the audio starts where it did, else there are 4 KB of it.
// An ID3v2 tag before "fLaC", which some programs write and FLAC doesn't
// know, goes: what it told that the comments don't is written in them.

// The types of metadata blocks.
const (
	blockStreamInfo = 0
	blockPadding    = 1
	blockComments   = 4
	blockPicture    = 6
)

// flacPadding is the padding of new metadata that doesn't fit where the
// old was.
const flacPadding = 4096

// maxBlock bounds the bytes of a metadata block: its length has 24 bits.
const maxBlock = 1<<24 - 1

type flacBlock struct {
	kind byte
	data []byte
}

func (f *file) writeFLAC(w io.Writer, t Tags, fields Fields) error {
	start, ok := f.id3End()
	if !ok {
		return fmt.Errorf("%w: its ID3 tag is longer than the file", ErrFormat)
	}
	var magic [4]byte
	if _, err := f.readAt(start, magic[:]); err != nil || string(magic[:]) != "fLaC" {
		return fmt.Errorf("%w: it isn't FLAC", ErrFormat)
	}
	blocks, audio, err := f.flacBlocks(start + 4)
	if err != nil {
		return err
	}
	var vc comments
	vcAt := -1
	for i, b := range blocks {
		if b.kind == blockComments {
			if vc, _, err = parseComments(b.data); err != nil {
				return err
			}
			vcAt = i
			break
		}
	}
	if vcAt < 0 {
		vc = comments{vendor: []byte("MixMusic")}
	}
	if start > 0 {
		// The ID3 tag goes: what it told, the comments and pictures take.
		t, fields, err = f.carryID3(t, fields)
		if err != nil {
			return err
		}
	}
	vc.edit(t, fields, nil)
	vcData := vc.bytes()
	var picture []byte
	if fields&Cover != 0 && t.Cover != nil {
		picture = pictureBlock(t.Cover)
	}
	if len(vcData) > maxBlock || len(picture) > maxBlock {
		return fmt.Errorf("%w: its tags would be larger than a FLAC block holds", ErrFormat)
	}
	var out []flacBlock
	put := func() {
		out = append(out, flacBlock{blockComments, vcData})
		if picture != nil {
			out = append(out, flacBlock{blockPicture, picture})
		}
	}
	for i, b := range blocks {
		switch {
		case b.kind == blockPadding:
		case b.kind == blockPicture && fields&Cover != 0:
		case i == vcAt:
			put()
		default:
			out = append(out, b)
			if i == 0 && vcAt < 0 {
				put()
			}
		}
	}
	size := int64(0)
	for _, b := range out {
		size += 4 + int64(len(b.data))
	}
	old := audio - (start + 4)
	pad := int64(flacPadding)
	if room := old - size - 4; room >= 0 && room <= maxBlock {
		pad = room
	}
	out = append(out, flacBlock{blockPadding, make([]byte, pad)})
	if _, err := io.WriteString(w, "fLaC"); err != nil {
		return err
	}
	for i, b := range out {
		h := []byte{b.kind, byte(len(b.data) >> 16), byte(len(b.data) >> 8), byte(len(b.data))}
		if i == len(out)-1 {
			h[0] |= 0x80
		}
		if _, err := w.Write(h); err != nil {
			return err
		}
		if _, err := w.Write(b.data); err != nil {
			return err
		}
	}
	return f.copyTo(w, audio, f.size)
}

// flacBlocks reads the metadata blocks from offset at, and returns where
// the audio starts after them. The first must be STREAMINFO.
func (f *file) flacBlocks(at int64) ([]flacBlock, int64, error) {
	var blocks []flacBlock
	total := int64(0)
	for {
		var h [4]byte
		if _, err := f.readAt(at, h[:]); err != nil {
			return nil, 0, fmt.Errorf("%w: its metadata ends before its audio", ErrFormat)
		}
		last, kind := h[0]&0x80 != 0, h[0]&0x7F
		size := int64(h[1])<<16 | int64(h[2])<<8 | int64(h[3])
		if kind == 127 || len(blocks) == 0 && kind != blockStreamInfo {
			return nil, 0, fmt.Errorf("%w: its metadata is broken", ErrFormat)
		}
		if total += 4 + size; total > maxRead {
			return nil, 0, fmt.Errorf("%w: its metadata is larger than %d MB", ErrFormat, maxRead>>20)
		}
		data, err := f.bytesAt(at+4, size)
		if err != nil {
			return nil, 0, err
		}
		blocks = append(blocks, flacBlock{kind, data})
		at += 4 + size
		if last {
			return blocks, at, nil
		}
	}
}

// carryID3 adds to the fields written of t those an ID3 tag before
// "fLaC" tells, as tags.Read reads them, which take the comments' place,
// so that the tag can go and the file tells what it told.
func (f *file) carryID3(t Tags, fields Fields) (Tags, Fields, error) {
	id3, err := f.readID3()
	if err != nil {
		return t, fields, err
	}
	if _, err := f.r.Seek(0, io.SeekStart); err != nil {
		return t, fields, err
	}
	was, err := tags.Read(f.r)
	if err != nil {
		return t, fields, nil
	}
	told := func(field Fields) bool {
		for _, fl := range id3Fields {
			if fl.field != field {
				continue
			}
			for _, id := range fl.ids {
				if id3.value(id) != "" {
					return true
				}
			}
		}
		return false
	}
	carry := func(field Fields, set func()) {
		if fields&field == 0 && told(field) {
			set()
			fields |= field
		}
	}
	carry(Title, func() { t.Title = was.Title })
	carry(Artists, func() { t.Artists = was.Artists })
	carry(Album, func() { t.Album = was.Album })
	carry(AlbumArtists, func() { t.AlbumArtists = was.AlbumArtists })
	carry(Track, func() { t.Track = was.Track })
	carry(Disc, func() { t.Disc = was.Disc })
	carry(Year, func() { t.Year = was.Year })
	carry(Genre, func() { t.Genre = was.Genre })
	carry(ISRC, func() { t.ISRC = was.ISRC })
	if p := was.Picture; p != nil && fields&Cover == 0 && id3.pictured(p.Data) {
		if mime, err := checkCover(&Picture{Data: p.Data}); err == nil {
			t.Cover, fields = &Picture{MIME: mime, Data: bytes.Clone(p.Data)}, fields|Cover
		}
	}
	return t, fields, nil
}
