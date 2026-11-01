package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// The types of the boxes the Reader reads, as ISO/IEC 14496-12 and 14496-14
// name them: their four characters, as a big-endian number.
const (
	typeFtyp = 'f'<<24 | 't'<<16 | 'y'<<8 | 'p'
	typeStyp = 's'<<24 | 't'<<16 | 'y'<<8 | 'p'
	typeMoov = 'm'<<24 | 'o'<<16 | 'o'<<8 | 'v'
	typeMdat = 'm'<<24 | 'd'<<16 | 'a'<<8 | 't'
	typeFree = 'f'<<24 | 'r'<<16 | 'e'<<8 | 'e'
	typeSkip = 's'<<24 | 'k'<<16 | 'i'<<8 | 'p'
	typeWide = 'w'<<24 | 'i'<<16 | 'd'<<8 | 'e'
	typePdin = 'p'<<24 | 'd'<<16 | 'i'<<8 | 'n'
	typeUUID = 'u'<<24 | 'u'<<16 | 'i'<<8 | 'd'
	typeSidx = 's'<<24 | 'i'<<16 | 'd'<<8 | 'x'
	typeMoof = 'm'<<24 | 'o'<<16 | 'o'<<8 | 'f'
	typeMfra = 'm'<<24 | 'f'<<16 | 'r'<<8 | 'a'
	typeSsix = 's'<<24 | 's'<<16 | 'i'<<8 | 'x'
	typePrft = 'p'<<24 | 'r'<<16 | 'f'<<8 | 't'
	typeEmsg = 'e'<<24 | 'm'<<16 | 's'<<8 | 'g'

	typeMvhd = 'm'<<24 | 'v'<<16 | 'h'<<8 | 'd'
	typeTrak = 't'<<24 | 'r'<<16 | 'a'<<8 | 'k'
	typeTkhd = 't'<<24 | 'k'<<16 | 'h'<<8 | 'd'
	typeEdts = 'e'<<24 | 'd'<<16 | 't'<<8 | 's'
	typeElst = 'e'<<24 | 'l'<<16 | 's'<<8 | 't'
	typeMdia = 'm'<<24 | 'd'<<16 | 'i'<<8 | 'a'
	typeMdhd = 'm'<<24 | 'd'<<16 | 'h'<<8 | 'd'
	typeHdlr = 'h'<<24 | 'd'<<16 | 'l'<<8 | 'r'
	typeMinf = 'm'<<24 | 'i'<<16 | 'n'<<8 | 'f'
	typeStbl = 's'<<24 | 't'<<16 | 'b'<<8 | 'l'
	typeStsd = 's'<<24 | 't'<<16 | 's'<<8 | 'd'
	typeStts = 's'<<24 | 't'<<16 | 't'<<8 | 's'
	typeStsz = 's'<<24 | 't'<<16 | 's'<<8 | 'z'
	typeStz2 = 's'<<24 | 't'<<16 | 'z'<<8 | '2'
	typeStsc = 's'<<24 | 't'<<16 | 's'<<8 | 'c'
	typeStco = 's'<<24 | 't'<<16 | 'c'<<8 | 'o'
	typeCo64 = 'c'<<24 | 'o'<<16 | '6'<<8 | '4'
	typeMvex = 'm'<<24 | 'v'<<16 | 'e'<<8 | 'x'
	typeMehd = 'm'<<24 | 'e'<<16 | 'h'<<8 | 'd'
	typeTrex = 't'<<24 | 'r'<<16 | 'e'<<8 | 'x'

	typeMp4a = 'm'<<24 | 'p'<<16 | '4'<<8 | 'a'
	typeEnca = 'e'<<24 | 'n'<<16 | 'c'<<8 | 'a'
	typeEsds = 'e'<<24 | 's'<<16 | 'd'<<8 | 's'
	typeWave = 'w'<<24 | 'a'<<16 | 'v'<<8 | 'e'
	typeAc3  = 'a'<<24 | 'c'<<16 | '-'<<8 | '3'
	typeEc3  = 'e'<<24 | 'c'<<16 | '-'<<8 | '3'
	typeDac3 = 'd'<<24 | 'a'<<16 | 'c'<<8 | '3'
	typeDec3 = 'd'<<24 | 'e'<<16 | 'c'<<8 | '3'

	typeTraf = 't'<<24 | 'r'<<16 | 'a'<<8 | 'f'
	typeTfhd = 't'<<24 | 'f'<<16 | 'h'<<8 | 'd'
	typeTfdt = 't'<<24 | 'f'<<16 | 'd'<<8 | 't'
	typeTrun = 't'<<24 | 'r'<<16 | 'u'<<8 | 'n'

	// handlerSound is the handler_type of the hdlr of audio tracks.
	handlerSound = 's'<<24 | 'o'<<16 | 'u'<<8 | 'n'
)

// maxHeaderLen is the length of the longest box header: a 32-bit size of
// 1, the type, a 64-bit size and the 16-byte extended type of a uuid box.
const maxHeaderLen = 32

