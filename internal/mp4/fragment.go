package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
)

// The flags of a tfhd.
const (
	tfhdBaseOffset    = 0x000001
	tfhdDescription   = 0x000002
	tfhdDuration      = 0x000008
	tfhdSize          = 0x000010
	tfhdDefaultBaseIs = 0x020000 // default-base-is-moof
)

// The flags of a trun: the fields it has, and those each of its samples
// has.
const (
	trunDataOffset  = 0x000001
	trunFirstFlags  = 0x000004
	trunDuration    = 0x000100
	trunSize        = 0x000200
	trunFlags       = 0x000400
	trunCompOffsets = 0x000800
)

// fragment is the samples of the track in a moof, those of its truns,
// which a cursor walks.
type fragment struct {
	runs       []run
	ri         int   // the run of the next sample
	start, end int64 // the time of its first sample and after its last, in ticks
}

// run is a trun of the track: samples whose data follow each other.
type run struct {
	count    uint32
	pos      int64  // the offset of the data of its first sample
	time     int64  // the time of its first sample, in ticks
	entries  []byte // the fields of its samples, of stride bytes each
	stride   int
	durAt    int    // the offset of the duration in the fields, or -1 when it is the default
	sizeAt   int    // that of the size, likewise
	duration uint32 // the defaults
	size     uint32

	// The cursor: the next sample, the offset of its data, and its time.
	n  uint32
	at int64
	t  int64
}

// fields returns the duration and the size of sample n.
func (u *run) fields(n uint32) (duration, size uint32) {
	duration, size = u.duration, u.size
	e := u.entries[int(n)*u.stride:]
	if u.durAt >= 0 {
		duration = binary.BigEndian.Uint32(e[u.durAt:])
	}
	if u.sizeAt >= 0 {
		size = binary.BigEndian.Uint32(e[u.sizeAt:])
	}
	return duration, size
}

func (u *run) rewind() { u.n, u.at, u.t = 0, u.pos, u.time }

// next returns the sample at the cursor, and moves the cursor to the
// next.
func (u *run) next() (sample, bool) {
	if u.n >= u.count {
		return sample{}, false
	}
	d, size := u.fields(u.n)
	s := sample{pos: u.at, size: size, time: u.t, duration: d}
	u.n++
	u.at += int64(size)
	u.t += int64(d)
	return s, true
}

// next returns the sample at the cursor, and moves the cursor to the
// next.
func (f *fragment) next() (sample, bool) {
	for ; f.ri < len(f.runs); f.ri++ {
		if s, ok := f.runs[f.ri].next(); ok {
			return s, true
		}
	}
	return sample{}, false
}

// parseMoof reads the samples of the track in the moof at offset at, of
// data b. Without a tfdt, they start at tick start.
func (r *Reader) parseMoof(at int64, b []byte, start int64) (fragment, error) {
	f := fragment{start: start, end: start}
	found := false
	// A traf without a base offset of its own starts at the moof, the first,
	// or after the data of the traf before.
	base := at
	err := eachBox(b, func(typ uint32, data []byte) error {
		if typ != typeTraf {
			return nil
		}
		var err error
		base, err = r.parseTraf(data, at, base, &f, &found)
		return err
	})
	return f, err
}

// parseTraf reads a traf of the moof at offset moof, whose data start at
// base unless it says otherwise, adding the runs of the track to f. It
// returns the offset after its data.
func (r *Reader) parseTraf(b []byte, moof, base int64, f *fragment, found *bool) (int64, error) {
	tfhd, err := findBox(b, typeTfhd)
	switch {
	case err != nil:
		return 0, err
	case tfhd == nil:
		return 0, fmt.Errorf("%w: traf without tfhd", ErrFormat)
	}
	_, flags, h := fullBox(tfhd)
	id := h.u32()
	defaults := r.trex[id]
	switch {
	case flags&tfhdBaseOffset != 0:
		v := h.u64()
		if v > maxPosition {
			return 0, fmt.Errorf("%w: traf data at byte %d", ErrFormat, v)
		}
		base = int64(v)
	case flags&tfhdDefaultBaseIs != 0:
		base = moof
	}
	if flags&tfhdDescription != 0 {
		h.skip(4)
	}
	if flags&tfhdDuration != 0 {
		defaults.duration = h.u32()
	}
	if flags&tfhdSize != 0 {
		defaults.size = h.u32()
	}
	if err := h.check(typeTfhd); err != nil {
		return 0, err
	}
	mine := id == r.track.ID

	t := f.end
	if mine {
		tfdt, err := findBox(b, typeTfdt)
		if err != nil {
			return 0, err
		}
		if tfdt != nil {
			v, _, d := fullBox(tfdt)
			start := d.uint(v == 1)
			if err := d.check(typeTfdt); err != nil {
				return 0, err
			}
			if start > uint64(r.maxTicks) {
				return 0, fmt.Errorf("%w: fragment at tick %d", ErrFormat, start)
			}
			t = int64(start)
		}
		if !*found {
			f.start, f.end, *found = t, t, true
		}
	}

	end := base
	err = eachBox(b, func(typ uint32, data []byte) error {
		if typ != typeTrun {
			return nil
		}
		u, err := parseTrun(data, base, end, t, defaults)
		if err != nil {
			return err
		}
		size, duration := u.totals()
		if end = u.pos + size; end > maxPosition {
			return fmt.Errorf("%w: trun data past byte %d", ErrFormat, int64(maxPosition))
		}
		if !mine {
			return nil
		}
		if duration > uint64(r.maxTicks-t) {
			return fmt.Errorf("%w: samples past tick %d", ErrFormat, r.maxTicks)
		}
		t += int64(duration)
		f.runs = append(f.runs, u)
		f.end = t
		return nil
	})
	return end, err
}

