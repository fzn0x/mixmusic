package mp4

import (
	"io"
	"sort"
	"time"
)

// fragPoint is a fragment to seek to and the time it starts at.
type fragPoint struct {
	time int64 // in ticks
	pos  int64 // the offset of its moof, or of the boxes before it
}

// Seek moves to the last packet that starts at or before t, or to the
// first, so that the next ReadPacket returns it. Callers that need t
// exactly decode from there, dropping the audio before t; as AAC needs
// some decoding before t, its callers seek before it.
//
// In a progressive file, Seek finds the packet by the sample table, and
// reads nothing. In a fragmented file, it reads the moof of the fragment
// the sidx says holds t, and the one before or after when the sidx is off.
// Without a sidx, it reads the moof boxes from the first, skipping their
// mdat: a read, or a seek, per fragment. It remembers them, as it does
// those ReadPacket reads in order, so that it reads each once.
func (r *Reader) Seek(t time.Duration) error {
	r.err = nil
	if err := r.seek(r.ticks(t)); err != nil {
		r.err = err
		return err
	}
	return nil
}

func (r *Reader) seek(t int64) error {
	if r.table.count > 0 && (!r.fragmented || t < r.table.end) {
		r.table.seek(t)
		r.inTable = true
		r.frag, r.after = fragment{start: r.table.end, end: r.table.end}, r.fragStart
		return nil
	}
	ok, err := r.seekFragment(t)
	if err != nil || ok {
		return err
	}
	// No fragments: the last sample of the table, if any.
	r.table.seek(t)
	r.inTable = true
	r.frag, r.after = fragment{start: r.table.end, end: r.table.end}, -1
	return nil
}

// seekFragment moves to the last sample at or before tick t of the
// fragments, or to their first, or returns false when there are none.
func (r *Reader) seekFragment(t int64) (bool, error) {
	for !r.scanned && t >= r.known {
		if err := r.scanFragment(); err != nil {
			return false, err
		}
	}
	if len(r.frags) == 0 {
		return false, nil
	}
	i := max(sort.Search(len(r.frags), func(i int) bool { return r.frags[i].time > t })-1, 0)
	var f fragment
	var after int64
	for {
		p := r.frags[i]
		var ok bool
		var err error
		f, after, ok, err = r.readFragment(p.pos, p.time, i < r.indexed)
		if err != nil && !isDataError(err) {
			return false, err
		}
		if err != nil || !ok {
			if i < r.indexed {
				// The sidx points elsewhere than at fragments: do without it.
				r.frags, r.indexed = nil, 0
				r.known, r.scanAt, r.scanned = r.table.end, r.fragStart, false
				return r.seekFragment(t)
			}
			return false, err
		}
		if f.start <= t || i == 0 {
			break
		}
		i-- // the sidx is off: the fragment before holds t
	}
	// Past the end of the fragment, t may be in the next: the sidx may be
	// off, and the fragments of other tracks hold none of this one's.
	for next := after; t >= f.end && next >= 0; {
		g, gAfter, ok, err := r.readFragment(next, f.end, false)
		if err != nil && !isDataError(err) {
			return false, err
		}
		if err != nil || !ok || g.start > t {
			break
		}
		next = gAfter
		if g.count() > 0 {
			f, after = g, gAfter
		}
	}
	f.seek(t)
	r.frag, r.after, r.inTable = f, after, false
	return true, nil
}

// scanFragment reads the moof after those known, and remembers where it
// starts and when, or sets r.scanned at the end of the file. A file that
// ends early, or with corrupt data, ends the scan there.
func (r *Reader) scanFragment() error {
	f, after, ok, err := r.readFragment(r.scanAt, r.known, false)
	if err != nil && !isDataError(err) {
		return err
	}
	r.addScanned(f, after, ok && err == nil)
	return nil
}

// addScanned notes fragment f, found at r.scanAt, after whose mdat the
// next starts at offset after; or the end of the scan, when not found.
func (r *Reader) addScanned(f fragment, after int64, found bool) {
	if !found {
		r.scanned = true
		return
	}
	t := f.start
	if n := len(r.frags); n > 0 && t < r.frags[n-1].time {
		t = r.frags[n-1].time // out of order: keep the times sorted for the search
	}
	r.frags = append(r.frags, fragPoint{time: t, pos: r.scanAt})
	r.known = max(r.known, f.end)
	r.scanAt = after
	r.scanned = after < 0
}

// setIndex keeps the fragments that the sidx of data b, which ends at
// offset end, indexes, if it indexes the track: its subsegments, which
// follow each other from the end of the sidx on. The times of a sidx are
// those of presentation, which start CodecDelay after those of the media.
// A sidx that indexes other sidx boxes, or a corrupt one, gives the
// fragments before them.
func (r *Reader) setIndex(b []byte, end int64) {
	v, _, f := fullBox(b)
	id := f.u32() // reference_ID
	scale := f.u32()
	t := f.uint(v == 1) // earliest_presentation_time
	pos := f.uint(v == 1)
	f.skip(2)
	n := f.u16()
	if f.short || id != r.track.ID || scale == 0 || pos > maxPosition {
		return
	}
	pos += uint64(end)
	var points []fragPoint
	for range n {
		ref, duration := f.u32(), f.u32()
		f.skip(4) // the stream access point
		if f.short || ref>>31 != 0 {
			break
		}
		at, ok := rescale(t, scale, r.track.Timescale, r.maxTicks-r.delay)
		if !ok {
			break
		}
		points = append(points, fragPoint{time: r.delay + at, pos: int64(pos)})
		pos += uint64(ref)
		t += uint64(duration)
	}
	at, ok := rescale(t, scale, r.track.Timescale, r.maxTicks-r.delay)
	if len(points) == 0 || !ok {
		return
	}
	r.frags, r.indexed = points, len(points)
	r.known, r.scanAt = r.delay+at, int64(pos)
}

// indexesAll reports whether the sidx indexes the fragments up to the end
// of the file: whether the file ends where the sidx's subsegments do, or
// with an mfra, as ffmpeg ends a fragmented file. ffmpeg's DASH files have
// a sidx before each fragment instead, of it alone.
func (r *Reader) indexesAll() bool {
	r.src.seekTo(r.scanAt)
	h, err := r.src.readHeader()
	return err == io.EOF || err == nil && h.typ == typeMfra
}
