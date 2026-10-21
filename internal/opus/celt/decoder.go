package celt

// Port of celt/celt_decoder.c.

import (
	"errors"

	"github.com/fzn0x/mixmusic/internal/opus/rangedec"
)

var (
	// ErrBadArg is OPUS_BAD_ARG.
	ErrBadArg = errors.New("celt: invalid argument")
	// ErrInternal is OPUS_INTERNAL_ERROR: the frame read past its end.
	ErrInternal = errors.New("celt: internal error")
)

const (
	frameNone = iota
	frameNormal
	framePLCNoise
	framePLCPeriodic
)

// Decoder is a CELT decoder, OpusCustomDecoder of the 48 kHz mode.
type Decoder struct {
	m              *mode
	overlap        int
	channels       int
	streamChannels int
	downsample     int
	start, end     int
	disableInv     bool

	// Everything from here clears on a reset.
	rng                 uint32
	err                 bool
	lastPitchIndex      int
	lossDuration        int
	plcDuration         int
	lastFrameType       int
	skipPLC             bool
	postfilterPeriod    int
	postfilterPeriodOld int
	postfilterGain      float32
	postfilterGainOld   float32
	postfilterTapset    int
	postfilterTapsetOld int
	prefilterAndFold    bool
	preemphMemD         [2]float32
	decodeMem           [2][]float32 // decodeBufferSize+overlap each
	oldEBands           [2 * maxBandsNb]float32
	oldLogE             [2 * maxBandsNb]float32
	oldLogE2            [2 * maxBandsNb]float32
	backgroundLogE      [2 * maxBandsNb]float32
	lpc                 [2 * celtLPCOrder]float32

	// Scratch space.
	ec            rangedec.Decoder
	x             []float32
	freq          []float32
	scratch       []float32
	etmp          []float32
	exc           []float32
	firTmp        []float32
	iirScratch    []float32
	lpPitchBuf    []float32
	tfRes         [maxBandsNb]int
	cap           [maxBandsNb]int
	offsets       [maxBandsNb]int
	collapseMasks [2 * maxBandsNb]uint8
	alloc         allocation
	bands         *bandScratch
}

// NewDecoder returns a decoder of channels channels, 1 or 2, at sampleRate,
// 48000, 24000, 16000, 12000 or 8000 Hz.
func NewDecoder(sampleRate, channels int) (*Decoder, error) {
	if channels < 1 || channels > 2 {
		return nil, ErrBadArg
	}
	m := mode48000_960_120
	st := &Decoder{
		m:              m,
		overlap:        m.overlap,
		channels:       channels,
		streamChannels: channels,
		start:          0,
		end:            m.effEBands,
		disableInv:     channels == 1,
		x:              make([]float32, 2*frameSizeMax),
		freq:           make([]float32, frameSizeMax),
		scratch:        make([]float32, frameSizeMax),
		etmp:           make([]float32, m.overlap),
		exc:            make([]float32, maxPeriod+celtLPCOrder),
		firTmp:         make([]float32, maxPeriod),
		iirScratch:     make([]float32, frameSizeMax+m.overlap+celtLPCOrder),
		lpPitchBuf:     make([]float32, decodeBufferSize>>1),
		bands:          newBandScratch(),
	}
	for c := 0; c < channels; c++ {
		st.decodeMem[c] = make([]float32, decodeBufferSize+m.overlap)
	}
	st.downsample = resamplingFactor(sampleRate)
	if st.downsample == 0 {
		return nil, ErrBadArg
	}
	st.Reset()
	return st, nil
}

// Reset is OPUS_RESET_STATE.
func (st *Decoder) Reset() {
	st.rng = 0
	st.err = false
	st.lastPitchIndex = 0
	st.lossDuration = 0
	st.plcDuration = 0
	st.postfilterPeriod, st.postfilterPeriodOld = 0, 0
	st.postfilterGain, st.postfilterGainOld = 0, 0
	st.postfilterTapset, st.postfilterTapsetOld = 0, 0
	st.prefilterAndFold = false
	st.preemphMemD = [2]float32{}
	for c := 0; c < st.channels; c++ {
		clear(st.decodeMem[c])
	}
	st.oldEBands = [2 * maxBandsNb]float32{}
	st.backgroundLogE = [2 * maxBandsNb]float32{}
	st.lpc = [2 * celtLPCOrder]float32{}
	for i := range st.oldLogE {
		st.oldLogE[i] = -28
		st.oldLogE2[i] = -28
	}
	st.skipPLC = true
	st.lastFrameType = frameNone
}

