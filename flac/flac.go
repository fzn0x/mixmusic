// Package flac decodes FLAC (RFC 9639), as music kept on a computer often
// is: the STREAMINFO of a file, which tells its audio, and its frames,
// into the samples of each channel, with seeking by the SEEKTABLE and by
// searching for frames.
//
// A Reader reads its io.ReadSeeker through a buffer, which holds a frame
// whole for its checksum. The sizes it reads are bounded, so that a
// corrupt one cannot allocate much memory, and malformed files make
// errors, never panics. A frame whose checksum fails is passed over, as a
// player passes over a frame it can't decode, and so is what isn't a
// frame between two, as a tag at the end of a file.
package flac

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

// ErrFormat is wrapped by the errors of data that is not FLAC, or is
// corrupt.
var ErrFormat = errors.New("flac: invalid data")

const (
	// maxSeekPoints bounds the points of a SEEKTABLE read: one every ten
	// seconds of a week of audio.
	maxSeekPoints = 1 << 16
	// maxBlocks bounds the metadata blocks before the frames.
	maxBlocks = 1024
)

// Info is what the STREAMINFO block of a file tells of its audio.
type Info struct {
	// SampleRate is the samples of each channel in a second, in Hz.
	SampleRate int
	// Channels is how many channels the audio has, from 1 to 8.
	Channels int
	// BitsPerSample is the bits of a sample, from 4 to 32.
	BitsPerSample int
	// Samples is how many samples each channel holds, 0 when the file
	// doesn't tell.
	Samples int64
	// MinBlock and MaxBlock bound the samples of each channel in a frame,
	// but the last.
	MinBlock, MaxBlock int
	// MD5 is the MD5 of the samples, interleaved, in little-endian bytes
	// as many as their bits take: zeros when the encoder didn't tell.
	MD5 [16]byte
}

// Duration returns how long the audio plays, 0 when the file doesn't
// tell.
func (i Info) Duration() time.Duration {
	if i.Samples <= 0 || i.SampleRate <= 0 {
		return 0
	}
	s := i.Samples / int64(i.SampleRate)
	rest := i.Samples % int64(i.SampleRate)
	return time.Duration(s)*time.Second + time.Duration(rest)*time.Second/time.Duration(i.SampleRate)
}

// seekPoint is a point of the SEEKTABLE: the frame at offset, from the
// first frame, starts at sample.
type seekPoint struct{ sample, offset int64 }

// Reader reads the frames of a FLAC file. It is not safe for concurrent
// use.
type Reader struct {
	src  source
	info Info
	// first is the offset of the first frame, and size that of the end of
	// the file.
	first, size int64
	points      []seekPoint
	// variable is set for a stream of frames of varying sizes, whose
	// headers tell their first samples rather than their numbers.
	variable bool
	frame    Frame
	// work holds a frame's channels as they decode, with the bit a side
	// channel has more than the others; res holds the coefficients of a
	// prediction.
	work [][]int64
	res  [32]int64
	// pending is set when SeekSample read the frame ReadFrame returns
	// next.
	pending bool
}

// NewReader reads the metadata of a file, from the start of r up to its
// first frame: an ID3v2 tag before it is passed over, as some programs
// write one.
func NewReader(r io.ReadSeeker) (*Reader, error) {
	rd := &Reader{src: source{r: r}}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	rd.size = size
	if err := rd.readMetadata(); err != nil {
		return nil, err
	}
	if err := rd.src.seekTo(rd.first); err != nil {
		return nil, err
	}
	// The first frame's header tells how the frames count their samples.
	h, err := rd.readHeader()
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("%w: no frames", ErrFormat)
		}
		return nil, err
	}
	rd.variable = h.variable
	if err := rd.src.seekTo(rd.first); err != nil {
		return nil, err
	}
	return rd, nil
}

// Info returns what the file's STREAMINFO tells of its audio.
func (r *Reader) Info() Info { return r.info }

