package fcasttest

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// The fake reads FlatBuffers strictly, as FlatBuffers' verifier does: each
// table, vtable, offset, string and vector aligned and inside the buffer,
// each string ended by its 0. It writes its own as simply.

type ftable struct {
	buf      []byte
	pos      int
	vt, vtsz int
	size     int
}

var errFlat = errors.New("fcasttest: a malformed flatbuffer")

func u16(b []byte, p int) int { return int(binary.LittleEndian.Uint16(b[p:])) }
func u32(b []byte, p int) int { return int(binary.LittleEndian.Uint32(b[p:])) }

func inside(b []byte, p, n int) bool { return p >= 0 && n >= 0 && p+n <= len(b) }

func readTable(b []byte, pos int) (ftable, error) {
	if pos%4 != 0 || !inside(b, pos, 4) {
		return ftable{}, fmt.Errorf("%w: a table at %d", errFlat, pos)
	}
	vt := pos - int(int32(binary.LittleEndian.Uint32(b[pos:])))
	if vt%2 != 0 || !inside(b, vt, 4) {
		return ftable{}, fmt.Errorf("%w: a vtable at %d", errFlat, vt)
	}
	vtsz, size := u16(b, vt), u16(b, vt+2)
	if vtsz < 4 || vtsz%2 != 0 || !inside(b, vt, vtsz) || size < 4 || !inside(b, pos, size) {
		return ftable{}, fmt.Errorf("%w: a vtable of %d, a table of %d", errFlat, vtsz, size)
	}
	return ftable{buf: b, pos: pos, vt: vt, vtsz: vtsz, size: size}, nil
}

func readRoot(b []byte) (ftable, error) {
	if len(b) < 4 {
		return ftable{}, errFlat
	}
	return readTable(b, u32(b, 0))
}

// field returns where slot is, checking it's aligned to and has n bytes.
func (t ftable) field(slot, n int) (int, bool, error) {
	o := 4 + 2*slot
	if o+2 > t.vtsz {
		return 0, false, nil
	}
	off := u16(t.buf, t.vt+o)
	if off == 0 {
		return 0, false, nil
	}
	p := t.pos + off
	if off+n > t.size || p%n != 0 {
		return 0, false, fmt.Errorf("%w: field %d at %d, of %d bytes", errFlat, slot, p, n)
	}
	return p, true, nil
}

func (t ftable) ref(slot int) (int, bool, error) {
	p, ok, err := t.field(slot, 4)
	if !ok || err != nil {
		return 0, ok, err
	}
	to := p + u32(t.buf, p)
	if to%4 != 0 || !inside(t.buf, to, 4) {
		return 0, false, fmt.Errorf("%w: field %d refers to %d", errFlat, slot, to)
	}
	return to, true, nil
}

func (t ftable) u8(slot int) (int, error) {
	p, ok, err := t.field(slot, 1)
	if !ok || err != nil {
		return 0, err
	}
	return int(t.buf[p]), nil
}

func (t ftable) f32(slot int) (float64, bool, error) {
	p, ok, err := t.field(slot, 4)
	if !ok || err != nil {
		return 0, ok, err
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(t.buf[p:]))), true, nil
}

// micros reads a Time struct, which is aligned to its 8 bytes.
func (t ftable) micros(slot int) (int64, bool, error) {
	p, ok, err := t.field(slot, 8)
	if !ok || err != nil {
		return 0, ok, err
	}
	return int64(binary.LittleEndian.Uint64(t.buf[p:])), true, nil
}

func stringAt(b []byte, to int) (string, error) {
	n := u32(b, to)
	if !inside(b, to+4, n+1) || b[to+4+n] != 0 {
		return "", fmt.Errorf("%w: a string at %d", errFlat, to)
	}
	return string(b[to+4 : to+4+n]), nil
}

func (t ftable) str(slot int) (string, error) {
	to, ok, err := t.ref(slot)
	if !ok || err != nil {
		return "", err
	}
	return stringAt(t.buf, to)
}

func (t ftable) table(slot int) (ftable, bool, error) {
	to, ok, err := t.ref(slot)
	if !ok || err != nil {
		return ftable{}, ok, err
	}
	c, err := readTable(t.buf, to)
	return c, err == nil, err
}

// union reads a union's type and value.
func (t ftable) union(slot int) (int, ftable, error) {
	typ, err := t.u8(slot)
	if err != nil || typ == 0 {
		return 0, ftable{}, err
	}
	v, ok, err := t.table(slot + 1)
	if err != nil {
		return 0, ftable{}, err
	}
	if !ok {
		return 0, ftable{}, fmt.Errorf("%w: a union of type %d without its value", errFlat, typ)
	}
	return typ, v, nil
}

// fb writes a table: fields by slot, each a scalar's bytes or a string, a
// vector of strings or a table, laid out the simplest way that keeps
// FlatBuffers' alignment.
type fb map[int]any

// fbScalar is a scalar's or struct's bytes, inline.
type fbScalar []byte

func fu8(v int) fbScalar { return fbScalar{byte(v)} }
func ff32(v float64) fbScalar {
	return binary.LittleEndian.AppendUint32(nil, math.Float32bits(float32(v)))
}
func ftime(us int64) fbScalar { return binary.LittleEndian.AppendUint64(nil, uint64(us)) }

func build(root fb) []byte {
	b := make([]byte, 4)
	pos := writeTable(&b, root)
	binary.LittleEndian.PutUint32(b, uint32(pos))
	return b
}

func pad(b *[]byte, n int) {
	for len(*b)%n != 0 {
		*b = append(*b, 0)
	}
}

func writeTable(b *[]byte, t fb) int {
	slots := 0
	for s := range t {
		slots = max(slots, s+1)
	}
	// Every field takes 8 bytes, each aligned on its own: wasteful, but
	// plain.
	size := 8 + 8*slots
	pad(b, 2)
	vt := len(*b)
	*b = binary.LittleEndian.AppendUint16(*b, uint16(4+2*slots))
	*b = binary.LittleEndian.AppendUint16(*b, uint16(size))
	for s := range slots {
		off := 0
		if _, ok := t[s]; ok {
			off = 8 + 8*s
		}
		*b = binary.LittleEndian.AppendUint16(*b, uint16(off))
	}
	pad(b, 8)
	pos := len(*b)
	*b = append(*b, make([]byte, size)...)
	binary.LittleEndian.PutUint32((*b)[pos:], uint32(pos-vt))
	for s := range slots {
		v, ok := t[s]
		if !ok {
			continue
		}
		at := pos + 8 + 8*s
		switch v := v.(type) {
		case fbScalar:
			copy((*b)[at:], v)
		default:
			to := writeRef(b, v)
			binary.LittleEndian.PutUint32((*b)[at:], uint32(to-at))
		}
	}
	return pos
}

func writeRef(b *[]byte, v any) int {
	switch v := v.(type) {
	case string:
		pad(b, 4)
		at := len(*b)
		*b = binary.LittleEndian.AppendUint32(*b, uint32(len(v)))
		*b = append(append(*b, v...), 0)
		return at
	case []string:
		pad(b, 4)
		at := len(*b)
		*b = binary.LittleEndian.AppendUint32(*b, uint32(len(v)))
		*b = append(*b, make([]byte, 4*len(v))...)
		for i, s := range v {
			el := at + 4 + 4*i
			to := writeRef(b, s)
			binary.LittleEndian.PutUint32((*b)[el:], uint32(to-el))
		}
		return at
	case fb:
		return writeTable(b, v)
	}
	panic(fmt.Sprintf("fcasttest: a field of %T", v))
}
