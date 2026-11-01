// Package celt is the CELT decoder of Opus (RFC 6716, section 4.3), the
// codec of music and of the high band of hybrid frames: a port of libopus
// 1.6.1's float build, under the BSD license of LICENSE.libopus in
// internal/opus.
package celt

// Port of celt/modes.h, celt/modes.c and celt/static_modes_float.h: the one
// mode Opus uses, 48 kHz with 20 ms frames of 960 samples.

const maxFactors = 8

// cpx is a complex number of the FFT.
type cpx struct{ r, i float32 }

// fftState is kiss_fft_state: the FFT of one size.
type fftState struct {
	nfft     int
	scale    float32
	shift    int
	factors  [2 * maxFactors]int16
	bitrev   []int16
	twiddles []cpx
}

// mdctLookup is mdct_lookup.
type mdctLookup struct {
	n        int
	maxshift int
	kfft     [4]*fftState
	trig     []float32
}

// pulseCache is PulseCache.
type pulseCache struct {
	size  int
	index []int16
	bits  []uint8
	caps  []uint8
}

// mode is OpusCustomMode.
type mode struct {
	fs             int
	overlap        int
	nbEBands       int
	effEBands      int
	preemph        [4]float32
	eBands         []int16
	maxLM          int
	nbShortMdcts   int
	shortMdctSize  int
	nbAllocVectors int
	allocVectors   []uint8
	logN           []int16
	window         []float32
	mdct           mdctLookup
	cache          pulseCache
}

var (
	fftState48000_960_0 = &fftState{
		nfft: 480, scale: 0.0020833334, shift: -1,
		factors: [16]int16{5, 96, 3, 32, 4, 8, 2, 4, 4, 1},
		bitrev:  fftBitrev480[:], twiddles: fftTwiddles48000_960[:],
	}
	fftState48000_960_1 = &fftState{
		nfft: 240, scale: 0.0041666669, shift: 1,
		factors: [16]int16{5, 48, 3, 16, 4, 4, 4, 1},
		bitrev:  fftBitrev240[:], twiddles: fftTwiddles48000_960[:],
	}
	fftState48000_960_2 = &fftState{
		nfft: 120, scale: 0.0083333338, shift: 2,
		factors: [16]int16{5, 24, 3, 8, 2, 4, 4, 1},
		bitrev:  fftBitrev120[:], twiddles: fftTwiddles48000_960[:],
	}
	fftState48000_960_3 = &fftState{
		nfft: 60, scale: 0.016666668, shift: 3,
		factors: [16]int16{5, 12, 3, 4, 4, 1},
		bitrev:  fftBitrev60[:], twiddles: fftTwiddles48000_960[:],
	}

	// mode48000_960_120 is the mode of every Opus stream.
	mode48000_960_120 = &mode{
		fs:             48000,
		overlap:        120,
		nbEBands:       21,
		effEBands:      21,
		preemph:        [4]float32{0.85000610, 0.0, 1.0, 1.0},
		eBands:         eband5ms[:],
		maxLM:          3,
		nbShortMdcts:   8,
		shortMdctSize:  120,
		nbAllocVectors: 11,
		allocVectors:   bandAllocation[:],
		logN:           logN400[:],
		window:         window120[:],
		mdct: mdctLookup{
			n: 1920, maxshift: 3,
			kfft: [4]*fftState{fftState48000_960_0, fftState48000_960_1, fftState48000_960_2, fftState48000_960_3},
			trig: mdctTwiddles960[:],
		},
		cache: pulseCache{size: 392, index: cacheIndex50[:], bits: cacheBits50[:], caps: cacheCaps50[:]},
	}
)
