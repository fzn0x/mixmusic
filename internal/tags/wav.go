package tags

import "encoding/binary"

// The items of a WAV file's INFO list the tags take.
var infoItems = map[string]string{
	"INAM": "title", "IART": "artist", "IPRD": "album", "ICRD": "year", "ITRK": "track", "IPRT": "track",
}

// wav reads the chunks of a WAV file: an ID3v2 tag in an "id3 " chunk,
// as some programs write, and its INFO list, for what the tag leaves out.
func (f *file) wav(t *Tags) error {
	at := int64(12)
	var info []byte
	for range 256 {
		var h [8]byte
		if _, err := f.readAt(at, h[:]); err != nil {
			break
		}
		size := int64(binary.LittleEndian.Uint32(h[4:]))
		switch string(h[:4]) {
		case "id3 ", "ID3 ":
			if _, err := f.id3v2(at+8, t); err != nil {
				return err
			}
		case "LIST":
			b, err := f.bytesAt(at+8, size, maxText*8)
			if err != nil {
				return err
			}
			if len(b) >= 4 && string(b[:4]) == "INFO" {
				info = b[4:]
			}
		}
		at += 8 + size + size&1
		if at >= f.size {
			break
		}
	}
	for len(info) >= 8 {
		size := int(binary.LittleEndian.Uint32(info[4:]))
		if size < 0 || size > len(info)-8 {
			break
		}
		if key := infoItems[string(info[:4])]; key != "" {
			v := info[8 : 8+size]
			for len(v) > 0 && v[len(v)-1] == 0 {
				v = v[:len(v)-1]
			}
			t.set(key, utf8OrLatin1(v))
		}
		next := 8 + size + size&1
		if next > len(info) {
			break
		}
		info = info[next:]
	}
	return nil
}
