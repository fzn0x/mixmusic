package tags

import (
	"encoding/binary"
	"strconv"
	"strings"
)

// The items of an MP4 file's ilst the tags take.
var mp4Items = map[string]string{
	"\xa9nam": "title", "\xa9ART": "artist", "\xa9alb": "album", "aART": "albumartist",
	"\xa9day": "year", "trkn": "track", "disk": "disc", "\xa9gen": "genre",
}

// maxBoxes bounds the boxes read of a level.
const maxBoxes = 4096

// mp4 reads the items of the ilst of an MP4 file's moov, in its udta's
// meta, or its meta's.
func (f *file) mp4(t *Tags) error {
	moov, err := f.findBox(0, f.size, "moov")
	if err != nil || moov == nil {
		return err
	}
	// The moov is read whole but for its tracks, whose sample tables may
	// be large: the tags are in its udta, or its meta.
	for _, kind := range []string{"udta", "meta"} {
		box, err := f.findBox(moov.data, moov.end, kind)
		if err != nil {
			return err
		}
		if box == nil {
			continue
		}
		meta := box
		if kind == "udta" {
			if meta, err = f.findBox(box.data, box.end, "meta"); err != nil || meta == nil {
				continue
			}
		}
		b, err := f.bytesAt(meta.data, meta.end-meta.data, maxTag)
		if err != nil {
			return err
		}
		ilst(b, t)
	}
	return nil
}

// box is a box of an MP4 file: where its data starts and where it ends.
type box struct{ data, end int64 }

// findBox returns the first box of a kind between offsets from and to, or
// nil.
func (f *file) findBox(from, to int64, kind string) (*box, error) {
	at := from
	for range maxBoxes {
		if at+8 > to {
			return nil, nil
		}
		var h [16]byte
		if _, err := f.readAt(at, h[:8]); err != nil {
			return nil, nil
		}
		size, data := int64(binary.BigEndian.Uint32(h[:])), at+8
		switch size {
		case 0:
			size = to - at // to the end
		case 1:
			if _, err := f.readAt(at+8, h[8:16]); err != nil {
				return nil, nil
			}
			size, data = int64(binary.BigEndian.Uint64(h[8:])), at+16
		}
		if size < data-at || at+size > to {
			return nil, nil
		}
		if string(h[4:8]) == kind {
			return &box{data, at + size}, nil
		}
		at += size
	}
	return nil, nil
}

// boxes calls fn with the type and the data of each box in b.
func boxes(b []byte, fn func(kind string, data []byte)) {
	for n := 0; len(b) >= 8 && n < maxBoxes; n++ {
		size := int64(binary.BigEndian.Uint32(b))
		if size < 8 || size > int64(len(b)) {
			return
		}
		fn(string(b[4:8]), b[8:size])
		b = b[size:]
	}
}

// ilst reads the items of the ilst of a meta box's data, which starts with
// its version and flags, but in some files of QuickTime, whose first box
// starts at once. An item of text may hold several values, a data box
// each, as some programs write several artists.
func ilst(meta []byte, t *Tags) {
	if len(meta) >= 8 && string(meta[4:8]) != "hdlr" && string(meta[4:8]) != "ilst" {
		meta = meta[4:]
	}
	boxes(meta, func(kind string, data []byte) {
		if kind != "ilst" {
			return
		}
		boxes(data, func(item string, data []byte) {
			var texts []string
			name := ""
			boxes(data, func(kind string, d []byte) {
				if kind == "name" && len(d) >= 4 {
					name = strings.ToUpper(string(d[4:]))
				}
				if kind != "data" || len(d) < 8 {
					return
				}
				typ, value := binary.BigEndian.Uint32(d)&0xFFFFFF, d[8:]
				switch key := mp4Items[item]; {
				case item == "covr":
					mime := ""
					switch typ {
					case 13:
						mime = "image/jpeg"
					case 14:
						mime = "image/png"
					}
					t.setPicture(&Picture{MIME: mime, Data: value, front: true})
				case key == "track" || key == "disc":
					// Two bytes, the number, then the count.
					if len(value) >= 4 {
						if n := int(binary.BigEndian.Uint16(value[2:])); n > 0 {
							t.set(key, strconv.Itoa(n))
						}
					}
				case item == "gnre":
					// The number of a genre of ID3v1, from 1.
					if len(value) >= 2 {
						if n := int(binary.BigEndian.Uint16(value)); n > 0 {
							t.set("genre", genreName(strconv.Itoa(n-1), ""))
						}
					}
				case item == "rtng":
					rtng(value, t)
				case (key != "" || item == "----") && typ == 1:
					texts = append(texts, string(value))
				}
			})
			switch key := mp4Items[item]; {
			case key != "":
				t.set(key, texts...)
			case item == "----" && name == "ISRC":
				t.set("isrc", texts...)
			case item == "----":
				t.setAdvisory(name, texts...)
			}
		})
	})
}
