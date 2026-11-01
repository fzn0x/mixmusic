package webm

import (
	"errors"
	"fmt"
	"io"
)

const (
	// bufSize is the size of the reads of the file.
	bufSize = 64 << 10
	// skipLimit is how far ahead the source reads on rather than seeking:
	// over HTTP, reading a little costs less than a new request.
	skipLimit = 256 << 10
	// maxElement bounds the size of the elements read into memory, the
	// headers and the blocks: larger sizes are corrupt.
	maxElement = 16 << 20
)

// errReadCount is returned for a reader that says it read more than it
// was asked.
var errReadCount = errors.New("webm: invalid count from Read")

// header is the start of an element.
type header struct {
	id    uint32
	start int64 // the offset of its ID
	data  int64 // the offset of its data
	size  int64 // the size of its data, or unknownSize
}

// end returns the offset after the element, or -1 when its size is
// unknown.
func (h header) end() int64 {
	if h.size == unknownSize {
		return -1
	}
	return h.data + h.size
}

// source reads a file through a buffer, keeping track of offsets, and
// seeks it only to go back or far ahead.
type source struct {
	r    io.ReadSeeker
	buf  []byte // buf[off:] is read and not consumed yet
	off  int
	pos  int64 // the offset of buf[0] in the file
	rpos int64 // the offset r reads next, or -1 when it is unknown
}

func newSource(r io.ReadSeeker) *source {
	return &source{r: r, buf: make([]byte, 0, bufSize), rpos: -1}
}

// offset returns the offset of the next byte to consume.
func (s *source) offset() int64 { return s.pos + int64(s.off) }

func (s *source) buffered() int { return len(s.buf) - s.off }

// seekTo moves to offset off: within the buffer at no cost, elsewhere by
// reading on to it or seeking, at the next read.
func (s *source) seekTo(off int64) {
	if off >= s.pos && off <= s.pos+int64(len(s.buf)) {
		s.off = int(off - s.pos)
		return
	}
	s.buf, s.off, s.pos = s.buf[:0], 0, off
}

// skip moves n bytes ahead.
func (s *source) skip(n int64) { s.seekTo(s.offset() + n) }

// fill reads until n bytes, at most bufSize, are buffered. It returns nil
// once they are, io.EOF when the file ends first, or the error of reading.
func (s *source) fill(n int) error {
	if s.buffered() >= n {
		return nil
	}
	// Drop the consumed bytes only when room runs out, so that going back a
	// little costs no seek.
	if cap(s.buf)-len(s.buf) < max(n-s.buffered(), bufSize/4) {
		k := copy(s.buf, s.buf[s.off:])
		s.buf, s.pos, s.off = s.buf[:k], s.pos+int64(s.off), 0
	}
	if end := s.pos + int64(len(s.buf)); s.rpos != end {
		if err := s.move(end); err != nil {
			return err
		}
	}
	for empty := 0; ; {
		free := s.buf[len(s.buf):cap(s.buf)]
		k, err := s.r.Read(free)
		if k < 0 || k > len(free) {
			s.rpos = -1
			return errReadCount
		}
		s.buf = s.buf[:len(s.buf)+k]
		s.rpos += int64(k)
		switch {
		case s.buffered() >= n:
			return nil
		case err != nil:
			if err != io.EOF {
				s.rpos = -1
			}
			return err
		case k == 0:
			if empty++; empty == 100 {
				return io.ErrNoProgress
			}
		}
	}
}

// move gets r to offset off, past the buffered bytes: by reading on when
// off is a little ahead, which costs less than a seek over a network, else
// by seeking.
func (s *source) move(off int64) error {
	if d := off - s.rpos; s.rpos >= 0 && d > 0 && d <= skipLimit {
		for empty := 0; d > 0; {
			free := s.buf[len(s.buf):cap(s.buf)]
			free = free[:min(d, int64(len(free)))]
			k, err := s.r.Read(free)
			if k < 0 || k > len(free) {
				s.rpos = -1
				return errReadCount
			}
			s.rpos += int64(k)
			d -= int64(k)
			switch {
			case d == 0:
			case err != nil:
				if err != io.EOF {
					s.rpos = -1
				}
				return err
			case k == 0:
				if empty++; empty == 100 {
					return io.ErrNoProgress
				}
			}
		}
		return nil
	}
	if _, err := s.r.Seek(off, io.SeekStart); err != nil {
		s.rpos = -1
		return err
	}
	s.rpos = off
	return nil
}

// peek returns the next n bytes, at most bufSize, without consuming them.
func (s *source) peek(n int) ([]byte, error) {
	if err := s.fill(n); err != nil {
		return nil, unexpected(err)
	}
	return s.buf[s.off : s.off+n], nil
}

// readHeader reads the header of the next element. It returns io.EOF at
// the end of the file, and io.ErrUnexpectedEOF when it ends in a header.
func (s *source) readHeader() (header, error) {
	start := s.offset()
	err := s.fill(maxHeaderLen)
	id, size, n, perr := parseHeader(s.buf[s.off:])
	switch {
	case perr == errShort:
		// Fewer than maxHeaderLen bytes, which hold any header, are left:
		// the file ends here, or within the header.
		if err == nil || err == io.EOF && s.buffered() > 0 {
			err = io.ErrUnexpectedEOF
		}
		return header{}, err
	case perr != nil:
		return header{}, fmt.Errorf("%w at byte %d", perr, start)
	}
	s.off += n
	return header{id: id, start: start, data: start + int64(n), size: size}, nil
}

// readBody reads the data of the element of header h into memory.
func (s *source) readBody(h header) ([]byte, error) {
	if h.size == unknownSize {
		return nil, fmt.Errorf("%w: element %#x at byte %d has an unknown size", ErrFormat, h.id, h.start)
	}
	if h.size > maxElement {
		return nil, fmt.Errorf("%w: element %#x at byte %d is %d bytes long", ErrFormat, h.id, h.start, h.size)
	}
	// The data grows as it is read, so that a corrupt size cannot allocate
	// much more than the file holds.
	n := int(h.size)
	b := make([]byte, 0, min(n, bufSize))
	for len(b) < n {
		if err := s.fill(1); err != nil {
			return nil, unexpected(err)
		}
		k := min(n-len(b), s.buffered())
		b = append(b, s.buf[s.off:s.off+k]...)
		s.off += k
	}
	return b, nil
}

// readUintElement reads the data of an unsigned integer element.
func (s *source) readUintElement(h header) (uint64, error) {
	if h.size > 8 {
		return 0, fmt.Errorf("%w: integer of %d bytes at byte %d", ErrFormat, h.size, h.start)
	}
	b, err := s.readBody(h)
	if err != nil {
		return 0, err
	}
	return readUint(b)
}

// unexpected turns io.EOF, met in the middle of something, into
// io.ErrUnexpectedEOF.
func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
