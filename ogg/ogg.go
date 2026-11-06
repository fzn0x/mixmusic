// Package ogg reads the packets of Ogg files (RFC 3533), as Opus and
// Vorbis audio are kept on computers: those of the file's first logical
// bitstream, with the granule positions of their pages, and seeking by
// them.
//
// A Reader reads its io.ReadSeeker a page at a time, through a buffer,
// and checks each page's checksum: a page whose checksum fails is passed
// over, with the packet it holds a part of, and so is what isn't a page
// between two. The sizes it reads are bounded, so that a corrupt one
// cannot allocate much memory, and malformed files make errors, never
// panics.
package ogg

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrFormat is wrapped by the errors of data that is not Ogg, or is
// corrupt.
var ErrFormat = errors.New("ogg: invalid data")

const (
	// headerSize is the bytes of a page's header before its segment table.
	headerSize = 27
	// maxPacket bounds the bytes of a packet: a comment header may hold a
	// cover of a few megabytes.
	maxPacket = 32 << 20
)

// The flags of a page's header type.
const (
	continued = 1 << iota
	first
	last
)

// Packet is a packet of the stream.
type Packet struct {
	// Data is the packet, which the Reader reuses two packets on.
	Data []byte
	// Granule is the granule position of the page the packet ends on, when
	// it is the last packet that ends there; else -1. For Opus it counts
	// the samples at 48 kHz up to the end of the packet, the pre-skip
	// included; for Vorbis, the samples of the stream's rate.
	Granule int64
	// Last is set for the last packet of the stream, as its last page
	// tells.
	Last bool
}

// page is a page of the stream.
type page struct {
	offset  int64
	flags   byte
	granule int64
	serial  uint32
	segs    []byte
	body    []byte
}

// size returns the bytes the page takes.
func (p *page) size() int64 { return int64(headerSize + len(p.segs) + len(p.body)) }

// Reader reads the packets of the first logical bitstream of an Ogg file.
// It is not safe for concurrent use.
type Reader struct {
	r    io.ReadSeeker
	br   *bufio.Reader
	off  int64 // the offset br reads next
	size int64
	// serial is the stream's, and audio the offset of its first page
	// after the headers, as StartAudio marked it.
	serial uint32
	audio  int64
	// p is the page being read, seg its next segment and at where that
	// starts in its body; ended is set once the last page was read.
	p     page
	seg   int
	at    int
	ended bool
	// packet is the packet being put together; spare the one returned
	// before. drop is set while the rest of a packet whose start was lost
	// is passed over.
	packet, spare []byte
	drop          bool
	// header and segs are room to read a page into.
	header [headerSize]byte
}

// NewReader reads the first page of a file, which starts its first
// logical bitstream.
func NewReader(r io.ReadSeeker) (*Reader, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	rd := &Reader{r: r, br: bufio.NewReaderSize(r, 64<<10), size: size}
	if err := rd.seekTo(0); err != nil {
		return nil, err
	}
	p, err := rd.readPage()
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrFormat) {
			return nil, fmt.Errorf("%w: not an Ogg file", ErrFormat)
		}
		return nil, err
	}
	if p.flags&first == 0 {
		return nil, fmt.Errorf("%w: the first page starts no stream", ErrFormat)
	}
	rd.serial = p.serial
	rd.take(p)
	return rd, nil
}

// StartAudio marks where the stream's audio starts: the next page, as the
// packets read so far were its headers, which end a page. Seeks go no
// further back.
func (r *Reader) StartAudio() { r.audio = r.off }

// seekTo moves to offset off of the file.
func (r *Reader) seekTo(off int64) error {
	if _, err := r.r.Seek(off, io.SeekStart); err != nil {
		return err
	}
	r.br.Reset(r.r)
	r.off = off
	return nil
}

