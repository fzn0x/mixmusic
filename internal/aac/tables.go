package aac

// Port of Syntax/ScaleFactorBands.cs.

import "sync"

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

// initTables builds the tables a Decoder needs, once.
var initTables = sync.OnceFunc(func() {
	initBooks()
})
