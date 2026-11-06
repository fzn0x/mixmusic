package flac

import (
	"errors"
	"fmt"
	"io"
)

// Frame is the audio of a frame.
type Frame struct {
	// Sample is the number of the frame's first sample in the stream,
	// from 0.
	Sample int64
	// Samples holds the samples of each channel, as many in each, which
	// the Reader reuses for the frame it reads next.
	Samples [][]int32
	// BitsPerSample is the bits of the samples.
	BitsPerSample int
}

// header is what a frame's header tells.
type header struct {
	variable bool
	// block is the samples of each channel the frame holds, and rate their
	// rate.
	block, rate int
	// assign is the frame's channel assignment: up to 7 the channels, less
	// one, coded each alone, and 8, 9 and 10 the stereo of a left and a
	// side channel, a side and a right one, and a mid and a side one.
	assign int
	bps    int
	// sample is the number of the frame's first sample.
	sample int64
}

func (h header) channels() int {
	if h.assign >= 8 {
		return 2
	}
	return h.assign + 1
}

// errSync is why what is at the start of a frame isn't one.
var errSync = fmt.Errorf("%w: no frame here", ErrFormat)

// The sample rates a frame's header codes in 4 bits, 0 when it codes none.
var rates = [16]int{0, 88200, 176400, 192000, 8000, 16000, 22050, 24000, 32000, 44100, 48000, 96000}

// The bits a sample a frame's header codes in 3 bits, 0 when it codes none.
var depths = [8]int{0, 8, 12, 0, 16, 20, 24, 32}

// readHeader reads the header of the frame that starts at the next byte,
// and checks it: errSync when there is no frame there, io.EOF at the end
// of the file.
func (r *Reader) readHeader() (header, error) {
	s := &r.src
	s.start()
	if err := s.fill(1); err == io.ErrUnexpectedEOF {
		return header{}, io.EOF
	} else if err != nil {
		return header{}, err
	}
	var h header
	v, err := s.read(16)
	if err != nil {
		return h, eofSync(err)
	}
	if v&0xFFFE != 0xFFF8 {
		return h, errSync
	}
	h.variable = v&1 != 0
	if v, err = s.read(16); err != nil {
		return h, eofSync(err)
	}
	blockCode, rateCode := int(v>>12), int(v>>8&15)
	h.assign, h.bps = int(v>>4&15), depths[v>>1&7]
	if v&1 != 0 || h.assign > 10 || rateCode == 15 || blockCode == 0 || v>>1&7 == 3 {
		return h, errSync
	}
	number, err := r.codedNumber(h.variable)
	if err != nil {
		return h, err
	}
	switch {
	case blockCode == 1:
		h.block = 192
	case blockCode <= 5:
		h.block = 576 << (blockCode - 2)
	case blockCode == 6:
		v, err = s.read(8)
		h.block = int(v) + 1
	case blockCode == 7:
		v, err = s.read(16)
		h.block = int(v) + 1
	default:
		h.block = 256 << (blockCode - 8)
	}
	if err != nil {
		return h, eofSync(err)
	}
	switch rateCode {
	case 0:
		h.rate = r.info.SampleRate
	case 12:
		v, err = s.read(8)
		h.rate = int(v) * 1000
	case 13:
		v, err = s.read(16)
		h.rate = int(v)
	case 14:
		v, err = s.read(16)
		h.rate = int(v) * 10
	default:
		h.rate = rates[rateCode]
	}
	if err != nil {
		return h, eofSync(err)
	}
	if h.bps == 0 {
		h.bps = r.info.BitsPerSample
	}
	sum := crc8(s.bytes())
	if v, err = s.read(8); err != nil {
		return h, eofSync(err)
	}
	if uint8(v) != sum {
		return h, errSync
	}
	// A frame of other channels, or of another rate, than the stream
	// tells is not one of its frames.
	if h.channels() != r.info.Channels || h.rate != r.info.SampleRate || h.block > 65535 {
		return h, errSync
	}
	if h.variable {
		h.sample = int64(number)
	} else {
		size := int64(h.block)
		if r.info.MinBlock == r.info.MaxBlock && r.info.MaxBlock > 0 {
			size = int64(r.info.MaxBlock)
		}
		h.sample = int64(number) * size
	}
	return h, nil
}

// eofSync turns the end of the file within a frame's header into errSync:
// what is there isn't a frame.
func eofSync(err error) error {
	if err == io.ErrUnexpectedEOF {
		return errSync
	}
	return err
}

