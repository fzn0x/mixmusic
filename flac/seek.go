package flac

import (
	"errors"
	"io"
)

// seekSpan is how close a seek's search for a frame comes before it reads
// the frames on to the one it seeks: 64 KB of a CD's FLAC is about half a
// second. Tests shorten it.
var seekSpan int64 = 64 << 10

// SeekSample moves to the frame that holds sample s, the number of a
// sample of each channel, or to the last frame for a sample past the end,
// and returns the number of that frame's first sample: ReadFrame reads it
// next. The SEEKTABLE's points narrow the search, which halves the file
// until a frame is near, and reads the frames on from there.
func (r *Reader) SeekSample(s int64) (int64, error) {
	r.pending = false
	lo := r.first
	for _, p := range r.points {
		if p.sample > s {
			break
		}
		if off := r.first + p.offset; off < r.size {
			lo = off
		}
	}
	hi := r.size
	for hi-lo > seekSpan {
		mid := lo + (hi-lo)/2
		off, h, err := r.findFrame(mid, hi)
		switch {
		case err == io.EOF:
			hi = mid
		case err != nil:
			return 0, err
		case h.sample <= s:
			lo = off
		default:
			hi = mid
		}
	}
	if err := r.src.seekTo(lo); err != nil {
		return 0, err
	}
	read := false
	for {
		f, err := r.ReadFrame()
		switch {
		case err == io.EOF && read:
			// Past the end: the last frame.
			r.pending = true
			return r.frame.Sample, nil
		case err == io.EOF:
			return 0, nil // no frame to read
		case err != nil:
			return 0, err
		case f.Sample+int64(len(f.Samples[0])) > s:
			r.pending = true
			return f.Sample, nil
		}
		read = true
	}
}

// findFrame finds the first frame from offset from on, which starts before
// limit, and moves to it: it returns its offset and its header, or io.EOF
// when there is none. A frame's header is found by its sync code and
// checked by its checksum, and by its counting of samples, its channels
// and its rate, which are the stream's.
func (r *Reader) findFrame(from, limit int64) (int64, header, error) {
	s := &r.src
	if err := s.seekTo(from); err != nil {
		return 0, header{}, err
	}
	for i := 0; ; i++ {
		if s.base+int64(i) >= limit {
			return 0, header{}, io.EOF
		}
		if i+2 > len(s.buf) {
			if i > chunk {
				// What was searched goes: the buffer stays small.
				s.bit = i * 8
				s.start()
				i = 0
			}
			if err := s.fill(i + 2); err == io.ErrUnexpectedEOF {
				return 0, header{}, io.EOF
			} else if err != nil {
				return 0, header{}, err
			}
		}
		if s.buf[i] != 0xFF || s.buf[i+1]&0xFE != 0xF8 {
			continue
		}
		s.bit = i * 8
		// readHeader drops what comes before the frame: it starts the
		// buffer.
		h, err := r.readHeader()
		if err == nil && h.variable == r.variable {
			s.bit = 0
			return s.base, h, nil
		}
		if err != nil && !errors.Is(err, ErrFormat) {
			return 0, header{}, err
		}
		i = 0
	}
}
