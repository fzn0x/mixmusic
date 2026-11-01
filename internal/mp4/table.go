package mp4

import (
	"encoding/binary"
	"fmt"
)

// table is the sample table of the track, of its stbl: when each sample
// is, its size, and the chunk of the file its data are in. A cursor walks
// it, sample by sample. The table keeps the stsz and stco as the file
// has them, and reads them as it goes, so that it takes no more memory
// than the moov.
type table struct {
	count    uint32     // the samples it describes
	end      int64      // the time after the last, in ticks
	times    []timeRun  // of the stts
	size     uint32     // of every sample, or 0 when sizes gives them
	sizes    []byte     // of each sample, in sizeBits bits
	sizeBits int        // 32 for an stsz; 4, 8 or 16 for an stz2
	chunks   []chunkRun // of the stsc
	offsets  []byte     // of each chunk, in 4 bytes, or 8 of a co64
	wide     bool
	nchunks  uint32

	// The cursor: the next sample, its time and the offset of its data,
	// the entry of the stts it is in and its index there, and the entry of
	// the stsc of its chunk, its chunk, and its index there.
	n  uint32
	t  int64
	at int64
	ti int
	tk uint32
	ci int
	c  uint32
	k  uint32
}

// timeRun is an entry of the stts: count samples of delta ticks each.
type timeRun struct{ count, delta uint32 }

// chunkRun is an entry of the stsc: the chunks from first on, counted from
// 0, up to the next entry's, hold samples samples each.
type chunkRun struct{ first, samples uint32 }

