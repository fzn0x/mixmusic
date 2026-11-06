package flac

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
)

const (
	// chunk is the size of the reads of the file.
	chunk = 64 << 10
	// maxFrame bounds the bytes of a frame: 65535 samples of 8 channels of
	// 33 bits, written as they are, take 2.2 MB, and a frame that takes
	// more is corrupt.
	maxFrame = 4 << 20
)

// errReadCount is returned for a reader that says it read more than it
// was asked.
var errReadCount = errors.New("flac: invalid count from Read")

// source reads a file through a buffer that keeps a frame whole, for its
// checksum, and bit by bit, as a frame is coded.
type source struct {
	r io.ReadSeeker
	// buf holds the file from base on, as read so far; bit is where the
	// next bit to read is in it, and mark where the frame being read
	// starts.
	buf  []byte
	base int64
	bit  int
	mark int
	// eof is set once r gave all it has.
	eof bool
}

// offset returns the offset in the file of the next byte to read, which
// must start a byte.
func (s *source) offset() int64 { return s.base + int64(s.bit>>3) }

// seekTo moves to offset off of the file, dropping what the buffer holds.
func (s *source) seekTo(off int64) error {
	if _, err := s.r.Seek(off, io.SeekStart); err != nil {
		return err
	}
	s.buf, s.base, s.bit, s.mark, s.eof = s.buf[:0], off, 0, 0, false
	return nil
}

// start marks the next byte as the start of a frame, dropping what comes
// before it from the buffer.
func (s *source) start() {
	i := s.bit >> 3
	n := copy(s.buf, s.buf[i:])
	s.buf = s.buf[:n]
	s.base += int64(i)
	s.bit &= 7
	s.mark = 0
}

// fill reads until the buffer holds n bytes, or the end of the file:
// io.ErrUnexpectedEOF then. A frame of more than maxFrame bytes is
// corrupt.
func (s *source) fill(n int) error {
	if n-s.mark > maxFrame {
		return fmt.Errorf("%w: a frame of more than %d bytes", ErrFormat, maxFrame)
	}
	for len(s.buf) < n {
		if s.eof {
			return io.ErrUnexpectedEOF
		}
		if cap(s.buf)-len(s.buf) < chunk {
			s.buf = append(s.buf[:cap(s.buf)], make([]byte, chunk)...)[:len(s.buf)]
		}
		k, err := s.r.Read(s.buf[len(s.buf):min(cap(s.buf), len(s.buf)+chunk)])
		if k < 0 || k > chunk {
			return errReadCount
		}
		s.buf = s.buf[:len(s.buf)+k]
		switch {
		case err == io.EOF:
			s.eof = true
		case err != nil:
			return err
		}
	}
	return nil
}

// word returns the 64 bits of the buffer from byte i, zeros past its end.
func (s *source) word(i int) uint64 {
	if i+8 <= len(s.buf) {
		return binary.BigEndian.Uint64(s.buf[i:])
	}
	var w uint64
	for k := range 8 {
		w <<= 8
		if i+k < len(s.buf) {
			w |= uint64(s.buf[i+k])
		}
	}
	return w
}

// read reads n bits, up to 57, as an unsigned number.
func (s *source) read(n uint) (uint64, error) {
	if n == 0 {
		return 0, nil
	}
	if err := s.fill((s.bit + int(n) + 7) >> 3); err != nil {
		return 0, err
	}
	w := s.word(s.bit >> 3)
	v := w << uint(s.bit&7) >> (64 - n)
	s.bit += int(n)
	return v, nil
}

// signed reads n bits, from 1 to 57, as a number in two's complement.
func (s *source) signed(n uint) (int64, error) {
	v, err := s.read(n)
	return int64(v<<(64-n)) >> (64 - n), err
}

// unary reads the zeros before a one, and the one, and returns how many
// zeros there were. The frame's bound on its size bounds the count.
func (s *source) unary() (uint64, error) {
	var q uint64
	for {
		i := s.bit >> 3
		if i >= len(s.buf) {
			if err := s.fill(i + 1); err != nil {
				return 0, err
			}
		}
		off := uint(s.bit & 7)
		avail := min(64, (len(s.buf)-i)*8) - int(off)
		w := s.word(i) << off
		if z := bits.LeadingZeros64(w); z < avail {
			s.bit += z + 1
			return q + uint64(z), nil
		}
		q += uint64(avail)
		s.bit += avail
	}
}

// align moves to the start of the next byte, unless at one.
func (s *source) align() { s.bit = (s.bit + 7) &^ 7 }

// bytes returns the buffer's bytes from the frame's start to the next
// byte, which must start a byte.
func (s *source) bytes() []byte { return s.buf[s.mark : s.bit>>3] }