// readMetadata reads the metadata blocks: the STREAMINFO, which comes
// first, and the SEEKTABLE; the others are passed over.
func (r *Reader) readMetadata() error {
	at, err := skipID3(r.src.r)
	if err != nil {
		return err
	}
	var head [4]byte
	if _, err := readAt(r.src.r, at, head[:]); err != nil {
		return fmt.Errorf("%w: not a FLAC file", ErrFormat)
	}
	if string(head[:]) != "fLaC" {
		return fmt.Errorf("%w: not a FLAC file", ErrFormat)
	}
	at += 4
	for n := 0; ; n++ {
		if n == maxBlocks {
			return fmt.Errorf("%w: more than %d metadata blocks", ErrFormat, maxBlocks)
		}
		if _, err := readAt(r.src.r, at, head[:]); err != nil {
			return fmt.Errorf("%w: the metadata ends early", ErrFormat)
		}
		last, kind := head[0]&0x80 != 0, head[0]&0x7F
		size := int64(head[1])<<16 | int64(head[2])<<8 | int64(head[3])
		body := at + 4
		if body+size > r.size {
			return fmt.Errorf("%w: a metadata block of %d bytes past the end", ErrFormat, size)
		}
		switch {
		case n == 0 && kind != 0:
			return fmt.Errorf("%w: no STREAMINFO", ErrFormat)
		case kind == 0 && n > 0:
			return fmt.Errorf("%w: a second STREAMINFO", ErrFormat)
		case kind == 0:
			if size != 34 {
				return fmt.Errorf("%w: a STREAMINFO of %d bytes", ErrFormat, size)
			}
			var b [34]byte
			if _, err := readAt(r.src.r, body, b[:]); err != nil {
				return err
			}
			if err := r.parseInfo(b[:]); err != nil {
				return err
			}
		case kind == 3:
			b := make([]byte, min(size, maxSeekPoints*18))
			if _, err := readAt(r.src.r, body, b); err != nil {
				return err
			}
			r.parseSeekTable(b)
		case kind == 127:
			return fmt.Errorf("%w: a metadata block of type 127", ErrFormat)
		}
		at = body + size
		if last {
			break
		}
	}
	r.first = at
	return nil
}

// parseInfo reads a STREAMINFO block.
func (r *Reader) parseInfo(b []byte) error {
	i := Info{
		MinBlock: int(binary.BigEndian.Uint16(b[0:])),
		MaxBlock: int(binary.BigEndian.Uint16(b[2:])),
	}
	v := binary.BigEndian.Uint64(b[10:])
	i.SampleRate = int(v >> 44)
	i.Channels = int(v>>41&7) + 1
	i.BitsPerSample = int(v>>36&31) + 1
	i.Samples = int64(v & (1<<36 - 1))
	copy(i.MD5[:], b[18:])
	switch {
	case i.SampleRate == 0:
		return fmt.Errorf("%w: a sample rate of 0", ErrFormat)
	case i.BitsPerSample < 4:
		return fmt.Errorf("%w: %d bits a sample", ErrFormat, i.BitsPerSample)
	case i.MaxBlock < i.MinBlock:
		return fmt.Errorf("%w: frames of %d to %d samples", ErrFormat, i.MinBlock, i.MaxBlock)
	}
	r.info = i
	return nil
}

// parseSeekTable reads the points of a SEEKTABLE, in the order of their
// samples, but the placeholders and those out of order.
func (r *Reader) parseSeekTable(b []byte) {
	for ; len(b) >= 18; b = b[18:] {
		sample := binary.BigEndian.Uint64(b)
		offset := binary.BigEndian.Uint64(b[8:])
		if sample == 1<<64-1 || sample >= 1<<62 || offset >= 1<<62 {
			continue
		}
		p := seekPoint{int64(sample), int64(offset)}
		if n := len(r.points); n > 0 && (p.sample <= r.points[n-1].sample || p.offset <= r.points[n-1].offset) {
			continue
		}
		r.points = append(r.points, p)
	}
}

// skipID3 returns the offset after the ID3v2 tag r starts with, or 0
// without one.
func skipID3(r io.ReadSeeker) (int64, error) {
	var h [10]byte
	if n, err := readAt(r, 0, h[:]); err != nil && n < len(h) {
		return 0, nil
	}
	if string(h[:3]) != "ID3" || h[6]|h[7]|h[8]|h[9] >= 0x80 {
		return 0, nil
	}
	size := int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9])
	if h[5]&0x10 != 0 {
		size += 10 // a footer
	}
	return 10 + size, nil
}

// readAt reads len(b) bytes at offset off of r.
func readAt(r io.ReadSeeker, off int64, b []byte) (int, error) {
	if _, err := r.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(r, b)
}
