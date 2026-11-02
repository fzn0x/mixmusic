package ac3

import (
	"errors"
	"fmt"
)

// syncWord starts every syncframe.
const syncWord = 0x0B77

// header is what the syncinfo and the bsi of a syncframe say.
type header struct {
	fscod      int
	frmsizecod int
	size       int // of the syncframe, in bytes
	bsid       int
	bsmod      int
	acmod      int
	nfchans    int // the full-bandwidth channels
	lfeon      bool
	cmixlev    int // -1 without three front channels
	surmixlev  int // -1 without surround channels
	dialnorm   int
	end        int // the bit after the bsi
	rate       int // the sampling frequency, in Hz
	// E-AC-3's, of Annex E: whether the frame is of its syntax (bsid 11
	// to 16), its stream type and substream, the blocks of the frame, and
	// the LFE's mix level code, or -1.
	eac3                 bool
	strmtyp, substreamid int
	numblks              int
	lfemixlevcod         int
	// The Lo/Ro mix levels of the alternate syntax of Annex D (bsid 6),
	// -1 when the frame doesn't give them.
	loroCmixlev, loroSurmixlev int
}

// frameWords returns the size of a syncframe of frmsizecod at fscod, in
// 16-bit words, as Table 5.18 gives it.
func frameWords(fscod, frmsizecod int) int {
	rate := bitrates[frmsizecod>>1]
	switch fscod {
	case 0:
		return 2 * rate
	case 1:
		return rate*320/147 + frmsizecod&1
	default:
		return 3 * rate
	}
}

// parse reads the syncinfo and the bsi of the syncframe r reads, from its
// start, and leaves r after the bsi: of AC-3, or of E-AC-3, as the bsid,
// which both have 5 bytes into the frame, tells. It needs only the first
// bytes of the frame, and doesn't check that r holds it whole.
func (h *header) parse(r *bitReader) error {
	start := r.pos
	if r.read(16) != syncWord {
		return fmt.Errorf("%w: no syncword", ErrFormat)
	}
	r.skip(24)
	h.bsid = r.read(5)
	r.pos = start + 16
	switch {
	case h.bsid > 10 && h.bsid <= 16:
		return h.parseEAC3(r)
	case h.bsid == 9 || h.bsid == 10:
		return fmt.Errorf("ac3: bsid %d: %w", h.bsid, errors.ErrUnsupported)
	case h.bsid > 16:
		return fmt.Errorf("%w: bsid %d", ErrFormat, h.bsid)
	}
	*h = header{bsid: h.bsid, numblks: 6, lfemixlevcod: -1}
	r.skip(16) // crc1
	h.fscod = r.read(2)
	h.frmsizecod = r.read(6)
	r.skip(5) // bsid
	if h.fscod == 3 {
		return fmt.Errorf("%w: fscod 3", ErrFormat)
	}
	if h.frmsizecod >= 2*len(bitrates) {
		return fmt.Errorf("%w: frmsizecod %d", ErrFormat, h.frmsizecod)
	}
	h.rate = sampleRates[h.fscod]
	h.size = 2 * frameWords(h.fscod, h.frmsizecod)
	h.bsmod = r.read(3)
	h.acmod = r.read(3)
	h.nfchans = modeChannels[h.acmod]
	h.cmixlev, h.surmixlev = -1, -1
	if h.acmod&1 != 0 && h.acmod != ModeMono {
		h.cmixlev = r.read(2)
	}
	if h.acmod&4 != 0 {
		h.surmixlev = r.read(2)
	}
	if h.acmod == ModeStereo {
		r.skip(2) // dsurmod
	}
	h.lfeon = r.flag()
	h.dialnorm = r.read(5)
	skipProgramInfo(r)
	if h.acmod == ModeDualMono {
		r.skip(5) // dialnorm2, of Ch2
		skipProgramInfo(r)
	}
	r.skip(2) // copyrightb, origbs
	h.loroCmixlev, h.loroSurmixlev = -1, -1
	if h.bsid == 6 {
		if r.flag() { // xbsi1e
			r.skip(2 + 3 + 3) // dmixmod, ltrtcmixlev, ltrtsurmixlev
			h.loroCmixlev = r.read(3)
			h.loroSurmixlev = r.read(3)
		}
		if r.flag() { // xbsi2e
			r.skip(14)
		}
	} else {
		if r.flag() { // timecod1e
			r.skip(14)
		}
		if r.flag() { // timecod2e
			r.skip(14)
		}
	}
	if r.flag() { // addbsie
		r.skip((r.read(6) + 1) * 8)
	}
	if r.over() {
		return fmt.Errorf("%w: syncframe too short", ErrFormat)
	}
	h.end = r.pos
	return nil
}

// skipProgramInfo skips what the bsi tells of a program after its
// dialnorm: compr, langcod, mixlevel and roomtyp, each when it is there.
func skipProgramInfo(r *bitReader) {
	if r.flag() { // compre
		r.skip(8) // compr
	}
	if r.flag() { // langcode
		r.skip(8) // langcod
	}
	if r.flag() { // audprodie
		r.skip(5 + 2) // mixlevel, roomtyp
	}
}

// crcTable is the CRC of each byte, of the generator x^16 + x^15 + x^2 + 1.
var crcTable = func() (t [256]uint16) {
	for i := range t {
		c := uint16(i) << 8
		for range 8 {
			if c&0x8000 != 0 {
				c = c<<1 ^ 0x8005
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return t
}()

// crc16 continues the CRC c over b.
func crc16(c uint16, b []byte) uint16 {
	for _, v := range b {
		c = c<<8 ^ crcTable[byte(c>>8)^v]
	}
	return c
}

// checkCRC checks the CRCs of a syncframe, whole in frame: of AC-3, both,
// of its first 5/8 and of all of it after the syncword; of E-AC-3, its
// one, of all of it after the syncword.
func checkCRC(frame []byte, eac3 bool) error {
	five8 := 2
	if !eac3 {
		words := len(frame) / 2
		five8 = 2 * (words>>1 + words>>3)
	}
	c := crc16(0, frame[2:five8])
	if c != 0 {
		return fmt.Errorf("%w: crc1 mismatch", ErrFormat)
	}
	if crc16(c, frame[five8:]) != 0 {
		return fmt.Errorf("%w: crc2 mismatch", ErrFormat)
	}
	return nil
}