// SetStartBand is CELT_SET_START_BAND: 17 for the CELT layer of hybrid
// frames, which SILK codes below.
func (st *Decoder) SetStartBand(start int) { st.start = start }

// SetEndBand is CELT_SET_END_BAND, which narrows the bandwidth.
func (st *Decoder) SetEndBand(end int) { st.end = end }

// SetStreamChannels is CELT_SET_CHANNELS: the channels of the stream,
// which the decoder mixes to or duplicates into its own.
func (st *Decoder) SetStreamChannels(channels int) { st.streamChannels = channels }

// FinalRange is OPUS_GET_FINAL_RANGE.
func (st *Decoder) FinalRange() uint32 { return st.rng }

// Window is the MDCT window, which the Opus decoder fades transitions
// between modes with.
func (st *Decoder) Window() []float32 { return st.m.window }

func (st *Decoder) deemphasis(in [2][]float32, pcm []float32, n, c int, accum bool) {
	coef0 := st.m.preemph[0]
	mem := &st.preemphMemD
	if st.downsample == 1 && c == 2 && !accum {
		// Both channels in one loop: the common case.
		x0, x1 := in[0], in[1]
		m0, m1 := mem[0], mem[1]
		for j := 0; j < n; j++ {
			// verySmall first, to shorten the dependency chain.
			tmp0 := x0[j] + verySmall + m0
			tmp1 := x1[j] + verySmall + m1
			m0 = coef0 * tmp0
			m1 = coef0 * tmp1
			pcm[2*j] = sig2res(tmp0)
			pcm[2*j+1] = sig2res(tmp1)
		}
		mem[0], mem[1] = m0, m1
		return
	}
	nd := n / st.downsample
	for ch := 0; ch < c; ch++ {
		x := in[ch]
		m := mem[ch]
		downsampled := false
		switch {
		case st.downsample > 1:
			for j := 0; j < n; j++ {
				tmp := x[j] + verySmall + m
				m = coef0 * tmp
				st.scratch[j] = tmp
			}
			downsampled = true
		case accum:
			for j := 0; j < n; j++ {
				tmp := x[j] + m + verySmall
				m = coef0 * tmp
				pcm[j*c+ch] += sig2res(tmp)
			}
		default:
			for j := 0; j < n; j++ {
				tmp := x[j] + verySmall + m
				m = coef0 * tmp
				pcm[j*c+ch] = sig2res(tmp)
			}
		}
		mem[ch] = m
		if downsampled {
			if accum {
				for j := 0; j < nd; j++ {
					pcm[j*c+ch] += sig2res(st.scratch[j*st.downsample])
				}
			} else {
				for j := 0; j < nd; j++ {
					pcm[j*c+ch] = sig2res(st.scratch[j*st.downsample])
				}
			}
		}
	}
}

