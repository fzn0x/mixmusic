package mp4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"testing"
)

// box returns a box of the given type and data, of a 32-bit size.
func box(typ string, data ...[]byte) []byte {
	d := slices.Concat(data...)
	return slices.Concat(u32(uint32(8+len(d))), []byte(typ), d)
}

// box64 returns a box of a 64-bit size.
func box64(typ string, data ...[]byte) []byte {
	d := slices.Concat(data...)
	return slices.Concat(u32(1), []byte(typ), u64(uint64(16+len(d))), d)
}

// boxToEnd returns a box of size 0, which runs to the end of its parent.
func boxToEnd(typ string, data ...[]byte) []byte {
	return slices.Concat(u32(0), []byte(typ), slices.Concat(data...))
}

// full returns the data of a full box: its version, its flags and data.
func full(version byte, flags uint32, data ...[]byte) []byte {
	return slices.Concat(u32(uint32(version)<<24|flags), slices.Concat(data...))
}

func u16(vs ...uint16) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.BigEndian.AppendUint16(b, v)
	}
	return b
}

func u32(vs ...uint32) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.BigEndian.AppendUint32(b, v)
	}
	return b
}

func u64(vs ...uint64) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.BigEndian.AppendUint64(b, v)
	}
	return b
}

func TestParseHeader(t *testing.T) {
	uuid := slices.Concat(u32(40), []byte("uuid"), make([]byte, 16), []byte("data"))
	for _, c := range []struct {
		b    []byte
		typ  uint32
		size int64
		n    int
		err  error
	}{
		{box("free", []byte("abc")), typeFree, 3, 8, nil},
		{box64("mdat", []byte("abcd")), typeMdat, 4, 16, nil},
		{boxToEnd("mdat", []byte("abcd")), typeMdat, toEnd, 8, nil},
		{uuid, typeUUID, 16, 24, nil}, // the 16 bytes of the extended type are of the header
		{box("free")[:7], 0, 0, 0, errShort},
		{box64("mdat")[:15], 0, 0, 0, errShort},
		{uuid[:23], 0, 0, 0, errShort},
		{slices.Concat(u32(7), []byte("free")), 0, 0, 0, ErrFormat},
		{slices.Concat(u32(1), []byte("mdat"), u64(15)), 0, 0, 0, ErrFormat},
		{slices.Concat(u32(1), []byte("mdat"), u64(1<<63)), 0, 0, 0, ErrFormat},
		{slices.Concat(u32(20), []byte("uuid"), make([]byte, 16)), 0, 0, 0, ErrFormat},
	} {
		typ, size, n, err := parseHeader(c.b)
		if !errors.Is(err, c.err) || err == nil && (typ != c.typ || size != c.size || n != c.n) {
			t.Errorf("% x: %s, %d, %d, %v; want %s, %d, %d, %v", c.b, typeName(typ), size, n, err, typeName(c.typ), c.size, c.n, c.err)
		}
	}
}

func TestEachBox(t *testing.T) {
	b := slices.Concat(box("free", []byte("a")), box64("skip", []byte("bc")), boxToEnd("mdat", []byte("def")))
	var got []string
	err := eachBox(b, func(typ uint32, data []byte) error {
		got = append(got, typeName(typ)+string(data))
		return nil
	})
	if want := []string{`"free"a`, `"skip"bc`, `"mdat"def`}; err != nil || !slices.Equal(got, want) {
		t.Errorf("%q, %v; want %q", got, err, want)
	}
	// A box that overflows: the boxes before it, then an error.
	for _, b := range [][]byte{
		slices.Concat(box("free", []byte("a")), box("skip", []byte("bc"))[:9]),
		slices.Concat(box("free", []byte("a")), []byte{0, 0, 0}),
	} {
		got = got[:0]
		err = eachBox(b, func(typ uint32, data []byte) error {
			got = append(got, typeName(typ))
			return nil
		})
		if !errors.Is(err, ErrFormat) || !slices.Equal(got, []string{`"free"`}) {
			t.Errorf("% x: %q, %v", b, got, err)
		}
	}
}

func TestFields(t *testing.T) {
	version, flags, f := fullBox(full(1, 0x020001, u16(0xABCD), u32(7), u64(1<<40), []byte{9}))
	if version != 1 || flags != 0x020001 {
		t.Errorf("version %d, flags %#x", version, flags)
	}
	if a, b, c, d := f.u16(), f.u32(), f.u64(), f.u8(); a != 0xABCD || b != 7 || c != 1<<40 || d != 9 || f.check(typeFree) != nil {
		t.Errorf("%#x %d %d %d, %v", a, b, c, d, f.check(typeFree))
	}
	if v := f.u32(); v != 0 || !errors.Is(f.check(typeFree), ErrFormat) {
		t.Errorf("past the end: %d, %v", v, f.check(typeFree))
	}
	f = fields{b: u32(0xFFFFFFFF, 5)}
	if d, e := f.duration(false), f.duration(false); d != 0 || e != 5 {
		t.Errorf("durations %d, %d: all ones is unknown", d, e)
	}
	f = fields{b: u64(1<<64-1, 6)}
	if d, e := f.duration(true), f.duration(true); d != 0 || e != 6 {
		t.Errorf("64-bit durations %d, %d", d, e)
	}
}