// codedNumber reads the number of a frame's header, coded as UTF-8 codes
// characters, in up to 7 bytes: a frame's number, of up to 31 bits, or of
// a variable stream its first sample's, of up to 36.
func (r *Reader) codedNumber(variable bool) (uint64, error) {
	s := &r.src
	v, err := s.read(8)
	if err != nil {
		return 0, eofSync(err)
	}
	if v < 0x80 {
		return v, nil
	}
	n := 0 // the bytes that follow
	for m := uint64(0x40); v&m != 0; m >>= 1 {
		n++
	}
	if n == 0 || n > 6 || !variable && n > 5 {
		return 0, errSync
	}
	v &= 0x3F >> n
	for range n {
		b, err := s.read(8)
		if err != nil {
			return 0, eofSync(err)
		}
		if b&0xC0 != 0x80 {
			return 0, errSync
		}
		v = v<<6 | b&0x3F
	}
	return v, nil
}

// ReadFrame reads the next frame, io.EOF after the last. The frame is the
// Reader's: it holds the samples until the next ReadFrame or Seek.
func (r *Reader) ReadFrame() (*Frame, error) {
	if r.pending {
		r.pending = false
		return &r.frame, nil
	}
	for {
		at := r.src.offset()
		err := r.readFrame()
		if err == nil {
			return &r.frame, nil
		}
		if !errors.Is(err, ErrFormat) {
			return nil, err
		}
		// What isn't a frame, or a frame whose checksum fails, is passed
		// over to the next frame: past the last, the file ends.
		if err := r.resync(at + 1); err != nil {
			return nil, err
		}
	}
}

// resync moves to the first frame from offset from on, or to the end.
func (r *Reader) resync(from int64) error {
	_, _, err := r.findFrame(from, r.size)
	if err == io.EOF {
		_ = r.src.seekTo(r.size)
	}
	return err
}

// readFrame reads a frame whole, and checks its checksum.
func (r *Reader) readFrame() error {
	h, err := r.readHeader()
	if err != nil {
		return err
	}
	ch := h.channels()
	for len(r.work) < ch {
		r.work = append(r.work, nil)
	}
	for c := range ch {
		bps := h.bps
		switch {
		case h.assign == 8 && c == 1, h.assign == 9 && c == 0, h.assign == 10 && c == 1:
			bps++ // the side channel
		}
		if cap(r.work[c]) < h.block {
			r.work[c] = make([]int64, h.block)
		}
		r.work[c] = r.work[c][:h.block]
		if err := r.subframe(r.work[c], bps); err != nil {
			return eofFormat(err)
		}
	}
	s := &r.src
	s.align()
	sum := crc16(s.bytes())
	v, err := s.read(16)
	if err != nil {
		return eofFormat(err)
	}
	if uint16(v) != sum {
		return fmt.Errorf("%w: the checksum of the frame at sample %d fails", ErrFormat, h.sample)
	}
	decorrelate(r.work[:ch], h.assign)
	f := &r.frame
	f.Sample, f.BitsPerSample = h.sample, h.bps
	for len(f.Samples) < ch {
		f.Samples = append(f.Samples, nil)
	}
	f.Samples = f.Samples[:ch]
	for c, w := range r.work[:ch] {
		if cap(f.Samples[c]) < h.block {
			f.Samples[c] = make([]int32, h.block)
		}
		out := f.Samples[c][:h.block]
		for i, v := range w {
			out[i] = int32(v)
		}
		f.Samples[c] = out
	}
	return nil
}

// eofFormat turns the end of the file within a frame into an ErrFormat.
func eofFormat(err error) error {
	if err == io.ErrUnexpectedEOF {
		return fmt.Errorf("%w: the file ends within a frame", ErrFormat)
	}
	return err
}

// subframe decodes a channel's subframe into out, of samples of bps bits.
func (r *Reader) subframe(out []int64, bps int) error {
	s := &r.src
	v, err := s.read(8)
	if err != nil {
		return err
	}
	kind := int(v >> 1 & 63)
	if v&0x80 != 0 {
		return fmt.Errorf("%w: a subframe's padding is set", ErrFormat)
	}
	wasted := 0
	if v&1 != 0 {
		k, err := s.unary()
		if err != nil {
			return err
		}
		if k+1 >= uint64(bps) {
			return fmt.Errorf("%w: %d wasted bits of %d", ErrFormat, k+1, bps)
		}
		wasted = int(k) + 1
		bps -= wasted
	}
	switch {
	case kind == 0:
		c, err := s.signed(uint(bps))
		if err != nil {
			return err
		}
		for i := range out {
			out[i] = c
		}
	case kind == 1:
		for i := range out {
			if out[i], err = s.signed(uint(bps)); err != nil {
				return err
			}
		}
	case kind >= 8 && kind <= 12:
		if err := r.fixed(out, bps, kind-8); err != nil {
			return err
		}
	case kind >= 32:
		if err := r.lpc(out, bps, kind-31); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: a subframe of the reserved type %d", ErrFormat, kind)
	}
	if wasted > 0 {
		for i := range out {
			out[i] <<= wasted
		}
	}
	return nil
}

