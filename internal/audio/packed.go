package audio

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/fzn0x/mixmusic/aac"
	"github.com/fzn0x/mixmusic/resample"
)

const (
	// maxJunk is how many bytes in a row a packed track passes over that
	// are neither an ID3 tag nor an ADTS frame before it gives its stream
	// up as no packed audio.
	maxJunk = 64 << 10
	// maxBadFrames is how many frames in a row a packed track's decoder may
	// fail on, each dropped, before the track fails: a frame the encoder
	// botched shouldn't end a stream that plays for hours.
	maxBadFrames = 8
	// id3Header is the size of an ID3v2 tag's header, and of its footer.
	id3Header = 10
)

// packedTrack is the AAC-LC of HLS packed audio (RFC 8216, section 3.4),
// as YouTube serves its live streams: ADTS frames, those of each segment
// after an ID3 tag that tells their timestamp. The track passes over the
// tags, and plays the frames as they come, from where it starts: a live
// stream has no length, and no time to seek to. A frame of another config,
// as after a discontinuity, decodes anew, at its rate.
type packedTrack struct {
	r   *bufio.Reader
	cfg aac.Config
	dec *aac.Decoder
	rs  *resample.Converter
	// h is the header of the frame read last, and frame its raw data
	// block.
	h     aac.ADTSHeader
	frame []byte
	// at is the frame of the stream, at Rate, of what decode gives next.
	at int64
	// skip is what is left to skip of an ID3 tag, junk counts the bytes in
	// a row that were no frame, and bad the frames in a row the decoder
	// failed on.
	skip, junk, bad int
	pcm             []float32 // a decoded frame, at the stream's rate
	out             []float32 // the converter's output
}

// openPacked returns the AAC-LC track of HLS packed audio, which starts
// with an ID3 tag or an ADTS frame. Its errors are worded for the player.
func openPacked(src io.Reader) (track, timing, error) {
	// A frame is of 8191 bytes at most, which Peek takes whole.
	t := &packedTrack{r: bufio.NewReaderSize(src, 16<<10), pcm: make([]float32, 2*aac.FrameLength)}
	h, err := t.header()
	if err == io.EOF {
		err = fmt.Errorf("%w: no ADTS frame", aac.ErrFormat)
	}
	if err != nil {
		return nil, timing{}, readError(err)
	}
	if err := t.configure(h.Config); err != nil {
		return nil, timing{}, err
	}
	return t, timing{live: true}, nil
}

// configure makes the decoder and the converter of a stream of cfg.
func (t *packedTrack) configure(cfg aac.Config) error {
	if err := (aac.ADTSHeader{Config: cfg, Blocks: 1}).Decodable(); err != nil {
		return fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	dec, err := aac.NewDecoder(cfg)
	if err != nil {
		return fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	// Decode writes stereo, a mono stream's in both channels.
	rs, err := resample.New(cfg.SampleRate, Rate, 2)
	if err != nil {
		return fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	t.cfg, t.dec, t.rs = cfg, dec, rs
	// The converter gives the frames of the input it holds back with those
	// of a frame, or at the end.
	t.out = make([]float32, 2*(rs.MaxOutput(aac.FrameLength)+rs.Latency()))
	return nil
}

// header passes over ID3 tags, and bytes that are no frame, and returns
// the header of the next frame, which it leaves to read.
func (t *packedTrack) header() (aac.ADTSHeader, error) {
	for {
		if t.skip > 0 {
			n, err := t.r.Discard(t.skip)
			t.skip -= n
			if err != nil {
				return aac.ADTSHeader{}, err
			}
		}
		b, err := t.r.Peek(id3Header)
		if len(b) < aac.ADTSHeaderSize {
			return aac.ADTSHeader{}, err // io.EOF at the end, but for a few bytes
		}
		if n, ok := id3Size(b); ok {
			t.skip = n
			continue
		}
		h, ferr := aac.ParseADTS(b)
		if ferr == nil && t.junk > 0 {
			// Past bytes that were no frame, the syncword may be one of them:
			// it starts a frame if another frame, a tag or the end follows.
			b, err = t.r.Peek(h.Length + 2)
			switch {
			case len(b) == h.Length && err == io.EOF:
			case len(b) < h.Length+2 && err != io.EOF:
				return aac.ADTSHeader{}, err
			case len(b) < h.Length+2 || !(b[h.Length] == 0xFF && b[h.Length+1]&0xF6 == 0xF0 || string(b[h.Length:]) == "ID"):
				ferr = aac.ErrFormat
			}
		}
		if ferr == nil {
			return h, nil
		}
		if t.junk++; t.junk > maxJunk {
			return aac.ADTSHeader{}, fmt.Errorf("%w: no ADTS frame in %d bytes", aac.ErrFormat, maxJunk)
		}
		t.r.Discard(1)
	}
}

// id3Size reads the header of an ID3v2 tag at the start of b, of
// id3Header bytes, and returns the size of the tag: its header, its
// frames, and the footer of a tag of version 4 that has one. Its size is
// a syncsafe integer: 28 bits, 7 to a byte.
func id3Size(b []byte) (int, bool) {
	if len(b) < id3Header || string(b[:3]) != "ID3" || b[3] == 0xFF || b[4] == 0xFF || (b[6]|b[7]|b[8]|b[9])&0x80 != 0 {
		return 0, false
	}
	n := id3Header + (int(b[6])<<21 | int(b[7])<<14 | int(b[8])<<7 | int(b[9]))
	if b[3] == 4 && b[5]&0x10 != 0 {
		n += id3Header
	}
	return n, true
}

// seek tells where the stream is: a live one can't move, but the frames
// that come next are at t.
func (t *packedTrack) seek(at time.Duration) error {
	t.at = frames(at)
	return nil
}

func (t *packedTrack) read() error {
	h, err := t.header()
	if err != nil {
		return err
	}
	b, err := t.r.Peek(h.Length)
	if len(b) < h.Length {
		return err // io.EOF for a frame cut short at the end
	}
	t.h = h
	t.frame = append(t.frame[:0], b[h.HeaderLength:]...)
	t.r.Discard(h.Length)
	t.junk = 0
	return nil
}

func (t *packedTrack) decode() (decoded, error) {
	if err := t.h.Decodable(); err != nil {
		return decoded{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	// A stream of another config, as after a discontinuity, follows what
	// the converter held back of the one before.
	var tail []float32
	if t.h.Config != t.cfg {
		tail = append(tail, t.end()...)
		if err := t.configure(t.h.Config); err != nil {
			return decoded{}, err
		}
	}
	n, err := t.dec.Decode(t.frame, t.pcm)
	switch {
	case errors.Is(err, errors.ErrUnsupported):
		return decoded{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	case err != nil:
		if t.bad++; t.bad > maxBadFrames {
			return decoded{}, fmt.Errorf("%w (%v)", errDecode, err)
		}
		n = 0 // dropped
	default:
		t.bad = 0
	}
	_, w := t.rs.Process(t.pcm[:2*n], t.out)
	pcm := t.out[:2*w]
	if tail != nil {
		pcm = append(tail, pcm...)
	}
	d := decoded{pcm: pcm, at: t.at, to: len(pcm) / 2}
	t.at += int64(d.to)
	return d, nil
}

func (t *packedTrack) end() []float32 {
	n := t.rs.Flush(t.out)
	return t.out[:2*n]
}
