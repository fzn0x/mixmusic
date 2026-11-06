package audio

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/fzn0x/mixmusic/ogg"
	"github.com/fzn0x/mixmusic/opus"
	"github.com/jfreymuth/vorbis"
)

// openOgg returns the track of an Ogg file: Opus, or Vorbis. Its errors
// are worded for the player.
func openOgg(src io.ReadSeeker) (track, timing, error) {
	r, err := ogg.NewReader(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	head, err := r.ReadPacket()
	if err != nil {
		return nil, timing{}, readError(err)
	}
	switch {
	case bytes.HasPrefix(head.Data, []byte("OpusHead")):
		return openOggOpus(r, head.Data)
	case bytes.HasPrefix(head.Data, []byte("\x01vorbis")):
		return openVorbis(r, head.Data)
	}
	return nil, timing{}, fmt.Errorf("%w (an Ogg file of another codec)", errUnsupported)
}

// oggOpusTrack is the Opus track of an Ogg file.
type oggOpusTrack struct {
	r   *ogg.Reader
	dec *opus.Decoder
	p   ogg.Packet // the packet read last
	// next is the granule position, the frame at Rate with the pre-skip,
	// that the packet read next starts at.
	next int64
	pcm  []float32
}

// openOggOpus returns the Opus track of r, whose OpusHead is head.
func openOggOpus(r *ogg.Reader, head []byte) (track, timing, error) {
	h, err := parseOpusHead(head)
	if err != nil {
		return nil, timing{}, err
	}
	if tags, err := r.ReadPacket(); err != nil || !bytes.HasPrefix(tags.Data, []byte("OpusTags")) {
		return nil, timing{}, fmt.Errorf("%w (%v: no OpusTags)", errUnsupported, ogg.ErrFormat)
	}
	r.StartAudio()
	last, err := r.LastGranule()
	if err != nil {
		return nil, timing{}, readError(err)
	}
	dec, err := opus.NewDecoder(Rate, 2)
	if err != nil {
		return nil, timing{}, errDecode
	}
	dec.SetGain(h.gain)
	tm := timing{delay: time.Duration(h.preSkip) * time.Second / Rate, preRoll: 80 * time.Millisecond}
	if last > 0 {
		tm.total = duration(last)
	}
	return &oggOpusTrack{r: r, dec: dec, pcm: make([]float32, 2*5760)}, tm, nil
}

func (t *oggOpusTrack) channels() int { return 2 }

func (t *oggOpusTrack) seek(at time.Duration) error {
	start, err := t.r.SeekGranule(frames(at))
	if err != nil {
		return err
	}
	t.next = start
	t.dec.Reset()
	return nil
}

func (t *oggOpusTrack) read() error {
	var err error
	t.p, err = t.r.ReadPacket()
	return err
}

func (t *oggOpusTrack) decode() (decoded, error) {
	n, err := t.dec.Decode(t.p.Data, t.pcm, false)
	if err != nil {
		return decoded{}, errDecode
	}
	d := decoded{pcm: t.pcm[:2*n], at: t.next, to: n}
	// The last page's granule position tells where the audio ends, short
	// of the last packet's end.
	if t.p.Last && t.p.Granule >= 0 && t.next+int64(n) > t.p.Granule {
		d.to = int(max(0, t.p.Granule-t.next))
	}
	t.next += int64(n)
	return d, nil
}

func (t *oggOpusTrack) end() []float32 { return nil }

// vorbisCodec decodes the Vorbis of an Ogg file, with the decoder of
// jfreymuth/vorbis.
type vorbisCodec struct {
	r   *ogg.Reader
	dec vorbis.Decoder
	// at is the frame of the next frame decoded, -1 until a page tells
	// it; ahead holds what a seek decoded on to learn it.
	at    int64
	ahead []float32
	pcm   []float32
	// order maps the channels of Vorbis's order to those of FLAC's, which
	// a pcmTrack mixes down.
	order []int
}

// vorbisOrders map the channels of Vorbis's order, of each count, to
// those of FLAC's: front left, right and center, LFE, the back pair and
// the side pair.
var vorbisOrders = map[int][]int{
	3: {0, 2, 1},
	5: {0, 2, 1, 3, 4},
	6: {0, 2, 1, 4, 5, 3},
	7: {0, 2, 1, 5, 6, 4, 3},
	8: {0, 2, 1, 6, 7, 4, 5, 3},
}

// openVorbis returns the Vorbis track of r, whose identification header
// is head.
func openVorbis(r *ogg.Reader, head []byte) (track, timing, error) {
	c := &vorbisCodec{r: r, at: -1}
	if err := c.guard(func() error { return c.dec.ReadHeader(head) }); err != nil {
		return nil, timing{}, err
	}
	for range 2 {
		p, err := r.ReadPacket()
		if err != nil {
			return nil, timing{}, readError(err)
		}
		if err := c.guard(func() error { return c.dec.ReadHeader(p.Data) }); err != nil {
			return nil, timing{}, err
		}
	}
	r.StartAudio()
	last, err := r.LastGranule()
	if err != nil {
		return nil, timing{}, readError(err)
	}
	rate, ch := c.dec.SampleRate(), c.dec.Channels()
	if rate <= 0 || ch < 1 || ch > 8 {
		return nil, timing{}, fmt.Errorf("%w (Vorbis of %d channels at %d Hz)", errUnsupported, ch, rate)
	}
	c.order = vorbisOrders[ch]
	c.pcm = make([]float32, c.dec.BufferSize())
	var total time.Duration
	if last > 0 {
		total = time.Duration(last) * time.Second / time.Duration(rate)
	}
	return newPCMTrack(c, rate, ch, timing{total: total})
}

// guard runs fn, which runs the Vorbis decoder: its errors, and its
// panics, as at a file made to break it, are the file's.
func (c *vorbisCodec) guard(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w (Vorbis: the decoder failed: %v)", errDecode, p)
		}
	}()
	if err := fn(); err != nil {
		return fmt.Errorf("%w (Vorbis: %v)", errDecode, err)
	}
	return nil
}

