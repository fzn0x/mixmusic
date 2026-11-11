package tagwrite

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"time"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// The Opus audio of a WebM file, as YouTube serves it and the downloads
// keep it, has no tags that other players read, and no cover: RemuxWebM
// writes it as an Ogg Opus file (RFC 7845), whose comment header holds
// them. The Opus packets are copied as they are. The identification
// header is the track's CodecPrivate, an OpusHead, which holds the
// pre-skip. Each page's granule position counts the samples at 48 kHz
// decoded by the end of the last packet that ends on it, from the start of
// the stream, the pre-skip among them, and -1 on a page where none ends.
// The last page's leaves out the samples the last block's DiscardPadding
// drops, so that the audio ends where the WebM's did. A page holds about a
// second of audio, so that a seek reads little.

// opusRate is the rate Ogg Opus counts its granule positions at.
const opusRate = 48000

// pageSamples is how much audio a page of RemuxWebM's holds, at least,
// before the next packet starts a page of its own.
const pageSamples = opusRate

// IsWebM reports whether r holds a WebM or Matroska file, by its first
// bytes: the ID of an EBML header.
func IsWebM(r io.ReadSeeker) bool {
	var head [4]byte
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return false
	}
	_, err := io.ReadFull(r, head[:])
	return err == nil && head == [4]byte{0x1A, 0x45, 0xDF, 0xA3}
}

// RemuxWebM writes to w the Opus audio of the WebM file src holds, as an
// Ogg Opus file whose tags are the fields of t that fields names, as
// Rewrite writes them. A file whose first audio track isn't Opus returns
// ErrUnsupported, and one whose blocks can't be read an error wrapping
// ErrFormat. A cover must be a JPEG or PNG image of at most MaxCover
// bytes.
func RemuxWebM(w io.Writer, src io.ReadSeeker, t Tags, fields Fields) error {
	if fields&Cover != 0 && t.Cover != nil {
		mime, err := checkCover(t.Cover)
		if err != nil {
			return err
		}
		t.Cover = &Picture{MIME: mime, Data: t.Cover.Data}
	}
	t = t.clean()
	if err := t.check(); err != nil {
		return err
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r, err := webm.NewReader(src)
	switch {
	case errors.Is(err, webm.ErrNoAudio):
		return ErrUnsupported
	case err != nil:
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	track := r.Track()
	if track.Codec != "A_OPUS" {
		return ErrUnsupported
	}
	head, err := opusHead(track)
	if err != nil {
		return err
	}
	serial := crc32.ChecksumIEEE(head) ^ uint32(r.Duration()/time.Millisecond)
	bw := bufio.NewWriterSize(w, 256<<10)
	ow := &oggWriter{w: bw, serial: serial}
	if err := ow.page(oggPage{flags: pageFirst, granule: 0, segs: []byte{byte(len(head))}, body: head}); err != nil {
		return err
	}
	vc := comments{vendor: []byte("MixMusic")}
	var picture []byte
	if fields&Cover != 0 && t.Cover != nil {
		picture = pictureBlock(t.Cover)
	}
	vc.edit(t, fields, picture)
	for _, p := range paginate([][]byte{append([]byte("OpusTags"), vc.bytes()...)}, serial, 1) {
		if err := ow.page(p); err != nil {
			return err
		}
	}
	if err := ow.audio(r); err != nil {
		return err
	}
	return bw.Flush()
}

// opusHead returns the OpusHead of an Opus track: its CodecPrivate, or, of
// a mono or stereo track without one, the one its channels and CodecDelay
// make.
func opusHead(t webm.Track) ([]byte, error) {
	h := t.CodecPrivate
	switch {
	case len(h) >= 19 && string(h[:8]) == "OpusHead" && h[8]&0xF0 == 0 && len(h) < 255:
		return bytes.Clone(h), nil
	case len(h) > 0 || t.Channels < 1 || t.Channels > 2:
		return nil, fmt.Errorf("%w: its Opus track has no header MixMusic can use", ErrFormat)
	}
	b := append([]byte("OpusHead"), 1, byte(t.Channels))
	b = binary.LittleEndian.AppendUint16(b, uint16(min(samplesOf(t.CodecDelay), math.MaxUint16)))
	b = binary.LittleEndian.AppendUint32(b, opusRate)
	return append(b, 0, 0, 0), nil // no gain, and channel mapping family 0
}

// samplesOf is how many samples at 48 kHz d lasts.
func samplesOf(d time.Duration) int64 {
	return int64(math.Round(d.Seconds() * opusRate))
}

// oggWriter writes the pages of one Ogg stream, numbering them in turn.
type oggWriter struct {
	w      io.Writer
	serial uint32
	seq    uint32
}

// page writes p as the stream's next page.
func (o *oggWriter) page(p oggPage) error {
	p.serial, p.seq = o.serial, o.seq
	o.seq++
	_, err := o.w.Write(p.bytes())
	return err
}

// audio writes the packets of r as the pages of the audio, the last of
// them flagged as the stream's last.
func (o *oggWriter) audio(r *webm.Reader) error {
	read := func() (webm.Packet, bool, error) {
		p, err := r.ReadPacket()
		switch {
		case err == io.EOF:
			return p, false, nil
		case err != nil:
			return p, false, fmt.Errorf("%w: %v", ErrFormat, err)
		}
		return p, true, nil
	}
	next, more, err := read()
	if err != nil {
		return err
	}
	if !more {
		return fmt.Errorf("%w: it holds no audio", ErrFormat)
	}
	var (
		cur      = oggPage{granule: ^uint64(0)}
		total    int64 // the samples of the packets read
		pageFrom int64 // the granule position the page started from
		last     int64 // the granule position of the page written last
	)
	flush := func(end bool) error {
		if end {
			cur.flags |= pageLast
		}
		if cur.granule != ^uint64(0) {
			last, pageFrom = int64(cur.granule), int64(cur.granule)
		}
		continued := len(cur.segs) > 0 && cur.segs[len(cur.segs)-1] == 255
		err := o.page(cur)
		cur = oggPage{granule: ^uint64(0)}
		if continued {
			cur.flags = pageContinued
		}
		return err
	}
	for more {
		p := next
		n, err := opus.PacketSamples(p.Data, opusRate)
		if err != nil {
			return fmt.Errorf("%w: an Opus packet at %v is broken", ErrFormat, p.Time)
		}
		if next, more, err = read(); err != nil {
			return err
		}
		total += int64(n)
		end := total
		if !more && p.DiscardPadding > 0 {
			// The end the WebM trimmed, which the last page's granule
			// position trims: never before the page written last.
			end = max(total-samplesOf(p.DiscardPadding), last)
		}
		for at := 0; ; {
			if len(cur.segs) == 255 {
				if err := flush(false); err != nil {
					return err
				}
			}
			l := min(len(p.Data)-at, 255)
			cur.segs = append(cur.segs, byte(l))
			cur.body = append(cur.body, p.Data[at:at+l]...)
			at += l
			if l < 255 {
				cur.granule = uint64(end)
				break
			}
		}
		if !more || end-pageFrom >= pageSamples {
			if err := flush(!more); err != nil {
				return err
			}
		}
	}
	return nil
}
