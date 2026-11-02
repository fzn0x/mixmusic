package stream

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Segment is a part of a video stream that a page appends to Media Source
// Extensions whole: bytes Start to End, inclusive, which begin at Time and
// last Duration. A WebM stream's segments are its clusters, an MP4's its
// fragments.
type Segment struct {
	Start, End     int64
	Time, Duration time.Duration
}

// Index tells where the parts of a video stream are: its initialization
// segment, bytes 0 to Init-1, then its segments.
type Index struct {
	Init     int64
	Segments []Segment
}

// headSize is how much of a video stream MixMusic reads to find its index:
// YouTube's DASH streams put it within the first few kilobytes, as WebM
// Cues before the clusters, or an MP4 sidx before the fragments.
const headSize = 64 << 10

// maxHead bounds how much of a stream MixMusic reads for an index that
// goes on past headSize.
const maxHead = 1 << 20

var (
	// errNoIndex is returned for a video stream whose index isn't at its
	// start, as YouTube's are.
	errNoIndex = errors.New("stream: the video stream has no index at its start")
	// errBadIndex is returned for an index that doesn't read.
	errBadIndex = errors.New("stream: the index of the video stream doesn't read")
	// errCut is returned for an element whose header goes on past the
	// bytes read.
	errCut = errors.New("stream: cut short")
)

// shortHead is returned when the index of a stream goes on past the bytes
// read: need bytes hold it.
type shortHead struct{ need int64 }

func (e shortHead) Error() string {
	return fmt.Sprintf("stream: the index of the video stream takes %d bytes", e.need)
}

// parseIndex reads the index of a video stream of size bytes from its
// first bytes, head: of a WebM stream or an MP4 one, as mimeType says.
func parseIndex(head []byte, mimeType string, size int64) (Index, error) {
	switch {
	case strings.HasPrefix(mimeType, "video/webm"):
		return webmIndex(head, size)
	case strings.HasPrefix(mimeType, "video/mp4"):
		return mp4Index(head, size)
	}
	return Index{}, fmt.Errorf("stream: no index of %s streams", mimeType)
}

// The EBML IDs of the elements webmIndex reads, marker bits included.
const (
	ebmlHeader     = 0x1A45DFA3
	ebmlSegment    = 0x18538067
	ebmlInfo       = 0x1549A966
	ebmlTracks     = 0x1654AE6B
	ebmlCues       = 0x1C53BB6B
	ebmlCluster    = 0x1F43B675
	ebmlScale      = 0x2AD7B1
	ebmlDuration   = 0x4489
	ebmlCuePoint   = 0xBB
	ebmlCueTime    = 0xB3
	ebmlCuePos     = 0xB7
	ebmlClusterPos = 0xF1
)