// parse reads the tables of the stbl of data b, which describe no samples
// in a fragmented file. Its times must end by maxTicks. Samples that some
// tables give and others do not are left out.
func (tb *table) parse(b []byte, maxTicks int64) error {
	var stts, stsz, stz2, stsc, stco, co64 []byte
	err := eachBox(b, func(typ uint32, data []byte) error {
		for _, t := range []struct {
			typ uint32
			b   *[]byte
		}{{typeStts, &stts}, {typeStsz, &stsz}, {typeStz2, &stz2}, {typeStsc, &stsc}, {typeStco, &stco}, {typeCo64, &co64}} {
			if typ == t.typ && *t.b == nil {
				*t.b = data
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// The sizes, of an stsz or the compact stz2.
	var count uint32
	switch {
	case stsz != nil:
		_, _, f := fullBox(stsz)
		tb.size, count = f.u32(), f.u32()
		if err := f.check(typeStsz); err != nil {
			return err
		}
		if tb.size > maxBox {
			return fmt.Errorf("%w: samples of %d bytes", ErrFormat, tb.size)
		}
		if tb.size == 0 {
			tb.sizes, tb.sizeBits = f.b, 32
			count = min(count, uint32(len(f.b)/4))
		}
	case stz2 != nil:
		_, _, f := fullBox(stz2)
		f.skip(3)
		tb.sizeBits = int(f.u8())
		count = f.u32()
		if err := f.check(typeStz2); err != nil {
			return err
		}
		switch tb.sizeBits {
		case 4, 8, 16:
		default:
			return fmt.Errorf("%w: stz2 of %d-bit sizes", ErrFormat, tb.sizeBits)
		}
		tb.sizes = f.b
		count = uint32(min(uint64(count), uint64(len(f.b))*8/uint64(tb.sizeBits)))
	}
	if count == 0 {
		return nil // no samples, as in a fragmented file
	}
	if stts == nil || stsc == nil || stco == nil && co64 == nil {
		return fmt.Errorf("%w: incomplete sample table", ErrFormat)
	}

	// The offsets of the chunks.
	typ, offsets := uint32(typeStco), stco
	if stco == nil {
		typ, offsets, tb.wide = typeCo64, co64, true
	}
	_, _, f := fullBox(offsets)
	n := f.u32()
	if err := f.check(typ); err != nil {
		return err
	}
	tb.offsets = f.b
	if tb.wide {
		tb.nchunks = min(n, uint32(len(f.b)/8))
		for c := range tb.nchunks {
			if tb.offset(c) > maxPosition || tb.offset(c) < 0 {
				return fmt.Errorf("%w: chunk at byte %d", ErrFormat, uint64(tb.offset(c)))
			}
		}
	} else {
		tb.nchunks = min(n, uint32(len(f.b)/4))
	}

	// The chunks of the samples: the entries of the stsc must go up, from
	// the first chunk. Those past the last chunk hold none.
	_, _, f = fullBox(stsc)
	n = min(f.u32(), uint32(len(stsc)/12))
	covered := uint64(0)
	for i := range n {
		first, samples := f.u32(), f.u32()
		f.skip(4) // sample_description_index
		if f.short {
			break
		}
		if i == 0 && first != 1 || i > 0 && first-1 <= tb.chunks[i-1].first {
			return fmt.Errorf("%w: stsc entry %d starts at chunk %d", ErrFormat, i, first)
		}
		if first-1 >= tb.nchunks {
			break
		}
		if i > 0 {
			prev := tb.chunks[i-1]
			covered += uint64(first-1-prev.first) * uint64(prev.samples)
		}
		tb.chunks = append(tb.chunks, chunkRun{first - 1, samples})
	}
	if len(tb.chunks) == 0 {
		return fmt.Errorf("%w: incomplete sample table", ErrFormat)
	}
	last := tb.chunks[len(tb.chunks)-1]
	covered += uint64(tb.nchunks-last.first) * uint64(last.samples)
	count = uint32(min(uint64(count), covered))

	// The times, up to the samples the other tables give.
	_, _, f = fullBox(stts)
	n = min(f.u32(), uint32(len(stts)/8))
	timed := uint32(0)
	for range n {
		c, delta := f.u32(), f.u32()
		if f.short || timed == count {
			break
		}
		if c = min(c, count-timed); c == 0 {
			continue
		}
		d := uint64(c) * uint64(delta)
		if d > uint64(maxTicks-tb.end) {
			return fmt.Errorf("%w: samples past tick %d", ErrFormat, maxTicks)
		}
		tb.end += int64(d)
		tb.times = append(tb.times, timeRun{c, delta})
		timed += c
	}
	tb.count = timed
	tb.rewind()
	return nil
}

// offset returns the offset of chunk c.
func (tb *table) offset(c uint32) int64 {
	if c >= tb.nchunks {
		return 0
	}
	if tb.wide {
		return int64(binary.BigEndian.Uint64(tb.offsets[8*c:]))
	}
	return int64(binary.BigEndian.Uint32(tb.offsets[4*c:]))
}

// sizeOf returns the size of sample n.
func (tb *table) sizeOf(n uint32) uint32 {
	switch tb.sizeBits {
	case 0:
		return tb.size
	case 32:
		return binary.BigEndian.Uint32(tb.sizes[4*n:])
	case 16:
		return uint32(binary.BigEndian.Uint16(tb.sizes[2*n:]))
	case 8:
		return uint32(tb.sizes[n])
	}
	return uint32(tb.sizes[n/2]>>(4-4*(n&1))) & 0xF
}

// rewind moves the cursor to the first sample.
func (tb *table) rewind() {
	tb.n, tb.t, tb.ti, tb.tk, tb.ci, tb.c, tb.k = 0, 0, 0, 0, 0, 0, 0
	tb.at = tb.offset(0)
}

// next returns the sample at the cursor, and moves the cursor to the
// next.
func (tb *table) next() (sample, bool) {
	if tb.n >= tb.count {
		return sample{}, false
	}
	// On to the next chunk that holds samples, at the end of one. As the
	// chunks hold the samples counted, one is left.
	for tb.k == tb.chunks[tb.ci].samples {
		tb.c, tb.k = tb.c+1, 0
		if tb.ci+1 < len(tb.chunks) && tb.c == tb.chunks[tb.ci+1].first {
			tb.ci++
		}
		tb.at = tb.offset(tb.c)
	}
	run := tb.times[tb.ti]
	s := sample{pos: tb.at, size: tb.sizeOf(tb.n), time: tb.t, duration: run.delta}
	tb.n, tb.k = tb.n+1, tb.k+1
	tb.at += int64(s.size)
	tb.t += int64(run.delta)
	if tb.tk++; tb.tk == run.count {
		tb.ti, tb.tk = tb.ti+1, 0
	}
	return s, true
}
