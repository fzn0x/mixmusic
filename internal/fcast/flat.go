package fcast

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"time"
)

// Protocol v4 sends its messages as FlatBuffers. What MixMusic sends and
// reads of them is a few tables of strings, scalars, the Time struct and
// unions, which this writes and reads by itself: each offset aligned as
// FlatBuffers' verifier asks, every table's vtable before it, and what a
// table refers to after it.

// table is a FlatBuffers table to write: its fields by slot, the order of
// the schema, a union taking two, its type and then its value.
type table struct {
	fields []field
}

// field is a field of a table: a scalar's or a struct's bytes, inline, or
// what it refers to.
type field struct {
	slot  int
	bytes []byte
	ref   any // string, []string or *table
}

func (t *table) put(slot int, b []byte) *table {
	t.fields = append(t.fields, field{slot: slot, bytes: b})
	return t
}

func (t *table) u8(slot int, v uint8) *table { return t.put(slot, []byte{v}) }

func (t *table) f32(slot int, v float32) *table {
	return t.put(slot, binary.LittleEndian.AppendUint32(nil, math.Float32bits(v)))
}

// time writes the Time struct, of its microseconds.
func (t *table) time(slot int, d time.Duration) *table {
	return t.put(slot, binary.LittleEndian.AppendUint64(nil, uint64(max(d, 0)/time.Microsecond)))
}

func (t *table) str(slot int, s string) *table {
	if s == "" {
		return t
	}
	t.fields = append(t.fields, field{slot: slot, ref: s})
	return t
}

func (t *table) strs(slot int, s []string) *table {
	t.fields = append(t.fields, field{slot: slot, ref: s})
	return t
}

func (t *table) table(slot int, c *table) *table {
	t.fields = append(t.fields, field{slot: slot, ref: c})
	return t
}

// union writes a union of the slots slot, its type typ, and slot+1, its
// value v.
func (t *table) union(slot int, typ uint8, v *table) *table { return t.u8(slot, typ).table(slot+1, v) }

// finish writes t as a buffer whose root it is.
func (t *table) finish() []byte {
	w := &flatWriter{buf: make([]byte, 4)}
	root := w.table(t)
	binary.LittleEndian.PutUint32(w.buf, uint32(root))
	return w.buf
}

type flatWriter struct{ buf []byte }

func (w *flatWriter) align(n int) {
	for len(w.buf)%n != 0 {
		w.buf = append(w.buf, 0)
	}
}

func (w *flatWriter) u32(v uint32) { w.buf = binary.LittleEndian.AppendUint32(w.buf, v) }

// table writes t, its vtable before it and what it refers to after it, and
// returns where it is.
func (w *flatWriter) table(t *table) int {
	fields := slices.Clone(t.fields)
	// The largest first, each aligned after the soffset.
	slices.SortStableFunc(fields, func(a, b field) int { return size(b) - size(a) })
	offs := make([]int, len(fields))
	end, align := 4, 4
	slots := 0
	for i, f := range fields {
		n := size(f)
		for end%n != 0 {
			end++
		}
		offs[i] = end
		end += n
		align = max(align, n)
		slots = max(slots, f.slot+1)
	}
	w.align(2)
	vt := len(w.buf)
	vtable := make([]byte, 4+2*slots)
	binary.LittleEndian.PutUint16(vtable, uint16(len(vtable)))
	binary.LittleEndian.PutUint16(vtable[2:], uint16(end))
	for i, f := range fields {
		binary.LittleEndian.PutUint16(vtable[4+2*f.slot:], uint16(offs[i]))
	}
	w.buf = append(w.buf, vtable...)
	w.align(align)
	at := len(w.buf)
	w.buf = append(w.buf, make([]byte, end)...)
	binary.LittleEndian.PutUint32(w.buf[at:], uint32(int32(at-vt)))
	for i, f := range fields {
		if f.ref == nil {
			copy(w.buf[at+offs[i]:], f.bytes)
		}
	}
	for i, f := range fields {
		if f.ref == nil {
			continue
		}
		pos := at + offs[i]
		target := w.ref(f.ref)
		binary.LittleEndian.PutUint32(w.buf[pos:], uint32(target-pos))
	}
	return at
}

// size is the inline size of a field: an offset's 4.
func size(f field) int {
	if f.ref != nil {
		return 4
	}
	return len(f.bytes)
}

// ref writes what a field refers to, and returns where it is.
func (w *flatWriter) ref(v any) int {
	switch v := v.(type) {
	case string:
		w.align(4)
		at := len(w.buf)
		w.u32(uint32(len(v)))
		w.buf = append(w.buf, v...)
		w.buf = append(w.buf, 0)
		return at
	case []string:
		w.align(4)
		at := len(w.buf)
		w.u32(uint32(len(v)))
		w.buf = append(w.buf, make([]byte, 4*len(v))...)
		for i, s := range v {
			elem := at + 4 + 4*i
			target := w.ref(s)
			binary.LittleEndian.PutUint32(w.buf[elem:], uint32(target-elem))
		}
		return at
	case *table:
		return w.table(v)
	}
	panic("fcast: a field of no kind")
}

// errFlat is why a FlatBuffers message isn't read: it isn't whole, or
// points outside itself.
var errFlat = errors.New("fcast: a malformed message")

