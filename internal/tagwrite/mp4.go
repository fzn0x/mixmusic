package tagwrite

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// An MP4 file's tags are the items of the ilst of its moov's udta's meta,
// as iTunes writes them, or of its moov's meta, as some QuickTime files
// have them: both change, and a file with neither gets the first. The new
// items come first, then those of the old the write doesn't change.
//
// When the moov changes size and the audio follows it, as in a file made
// to start playing as it downloads, the audio moves: a free box after the
// ilst, or after the moov, takes up the difference where it can, so that
// the audio stays where it was; else the chunk offsets of the moov's
// sample tables (stco and co64), the base offsets of fragments (tfhd) and
// those of the random access index (tfra) move with it. Offsets into
// media before the moov stay.

// mp4Start reports whether an MP4 file may start with a box of this type.
func mp4Start(kind string) bool {
	switch kind {
	case "ftyp", "moov", "mdat", "free", "skip", "wide", "pdin", "uuid":
		return true
	}
	return false
}

// maxBoxes bounds the boxes of a level.
const maxBoxes = 1 << 16

// The items of each field.
var mp4Fields = []struct {
	field Fields
	items []string
}{
	{Title, []string{"\xa9nam"}},
	{Artists, []string{"\xa9ART"}},
	{Album, []string{"\xa9alb"}},
	{AlbumArtists, []string{"aART"}},
	{Track, []string{"trkn"}},
	{Disc, []string{"disk"}},
	{Year, []string{"\xa9day"}},
	{Genre, []string{"\xa9gen", "gnre"}},
	{Cover, []string{"covr"}},
}

// topBox is a box of the file's top level: where it starts, how long its
// header is, and its size.
type topBox struct {
	kind           string
	at, head, size int64
}

func (b topBox) end() int64 { return b.at + b.size }

// topBoxes reads the boxes of the file's top level, and where the last
// ends: bytes after it, too few for a box, are kept as they are.
func (f *file) topBoxes() ([]topBox, int64, error) {
	var out []topBox
	at := int64(0)
	for len(out) < maxBoxes && f.size-at >= 8 {
		var h [16]byte
		if _, err := f.readAt(at, h[:8]); err != nil {
			return nil, 0, err
		}
		b := topBox{kind: string(h[4:8]), at: at, head: 8, size: int64(binary.BigEndian.Uint32(h[:]))}
		switch b.size {
		case 0:
			b.size = f.size - at // to the end
		case 1:
			if _, err := f.readAt(at+8, h[8:16]); err != nil {
				return nil, 0, fmt.Errorf("%w: a box is cut short", ErrFormat)
			}
			b.head, b.size = 16, int64(binary.BigEndian.Uint64(h[8:]))
		}
		if b.size < b.head || b.size > f.size-at {
			return nil, 0, fmt.Errorf("%w: a box at byte %d is longer than the file", ErrFormat, at)
		}
		out = append(out, b)
		at += b.size
	}
	return out, at, nil
}

// box is a box read whole: its type and its data.
type box struct {
	kind string
	data []byte
}

// boxesOf reads the boxes of b, which must hold them whole.
func boxesOf(b []byte) ([]box, error) {
	var out []box
	for len(b) > 0 {
		if len(b) < 8 || len(out) >= maxBoxes {
			return nil, fmt.Errorf("%w: its boxes are broken", ErrFormat)
		}
		size, head := uint64(binary.BigEndian.Uint32(b)), 8
		switch size {
		case 0:
			size = uint64(len(b))
		case 1:
			if len(b) < 16 {
				return nil, fmt.Errorf("%w: its boxes are broken", ErrFormat)
			}
			size, head = binary.BigEndian.Uint64(b[8:]), 16
		}
		if size < uint64(head) || size > uint64(len(b)) {
			return nil, fmt.Errorf("%w: its boxes are broken", ErrFormat)
		}
		out = append(out, box{string(b[4:8]), b[head:size]})
		b = b[size:]
	}
	return out, nil
}

// boxBytes is a box of a type and data.
func boxBytes(kind string, data ...[]byte) []byte {
	n := 8
	for _, d := range data {
		n += len(d)
	}
	var b []byte
	if n > 1<<32-1 {
		b = binary.BigEndian.AppendUint32(nil, 1)
		b = append(b, kind...)
		b = binary.BigEndian.AppendUint64(b, uint64(n+8))
	} else {
		b = binary.BigEndian.AppendUint32(nil, uint32(n))
		b = append(b, kind...)
	}
	for _, d := range data {
		b = append(b, d...)
	}
	return b
}

