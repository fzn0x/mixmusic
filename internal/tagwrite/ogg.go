package tagwrite

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// An Ogg file's tags are the comment header of its stream: of Opus, its
// second packet, OpusTags; of Vorbis, its second, before the setup header.
// The first page holds the identification header alone, and the headers
// end a page, after which the audio starts on a page of its own (RFC 7845
// and the Vorbis I specification). The new comment header, and Vorbis's
// setup header as it was, go on new pages after the first, whose granule
// position is 0 where a packet ends and -1 where none does; the pages of
// the audio follow as they were, but for their sequence numbers, which
// follow the new pages', and their checksums. Pages of other streams are
// copied as they are.

// The flags of a page's header type.
const (
	pageContinued = 1 << iota
	pageFirst
	pageLast
)

// oggPage is a page of an Ogg file: its header's fields, its segment
// table and body, and the page as it was read.
type oggPage struct {
	flags   byte
	granule uint64
	serial  uint32
	seq     uint32
	segs    []byte
	body    []byte
	raw     []byte
	// sound is set when the page's checksum was right.
	sound bool
}

// bytes is the page as a file holds it, with its checksum.
func (p oggPage) bytes() []byte {
	b := make([]byte, 27, 27+len(p.segs)+len(p.body))
	copy(b, "OggS")
	b[5] = p.flags
	binary.LittleEndian.PutUint64(b[6:], p.granule)
	binary.LittleEndian.PutUint32(b[14:], p.serial)
	binary.LittleEndian.PutUint32(b[18:], p.seq)
	b[26] = byte(len(p.segs))
	b = append(append(b, p.segs...), p.body...)
	binary.LittleEndian.PutUint32(b[22:], oggCRC(b))
	return b
}

// pageReader reads the pages of an Ogg file in turn.
type pageReader struct {
	r   *bufio.Reader
	off int64 // the offset of the next page
}

// next reads the next page: io.EOF at the end of the file, and ErrFormat
// where what follows isn't a page.
func (pr *pageReader) next() (oggPage, error) {
	var p oggPage
	h := make([]byte, 27, 27+255)
	if _, err := io.ReadFull(pr.r, h); err != nil {
		if err == io.EOF {
			return p, io.EOF
		}
		return p, fmt.Errorf("%w: a page is cut short at byte %d", ErrFormat, pr.off)
	}
	if string(h[:4]) != "OggS" || h[4] != 0 {
		return p, fmt.Errorf("%w: no page at byte %d", ErrFormat, pr.off)
	}
	p.flags = h[5]
	p.granule = binary.LittleEndian.Uint64(h[6:])
	p.serial = binary.LittleEndian.Uint32(h[14:])
	p.seq = binary.LittleEndian.Uint32(h[18:])
	segs := make([]byte, h[26])
	if _, err := io.ReadFull(pr.r, segs); err != nil {
		return p, fmt.Errorf("%w: a page is cut short at byte %d", ErrFormat, pr.off)
	}
	n := 0
	for _, l := range segs {
		n += int(l)
	}
	raw := make([]byte, 27+len(segs)+n)
	copy(raw, h)
	copy(raw[27:], segs)
	if _, err := io.ReadFull(pr.r, raw[27+len(segs):]); err != nil {
		return p, fmt.Errorf("%w: a page is cut short at byte %d", ErrFormat, pr.off)
	}
	p.raw, p.segs, p.body = raw, raw[27:27+len(segs)], raw[27+len(segs):]
	want := binary.LittleEndian.Uint32(raw[22:])
	check := bytes.Clone(raw[:27])
	clear(check[22:26])
	c := oggUpdate(0, check)
	p.sound = oggUpdate(c, raw[27:]) == want
	pr.off += int64(len(raw))
	return p, nil
}

// oggKind tells whether an Ogg file holds Opus or Vorbis, by the first
// packet of its first page.
func (f *file) oggKind() (Kind, error) {
	if _, err := f.r.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	pr := &pageReader{r: bufio.NewReader(f.r)}
	p, err := pr.next()
	if err != nil {
		return "", fmt.Errorf("%w: it isn't Ogg", ErrFormat)
	}
	switch {
	case bytes.HasPrefix(p.body, []byte("OpusHead")):
		return Opus, nil
	case bytes.HasPrefix(p.body, []byte("\x01vorbis")):
		return Vorbis, nil
	}
	return "", ErrUnsupported
}