func TestDescriptor(t *testing.T) {
	for _, c := range []struct {
		b         []byte
		tag       byte
		data      string
		rest      string
		malformed bool
	}{
		{[]byte{5, 2, 'a', 'b', 'c'}, 5, "ab", "c", false},
		{[]byte{5, 0x80, 0x80, 0x80, 2, 'a', 'b'}, 5, "ab", "", false}, // as ffmpeg writes sizes
		{[]byte{4, 0x81, 0x00}, 0, "", "", true},                       // 128 bytes, of none
		{[]byte{4, 0x80, 0x80, 0x80, 0x80, 1, 'a'}, 0, "", "", true},   // a size of 5 bytes
		{[]byte{4}, 0, "", "", true},
		{[]byte{4, 0x80}, 0, "", "", true},
	} {
		tag, data, rest, err := descriptor(c.b)
		if c.malformed != (err != nil) || err == nil && (tag != c.tag || string(data) != c.data || string(rest) != c.rest) {
			t.Errorf("% x: %d %q %q, %v", c.b, tag, data, rest, err)
		}
		if err != nil && !errors.Is(err, ErrFormat) {
			t.Errorf("% x: %v", c.b, err)
		}
	}
}

func TestAudioConfig(t *testing.T) {
	for _, c := range []struct {
		asc  []byte
		want audioConfig
		ok   bool
	}{
		{[]byte{0x12, 0x10}, audioConfig{2, 44100, 2}, true},
		{[]byte{0x12, 0x10, 0x56, 0xE5, 0x00}, audioConfig{2, 44100, 2}, true},                // ffmpeg's, with a sync extension
		{slices.Concat([]byte{0x12, 0x10}, make([]byte, 14)), audioConfig{2, 44100, 2}, true}, // YouTube's, padded to 16 bytes
		{[]byte{0x11, 0x88}, audioConfig{2, 48000, 1}, true},
		{[]byte{0x13, 0x88}, audioConfig{2, 22050, 1}, true},
		{[]byte{0x12, 0x38}, audioConfig{2, 44100, 8}, true},                  // channelConfiguration 7
		{[]byte{0x12, 0x00}, audioConfig{2, 44100, 0}, true},                  // channels in the stream
		{[]byte{0x17, 0x80, 0x01, 0xF4, 0x10}, audioConfig{2, 1000, 2}, true}, // an explicit frequency
		{[]byte{0x2B, 0x92, 0x08, 0x00}, audioConfig{5, 44100, 2}, true},      // HE-AAC: 22050 Hz, doubled
		{[]byte{0xF9, 0x46, 0x40}, audioConfig{42, 48000, 2}, true},           // an escaped type
		{[]byte{0x12}, audioConfig{}, false},
		{nil, audioConfig{}, false},
	} {
		got, ok := parseAudioConfig(c.asc)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("% x: %+v, %v; want %+v, %v", c.asc, got, ok, c.want, c.ok)
		}
	}
}

func TestRescale(t *testing.T) {
	for _, c := range []struct {
		v        uint64
		from, to uint32
		limit    int64
		want     int64
		ok       bool
	}{
		{3000, 1000, 44100, 1 << 40, 132300, true},
		{1, 3, 2, 10, 0, true}, // rounded down
		{1<<64 - 1, 1, 1 << 31, 1 << 62, 0, false},
		{100, 0, 44100, 1 << 40, 0, false},
		{100, 1, 44100, 4410000 - 1, 4410000, false},
	} {
		got, ok := rescale(c.v, c.from, c.to, c.limit)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("rescale(%d, %d, %d): %d, %v", c.v, c.from, c.to, got, ok)
		}
	}
}

// TestTicks checks that a time converted to ticks is the last tick whose
// time is at or before it, as Seek needs, at every timescale of AAC.
func TestTicks(t *testing.T) {
	for _, scale := range []int64{1000, 7350, 22050, 44100, 48000, 90000, 1<<32 - 1} {
		r := &Reader{scale: scale, maxTicks: maxSeconds * scale}
		for _, k := range []int64{0, 1, 1023, 1024, 1025, 6946240, 1 << 33, r.maxTicks - 1} {
			at := r.time(k)
			if r.ticks(at) < k || r.time(r.ticks(at)) > at || r.time(r.ticks(at)+1) <= at {
				t.Errorf("timescale %d, tick %d at %v: ticks(%v) = %d", scale, k, at, at, r.ticks(at))
			}
			if k > 0 && r.ticks(at-1) >= k {
				t.Errorf("timescale %d, tick %d: ticks(%v) = %d", scale, k, at-1, r.ticks(at-1))
			}
		}
		if r.ticks(-1) != -1 || r.ticks(1<<62) != r.maxTicks {
			t.Errorf("timescale %d: %d, %d", scale, r.ticks(-1), r.ticks(1<<62))
		}
	}
}

// TestSourceHeaders reads box headers through a source, at the end of the
// file and within one.
func TestSourceHeaders(t *testing.T) {
	file := slices.Concat(box("ftyp", []byte("isom")), box64("free", []byte("x")), box("mdat", []byte("abc")))
	s := newSource(bytes.NewReader(file))
	var types []string
	for {
		h, err := s.readHeader()
		if err != nil {
			if err.Error() != "EOF" {
				t.Fatal(err)
			}
			break
		}
		types = append(types, typeName(h.typ))
		s.skip(h.size)
	}
	if want := []string{`"ftyp"`, `"free"`, `"mdat"`}; !slices.Equal(types, want) {
		t.Errorf("%q", types)
	}
	for n := 1; n < 8; n++ {
		s := newSource(bytes.NewReader(file[:n]))
		if _, err := s.readHeader(); err.Error() != "unexpected EOF" {
			t.Errorf("a file of %d bytes: %v", n, err)
		}
	}
}