func encode(bs []box) []byte {
	var out []byte
	for _, b := range bs {
		out = append(out, boxBytes(b.kind, b.data)...)
	}
	return out
}

func (f *file) writeMP4(w io.Writer, t Tags, fields Fields) error {
	top, end, err := f.topBoxes()
	if err != nil {
		return err
	}
	mi := -1
	for i, b := range top {
		if b.kind == "moov" {
			if mi >= 0 {
				return fmt.Errorf("%w: it has two moov boxes", ErrFormat)
			}
			mi = i
		}
	}
	if mi < 0 {
		return fmt.Errorf("%w: it has no moov box", ErrFormat)
	}
	moov := top[mi]
	data, err := f.bytesAt(moov.at+moov.head, moov.size-moov.head)
	if err != nil {
		return err
	}
	kids, err := boxesOf(data)
	if err != nil {
		return err
	}
	if kids, err = editMoov(kids, t, fields); err != nil {
		return err
	}
	newMoov := boxBytes("moov", encode(kids))
	delta := int64(len(newMoov)) - moov.size
	after := top[mi+1:]
	media := false
	for _, b := range after {
		media = media || b.kind == "mdat" || b.kind == "moof"
	}
	// A free box after the moov takes up the difference, where it can.
	freeSize := int64(-1)
	if delta != 0 && media && len(after) > 0 && (after[0].kind == "free" || after[0].kind == "skip") && after[0].head == 8 {
		if n := after[0].size - delta; n == 0 || n >= 8 && n <= 1<<32-1 {
			freeSize, delta = n, 0
		}
	}
	shift := int64(0)
	if delta != 0 && media {
		shift = delta
		head := 8
		if binary.BigEndian.Uint32(newMoov) == 1 {
			head = 16
		}
		if err := shiftOffsets(newMoov[head:], moov.end(), shift); err != nil {
			return err
		}
	}
	if err := f.copyTo(w, 0, moov.at); err != nil {
		return err
	}
	if _, err := w.Write(newMoov); err != nil {
		return err
	}
	for i, b := range after {
		switch {
		case i == 0 && freeSize >= 0:
			if freeSize > 0 {
				if _, err := w.Write(boxBytes(after[0].kind, make([]byte, freeSize-8))); err != nil {
					return err
				}
			}
		case shift != 0 && (b.kind == "moof" || b.kind == "mfra"):
			whole, err := f.bytesAt(b.at, b.size)
			if err != nil {
				return err
			}
			if err := shiftOffsets(whole[b.head:], moov.end(), shift); err != nil {
				return err
			}
			if _, err := w.Write(whole); err != nil {
				return err
			}
		default:
			if err := f.copyTo(w, b.at, b.end()); err != nil {
				return err
			}
		}
	}
	return f.copyTo(w, end, f.size)
}

// editMoov changes the items of the ilst of the moov's meta, if it has
// one, and of its udta's meta, which it makes, unless the moov's meta had
// an ilst and the moov has no udta's meta.
func editMoov(kids []box, t Tags, fields Fields) ([]box, error) {
	quickTime := false
	for i := range kids {
		if kids[i].kind != "meta" {
			continue
		}
		data, ok, err := editMeta(kids[i].data, t, fields, false)
		if err != nil {
			return nil, err
		}
		if ok {
			kids[i].data, quickTime = data, true
		}
	}
	for i := range kids {
		if kids[i].kind != "udta" {
			continue
		}
		inner, err := boxesOf(kids[i].data)
		if err != nil {
			return nil, err
		}
		found := false
		for j := range inner {
			if inner[j].kind == "meta" {
				if inner[j].data, _, err = editMeta(inner[j].data, t, fields, true); err != nil {
					return nil, err
				}
				found = true
				break
			}
		}
		if !found && !quickTime {
			data, _, err := editMeta(nil, t, fields, true)
			if err != nil {
				return nil, err
			}
			inner = append(inner, box{"meta", data})
		}
		kids[i].data = encode(inner)
		return kids, nil
	}
	if !quickTime {
		data, _, err := editMeta(nil, t, fields, true)
		if err != nil {
			return nil, err
		}
		kids = append(kids, box{"udta", boxBytes("meta", data)})
	}
	return kids, nil
}