// synthesis is celt_synthesis: the samples of the frame from its shapes x
// and energies oldBandE, overlapped into outSyn.
func (st *Decoder) synthesis(x []float32, outSyn [2][]float32, oldBandE []float32, start, effEnd, c, cc int,
	isTransient bool, lm int, silence bool) {
	m := st.m
	overlap := m.overlap
	nb := m.nbEBands
	n := m.shortMdctSize << lm
	freq := st.freq[:n]
	mm := 1 << lm
	bb, nbs, shift := 1, m.shortMdctSize<<lm, m.maxLM-lm
	if isTransient {
		bb, nbs, shift = mm, m.shortMdctSize, m.maxLM
	}
	switch {
	case cc == 2 && c == 1:
		// A mono stream to two channels.
		m.denormaliseBands(x, freq, oldBandE, start, effEnd, mm, st.downsample, silence)
		// A copy in the output buffer, as the IMDCT destroys its input.
		freq2 := outSyn[1][overlap/2:]
		copy(freq2[:n], freq)
		for b := 0; b < bb; b++ {
			mdctBackward(&m.mdct, freq2[b:], outSyn[0][nbs*b:], m.window, overlap, shift, bb)
		}
		for b := 0; b < bb; b++ {
			mdctBackward(&m.mdct, freq[b:], outSyn[1][nbs*b:], m.window, overlap, shift, bb)
		}
	case cc == 1 && c == 2:
		// A stereo stream mixed to mono, through the output buffer.
		freq2 := outSyn[0][overlap/2:]
		m.denormaliseBands(x, freq, oldBandE, start, effEnd, mm, st.downsample, silence)
		m.denormaliseBands(x[n:], freq2, oldBandE[nb:], start, effEnd, mm, st.downsample, silence)
		for i := 0; i < n; i++ {
			freq[i] = .5*freq[i] + .5*freq2[i]
		}
		for b := 0; b < bb; b++ {
			mdctBackward(&m.mdct, freq[b:], outSyn[0][nbs*b:], m.window, overlap, shift, bb)
		}
	default:
		for ch := 0; ch < cc; ch++ {
			m.denormaliseBands(x[ch*n:], freq, oldBandE[ch*nb:], start, effEnd, mm, st.downsample, silence)
			for b := 0; b < bb; b++ {
				mdctBackward(&m.mdct, freq[b:], outSyn[ch][nbs*b:], m.window, overlap, shift, bb)
			}
		}
	}
}

func tfDecode(start, end int, isTransient bool, tfRes []int, lm int, dec *rangedec.Decoder) {
	budget := dec.Storage() * 8
	tell := uint32(dec.Tell())
	logp := 4
	if isTransient {
		logp = 2
	}
	tfSelectRsv := uint32(0)
	if lm > 0 && tell+uint32(logp)+1 <= budget {
		tfSelectRsv = 1
	}
	budget -= tfSelectRsv
	tfChanged, curr := 0, 0
	for i := start; i < end; i++ {
		if tell+uint32(logp) <= budget {
			curr ^= b2i(dec.BitLogp(uint(logp)))
			tell = uint32(dec.Tell())
			tfChanged |= curr
		}
		tfRes[i] = curr
		logp = 5
		if isTransient {
			logp = 4
		}
	}
	tfSelect := 0
	it := b2i(isTransient)
	if tfSelectRsv != 0 && tfSelectTable[lm][4*it+0+tfChanged] != tfSelectTable[lm][4*it+2+tfChanged] {
		tfSelect = b2i(dec.BitLogp(1))
	}
	for i := start; i < end; i++ {
		tfRes[i] = int(tfSelectTable[lm][4*it+2*tfSelect+tfRes[i]])
	}
}

func (st *Decoder) plcPitchSearch(c int) int {
	lp := st.lpPitchBuf[:decodeBufferSize>>1]
	pitchDownsample(st.decodeMem, lp, decodeBufferSize>>1, c, 2)
	pitchIndex := pitchSearch(lp[plcPitchLagMax>>1:], lp, decodeBufferSize-plcPitchLagMax,
		plcPitchLagMax-plcPitchLagMin)
	return plcPitchLagMax - pitchIndex
}

// prefilterAndFoldFn is prefilter_and_fold: it applies the pre-filter to
// the overlap of concealed audio, and simulates TDAC on it, so that it
// blends with the MDCT of the next frame.
func (st *Decoder) prefilterAndFoldFn(n int) {
	m := st.m
	overlap := st.overlap
	etmp := st.etmp[:overlap]
	for ch := 0; ch < st.channels; ch++ {
		dm := st.decodeMem[ch]
		combFilter(etmp, dm, decodeBufferSize-n, st.postfilterPeriodOld, st.postfilterPeriod, overlap,
			-st.postfilterGainOld, -st.postfilterGain, st.postfilterTapsetOld, st.postfilterTapset, nil, 0)
		for i := 0; i < overlap/2; i++ {
			dm[decodeBufferSize-n+i] = m.window[i]*etmp[overlap-1-i] + m.window[overlap-i-1]*etmp[i]
		}
	}
}