// readPage reads the page at the reader's offset, and checks it.
func (r *Reader) readPage() (page, error) {
	p := page{offset: r.off}
	h := r.header[:]
	if _, err := io.ReadFull(r.br, h); err != nil {
		if err == io.EOF {
			return p, io.EOF
		}
		return p, io.ErrUnexpectedEOF
	}
	if string(h[:4]) != "OggS" || h[4] != 0 {
		return p, fmt.Errorf("%w: no page at byte %d", ErrFormat, p.offset)
	}
	p.flags = h[5]
	p.granule = int64(binary.LittleEndian.Uint64(h[6:]))
	p.serial = binary.LittleEndian.Uint32(h[14:])
	n := int(h[26])
	p.segs = make([]byte, n)
	if _, err := io.ReadFull(r.br, p.segs); err != nil {
		return p, io.ErrUnexpectedEOF
	}
	body := 0
	for _, l := range p.segs {
		body += int(l)
	}
	p.body = make([]byte, body)
	if _, err := io.ReadFull(r.br, p.body); err != nil {
		return p, io.ErrUnexpectedEOF
	}
	r.off += p.size()
	want := binary.LittleEndian.Uint32(h[22:])
	h[22], h[23], h[24], h[25] = 0, 0, 0, 0
	c := update(0, h)
	c = update(c, p.segs)
	c = update(c, p.body)
	if c != want {
		return p, fmt.Errorf("%w: the checksum of the page at byte %d fails", ErrFormat, p.offset)
	}
	return p, nil
}

// take makes p the page whose packets are read next.
func (r *Reader) take(p page) {
	if p.flags&continued == 0 {
		// A page that starts with a packet: one the page before left
		// unfinished never ends.
		r.packet, r.drop = r.packet[:0], false
	} else if len(r.packet) == 0 {
		// The start of the packet this page goes on with was lost.
		r.drop = true
	}
	r.p, r.seg, r.at = p, 0, 0
	if p.flags&last != 0 {
		r.ended = true
	}
}

// nextPage reads the stream's next page: those of other streams are passed
// over, and so is what isn't a page, after which the packet the next page
// goes on with is too. io.EOF at the end of the file, or of the stream.
func (r *Reader) nextPage() error {
	for {
		if r.ended {
			return io.EOF
		}
		p, err := r.readPage()
		switch {
		case err == io.EOF:
			return io.EOF
		case err == io.ErrUnexpectedEOF:
			return io.EOF // a page cut short ends the file
		case errors.Is(err, ErrFormat):
			r.packet = r.packet[:0]
			if p, err = r.findPage(p.offset+1, r.size, false); err != nil {
				return err
			}
		case err != nil:
			return err
		}
		if p.serial != r.serial {
			if p.flags&first != 0 && p.offset > 0 && r.audio > 0 {
				// A chained stream starts after this one.
				return io.EOF
			}
			continue
		}
		r.take(p)
		return nil
	}
}

// ReadPacket reads the next packet of the stream, io.EOF after the last.
func (r *Reader) ReadPacket() (Packet, error) {
	for {
		if r.seg >= len(r.p.segs) {
			if err := r.nextPage(); err != nil {
				return Packet{}, err
			}
			continue
		}
		l := int(r.p.segs[r.seg])
		r.seg++
		part := r.p.body[r.at : r.at+l]
		r.at += l
		if !r.drop {
			if len(r.packet)+l > maxPacket {
				return Packet{}, fmt.Errorf("%w: a packet of more than %d bytes", ErrFormat, maxPacket)
			}
			r.packet = append(r.packet, part...)
		}
		if l == 255 {
			continue
		}
		if r.drop {
			r.drop = false
			continue
		}
		pkt := Packet{Data: r.packet, Granule: -1}
		if r.endsPage() {
			pkt.Granule = r.p.granule
			pkt.Last = r.p.flags&last != 0
		}
		r.packet, r.spare = r.spare[:0], r.packet
		return pkt, nil
	}
}

// endsPage reports whether the packet that ended at the segment before
// the next is the last that ends on the page.
func (r *Reader) endsPage() bool {
	for _, l := range r.p.segs[r.seg:] {
		if l < 255 {
			return false
		}
	}
	return true
}