// editMeta changes the ilst of a meta box's data, which starts with its
// version and flags, but in some QuickTime files, whose first box starts
// at once: false for one without an ilst, unless create is set, which makes
// one, and the meta itself when data is nil. A free box after the ilst
// takes up the difference in size, where it can.
func editMeta(data []byte, t Tags, fields Fields, create bool) ([]byte, bool, error) {
	var head []byte
	if data == nil {
		head = []byte{0, 0, 0, 0}
		hdlr := []byte{0, 0, 0, 0, 0, 0, 0, 0, 'm', 'd', 'i', 'r', 'a', 'p', 'p', 'l', 0, 0, 0, 0, 0, 0, 0, 0, 0}
		data = append(head, boxBytes("hdlr", hdlr)...)
	}
	if len(data) >= 8 && string(data[4:8]) != "hdlr" && string(data[4:8]) != "ilst" {
		if len(data) < 4 {
			return nil, false, fmt.Errorf("%w: its meta box is broken", ErrFormat)
		}
		head, data = data[:4], data[4:]
	}
	kids, err := boxesOf(data)
	if err != nil {
		return nil, false, err
	}
	at := -1
	for i, k := range kids {
		if k.kind == "ilst" {
			at = i
			break
		}
	}
	if at < 0 {
		if !create {
			return nil, false, nil
		}
		kids = append(kids, box{"ilst", nil})
		at = len(kids) - 1
	}
	items, err := editIlst(kids[at].data, t, fields)
	if err != nil {
		return nil, false, err
	}
	delta := len(items) - len(kids[at].data)
	kids[at].data = items
	if at+1 < len(kids) && (kids[at+1].kind == "free" || kids[at+1].kind == "skip") {
		switch n := len(kids[at+1].data) + 8 - delta; {
		case n == 0:
			kids = append(kids[:at+1], kids[at+2:]...)
		case n >= 8:
			kids[at+1].data = make([]byte, n-8)
		}
	}
	return append(bytes.Clone(head), encode(kids)...), true, nil
}

// editIlst changes the items of an ilst's data: the new ones first.
func editIlst(data []byte, t Tags, fields Fields) ([]byte, error) {
	items, err := boxesOf(data)
	if err != nil {
		return nil, err
	}
	drop := map[string]bool{}
	for _, f := range mp4Fields {
		if fields&f.field != 0 {
			for _, it := range f.items {
				drop[it] = true
			}
		}
	}
	var out []byte
	item := func(kind string, typ uint32, values ...[]byte) {
		var datas []byte
		for _, v := range values {
			if len(v) > 0 {
				datas = append(datas, boxBytes("data", binary.BigEndian.AppendUint32(nil, typ), []byte{0, 0, 0, 0}, v)...)
			}
		}
		if datas != nil {
			out = append(out, boxBytes(kind, datas)...)
		}
	}
	text := func(kind string, values ...string) {
		var vs [][]byte
		for _, v := range values {
			vs = append(vs, []byte(v))
		}
		item(kind, 1, vs...)
	}
	number := func(kind string, n, of, size int) {
		if n <= 0 {
			return
		}
		v := make([]byte, size)
		binary.BigEndian.PutUint16(v[2:], uint16(min(n, 0xFFFF)))
		binary.BigEndian.PutUint16(v[4:], uint16(min(of, 0xFFFF)))
		item(kind, 0, v)
	}
	if fields&Title != 0 {
		text("\xa9nam", t.Title)
	}
	if fields&Artists != 0 {
		text("\xa9ART", t.Artists...)
	}
	if fields&Album != 0 {
		text("\xa9alb", t.Album)
	}
	if fields&AlbumArtists != 0 {
		text("aART", t.AlbumArtists...)
	}
	if fields&Track != 0 {
		number("trkn", t.Track, count(items, "trkn"), 8)
	}
	if fields&Disc != 0 {
		number("disk", t.Disc, count(items, "disk"), 6)
	}
	if fields&Year != 0 && t.Year > 0 {
		text("\xa9day", fmt.Sprint(t.Year))
	}
	if fields&Genre != 0 {
		text("\xa9gen", t.Genre)
	}
	if fields&ISRC != 0 && t.ISRC != "" {
		mean := boxBytes("mean", []byte{0, 0, 0, 0}, []byte("com.apple.iTunes"))
		name := boxBytes("name", []byte{0, 0, 0, 0}, []byte("ISRC"))
		value := boxBytes("data", []byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte(t.ISRC))
		out = append(out, boxBytes("----", mean, name, value)...)
	}
	if fields&Cover != 0 && t.Cover != nil {
		typ := uint32(13) // JPEG
		if t.Cover.MIME == "image/png" {
			typ = 14
		}
		item("covr", typ, t.Cover.Data)
	}
	for _, it := range items {
		if drop[it.kind] || fields&ISRC != 0 && it.kind == "----" && freeformName(it.data) == "ISRC" {
			continue
		}
		out = append(out, boxBytes(it.kind, it.data)...)
	}
	return out, nil
}