// decodeLost is celt_decode_lost: it conceals a lost frame of n samples,
// with noise, or by extrapolating the pitch of what played.
func (st *Decoder) decodeLost(n, lm int) {
	c := st.channels
	m := st.m
	nb := m.nbEBands
	overlap := m.overlap
	eBands := m.eBands
	var outSyn [2][]float32
	for ch := 0; ch < c; ch++ {
		outSyn[ch] = st.decodeMem[ch][decodeBufferSize-n:]
	}
	lossDuration := st.lossDuration
	start := st.start
	currFrameType := framePLCPeriodic
	if st.plcDuration >= 40 || start != 0 || st.skipPLC {
		currFrameType = framePLCNoise
	}
	if currFrameType == framePLCNoise {
		// Noise-based concealment.
		end := st.end
		effEnd := max(start, min(end, m.effEBands))
		x := st.x[:c*n]
		for ch := 0; ch < c; ch++ {
			copy(st.decodeMem[ch], st.decodeMem[ch][n:decodeBufferSize+overlap])
		}
		if st.prefilterAndFold {
			st.prefilterAndFoldFn(n)
		}
		// The energy decays.
		decay := float32(.5)
		if lossDuration == 0 {
			decay = 1.5
		}
		for ch := 0; ch < c; ch++ {
			for i := start; i < end; i++ {
				k := ch*nb + i
				st.oldEBands[k] = max(st.backgroundLogE[k], st.oldEBands[k]-decay)
			}
		}
		seed := st.rng
		for ch := 0; ch < c; ch++ {
			for i := start; i < effEnd; i++ {
				boffs := n*ch + int(eBands[i])<<lm
				blen := (int(eBands[i+1]) - int(eBands[i])) << lm
				for j := 0; j < blen; j++ {
					seed = celtLCGRand(seed)
					x[boffs+j] = float32(int32(seed) >> 20)
				}
				renormaliseVector(x[boffs:], blen, q31One)
			}
		}
		st.rng = seed
		st.synthesis(x, outSyn, st.oldEBands[:], start, effEnd, c, c, false, lm, false)
		// The post-filter with its last parameters.
		for ch := 0; ch < c; ch++ {
			st.postfilterPeriod = max(st.postfilterPeriod, combfilterMinPeriod)
			st.postfilterPeriodOld = max(st.postfilterPeriodOld, combfilterMinPeriod)
			dm := st.decodeMem[ch]
			off := decodeBufferSize - n
			combFilter(dm[off:], dm, off, st.postfilterPeriodOld, st.postfilterPeriod, m.shortMdctSize,
				st.postfilterGainOld, st.postfilterGain, st.postfilterTapsetOld, st.postfilterTapset, m.window, overlap)
			if lm != 0 {
				off2 := off + m.shortMdctSize
				combFilter(dm[off2:], dm, off2, st.postfilterPeriod, st.postfilterPeriod, n-m.shortMdctSize,
					st.postfilterGain, st.postfilterGain, st.postfilterTapset, st.postfilterTapset, m.window, overlap)
			}
		}
		st.postfilterPeriodOld = st.postfilterPeriod
		st.postfilterGainOld = st.postfilterGain
		st.postfilterTapsetOld = st.postfilterTapset
		st.prefilterAndFold = false
		// No regular concealment until two consecutive packets came.
		st.skipPLC = true
	} else {
		// Pitch-based concealment.
		fade := q15One
		var pitchIndex int
		if st.lastFrameType != framePLCPeriodic {
			pitchIndex = st.plcPitchSearch(c)
			st.lastPitchIndex = pitchIndex
		} else {
			pitchIndex = st.lastPitchIndex
			fade = .8
		}
		// The excitation of 2 pitch periods, to look for a decay, but no
		// more than maxPeriod.
		excLength := min(2*pitchIndex, maxPeriod)
		// exc[i] is excBuf[celtLPCOrder+i].
		excBuf := st.exc[:maxPeriod+celtLPCOrder]
		firTmp := st.firTmp[:excLength]
		window := m.window
		for ch := 0; ch < c; ch++ {
			buf := st.decodeMem[ch]
			var s1 float32
			for i := 0; i < maxPeriod+celtLPCOrder; i++ {
				excBuf[i] = buf[decodeBufferSize-maxPeriod-celtLPCOrder+i]
			}
			lpc := st.lpc[ch*celtLPCOrder : (ch+1)*celtLPCOrder]
			if st.lastFrameType != framePLCPeriodic {
				// The LPC of the last maxPeriod samples before the loss, to
				// work in the excitation domain.
				var ac [celtLPCOrder + 1]float32
				celtAutocorr(excBuf[celtLPCOrder:], ac[:], window, overlap, celtLPCOrder, maxPeriod)
				// A noise floor of -40 dB.
				ac[0] *= 1.0001
				// Lag windowing stabilizes the Levinson-Durbin recursion.
				for i := 1; i <= celtLPCOrder; i++ {
					ac[i] -= ac[i] * (0.008 * 0.008) * float32(i) * float32(i)
				}
				celtLPC(lpc, ac[:], celtLPCOrder)
			}
			// The excitation of excLength samples before the loss.
			celtFIR(excBuf, celtLPCOrder+maxPeriod-excLength, lpc, firTmp, excLength, celtLPCOrder)
			copy(excBuf[celtLPCOrder+maxPeriod-excLength:], firTmp)
			// Is the waveform decaying, and how fast? Concealing a decaying
			// segment must not add energy.
			e1, e2 := float32(1), float32(1)
			decayLength := excLength >> 1
			for i := 0; i < decayLength; i++ {
				e := excBuf[celtLPCOrder+maxPeriod-decayLength+i]
				e1 += e * e
				e = excBuf[celtLPCOrder+maxPeriod-2*decayLength+i]
				e2 += e * e
			}
			e1 = min(e1, e2)
			decay := celtSqrt(fracDiv32(e1, e2))
			// Make room for the new frame; the overlap past the buffer's
			// end isn't used.
			copy(buf, buf[n:decodeBufferSize])
			// Extrapolate the end of the excitation with a period of
			// pitchIndex, each period scaled down by decay more.
			extrapolationOffset := maxPeriod - pitchIndex
			// Enough to cover a whole MDCT window, with its overlap.
			extrapolationLen := n + overlap
			// Fade when this is not the first loss.
			attenuation := fade * decay
			for i, j := 0, 0; i < extrapolationLen; i, j = i+1, j+1 {
				if j >= pitchIndex {
					j -= pitchIndex
					attenuation = attenuation * decay
				}
				buf[decodeBufferSize-n+i] = attenuation * excBuf[celtLPCOrder+extrapolationOffset+j]
				// The energy of the signal whose excitation this copies.
				tmp := buf[decodeBufferSize-maxPeriod-n+extrapolationOffset+j]
				s1 += tmp * tmp
			}
			// The synthesis filter, from the last samples before the
			// overlap, back to the signal domain.
			var lpcMem [celtLPCOrder]float32
			for i := 0; i < celtLPCOrder; i++ {
				lpcMem[i] = buf[decodeBufferSize-n-1-i]
			}
			celtIIR(buf[decodeBufferSize-n:], lpc, extrapolationLen, celtLPCOrder, lpcMem[:], st.iirScratch)
			// Attenuate when the synthesis has more energy than expected,
			// as when the signal changes within the window.
			var s2 float32
			for i := 0; i < extrapolationLen; i++ {
				tmp := buf[decodeBufferSize-n+i]
				s2 += tmp * tmp
			}
			// An explosion of the synthesis; written so to catch NaNs too.
			if !(s1 > 0.2*s2) {
				for i := 0; i < extrapolationLen; i++ {
					buf[decodeBufferSize-n+i] = 0
				}
			} else if s1 < s2 {
				ratio := celtSqrt(fracDiv32(s1+1, s2+1))
				for i := 0; i < overlap; i++ {
					tmpG := q15One - window[i]*(q15One-ratio)
					buf[decodeBufferSize-n+i] = tmpG * buf[decodeBufferSize-n+i]
				}
				for i := overlap; i < extrapolationLen; i++ {
					buf[decodeBufferSize-n+i] = ratio * buf[decodeBufferSize-n+i]
				}
			}
		}
		st.prefilterAndFold = true
	}
	// Saturate, to avoid wrap-around.
	st.lossDuration = min(10000, lossDuration+(1<<lm))
	st.plcDuration = min(10000, st.plcDuration+(1<<lm))
	st.lastFrameType = currFrameType
}

