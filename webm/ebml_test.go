package webm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"slices"
	"testing"
)

// The IDs of elements only the tests write.
const (
	idEBMLVersion        = 0x4286
	idDocTypeReadVersion = 0x4285
	idVoid               = 0xEC
	idCRC32              = 0xBF
	idContentEncoding    = 0x6240
)

// vint codes v on n bytes, marker included.
func vint(v uint64, n int) []byte {
	b := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		b[i] = byte(v)
		v >>= 8
	}
	b[0] |= 0x80 >> (n - 1)
	return b
}

// sizeLen returns the length of the shortest size of n bytes: a size of
// all ones is unknown.
func sizeLen(n int) int {
	l := 1
	for uint64(n) >= 1<<(7*l)-1 {
		l++
	}
	return l
}

// idBytes returns the bytes of an ID.
func idBytes(id uint32) []byte {
	b := binary.BigEndian.AppendUint32(nil, id)
	for len(b) > 1 && b[0] == 0 {
		b = b[1:]
	}
	return b
}

// el returns an element of the given data, with the shortest size.
func el(id uint32, data ...[]byte) []byte { return elSize(id, 0, data...) }

// elSize returns an element whose size takes n bytes, or more if need be.
func elSize(id uint32, n int, data ...[]byte) []byte {
	d := slices.Concat(data...)
	return slices.Concat(idBytes(id), vint(uint64(len(d)), max(n, sizeLen(len(d)))), d)
}

// elUnknown returns an element of unknown size.
func elUnknown(id uint32, data ...[]byte) []byte {
	return slices.Concat(idBytes(id), vint(1<<56-1, 8), slices.Concat(data...))
}

// uintData returns the shortest bytes of v.
func uintData(v uint64) []byte {
	b := binary.BigEndian.AppendUint64(nil, v)
	for len(b) > 1 && b[0] == 0 {
		b = b[1:]
	}
	return b
}

func uintEl(id uint32, v uint64) []byte { return el(id, uintData(v)) }
func intEl(id uint32, v int64) []byte   { return el(id, binary.BigEndian.AppendUint64(nil, uint64(v))) }
func strEl(id uint32, s string) []byte  { return el(id, []byte(s)) }
func floatEl(id uint32, f float64) []byte {
	return el(id, binary.BigEndian.AppendUint64(nil, math.Float64bits(f)))
}

// pos codes a position on 8 bytes, so that the size of the SeekHead and
// the Cues does not depend on it.
func pos(p uint64) []byte { return binary.BigEndian.AppendUint64(nil, p) }

var opusHead = []byte("OpusHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\x00\x00")

func ebmlHeader(docType string) []byte {
	return el(idEBML, uintEl(idEBMLVersion, 1), uintEl(idEBMLReadVersion, 1), strEl(idDocType, docType),
		uintEl(idDocTypeReadVersion, 2))
}

func infoEl(scale uint64, duration float64) []byte {
	return el(idInfo, uintEl(idTimestampScale, scale), floatEl(idDuration, duration))
}

func opusTrack(number uint64) []byte {
	return el(idTrackEntry, uintEl(idTrackNumber, number), uintEl(idTrackType, trackAudio), strEl(idCodecID, "A_OPUS"),
		el(idCodecPrivate, opusHead), uintEl(idCodecDelay, 6_500_000), uintEl(idSeekPreRoll, 80_000_000),
		el(idAudio, floatEl(idSamplingFrequency, 48000), uintEl(idChannels, 2)))
}

func videoTrack(number uint64) []byte {
	return el(idTrackEntry, uintEl(idTrackNumber, number), uintEl(idTrackType, 1), strEl(idCodecID, "V_VP9"))
}

// block returns the data of a Block or SimpleBlock: its track, timestamp
// and flags, then its frames, laced already.
func block(track uint64, rel int16, flags byte, frames ...[]byte) []byte {
	return slices.Concat(vint(track, 1), binary.BigEndian.AppendUint16(nil, uint16(rel)), []byte{flags}, slices.Concat(frames...))
}

// seekHead returns a SeekHead of pairs of IDs and positions.
func seekHead(pairs ...uint64) []byte {
	var seeks [][]byte
	for i := 0; i < len(pairs); i += 2 {
		seeks = append(seeks, el(idSeek, el(idSeekID, idBytes(uint32(pairs[i]))), el(idSeekPosition, pos(pairs[i+1]))))
	}
	return el(idSeekHead, seeks...)
}

// offsets returns the position of each of children in a Segment of them.
func offsets(children ...[]byte) []uint64 {
	var offs []uint64
	var p uint64
	for _, c := range children {
		offs = append(offs, p)
		p += uint64(len(c))
	}
	return offs
}

func TestVint(t *testing.T) {
	for n := 1; n <= 8; n++ {
		largest := uint64(1)<<(7*n) - 2 // all ones is an unknown size
		for _, v := range []uint64{0, 1, largest / 3, largest} {
			b := vint(v, n)
			size, m, err := readSize(append(b, 0xAA))
			if err != nil || m != n || size != int64(v) {
				t.Errorf("size %d on %d bytes: read %d on %d bytes, %v", v, n, size, m, err)
			}
			if _, _, err := readSize(b[:n-1]); err != errShort {
				t.Errorf("%d bytes of a size of %d: %v", n-1, n, err)
			}
		}
		if size, m, err := readSize(vint(1<<(7*n)-1, n)); err != nil || m != n || size != unknownSize {
			t.Errorf("unknown size on %d bytes: %d on %d bytes, %v", n, size, m, err)
		}
	}
	if _, _, err := readVint([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8}); !errors.Is(err, ErrFormat) {
		t.Errorf("a vint of 9 bytes: %v", err)
	}
}