// count is the count of a track's or a disc's item, as "of 12": 0 for
// none.
func count(items []box, kind string) int {
	for _, it := range items {
		if it.kind != kind {
			continue
		}
		datas, _ := boxesOf(it.data)
		for _, d := range datas {
			if d.kind == "data" && len(d.data) >= 14 {
				return int(binary.BigEndian.Uint16(d.data[12:]))
			}
		}
	}
	return 0
}

// freeformName is the name of a freeform item, "----", in upper case.
func freeformName(data []byte) string {
	kids, _ := boxesOf(data)
	for _, k := range kids {
		if k.kind == "name" && len(k.data) >= 4 {
			return strings.ToUpper(string(k.data[4:]))
		}
	}
	return ""
}

// The boxes whose boxes shiftOffsets looks into.
var containers = map[string]bool{
	"moov": true, "trak": true, "mdia": true, "minf": true, "stbl": true, "moof": true, "traf": true, "mfra": true,
}

// shiftOffsets moves by delta the offsets into the file, at or after from,
// that the boxes of b hold: the chunk offsets of stco and co64, the base
// data offsets of tfhd, and the moof offsets of tfra. b is changed in
// place.
func shiftOffsets(b []byte, from, delta int64) error {
	for len(b) >= 8 {
		size, head := int64(binary.BigEndian.Uint32(b)), int64(8)
		switch size {
		case 0:
			size = int64(len(b))
		case 1:
			if len(b) < 16 {
				return fmt.Errorf("%w: its boxes are broken", ErrFormat)
			}
			size, head = int64(binary.BigEndian.Uint64(b[8:])), 16
		}
		if size < head || size > int64(len(b)) {
			return fmt.Errorf("%w: its boxes are broken", ErrFormat)
		}
		kind, data := string(b[4:8]), b[head:size]
		var err error
		switch {
		case containers[kind]:
			err = shiftOffsets(data, from, delta)
		case kind == "stco":
			err = shiftTable(data, 4, from, delta)
		case kind == "co64":
			err = shiftTable(data, 8, from, delta)
		case kind == "tfhd":
			if len(data) >= 16 && data[3]&1 != 0 {
				shift64(data[8:16], from, delta)
			}
		case kind == "tfra":
			err = shiftTfra(data, from, delta)
		}
		if err != nil {
			return err
		}
		b = b[size:]
	}
	return nil
}

// shiftTable moves the offsets of a chunk offset table, of entries of
// width bytes after its version, flags and count.
func shiftTable(data []byte, width int, from, delta int64) error {
	if len(data) < 8 {
		return fmt.Errorf("%w: a chunk offset table is broken", ErrFormat)
	}
	n := int(binary.BigEndian.Uint32(data[4:]))
	if n < 0 || n > (len(data)-8)/width {
		return fmt.Errorf("%w: a chunk offset table is broken", ErrFormat)
	}
	for i := range n {
		e := data[8+i*width : 8+(i+1)*width]
		if width == 8 {
			shift64(e, from, delta)
			continue
		}
		v := int64(binary.BigEndian.Uint32(e))
		if v < from {
			continue
		}
		if v += delta; v < 0 || v > 1<<32-1 {
			return fmt.Errorf("%w: its audio would move past where 32-bit offsets reach", ErrFormat)
		}
		binary.BigEndian.PutUint32(e, uint32(v))
	}
	return nil
}

func shift64(e []byte, from, delta int64) {
	if v := int64(binary.BigEndian.Uint64(e)); v >= from {
		binary.BigEndian.PutUint64(e, uint64(v+delta))
	}
}

// shiftTfra moves the moof offsets of a track fragment random access box.
func shiftTfra(data []byte, from, delta int64) error {
	bad := fmt.Errorf("%w: its random access index is broken", ErrFormat)
	if len(data) < 16 {
		return bad
	}
	version := data[0]
	sizes := binary.BigEndian.Uint32(data[8:])
	n := int(binary.BigEndian.Uint32(data[12:]))
	rest := int(sizes>>4&3) + 1 + int(sizes>>2&3) + 1 + int(sizes&3) + 1
	width := 8
	if version == 1 {
		width = 16
	}
	entry := width + rest
	if n < 0 || n > (len(data)-16)/entry {
		return bad
	}
	for i := range n {
		e := data[16+i*entry:]
		if version == 1 {
			shift64(e[8:16], from, delta)
			continue
		}
		v := int64(binary.BigEndian.Uint32(e[4:]))
		if v < from {
			continue
		}
		if v += delta; v < 0 || v > 1<<32-1 {
			return bad
		}
		binary.BigEndian.PutUint32(e[4:], uint32(v))
	}
	return nil
}