// parseTrun reads a trun of a traf whose data start at base, where the
// data of the trun before end, at tick t.
func parseTrun(b []byte, base, end, t int64, defaults trex) (run, error) {
	_, flags, f := fullBox(b)
	u := run{count: f.u32(), pos: end, time: t, durAt: -1, sizeAt: -1, duration: defaults.duration, size: defaults.size}
	if flags&trunDataOffset != 0 {
		u.pos = base + int64(int32(f.u32()))
	}
	if flags&trunFirstFlags != 0 {
		f.skip(4)
	}
	if err := f.check(typeTrun); err != nil {
		return run{}, err
	}
	if u.pos < 0 || u.pos > maxPosition {
		return run{}, fmt.Errorf("%w: trun data at byte %d", ErrFormat, u.pos)
	}
	if flags&trunDuration != 0 {
		u.durAt = u.stride
		u.stride += 4
	}
	if flags&trunSize != 0 {
		u.sizeAt = u.stride
		u.stride += 4
	}
	if flags&trunFlags != 0 {
		u.stride += 4
	}
	if flags&trunCompOffsets != 0 {
		u.stride += 4
	}
	if u.stride > 0 && uint64(u.count) > uint64(len(f.b)/u.stride) {
		return run{}, fmt.Errorf("%w: trun of %d samples in %d bytes", ErrFormat, u.count, len(f.b))
	}
	if u.sizeAt < 0 && u.count > 0 && u.size > maxBox {
		return run{}, fmt.Errorf("%w: samples of %d bytes", ErrFormat, u.size)
	}
	u.entries = f.b[:int(u.count)*u.stride]
	u.rewind()
	return u, nil
}

// totals returns the size of the data of the samples of the run, and
// their duration.
func (u *run) totals() (size int64, duration uint64) {
	if u.durAt < 0 && u.sizeAt < 0 {
		return int64(u.count) * int64(u.size), uint64(u.count) * uint64(u.duration)
	}
	for n := range u.count {
		d, s := u.fields(n)
		size += int64(s)
		duration += uint64(d)
	}
	return size, duration
}

// readFragment reads the first moof from offset at on, skipping the boxes
// before it, and returns the samples of the track it describes, starting
// at tick start without a tfdt, and the offset of the box after its mdat.
// It returns false at the end of the fragments: at the end of the file, or
// of a file chained after.
func (r *Reader) readFragment(at, start int64) (fragment, int64, bool, error) {
	if at < 0 {
		return fragment{}, -1, false, nil
	}
	s := r.src
	s.seekTo(at)
	for {
		h, err := s.readHeader()
		if err == io.EOF {
			return fragment{}, -1, false, nil
		}
		if err != nil {
			return fragment{}, -1, false, err
		}
		switch h.typ {
		case typeMoof:
			b, err := s.readBody(h)
			if err != nil {
				return fragment{}, -1, false, err
			}
			f, err := r.parseMoof(h.start, b, start)
			if err != nil {
				return fragment{}, -1, false, err
			}
			after := h.end()
			// The samples are in the mdat after the moof, which the next
			// fragment comes after.
			if after >= 0 {
				if next, err := s.readHeader(); err == nil && next.typ == typeMdat {
					after = next.end()
				}
			}
			return f, after, true, nil
		case typeFtyp, typeMoov:
			return fragment{}, -1, false, nil
		}
		if h.size == toEnd {
			return fragment{}, -1, false, nil
		}
		s.skip(h.size)
	}
}

// nextFragment reads the fragment after the one read.
func (r *Reader) nextFragment() error {
	f, after, ok, err := r.readFragment(r.after, r.frag.end)
	if err != nil {
		return err
	}
	if !ok {
		return io.EOF
	}
	r.frag, r.after = f, after
	return nil
}