// webmIndex reads the index of a WebM stream: its initialization segment
// is what comes before the clusters but the Cues, which say where each
// cluster starts and at what time.
func webmIndex(head []byte, size int64) (Index, error) {
	id, n, body, err := ebmlElement(head, 0)
	if err == nil && (id != ebmlHeader || body < 0) {
		err = errBadIndex
	}
	if err != nil {
		return Index{}, cutShort(err, headSize)
	}
	pos := int64(n) + body
	id, n, body, err = ebmlElement(head, pos)
	if err == nil && id != ebmlSegment {
		err = errBadIndex
	}
	if err != nil {
		return Index{}, cutShort(err, headSize)
	}
	segment := pos + int64(n) // where the positions of the Cues count from
	end := size
	if body >= 0 && segment+body < end {
		end = segment + body
	}
	scale := int64(time.Millisecond) // the default TimecodeScale, in ns
	var duration float64             // in ticks of scale
	var init int64
	var cues []byte
	for pos = segment; pos < end && (init == 0 || cues == nil); {
		if pos >= int64(len(head)) {
			return Index{}, shortHead{min(pos+headSize, end)}
		}
		id, n, body, err = ebmlElement(head, pos)
		if err != nil {
			return Index{}, cutShort(err, min(pos+headSize, end))
		}
		if body < 0 {
			return Index{}, errNoIndex // a live stream's cluster
		}
		if id == ebmlCluster {
			break
		}
		next := pos + int64(n) + body
		whole := next <= int64(len(head))
		switch id {
		case ebmlInfo, ebmlTracks, ebmlCues:
			if !whole {
				return Index{}, shortHead{next}
			}
			data := head[pos+int64(n) : next]
			switch id {
			case ebmlInfo:
				if err := eachEBML(data, func(id uint64, v []byte) {
					switch id {
					case ebmlScale:
						if s := beUint(v); s > 0 {
							scale = int64(s)
						}
					case ebmlDuration:
						duration = beFloat(v)
					}
				}); err != nil {
					return Index{}, errBadIndex
				}
				init = max(init, next)
			case ebmlTracks:
				init = max(init, next)
			case ebmlCues:
				cues = data
			}
		}
		pos = next
	}
	if init == 0 || cues == nil {
		return Index{}, errNoIndex
	}
	type cue struct{ time, at int64 }
	var points []cue
	err = eachEBML(cues, func(id uint64, v []byte) {
		if id != ebmlCuePoint {
			return
		}
		c := cue{time: -1, at: -1}
		_ = eachEBML(v, func(id uint64, v []byte) {
			switch id {
			case ebmlCueTime:
				c.time = int64(beUint(v))
			case ebmlCuePos:
				_ = eachEBML(v, func(id uint64, v []byte) {
					if id == ebmlClusterPos && c.at < 0 {
						c.at = int64(beUint(v))
					}
				})
			}
		})
		if c.time >= 0 && c.at >= 0 {
			points = append(points, c)
		}
	})
	if err != nil || len(points) == 0 {
		return Index{}, errBadIndex
	}
	ix := Index{Init: init}
	total := time.Duration(duration * float64(scale))
	for i, c := range points {
		s := Segment{Start: segment + c.at, End: end - 1, Time: time.Duration(c.time * scale)}
		if i+1 < len(points) {
			s.End = segment + points[i+1].at - 1
			s.Duration = time.Duration((points[i+1].time - c.time) * scale)
		} else {
			s.Duration = max(total-s.Time, 0)
		}
		if s.Start < init || s.End < s.Start || i > 0 && s.Start <= ix.Segments[i-1].Start {
			return Index{}, errBadIndex
		}
		ix.Segments = append(ix.Segments, s)
	}
	return ix, nil
}

// cutShort returns, for errCut, that need bytes may hold the index, else
// err.
func cutShort(err error, need int64) error {
	if errors.Is(err, errCut) {
		return shortHead{need}
	}
	return err
}

// ebmlElement reads the header of the EBML element at pos of b: its ID,
// the length of the header, and the length of its body, -1 when unknown.
func ebmlElement(b []byte, pos int64) (id uint64, n int, body int64, err error) {
	if pos < 0 || pos >= int64(len(b)) {
		return 0, 0, 0, errCut
	}
	b = b[pos:]
	id, idLen, err := ebmlVint(b, true)
	if err != nil {
		return 0, 0, 0, err
	}
	size, sizeLen, err := ebmlVint(b[idLen:], false)
	if err != nil {
		return 0, 0, 0, err
	}
	body = int64(size)
	if size == 1<<(7*sizeLen)-1 {
		body = -1 // all ones: unknown
	}
	return id, idLen + sizeLen, body, nil
}

// ebmlVint reads a variable-length integer at the start of b, with its
// marker bit when it is an ID.
func ebmlVint(b []byte, id bool) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, errCut
	}
	if b[0] == 0 {
		return 0, 0, errBadIndex
	}
	n := 1
	for mask := byte(0x80); b[0]&mask == 0; mask >>= 1 {
		n++
	}
	switch {
	case id && n > 4:
		return 0, 0, errBadIndex
	case len(b) < n:
		return 0, 0, errCut
	}
	v := uint64(b[0])
	if !id {
		v &= 0xFF >> n
	}
	for _, c := range b[1:n] {
		v = v<<8 | uint64(c)
	}
	return v, n, nil
}

