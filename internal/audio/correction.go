package audio

import (
	"math"
	"slices"
)

// The correction of headphones (#616): the preamp and the filters of a
// ParametricEQ.txt, as AutoEq makes one for a model of headphones, or as a
// file of the user's has them. It plays after the equalizer and the stages
// after it, on every channel, and is skipped while there is none. A new
// correction fades in over the one before, both playing for a moment, so
// as not to click.

// MaxCorrectionFilters is the most filters a correction plays.
const MaxCorrectionFilters = 20

// The types of a correction's filters, as Equalizer APO names them.
const (
	// CorrectionPeak is a peaking filter: Gain at Hz, Q how narrow.
	CorrectionPeak = "PK"
	// CorrectionLowShelf and CorrectionHighShelf are shelves of Gain below
	// and above Hz, where they are half way, of a Q of their own.
	CorrectionLowShelf  = "LSC"
	CorrectionHighShelf = "HSC"
	// CorrectionLowPass and CorrectionHighPass cut above and below Hz, 12
	// dB an octave, Q telling how much they rise about it; they take no
	// gain.
	CorrectionLowPass  = "LP"
	CorrectionHighPass = "HP"
)

// correctionKinds are the filters of each type.
var correctionKinds = map[string]filterKind{
	CorrectionPeak:      peaking,
	CorrectionLowShelf:  lowShelfQ,
	CorrectionHighShelf: highShelfQ,
	CorrectionLowPass:   lowPass,
	CorrectionHighPass:  highPass,
}

// Correction is a correction of headphones: a preamp in dB, and filters,
// which play in order. One of no filters and no preamp is none.
type Correction struct {
	Preamp  float64
	Filters []CorrectionFilter
}

// CorrectionFilter is a filter of a correction: of Type, at Hz, with Gain
// in dB, but for a low-pass or a high-pass, and a Q.
type CorrectionFilter struct {
	Type        string
	Hz, Gain, Q float64
}

// none reports whether c changes nothing.
func (c Correction) none() bool { return len(c.Filters) == 0 && c.Preamp == 0 }

// equal reports whether c and d are the same correction.
func (c Correction) equal(d Correction) bool {
	return c.Preamp == d.Preamp && slices.Equal(c.Filters, d.Filters)
}

// chain is a correction's preamp and filters, and their memory.
type chain struct {
	gain    float32
	filters []biquad
}

// newChain returns the chain of c, of its first MaxCorrectionFilters
// filters of a type it knows, or nil for none.
func newChain(c Correction) *chain {
	if c.none() {
		return nil
	}
	ch := &chain{gain: float32(math.Pow(10, c.Preamp/20))}
	for _, f := range c.Filters[:min(len(c.Filters), MaxCorrectionFilters)] {
		kind, ok := correctionKinds[f.Type]
		if !ok {
			continue
		}
		var b biquad
		b.set(kind, f.Hz, f.Q, f.Gain)
		ch.filters = append(ch.filters, b)
	}
	return ch
}

// process runs frames of ch channels interleaved through the chain: none,
// for nil.
func (c *chain) process(buf []float32, ch int) {
	if c == nil {
		return
	}
	if c.gain != 1 {
		for i := range buf {
			buf[i] *= c.gain
		}
	}
	for i := range c.filters {
		c.filters[i].process(buf, ch)
	}
}

// forget clears the memory of the chain's channels from channel from on.
func (c *chain) forget(from int) {
	if c == nil {
		return
	}
	for i := range c.filters {
		c.filters[i].forget(from)
	}
}

// correction is the stage of the correction, on the audio thread.
type correction struct {
	// set is the correction SetCorrection gave, and want its chain, which
	// plays once the fade under way, if any, ends.
	set  Correction
	want *chain
	// cur is the chain that plays, nil for none, and next, while fading,
	// the one that fades in over it, by mix's share.
	cur, next *chain
	fading    bool
	mix       smoother
	// wet is the quantum next plays, while fading.
	wet []float32
}

func newCorrection() *correction {
	return &correction{mix: newSmoother(0, glide), wet: make([]float32, maxChannels*quantum)}
}

// setTo has the stage play c, fading to it from what it plays.
func (f *correction) setTo(c Correction) {
	if c.equal(f.set) {
		return
	}
	f.set = Correction{Preamp: c.Preamp, Filters: slices.Clone(c.Filters)}
	f.want = newChain(c)
}

// process runs a quantum of frames of ch channels interleaved through the
// correction, or, while it changes, through the one before and the new
// one, fading from the first to the second.
func (f *correction) process(buf []float32, ch int) {
	if !f.fading && f.want != f.cur {
		f.next, f.fading = f.want, true
		f.mix.value, f.mix.target = 0, 1
	}
	if !f.fading {
		f.cur.process(buf, ch)
		return
	}
	wet := f.wet[:len(buf)]
	copy(wet, buf)
	f.cur.process(buf, ch)
	f.next.process(wet, ch)
	for i := 0; i < len(buf); i += ch {
		m := float32(f.mix.next())
		for c := i; c < i+ch; c++ {
			buf[c] += m * (wet[c] - buf[c])
		}
	}
	if f.mix.settled() {
		f.cur, f.next, f.fading = f.next, nil, false
	}
}

// forget clears the memory of the filters' channels from channel from on,
// which play anew.
func (f *correction) forget(from int) {
	f.cur.forget(from)
	f.next.forget(from)
	f.want.forget(from)
}

// SetCorrection has the engine play the correction of headphones c after
// the equalizer and the stages after it, fading to it from the one it
// plays: a correction of no filters and no preamp is none, and the stage
// is then skipped.
func (e *Engine) SetCorrection(c Correction) {
	e.mu.Lock()
	e.corr.setTo(c)
	e.mu.Unlock()
}

// Correction returns the correction of headphones SetCorrection set.
func (e *Engine) Correction() Correction {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.corr.set
	c.Filters = slices.Clone(c.Filters)
	return c
}