func (f *file) writeOgg(w io.Writer, t Tags, fields Fields) error {
	kind, err := f.oggKind()
	if err != nil {
		return err
	}
	if _, err := f.r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	pr := &pageReader{r: bufio.NewReaderSize(f.r, 64<<10)}
	first, err := pr.next()
	if err != nil {
		return err
	}
	damaged := func(p oggPage) error {
		return fmt.Errorf("%w: it's damaged at byte %d", ErrFormat, pr.off-int64(len(p.raw)))
	}
	if !first.sound {
		return damaged(first)
	}
	if first.flags&pageFirst == 0 || len(first.segs) == 0 || first.segs[len(first.segs)-1] == 255 ||
		bytes.Count(append([]byte{}, first.segs[:len(first.segs)-1]...), []byte{255}) != len(first.segs)-1 {
		return fmt.Errorf("%w: its first page holds more than its identification header", ErrFormat)
	}
	prefix, need := []byte("OpusTags"), 1
	if kind == Vorbis {
		prefix, need = []byte("\x03vorbis"), 2
	}
	// The header packets after the first page, and the pages they were on.
	var (
		packets  [][]byte
		packet   []byte
		others   [][]byte
		oldPages int
	)
	for len(packets) < need {
		p, err := pr.next()
		if err == io.EOF {
			return fmt.Errorf("%w: it ends before its headers do", ErrFormat)
		}
		if err != nil {
			return err
		}
		if p.serial != first.serial {
			others = append(others, p.raw)
			continue
		}
		if !p.sound {
			return damaged(p)
		}
		oldPages++
		at := 0
		for i, l := range p.segs {
			packet = append(packet, p.body[at:at+int(l)]...)
			at += int(l)
			if len(packet) > maxRead {
				return fmt.Errorf("%w: its headers are larger than %d MB", ErrFormat, maxRead>>20)
			}
			if l == 255 {
				continue
			}
			packets, packet = append(packets, packet), nil
			if len(packets) == need && i != len(p.segs)-1 {
				return fmt.Errorf("%w: its headers share a page with its audio", ErrFormat)
			}
		}
		if p.flags&pageLast != 0 && len(packets) < need {
			return fmt.Errorf("%w: it ends before its headers do", ErrFormat)
		}
	}
	if !bytes.HasPrefix(packets[0], prefix) {
		return fmt.Errorf("%w: its comment header is missing", ErrFormat)
	}
	vc, rest, err := parseComments(packets[0][len(prefix):])
	if err != nil {
		return err
	}
	var picture []byte
	if fields&Cover != 0 && t.Cover != nil {
		picture = pictureBlock(t.Cover)
	}
	vc.edit(t, fields, picture)
	packets[0] = append(append(append([]byte{}, prefix...), vc.bytes()...), rest...)
	pages := paginate(packets, first.serial, first.seq+1)
	delta := uint32(len(pages) - oldPages)

	for _, b := range append([][]byte{first.raw}, others...) {
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	for _, p := range pages {
		if _, err := w.Write(p.bytes()); err != nil {
			return err
		}
	}
	if delta == 0 {
		return f.copyTo(w, pr.off, f.size)
	}
	// The pages of the stream after the headers take the sequence numbers
	// that follow the new pages'; those of other streams, and of streams
	// chained after it, stay as they are.
	ended := false
	for {
		p, err := pr.next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if p.serial != first.serial || ended {
			if _, err := w.Write(p.raw); err != nil {
				return err
			}
			continue
		}
		if !p.sound {
			return damaged(p)
		}
		p.seq += delta
		if _, err := w.Write(p.bytes()); err != nil {
			return err
		}
		ended = p.flags&pageLast != 0
	}
}

// paginate lays packets out on pages of a stream, of up to 255 segments,
// from sequence number seq: each page whose segments end a packet has the
// granule position 0, and the others -1; one that goes on with a packet of
// the page before says so.
func paginate(packets [][]byte, serial, seq uint32) []oggPage {
	var pages []oggPage
	p := oggPage{serial: serial, seq: seq, granule: ^uint64(0)}
	flush := func() {
		pages = append(pages, p)
		continued := byte(0)
		if p.segs[len(p.segs)-1] == 255 {
			continued = pageContinued
		}
		p = oggPage{flags: continued, serial: serial, seq: p.seq + 1, granule: ^uint64(0)}
	}
	for _, packet := range packets {
		for at := 0; ; {
			if len(p.segs) == 255 {
				flush()
			}
			l := min(len(packet)-at, 255)
			p.segs = append(p.segs, byte(l))
			p.body = append(p.body, packet[at:at+l]...)
			at += l
			if l < 255 {
				p.granule = 0 // a packet ends on the page
				break
			}
		}
	}
	if len(p.segs) > 0 {
		pages = append(pages, p)
	}
	return pages
}

// oggTable is of the CRC-32 of pages: the polynomial 0x04C11DB7, from
// zero, unreflected.
var oggTable = func() (t [256]uint32) {
	for i := range 256 {
		c := uint32(i) << 24
		for range 8 {
			if c&0x80000000 != 0 {
				c = c<<1 ^ 0x04C11DB7
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return
}()

// oggUpdate adds b to the checksum c, and oggCRC is the checksum of a page
// whose checksum field is still 0.
func oggUpdate(c uint32, b []byte) uint32 {
	for _, v := range b {
		c = c<<8 ^ oggTable[byte(c>>24)^v]
	}
	return c
}

func oggCRC(page []byte) uint32 { return oggUpdate(0, page) }