// Decode is celt_decode_with_ec: it decodes frameSize samples per channel
// of data into pcm, interleaved, adding to what pcm holds when accum. A nil
// or one-byte data conceals a lost frame. dec, when not nil, is the range
// decoder of a hybrid frame, which SILK read the start of. It returns the
// number of samples per channel.
func (st *Decoder) Decode(data []byte, pcm []float32, frameSize int, dec *rangedec.Decoder, accum bool) (int, error) {
	m := st.m
	nb := m.nbEBands
	overlap := m.overlap
	eBands := m.eBands
	start, end := st.start, st.end
	frameSize *= st.downsample
	cc := st.channels
	c := st.streamChannels
	lm := 0
	for ; lm <= m.maxLM; lm++ {
		if m.shortMdctSize<<lm == frameSize {
			break
		}
	}
	if lm > m.maxLM {
		return 0, ErrBadArg
	}
	mm := 1 << lm
	if len(data) > 1275 || pcm == nil {
		return 0, ErrBadArg
	}
	n := mm * m.shortMdctSize
	var outSyn [2][]float32
	for ch := 0; ch < cc; ch++ {
		outSyn[ch] = st.decodeMem[ch][decodeBufferSize-n:]
	}
	effEnd := min(end, m.effEBands)

	if len(data) <= 1 {
		st.decodeLost(n, lm)
		st.deemphasis(outSyn, pcm, n, cc, accum)
		return frameSize / st.downsample, nil
	}

	// The pitch-based concealment waits for two consecutive packets.
	if st.lossDuration == 0 {
		st.skipPLC = false
	}
	if dec == nil {
		st.ec.Init(data)
		dec = &st.ec
	}
	if c == 1 {
		for i := 0; i < nb; i++ {
			st.oldEBands[i] = max(st.oldEBands[i], st.oldEBands[nb+i])
		}
	}

	totalBits := int32(len(data) * 8)
	tell := int32(dec.Tell())
	silence := false
	if tell >= totalBits {
		silence = true
	} else if tell == 1 {
		silence = dec.BitLogp(15)
	}
	if silence {
		// Pretend all the bits were read.
		tell = int32(len(data) * 8)
		dec.SetTell(int(tell))
	}

	postfilterGain := float32(0)
	postfilterPitch := 0
	postfilterTapset := 0
	if start == 0 && tell+16 <= totalBits {
		if dec.BitLogp(1) {
			octave := int(dec.Uint(6))
			postfilterPitch = (16 << octave) + int(dec.Bits(uint(4+octave))) - 1
			qg := int(dec.Bits(3))
			if int32(dec.Tell())+2 <= totalBits {
				postfilterTapset = dec.ICDF(tapsetICDF, 2)
			}
			postfilterGain = .09375 * float32(qg+1)
		}
		tell = int32(dec.Tell())
	}

	isTransient := false
	if lm > 0 && tell+3 <= totalBits {
		isTransient = dec.BitLogp(3)
		tell = int32(dec.Tell())
	}

	// The global flags, the first symbols of the stream.
	intraEner := false
	if tell+3 <= totalBits {
		intraEner = dec.BitLogp(3)
	}
	// After a loss, keep the energy prediction safe, to reduce the risk of
	// loud artifacts.
	if !intraEner && st.lossDuration != 0 {
		for ch := 0; ch < 2; ch++ {
			var safety float32
			missing := min(10, st.lossDuration>>lm)
			if lm == 0 {
				safety = 1.5
			} else if lm == 1 {
				safety = .5
			}
			for i := start; i < end; i++ {
				k := ch*nb + i
				if st.oldEBands[k] < max(st.oldLogE[k], st.oldLogE2[k]) {
					// The energy is going down already: continue the trend.
					e0, e1, e2 := st.oldEBands[k], st.oldLogE[k], st.oldLogE2[k]
					slope := max(e1-e0, .5*(e2-e0))
					slope = min(slope, 2)
					e0 -= max(0, float32(1+missing)*slope)
					st.oldEBands[k] = max(-20, e0)
				} else {
					// Else the least of the last frames.
					st.oldEBands[k] = min(min(st.oldEBands[k], st.oldLogE[k]), st.oldLogE2[k])
				}
				// Shorter frames fluctuate more: play it safe.
				st.oldEBands[k] -= safety
			}
		}
	}

	// The band energies.
	m.unquantCoarseEnergy(start, end, st.oldEBands[:], intraEner, dec, c, lm)

	tfRes := st.tfRes[:nb]
	tfDecode(start, end, isTransient, tfRes, lm, dec)

	tell = int32(dec.Tell())
	spreadDecision := spreadNormal
	if tell+4 <= totalBits {
		spreadDecision = dec.ICDF(spreadICDF, 5)
	}

	capv := st.cap[:nb]
	m.initCaps(capv, lm, c)
	offsets := st.offsets[:nb]
	dynallocLogp := 6
	totalBits <<= bitRes
	tellFrac := int32(dec.TellFrac())
	for i := start; i < end; i++ {
		width := c * (int(eBands[i+1]) - int(eBands[i])) << lm
		// quanta is 6 bits, but no more than 1 bit per sample and no less
		// than 1/8 bit per sample.
		quanta := min(width<<bitRes, max(6<<bitRes, width))
		dynallocLoopLogp := dynallocLogp
		boost := 0
		for tellFrac+int32(dynallocLoopLogp<<bitRes) < totalBits && boost < capv[i] {
			flag := dec.BitLogp(uint(dynallocLoopLogp))
			tellFrac = int32(dec.TellFrac())
			if !flag {
				break
			}
			boost += quanta
			totalBits -= int32(quanta)
			dynallocLoopLogp = 1
		}
		offsets[i] = boost
		// Make dynalloc more likely.
		if boost > 0 {
			dynallocLogp = max(2, dynallocLogp-1)
		}
	}

	allocTrim := 5
	if tellFrac+(6<<bitRes) <= totalBits {
		allocTrim = dec.ICDF(trimICDF, 7)
	}

	bits := (int32(len(data)*8) << bitRes) - int32(dec.TellFrac()) - 1
	antiCollapseRsv := int32(0)
	if isTransient && lm >= 2 && bits >= int32((lm+2)<<bitRes) {
		antiCollapseRsv = 1 << bitRes
	}
	bits -= antiCollapseRsv
	alloc := &st.alloc
	m.computeAllocation(alloc, start, end, offsets, capv, allocTrim, bits, c, lm, dec)

	m.unquantFineEnergy(start, end, st.oldEBands[:], alloc.fineQuant[:], dec, c)

	x := st.x[:c*n]
	for ch := 0; ch < cc; ch++ {
		copy(st.decodeMem[ch], st.decodeMem[ch][n:decodeBufferSize+overlap])
	}

	// The fixed codebook.
	collapseMasks := st.collapseMasks[:c*nb]
	var y []float32
	if c == 2 {
		y = x[n : 2*n]
	}
	m.quantAllBands(start, end, x[:n], y, collapseMasks, alloc.pulses[:], isTransient, spreadDecision,
		alloc.dualStereo, alloc.intensity, tfRes, int32(len(data)*(8<<bitRes))-antiCollapseRsv, alloc.balance,
		dec, lm, alloc.codedBands, &st.rng, st.disableInv, st.bands)

	antiCollapseOn := false
	if antiCollapseRsv > 0 {
		antiCollapseOn = dec.Bits(1) != 0
	}

	m.unquantEnergyFinalise(start, end, st.oldEBands[:], alloc.fineQuant[:], alloc.finePriority[:],
		len(data)*8-dec.Tell(), dec, c)

	if antiCollapseOn {
		m.antiCollapse(x, collapseMasks, lm, c, n, start, end, st.oldEBands[:], st.oldLogE[:], st.oldLogE2[:],
			alloc.pulses[:], st.rng)
	}

	if silence {
		for i := 0; i < c*nb; i++ {
			st.oldEBands[i] = -28
		}
	}
	if st.prefilterAndFold {
		st.prefilterAndFoldFn(n)
	}
	st.synthesis(x, outSyn, st.oldEBands[:], start, effEnd, c, cc, isTransient, lm, silence)

	for ch := 0; ch < cc; ch++ {
		st.postfilterPeriod = max(st.postfilterPeriod, combfilterMinPeriod)
		st.postfilterPeriodOld = max(st.postfilterPeriodOld, combfilterMinPeriod)
		dm := st.decodeMem[ch]
		off := decodeBufferSize - n
		combFilter(dm[off:], dm, off, st.postfilterPeriodOld, st.postfilterPeriod, m.shortMdctSize,
			st.postfilterGainOld, st.postfilterGain, st.postfilterTapsetOld, st.postfilterTapset, m.window, overlap)
		if lm != 0 {
			off2 := off + m.shortMdctSize
			combFilter(dm[off2:], dm, off2, st.postfilterPeriod, postfilterPitch, n-m.shortMdctSize,
				st.postfilterGain, postfilterGain, st.postfilterTapset, postfilterTapset, m.window, overlap)
		}
	}
	st.postfilterPeriodOld = st.postfilterPeriod
	st.postfilterGainOld = st.postfilterGain
	st.postfilterTapsetOld = st.postfilterTapset
	st.postfilterPeriod = postfilterPitch
	st.postfilterGain = postfilterGain
	st.postfilterTapset = postfilterTapset
	if lm != 0 {
		st.postfilterPeriodOld = st.postfilterPeriod
		st.postfilterGainOld = st.postfilterGain
		st.postfilterTapsetOld = st.postfilterTapset
	}

	if c == 1 {
		copy(st.oldEBands[nb:2*nb], st.oldEBands[:nb])
	}
	if !isTransient {
		st.oldLogE2 = st.oldLogE
		st.oldLogE = st.oldEBands
	} else {
		for i := 0; i < 2*nb; i++ {
			st.oldLogE[i] = min(st.oldLogE[i], st.oldEBands[i])
		}
	}
	// The noise floor may increase by 2.4 dB/second at most, or by all
	// missing packets in DTX.
	maxBackgroundIncrease := float32(min(160, st.lossDuration+mm)) * .001
	for i := 0; i < 2*nb; i++ {
		st.backgroundLogE[i] = min(st.backgroundLogE[i]+maxBackgroundIncrease, st.oldEBands[i])
	}
	// In case start or end were to change.
	for ch := 0; ch < 2; ch++ {
		for i := 0; i < start; i++ {
			k := ch*nb + i
			st.oldEBands[k] = 0
			st.oldLogE[k] = -28
			st.oldLogE2[k] = -28
		}
		for i := end; i < nb; i++ {
			k := ch*nb + i
			st.oldEBands[k] = 0
			st.oldLogE[k] = -28
			st.oldLogE2[k] = -28
		}
	}
	st.rng = dec.Range()

	st.deemphasis(outSyn, pcm, n, cc, accum)
	st.lossDuration = 0
	st.plcDuration = 0
	st.lastFrameType = frameNormal
	st.prefilterAndFold = false

	if dec.Tell() > 8*len(data) {
		return 0, ErrInternal
	}
	if dec.Err() {
		st.err = true
	}
	return frameSize / st.downsample, nil
}