// seek moves to the packet after the last that ends at or before frame f.
// A packet's audio overlaps the one before it, which a seek doesn't have:
// the first after a seek gives none, and where the next starts only a
// page's granule position tells, so the seek decodes on to the end of a
// page. From the start of the stream, the audio starts at 0. The last
// page's granule position tells where the audio ends, short of its last
// packet's end, which places the audio before it only roughly.
func (c *vorbisCodec) seek(f int64) (int64, error) {
	start, err := c.r.SeekGranule(f)
	if err != nil {
		return 0, err
	}
	c.dec.Clear()
	c.ahead, c.at = c.ahead[:0], -1
	if start == 0 {
		c.at = 0
		return 0, nil
	}
	ch := int64(c.dec.Channels())
	for {
		pcm, g, last, err := c.next()
		if err == io.EOF {
			c.at = start + int64(len(c.ahead))/ch
			return start, nil
		}
		if err != nil {
			return 0, err
		}
		c.ahead = append(c.ahead, pcm...)
		if g >= 0 {
			c.at = g
			if last {
				c.at = max(g, start+int64(len(c.ahead))/ch)
			}
			return c.at - int64(len(c.ahead))/ch, nil
		}
	}
}

// next decodes the next packet, and returns its frames, and the frame its
// audio ends at, when its page tells it, else -1, and whether it is the
// last: the last page's granule position tells where the audio ends,
// short of the last packet's end, which next cuts there.
func (c *vorbisCodec) next() ([]float32, int64, bool, error) {
	p, err := c.r.ReadPacket()
	if err != nil {
		return nil, -1, false, err
	}
	var pcm []float32
	err = c.guard(func() (err error) {
		pcm, err = c.dec.DecodeInto(p.Data, c.pcm)
		return err
	})
	if err != nil {
		return nil, -1, false, err
	}
	ch := c.dec.Channels()
	if p.Last && p.Granule >= 0 && c.at >= 0 {
		if keep := p.Granule - c.at; keep < int64(len(pcm)/ch) {
			pcm = pcm[:ch*int(max(0, keep))]
		}
	}
	if c.order != nil {
		reorder(pcm, ch, c.order)
	}
	return pcm, p.Granule, p.Last, nil
}

func (c *vorbisCodec) read() ([]float32, error) {
	if len(c.ahead) > 0 {
		pcm := c.ahead
		c.ahead = c.ahead[len(c.ahead):]
		return pcm, nil
	}
	pcm, g, last, err := c.next()
	if err != nil {
		if errors.Is(err, ogg.ErrFormat) {
			err = fmt.Errorf("%w (%v)", errUnsupported, err)
		}
		return nil, err
	}
	if c.at >= 0 {
		c.at += int64(len(pcm) / c.dec.Channels())
	}
	if g >= 0 && !last {
		c.at = g
	}
	return pcm, nil
}

// reorder moves the channels of each frame of pcm, of ch channels, to
// where order puts them.
func reorder(pcm []float32, ch int, order []int) {
	var frame [8]float32
	for i := 0; i+ch <= len(pcm); i += ch {
		for c, to := range order {
			frame[to] = pcm[i+c]
		}
		copy(pcm[i:i+ch], frame[:ch])
	}
}
