package ogg

import (
	"bytes"
	"errors"
	"io"
)

// seekSpan is how close a seek's search for a page comes before it reads
// the pages on to the one it seeks. Tests shorten it.
var seekSpan int64 = 64 << 10

// SeekGranule moves to the packet after the last that ends at or before
// granule position g, or to the first after the headers when none does,
// and returns the granule position that packet starts at: the end of the
// packet before it, or 0. Decoding goes on from there. The search halves
// the file until a page is near, and reads the pages on from there.
func (r *Reader) SeekGranule(g int64) (int64, error) {
	lo, hi := r.audio, r.size
	for hi-lo > seekSpan {
		mid := lo + (hi-lo)/2
		p, err := r.findPage(mid, hi, true)
		switch {
		case err == io.EOF:
			hi = mid
		case err != nil:
			return 0, err
		case p.granule <= g:
			lo = p.offset
		default:
			hi = mid
		}
	}
	if err := r.seekTo(lo); err != nil {
		return 0, err
	}
	var best page
	found := false
	p, err := r.readPage()
	for {
		if errors.Is(err, ErrFormat) {
			p, err = r.findPage(p.offset+1, r.size, false)
			continue
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return 0, err
		}
		if p.serial == r.serial {
			if p.granule > g {
				break
			}
			if p.granule >= 0 {
				best, found = p, true
			}
			if p.flags&last != 0 {
				break
			}
		}
		p, err = r.readPage()
	}
	r.packet, r.spare = r.packet[:0], r.spare[:0]
	if !found {
		return 0, r.rewind()
	}
	if err := r.seekTo(best.offset + best.size()); err != nil {
		return 0, err
	}
	r.ended = false
	r.take(best)
	// The packets that end on the page went before: the one after starts
	// at its granule position.
	end := 0
	for i, l := range best.segs {
		if l < 255 {
			end = i + 1
		}
	}
	r.seg, r.at = end, 0
	for _, l := range best.segs[:end] {
		r.at += int(l)
	}
	r.drop = false
	return best.granule, nil
}

// rewind moves to the first page after the headers.
func (r *Reader) rewind() error {
	if err := r.seekTo(r.audio); err != nil {
		return err
	}
	r.p, r.seg, r.at = page{}, 0, 0
	r.packet, r.drop, r.ended = r.packet[:0], false, false
	return nil
}

// LastGranule returns the granule position of the stream's last page that
// tells one, or -1 when none does: for Opus and Vorbis, where the audio
// ends. It reads the end of the file, then moves to the first page after
// the headers, as StartAudio marked it.
func (r *Reader) LastGranule() (int64, error) {
	g := int64(-1)
	for back := int64(64 << 10); g < 0; back *= 4 {
		from := max(r.audio, r.size-back)
		p, err := r.findPage(from, r.size, true)
		for err == nil {
			if p.serial == r.serial && p.granule >= 0 {
				g = p.granule
			}
			if p, err = r.readPage(); errors.Is(err, ErrFormat) {
				p, err = r.findPage(p.offset+1, r.size, true)
			}
		}
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return 0, err
		}
		if from == r.audio {
			break
		}
	}
	return g, r.rewind()
}

// findPage finds the first page of the stream from offset from on, which
// starts before limit, and one with a granule position when granule is
// set: it moves past it, and returns it, or io.EOF when there is none. A
// page is found by its capture pattern, and checked by its checksum.
func (r *Reader) findPage(from, limit int64, granule bool) (page, error) {
	buf := make([]byte, 64<<10)
	for from < limit {
		if err := r.seekTo(from); err != nil {
			return page{}, err
		}
		n, err := io.ReadFull(r.br, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return page{}, err
		}
		window := buf[:n]
		for i := 0; i < len(window); i++ {
			k := bytes.Index(window[i:], []byte("OggS"))
			if k < 0 {
				break
			}
			i += k
			at := from + int64(i)
			if at >= limit {
				return page{}, io.EOF
			}
			if err := r.seekTo(at); err != nil {
				return page{}, err
			}
			p, err := r.readPage()
			if err == nil && p.serial == r.serial && (!granule || p.granule >= 0) {
				return p, nil
			}
			if err != nil && !errors.Is(err, ErrFormat) && err != io.ErrUnexpectedEOF && err != io.EOF {
				return page{}, err
			}
		}
		if n < len(buf) {
			break
		}
		from += int64(n) - 3
	}
	return page{}, io.EOF
}