func TestID(t *testing.T) {
	for _, id := range []uint32{idSimpleBlock, idEBMLVersion, idTimestampScale, idEBML} {
		got, n, err := readID(append(idBytes(id), 0x81))
		if err != nil || got != id || n != len(idBytes(id)) {
			t.Errorf("ID %#x: %#x of %d bytes, %v", id, got, n, err)
		}
	}
	for _, b := range [][]byte{{0x08, 1, 2, 3, 4}, {0x80}, {0xFF}, {0x40, 0x00}, {0x7F, 0xFF}, {0x1F, 0xFF, 0xFF, 0xFF}} {
		if _, _, err := readID(b); !errors.Is(err, ErrFormat) {
			t.Errorf("ID % x: %v", b, err)
		}
	}
	if _, _, err := readID([]byte{0x1A, 0x45}); err != errShort {
		t.Errorf("half an ID: %v", err)
	}
}

func TestValues(t *testing.T) {
	for _, c := range []struct {
		b    []byte
		want int64
	}{{nil, 0}, {[]byte{0x7F}, 127}, {[]byte{0xFF}, -1}, {[]byte{0xFF, 0x00}, -256}, {pos(math.MaxUint64), -1}} {
		if v, err := readInt(c.b); err != nil || v != c.want {
			t.Errorf("int % x: %d, %v", c.b, v, err)
		}
	}
	if _, err := readUint(make([]byte, 9)); !errors.Is(err, ErrFormat) {
		t.Errorf("uint of 9 bytes: %v", err)
	}
	if f, err := readFloat([]byte{0x47, 0x3B, 0x80, 0x00}); err != nil || f != 48000 {
		t.Errorf("float32: %v, %v", f, err)
	}
	if f, err := readFloat(pos(math.Float64bits(-1.5))); err != nil || f != -1.5 {
		t.Errorf("float64: %v, %v", f, err)
	}
	if _, err := readFloat([]byte{1, 2, 3}); !errors.Is(err, ErrFormat) {
		t.Errorf("float of 3 bytes: %v", err)
	}
	if s := readString([]byte("webm\x00\x00")); s != "webm" {
		t.Errorf("padded string: %q", s)
	}
}

func TestEachChild(t *testing.T) {
	var ids []uint32
	err := eachChild(slices.Concat(uintEl(idTrackNumber, 1), el(idVoid), strEl(idCodecID, "A_OPUS")), func(id uint32, data []byte) error {
		ids = append(ids, id)
		return nil
	})
	if err != nil || !slices.Equal(ids, []uint32{idTrackNumber, idVoid, idCodecID}) {
		t.Errorf("children %x, %v", ids, err)
	}
	for _, b := range [][]byte{
		{idTrackNumber, 0x82, 1},          // longer than its parent
		{idTrackNumber},                   // no size
		elUnknown(idTrackNumber),          // of unknown size
		{0x0F, 0x81, 0},                   // an invalid ID
		{idTrackNumber, 0x00, 0x81, 0x01}, // an invalid size
	} {
		if err := eachChild(b, func(uint32, []byte) error { return nil }); !errors.Is(err, ErrFormat) {
			t.Errorf("children % x: %v", b, err)
		}
	}
}

func TestSourceSeeksOnlyWhenItMust(t *testing.T) {
	data := make([]byte, 4*bufSize+2*skipLimit)
	for i := range data {
		data[i] = byte(i * 7)
	}
	c := &countingReader{r: bytes.NewReader(data)}
	s := newSource(c)
	check := func(off int64, seeks int) {
		t.Helper()
		s.seekTo(off)
		b, err := s.peek(4)
		if err != nil || !bytes.Equal(b, data[off:off+4]) {
			t.Fatalf("at %d: % x, %v", off, b, err)
		}
		if c.seeks != seeks {
			t.Fatalf("at %d: %d seeks, want %d", off, c.seeks, seeks)
		}
	}
	check(0, 1)                    // the start: the reader may not be there yet
	check(100, 1)                  // in the buffer
	check(bufSize+10, 1)           // a little past it: read on
	check(20, 2)                   // back, out of it: seek
	check(bufSize/2, 2)            // back in it
	check(20+bufSize+skipLimit, 2) // as far ahead as reading on goes
	check(int64(len(data))-4, 3)   // farther: seek

	s.seekTo(int64(len(data)) - 2)
	if _, err := s.peek(4); err != io.ErrUnexpectedEOF {
		t.Errorf("peek over the end: %v", err)
	}
	s.seekTo(int64(len(data)))
	if _, err := s.readHeader(); err != io.EOF {
		t.Errorf("a header at the end: %v", err)
	}
	if c.seeks != 3 {
		t.Errorf("%d seeks at the end", c.seeks)
	}
}

// badReader returns extra bytes more than it is asked to read, or nothing
// and no error when extra is 0.
type badReader struct{ extra int }

func (b badReader) Read(p []byte) (int, error) {
	if b.extra == 0 {
		return 0, nil
	}
	return len(p) + b.extra, nil
}

func (badReader) Seek(int64, int) (int64, error) { return 0, nil }

func TestBadReaders(t *testing.T) {
	for _, c := range []struct {
		r    badReader
		want error
	}{{badReader{1}, errReadCount}, {badReader{-1 << 20}, errReadCount}, {badReader{0}, io.ErrNoProgress}} {
		if _, err := NewReader(c.r); err != c.want {
			t.Errorf("a reader that reads %d more bytes than asked: %v, want %v", c.r.extra, err, c.want)
		}
	}
}
