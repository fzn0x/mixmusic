package aac

// Port of Syntax/ScaleFactorBands.cs, Syntax/IQTable.cs and
// Syntax/ScaleFactorTable.cs, whose tables of powers this package
// computes rather than holds.

import (
	"math"
	"sync"
)

// rateTables are the tables of a sampling frequency index.
type rateTables struct {
	// long and short are the offsets of the scalefactor bands of a long
	// window and of a short one.
	long, short []uint16
	// tnsLong and tnsShort are TNS_MAX_BANDS of AAC-LC, after its table
	// in the standard: the bands temporal noise shaping may filter.
	tnsLong, tnsShort int
}

// rates are the tables of each sampling frequency index but 12, 7350 Hz,
// which has those of 8000 Hz.
var rates = [12]rateTables{
	{swb1024x96[:], swb128x96[:], 31, 9},  // 96000
	{swb1024x96[:], swb128x96[:], 31, 9},  // 88200
	{swb1024x64[:], swb128x64[:], 34, 10}, // 64000
	{swb1024x48[:], swb128x48[:], 40, 14}, // 48000
	{swb1024x48[:], swb128x48[:], 42, 14}, // 44100
	{swb1024x32[:], swb128x48[:], 51, 14}, // 32000
	{swb1024x24[:], swb128x24[:], 46, 14}, // 24000
	{swb1024x24[:], swb128x24[:], 46, 14}, // 22050
	{swb1024x16[:], swb128x16[:], 42, 14}, // 16000
	{swb1024x16[:], swb128x16[:], 42, 14}, // 12000
	{swb1024x16[:], swb128x16[:], 42, 14}, // 11025
	{swb1024x8[:], swb128x8[:], 39, 14},   // 8000
}

// Windows by window shape: sine, then KBD. Each holds the rising half of
// a window, whose falling half mirrors it.
var (
	longWindows  = [2]*[1024]float32{&sine1024, &kbd1024}
	shortWindows = [2]*[128]float32{&sine128, &kbd128}
)

// outScale scales the spectrum, whose samples come in the range of 16
// bits, to [-1, 1].
const outScale = 1.0 / 32768

var (
	// iqTable is the inverse quantization: |q|^(4/3) of each quantized
	// value |q| up to 8191, the most an escape codes.
	iqTable [8192]float32
	// pow2q is the gain of each scalefactor and noise energy x, and the
	// inverse of each intensity position: 2^((x-100)/4).
	pow2q [256]float32
)

// initTables builds the tables a Decoder needs, once.
var initTables = sync.OnceFunc(func() {
	initBooks()
	for i := range iqTable {
		iqTable[i] = float32(float64(i) * math.Cbrt(float64(i)))
	}
	for i := range pow2q {
		pow2q[i] = float32(math.Exp2(float64(i-100) / 4))
	}
	mdctLong.init(2048)
	mdctShort.init(256)
})

// dequantize is the inverse quantization of q: sign(q) |q|^(4/3).
func dequantize(q int32) float32 {
	a := q
	if a < 0 {
		a = -a
	}
	var v float32
	if a < int32(len(iqTable)) {
		v = iqTable[a]
	} else {
		// Pulses may add up to 60 to an escaped value.
		v = float32(float64(a) * math.Cbrt(float64(a)))
	}
	if q < 0 {
		return -v
	}
	return v
}