// warmup reads the first order samples of a predicted subframe, as they
// are.
func (r *Reader) warmup(out []int64, bps, order int) error {
	if order > len(out) {
		return fmt.Errorf("%w: a prediction of order %d in a frame of %d samples", ErrFormat, order, len(out))
	}
	var err error
	for i := range order {
		if out[i], err = r.src.signed(uint(bps)); err != nil {
			return err
		}
	}
	return nil
}

// fixed decodes a subframe of a fixed predictor of order up to 4.
func (r *Reader) fixed(out []int64, bps, order int) error {
	if err := r.warmup(out, bps, order); err != nil {
		return err
	}
	if err := r.residual(out, order); err != nil {
		return err
	}
	switch order {
	case 1:
		for i := 1; i < len(out); i++ {
			out[i] += out[i-1]
		}
	case 2:
		for i := 2; i < len(out); i++ {
			out[i] += 2*out[i-1] - out[i-2]
		}
	case 3:
		for i := 3; i < len(out); i++ {
			out[i] += 3*out[i-1] - 3*out[i-2] + out[i-3]
		}
	case 4:
		for i := 4; i < len(out); i++ {
			out[i] += 4*out[i-1] - 6*out[i-2] + 4*out[i-3] - out[i-4]
		}
	}
	return nil
}

// lpc decodes a subframe of a linear predictor of order up to 32.
func (r *Reader) lpc(out []int64, bps, order int) error {
	s := &r.src
	if err := r.warmup(out, bps, order); err != nil {
		return err
	}
	v, err := s.read(4)
	if err != nil {
		return err
	}
	if v == 15 {
		return fmt.Errorf("%w: coefficients of the reserved precision", ErrFormat)
	}
	precision := uint(v) + 1
	shift, err := s.signed(5)
	if err != nil {
		return err
	}
	if shift < 0 {
		return fmt.Errorf("%w: a prediction shifted by %d", ErrFormat, shift)
	}
	coefs := r.res[:order]
	for i := range coefs {
		if coefs[i], err = s.signed(precision); err != nil {
			return err
		}
	}
	if err := r.residual(out, order); err != nil {
		return err
	}
	for i := order; i < len(out); i++ {
		var sum int64
		for j, c := range coefs {
			sum += c * out[i-j-1]
		}
		out[i] += sum >> shift
	}
	return nil
}

// residual reads the residual of a predicted subframe into out from
// order on, in partitions of Rice codes, or of numbers as they are.
func (r *Reader) residual(out []int64, order int) error {
	s := &r.src
	v, err := s.read(6)
	if err != nil {
		return err
	}
	method, partitionOrder := v>>4, uint(v&15)
	if method > 1 {
		return fmt.Errorf("%w: a residual of the reserved coding %d", ErrFormat, method)
	}
	paramBits, escape := uint(4), uint64(15)
	if method == 1 {
		paramBits, escape = 5, 31
	}
	size := len(out) >> partitionOrder
	if size<<partitionOrder != len(out) || size < order {
		return fmt.Errorf("%w: %d partitions of a residual of %d samples", ErrFormat, 1<<partitionOrder, len(out)-order)
	}
	i := order
	for p := range 1 << partitionOrder {
		end := (p + 1) * size
		param, err := s.read(paramBits)
		if err != nil {
			return err
		}
		if param == escape {
			n, err := s.read(5)
			if err != nil {
				return err
			}
			for ; i < end; i++ {
				if n == 0 {
					out[i] = 0
				} else if out[i], err = s.signed(uint(n)); err != nil {
					return err
				}
			}
			continue
		}
		k := uint(param)
		for ; i < end; i++ {
			q, err := s.unary()
			if err != nil {
				return err
			}
			low, err := s.read(k)
			if err != nil {
				return err
			}
			u := q<<k | low
			out[i] = int64(u>>1) ^ -int64(u&1)
		}
	}
	return nil
}

// decorrelate turns the stereo of a left and a side channel, of a side
// and a right one, or of a mid and a side one, into left and right.
func decorrelate(ch [][]int64, assign int) {
	switch assign {
	case 8:
		for i, side := range ch[1] {
			ch[1][i] = ch[0][i] - side
		}
	case 9:
		for i, side := range ch[0] {
			ch[0][i] = side + ch[1][i]
		}
	case 10:
		for i, side := range ch[1] {
			mid := ch[0][i]<<1 | side&1
			ch[0][i], ch[1][i] = (mid+side)>>1, (mid-side)>>1
		}
	}
}