// tableRef is a table of a buffer that is read.
type tableRef struct {
	buf []byte
	pos int
	vt  int
	vtn int
}

// rootOf returns the root table of buf.
func rootOf(buf []byte) (tableRef, error) {
	if len(buf) < 4 {
		return tableRef{}, errFlat
	}
	return tableAt(buf, int(binary.LittleEndian.Uint32(buf)))
}

func tableAt(buf []byte, pos int) (tableRef, error) {
	if pos < 0 || pos+4 > len(buf) {
		return tableRef{}, errFlat
	}
	vt := pos - int(int32(binary.LittleEndian.Uint32(buf[pos:])))
	if vt < 0 || vt+4 > len(buf) {
		return tableRef{}, errFlat
	}
	vtn := int(binary.LittleEndian.Uint16(buf[vt:]))
	if vtn < 4 || vtn%2 != 0 || vt+vtn > len(buf) {
		return tableRef{}, errFlat
	}
	return tableRef{buf: buf, pos: pos, vt: vt, vtn: vtn}, nil
}

// at returns where the field of slot is, n bytes of it, or false when the
// table doesn't have it.
func (t tableRef) at(slot, n int) (int, bool) {
	o := 4 + 2*slot
	if t.buf == nil || o+2 > t.vtn {
		return 0, false
	}
	off := int(binary.LittleEndian.Uint16(t.buf[t.vt+o:]))
	if off == 0 || t.pos+off+n > len(t.buf) {
		return 0, false
	}
	return t.pos + off, true
}

func (t tableRef) u8(slot int) (uint8, bool) {
	p, ok := t.at(slot, 1)
	if !ok {
		return 0, false
	}
	return t.buf[p], true
}

func (t tableRef) f32(slot int) (float32, bool) {
	p, ok := t.at(slot, 4)
	if !ok {
		return 0, false
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(t.buf[p:])), true
}

func (t tableRef) u32(slot int) (uint32, bool) {
	p, ok := t.at(slot, 4)
	if !ok {
		return 0, false
	}
	return binary.LittleEndian.Uint32(t.buf[p:]), true
}

// time reads the Time struct of slot.
func (t tableRef) time(slot int) (time.Duration, bool) {
	p, ok := t.at(slot, 8)
	if !ok {
		return 0, false
	}
	us := binary.LittleEndian.Uint64(t.buf[p:])
	if us > uint64(math.MaxInt64/int64(time.Microsecond)) {
		return 0, false
	}
	return time.Duration(us) * time.Microsecond, true
}

// target returns where the offset at p points.
func (t tableRef) target(p int) (int, bool) {
	if p+4 > len(t.buf) {
		return 0, false
	}
	to := p + int(binary.LittleEndian.Uint32(t.buf[p:]))
	if to < p || to >= len(t.buf) {
		return 0, false
	}
	return to, true
}

func (t tableRef) strAt(p int) (string, bool) {
	to, ok := t.target(p)
	if !ok || to+4 > len(t.buf) {
		return "", false
	}
	n := int(binary.LittleEndian.Uint32(t.buf[to:]))
	if n < 0 || to+4+n > len(t.buf) {
		return "", false
	}
	return string(t.buf[to+4 : to+4+n]), true
}

func (t tableRef) str(slot int) string {
	p, ok := t.at(slot, 4)
	if !ok {
		return ""
	}
	s, _ := t.strAt(p)
	return s
}

func (t tableRef) strs(slot int) []string {
	p, ok := t.at(slot, 4)
	if !ok {
		return nil
	}
	to, ok := t.target(p)
	if !ok || to+4 > len(t.buf) {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(t.buf[to:]))
	if n < 0 || n > (len(t.buf)-to-4)/4 {
		return nil
	}
	out := make([]string, 0, n)
	for i := range n {
		if s, ok := t.strAt(to + 4 + 4*i); ok {
			out = append(out, s)
		}
	}
	return out
}

func (t tableRef) table(slot int) (tableRef, bool) {
	p, ok := t.at(slot, 4)
	if !ok {
		return tableRef{}, false
	}
	to, ok := t.target(p)
	if !ok {
		return tableRef{}, false
	}
	c, err := tableAt(t.buf, to)
	return c, err == nil
}

// tables reads a vector of tables.
func (t tableRef) tables(slot int) []tableRef {
	p, ok := t.at(slot, 4)
	if !ok {
		return nil
	}
	to, ok := t.target(p)
	if !ok || to+4 > len(t.buf) {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(t.buf[to:]))
	if n < 0 || n > (len(t.buf)-to-4)/4 {
		return nil
	}
	var out []tableRef
	for i := range n {
		if c, ok := t.target(to + 4 + 4*i); ok {
			if r, err := tableAt(t.buf, c); err == nil {
				out = append(out, r)
			}
		}
	}
	return out
}

// union reads the union of slot and slot+1: its type, 0 when it has none,
// and its table.
func (t tableRef) union(slot int) (uint8, tableRef) {
	typ, _ := t.u8(slot)
	if typ == 0 {
		return 0, tableRef{}
	}
	v, ok := t.table(slot + 1)
	if !ok {
		return 0, tableRef{}
	}
	return typ, v
}