// eachEBML calls fn with each element of b, a body of elements, and its
// body.
func eachEBML(b []byte, fn func(id uint64, body []byte)) error {
	for pos := int64(0); pos < int64(len(b)); {
		id, n, body, err := ebmlElement(b, pos)
		if err != nil || body < 0 || int64(n)+body > int64(len(b))-pos {
			return errBadIndex
		}
		fn(id, b[pos+int64(n):pos+int64(n)+body])
		pos += int64(n) + body
	}
	return nil
}

// beUint reads an unsigned integer of up to 8 bytes, big-endian.
func beUint(b []byte) uint64 {
	var v uint64
	for _, c := range b[:min(len(b), 8)] {
		v = v<<8 | uint64(c)
	}
	return v
}

// beFloat reads a float of 4 or 8 bytes, big-endian.
func beFloat(b []byte) float64 {
	switch len(b) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	}
	return 0
}

// mp4Index reads the index of a fragmented MP4 stream: its initialization
// segment is its ftyp and moov boxes, and its sidx says where each
// fragment starts and how long it lasts.
func mp4Index(head []byte, size int64) (Index, error) {
	var ix Index
	for pos := int64(0); pos < size; {
		if pos+8 > int64(len(head)) {
			return Index{}, shortHead{min(pos+headSize, size)}
		}
		n := int64(binary.BigEndian.Uint32(head[pos:]))
		kind := string(head[pos+4 : pos+8])
		hdr := int64(8)
		switch n {
		case 1:
			if pos+16 > int64(len(head)) {
				return Index{}, shortHead{pos + 16}
			}
			n, hdr = int64(binary.BigEndian.Uint64(head[pos+8:])), 16
		case 0:
			n = size - pos
		}
		if n < hdr || pos == 0 && kind != "ftyp" {
			return Index{}, errBadIndex
		}
		next := pos + n
		switch kind {
		case "moov":
			ix.Init = next
		case "sidx":
			if next > int64(len(head)) {
				return Index{}, shortHead{next}
			}
			if ix.Init == 0 {
				return Index{}, errBadIndex
			}
			segs, err := parseSidx(head[pos+hdr:next], next, size)
			if err != nil {
				return Index{}, err
			}
			ix.Segments = segs
			return ix, nil
		case "moof", "mdat":
			return Index{}, errNoIndex
		}
		pos = next
	}
	return Index{}, errNoIndex
}

// parseSidx reads the references of a sidx box's body, whose first byte
// after it is at anchor, of a stream of size bytes.
func parseSidx(b []byte, anchor, size int64) ([]Segment, error) {
	if len(b) < 12 {
		return nil, errBadIndex
	}
	version := b[0]
	scale := int64(binary.BigEndian.Uint32(b[8:]))
	b = b[12:]
	var earliest, first int64
	switch {
	case version == 0 && len(b) >= 8:
		earliest, first = int64(binary.BigEndian.Uint32(b)), int64(binary.BigEndian.Uint32(b[4:]))
		b = b[8:]
	case version == 1 && len(b) >= 16:
		earliest, first = int64(binary.BigEndian.Uint64(b)), int64(binary.BigEndian.Uint64(b[8:]))
		b = b[16:]
	default:
		return nil, errBadIndex
	}
	if scale <= 0 || len(b) < 4 || earliest < 0 || first < 0 {
		return nil, errBadIndex
	}
	count := int(binary.BigEndian.Uint16(b[2:]))
	b = b[4:]
	if count == 0 || len(b) < 12*count {
		return nil, errBadIndex
	}
	ticks := func(t int64) time.Duration {
		return time.Duration(float64(t) / float64(scale) * float64(time.Second))
	}
	at, t := anchor+first, earliest
	out := make([]Segment, 0, count)
	for i := range count {
		r := b[12*i:]
		ref := binary.BigEndian.Uint32(r)
		if ref>>31 != 0 {
			return nil, errBadIndex // a reference to another sidx
		}
		n, d := int64(ref&0x7FFFFFFF), int64(binary.BigEndian.Uint32(r[4:]))
		if n == 0 || at+n > size {
			return nil, errBadIndex
		}
		out = append(out, Segment{Start: at, End: at + n - 1, Time: ticks(t), Duration: ticks(t+d) - ticks(t)})
		at, t = at+n, t+d
	}
	return out, nil
}