// toEnd is the size of a box that runs to the end of the file, or of its
// parent, as its size 0 says: an mdat written by a muxer that could not
// go back to write its size.
const toEnd = -1

var (
	// errShort is returned by parseHeader given too few bytes.
	errShort = errors.New("mp4: short header")

	errOverflow = fmt.Errorf("%w: a box overflows its parent", ErrFormat)
)

// typeName returns the four characters of a box type, for errors.
func typeName(typ uint32) string {
	return fmt.Sprintf("%q", binary.BigEndian.AppendUint32(nil, typ))
}

// parseHeader reads the header of a box from the start of b: its type,
// the size of its data, or toEnd, and the length of the header.
func parseHeader(b []byte) (typ uint32, size int64, n int, err error) {
	if len(b) < 8 {
		return 0, 0, 0, errShort
	}
	size32 := binary.BigEndian.Uint32(b)
	typ = binary.BigEndian.Uint32(b[4:])
	n = 8
	total := int64(size32)
	if size32 == 1 {
		// A 64-bit size follows the type.
		if len(b) < 16 {
			return 0, 0, 0, errShort
		}
		v := binary.BigEndian.Uint64(b[8:])
		if v > math.MaxInt64 {
			return 0, 0, 0, fmt.Errorf("%w: box %s of %d bytes", ErrFormat, typeName(typ), v)
		}
		total, n = int64(v), 16
	}
	if typ == typeUUID {
		// The extended type, which no box the Reader reads has.
		if len(b) < n+16 {
			return 0, 0, 0, errShort
		}
		n += 16
	}
	if size32 == 0 {
		return typ, toEnd, n, nil
	}
	if total < int64(n) {
		return 0, 0, 0, fmt.Errorf("%w: box %s of %d bytes is shorter than its header", ErrFormat, typeName(typ), total)
	}
	return typ, total - int64(n), n, nil
}

// eachBox calls f with the type and data of each box of b, in order, up to
// an error. A box of size 0 runs to the end of b. A box that overflows b
// makes errOverflow, once f has had the boxes before it.
func eachBox(b []byte, f func(typ uint32, data []byte) error) error {
	for len(b) > 0 {
		typ, size, n, err := parseHeader(b)
		if err == errShort {
			return errOverflow
		}
		if err != nil {
			return err
		}
		if size == toEnd {
			size = int64(len(b) - n)
		}
		if size > int64(len(b)-n) {
			return errOverflow
		}
		end := n + int(size)
		if err := f(typ, b[n:end:end]); err != nil {
			return err
		}
		b = b[end:]
	}
	return nil
}

// findBox returns the data of the first box of type typ in b, or nil.
func findBox(b []byte, typ uint32) ([]byte, error) {
	var found []byte
	err := eachBox(b, func(t uint32, data []byte) error {
		if t == typ && found == nil {
			found = data
		}
		return nil
	})
	return found, err
}

// fields reads the big-endian fields of the data of a box, in order.
// Reading past the end of the data gives zeros, and sets short.
type fields struct {
	b     []byte
	short bool
}

// fullBox returns the version and the flags of the data of a full box,
// and the fields after them.
func fullBox(b []byte) (version byte, flags uint32, f fields) {
	f.b = b
	v := f.u32()
	return byte(v >> 24), v & 0xFFFFFF, f
}

// next returns the next n bytes, or nil when fewer are left.
func (f *fields) next(n int) []byte {
	if n < 0 || n > len(f.b) {
		f.b, f.short = nil, true
		return nil
	}
	b := f.b[:n:n]
	f.b = f.b[n:]
	return b
}

func (f *fields) skip(n int) { f.next(n) }

func (f *fields) u8() uint8 {
	if b := f.next(1); len(b) == 1 {
		return b[0]
	}
	return 0
}

func (f *fields) u16() uint16 {
	if b := f.next(2); len(b) == 2 {
		return binary.BigEndian.Uint16(b)
	}
	return 0
}

func (f *fields) u32() uint32 {
	if b := f.next(4); len(b) == 4 {
		return binary.BigEndian.Uint32(b)
	}
	return 0
}

func (f *fields) u64() uint64 {
	if b := f.next(8); len(b) == 8 {
		return binary.BigEndian.Uint64(b)
	}
	return 0
}

// uint reads an unsigned integer of 8 bytes when wide, as version 1 of
// a box has them, or else of 4.
func (f *fields) uint(wide bool) uint64 {
	if wide {
		return f.u64()
	}
	return uint64(f.u32())
}

// duration reads a duration as uint does, which is 0 when unknown: all
// ones, as ISO/IEC 14496-12 has it.
func (f *fields) duration(wide bool) uint64 {
	v := f.uint(wide)
	if wide && v == math.MaxUint64 || !wide && v == math.MaxUint32 {
		return 0
	}
	return v
}

// check returns an ErrFormat when the fields of the box of type typ ran
// past its data.
func (f *fields) check(typ uint32) error {
	if f.short {
		return fmt.Errorf("%w: box %s too short", ErrFormat, typeName(typ))
	}
	return nil
}
