package audio

import "math"

// Bands are the center frequencies of the equalizer's filters, in Hz: a low
// shelf, eight peaks and a high shelf, as the web interface had them.
var Bands = [10]float64{32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// bandQ is the quality of the peaking filters: 1.1, as the float an
// AudioParam keeps it in.
var bandQ = float64(float32(1.1))

const (
	// glide is the time constant, in seconds, of the equalizer's and the
	// volume's changes, which the web player gives setTargetAtTime.
	glide = 0.02
)

func bandKind(i int) filterKind {
	switch i {
	case 0:
		return lowShelf
	case len(Bands) - 1:
		return highShelf
	}
	return peaking
}

// equalizer is the preamp and the filters of the web player.
type equalizer struct {
	preamp  smoother // a linear gain
	gains   [len(Bands)]smoother
	filters [len(Bands)]biquad
	// at is the gain of the coefficients of each filter.
	at [len(Bands)]float64
}

func newEqualizer() *equalizer {
	e := &equalizer{preamp: newSmoother(1, glide)}
	for i := range e.filters {
		e.gains[i] = newSmoother(0, glide)
		e.filters[i].set(bandKind(i), Bands[i], bandQ, 0)
	}
	return e
}

// set changes the equalizer, as the web player's applyEq: the gains of the
// bands and the preamp in dB, or flat when it is off.
func (e *equalizer) set(enabled bool, gains [len(Bands)]float64, preamp float64) {
	e.preamp.target = 1
	if enabled {
		e.preamp.target = math.Pow(10, preamp/20)
	}
	for i := range e.gains {
		e.gains[i].target = 0
		if enabled {
			e.gains[i].target = gains[i]
		}
	}
}

// process runs a quantum of frames of ch channels interleaved, 2 or 6,
// through the preamp and the filters. A filter moving to a new gain takes
// the gain of the start of each quantum, as a k-rate parameter would.
func (e *equalizer) process(buf []float32, ch int) {
	if !e.preamp.settled() || e.preamp.value != 1 {
		for i := 0; i < len(buf); i += ch {
			g := float32(e.preamp.next())
			for c := range ch {
				buf[i+c] *= g
			}
		}
	}
	for i := range e.filters {
		f, g := &e.filters[i], &e.gains[i]
		moving := !g.settled()
		if e.at[i] != g.value {
			f.set(bandKind(i), Bands[i], bandQ, g.value)
			e.at[i] = g.value
		}
		if moving {
			g.advance(len(buf) / ch)
		} else if g.value == 0 {
			// Flat: the filter is the identity.
			f.pass(buf, ch)
			continue
		}
		f.process(buf, ch)
	}
}

// forget clears the memory of the filters' channels from channel from on,
// which play anew.
func (e *equalizer) forget(from int) {
	for i := range e.filters {
		e.filters[i].forget(from)
	}
}
