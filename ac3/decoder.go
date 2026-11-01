package ac3

import (
	"fmt"
)

// The channels of a stream's state, by their index in it: its
// full-bandwidth channels in the order of the audio coding mode, then the
// LFE, then the coupling channel.
const (
	maxFull   = 5 // full-bandwidth channels
	lfeIndex  = 5
	cplIndex  = 6
	maxStates = 7
)

// maxBins is the transform coefficients of a block of a channel.
const maxBins = 256

// channel is the state of a channel of a stream: what the decoder read of
// it in the block, what it keeps for the next blocks of the frame, and
// what it keeps of the last block for the overlap.
type channel struct {
	// The exponents, and the bins they are of: start to end.
	exp        [maxBins]uint8
	start, end int
	// The bit allocation's parameters of the channel.
	fsnroffst, fgaincod int
	// The delta bit allocation: deltbae, as the block read it, whether
	// delta applies, and its segments.
	deltbae                    int
	delta                      bool
	deltnseg                   int
	deltoffst, deltlen, deltba [8]int
	// bap is the bit allocation of each bin.
	bap [maxBins]uint8
	// coef is the transform coefficients of the block.
	coef [maxBins]float32
	// overlap is the second half of the last block's window, which the
	// first half of this block's overlaps.
	overlap [maxBins]float32

	blksw, dithflag bool
}

// Decoder decodes an AC-3 stream. It is not safe for concurrent use.
type Decoder struct {
	info Info
	r    bitReader
	h    header
	ch   [maxStates]channel

	// The dynamic range gains of the block: of the program, and of Ch2 of
	// a 1+1 stream.
	dynrng, dynrng2 float32

	// Coupling.
	cplinu    bool
	chincpl   [maxFull]bool
	phsflginu bool
	cplbegf   int
	cplendf   int
	// cplbndstrc tells, of each sub-band, whether it continues the band
	// of the one before it.
	cplbndstrc [18]bool
	ncplbnd    int
	// cplco is the coupling coordinate of each channel in each sub-band.
	cplco  [maxFull][18]float32
	phsflg [18]bool // of each sub-band
	// Rematrixing, of a 2/0 stream.
	rematflg [4]bool

	// The bit allocation's parameters of the block.
	sdcycod, fdcycod, sgaincod, dbpbcod, floorcod int
	csnroffst                                     int
	cplfleak, cplsleak                            int

	// random is the state of the generator of the dither.
	random uint32

	// ditherAt is where the dithflags of the block start, in bits,
	// phsflginuAt where its phsflginu is and phsflgAt where its phase
	// flags are or would be, or -1, deltaAt where its deltbaie is, mantAt
	// where its mantissas start, and onBlock, when set, is called after
	// each block is read: for tests.
	noCRC                 bool // for fuzzing, which CRCs would stop early
	ditherAt              int
	phsflginuAt, phsflgAt int
	deltaAt, mantAt       int
	onBlock               func(blk int)
	expstr                [maxFull]int // the exponent strategies of the block
	cplexpstr             int

	// Scratch of the bit allocation, and of the transform.
	psd    [maxBins]int
	bndpsd [50]int
	excite [50]int
	mask   [50]int
	tf     transform
	out    [BlockLength]float32
}

// NewDecoder returns a decoder.
func NewDecoder() *Decoder {
	d := &Decoder{}
	d.tf.init()
	d.Reset()
	return d
}

// Info returns what the syncframes decoded last tell of the stream: the
// zero Info before the first.
func (d *Decoder) Info() Info { return d.info }

// Reset makes the decoder forget the stream, as for a new one or after a
// seek: the block decoded next overlaps silence.
func (d *Decoder) Reset() {
	for c := range d.ch {
		clear(d.ch[c].overlap[:])
	}
	d.random = 1
}

// Decode decodes the syncframes of data, as an MP4 sample holds them, into
// pcm, and returns the samples per channel: FrameLength for an MP4
// sample. The samples are interleaved, of MaxChannels channels in the
// order of Left to RightSurround, those the stream lacks silent, as
// float32 of about [-1, 1], not clipped, but finite. pcm must have room
// for MaxChannels times the samples of data's frames: MaxChannels *
// FrameLength for an MP4 sample.
//
// A block decodes into the second half of the last one's window and the
// first half of its own: the first block of a stream, or the first after
// Reset, holds only half a window of audio. On an error, Decode returns
// the samples of the frames before the bad one, and the next call decodes
// as usual.
func (d *Decoder) Decode(data []byte, pcm []float32) (int, error) {
	n := 0
	for len(data) > 0 {
		var h header
		d.r.init(data)
		if err := h.parse(&d.r); err != nil {
			return n, err
		}
		if len(data) < h.size {
			return n, fmt.Errorf("%w: a syncframe of %d bytes in %d", ErrFormat, h.size, len(data))
		}
		if len(pcm) < MaxChannels*(n+FrameLength) {
			return n, ErrBufferTooSmall
		}
		frame := data[:h.size]
		data = data[h.size:]
		if err := checkCRC(frame); err != nil && !d.noCRC {
			return n, err
		}
		d.h = h
		d.r.init(frame)
		d.r.pos = h.end
		if err := d.decodeFrame(pcm[MaxChannels*n:]); err != nil {
			return n, err
		}
		d.setInfo()
		n += FrameLength
	}
	return n, nil
}

// decodeFrame decodes the six audio blocks of the syncframe whose bsi r
// read last, into pcm.
func (d *Decoder) decodeFrame(pcm []float32) error {
	for blk := range 6 {
		if err := d.parseBlock(blk); err != nil {
			return fmt.Errorf("block %d: %w", blk, err)
		}
		d.decodeBlock(pcm[MaxChannels*BlockLength*blk:])
	}
	return nil
}

// outputOf is, for each audio coding mode, the channel of the output of
// each full-bandwidth channel of the stream, in their order; -1 for the
// single surround channel, which goes to both surround outputs.
var outputOf = [8][maxFull]int{
	{Left, Right},
	{Center},
	{Left, Right},
	{Left, Center, Right},
	{Left, Right, -1},
	{Left, Center, Right, -1},
	{Left, Right, LeftSurround, RightSurround},
	{Left, Center, Right, LeftSurround, RightSurround},
}

// minus3dB is the gain of -3 dB.
const minus3dB = 0.70710677

// decodeBlock turns the coefficients of the block parseBlock read into
// samples, in pcm.
func (d *Decoder) decodeBlock(pcm []float32) {
	pcm = pcm[:MaxChannels*BlockLength]
	clear(pcm)
	for c := range d.h.nfchans {
		ch := &d.ch[c]
		d.tf.inverse(&ch.coef, &ch.overlap, ch.blksw, &d.out)
		if o := outputOf[d.h.acmod][c]; o >= 0 {
			d.put(pcm, o, 1)
		} else {
			// The single surround channel goes to both surround outputs at
			// -3 dB, as section 7.8.1 routes it.
			d.put(pcm, LeftSurround, minus3dB)
			d.put(pcm, RightSurround, minus3dB)
		}
	}
	if d.h.lfeon {
		ch := &d.ch[lfeIndex]
		d.tf.inverse(&ch.coef, &ch.overlap, false, &d.out)
		d.put(pcm, LFE, 1)
	}
}

// put writes the block of samples the transform made into channel c of
// pcm, times gain.
func (d *Decoder) put(pcm []float32, c int, gain float32) {
	for i, v := range d.out {
		pcm[MaxChannels*i+c] = v * gain
	}
}
