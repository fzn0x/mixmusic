package settings

// The correction of the user's headphones (#616): an AutoEq profile of
// their model, or a ParametricEQ.txt of their own, which plays after the
// equalizer's bands. Its filters are kept here, so that it plays offline.

// The limits of a correction of headphones.
const (
	// MaxHeadphoneFilters is the most filters a correction has.
	MaxHeadphoneFilters = 20
	// MinFilterHz and MaxFilterHz bound a filter's frequency, in Hz: up to
	// below half of 44.1 kHz.
	MinFilterHz = 1
	MaxFilterHz = 22000
	// MaxFilterGain bounds a filter's gain, and the preamp, either way, in
	// dB.
	MaxFilterGain = 30
	// MinFilterQ and MaxFilterQ bound a filter's Q.
	MinFilterQ = 0.01
	MaxFilterQ = 100
)

// The types of a correction's filters, as Equalizer APO's ParametricEQ.txt
// names them.
const (
	// FilterPeak is a peaking filter.
	FilterPeak = "PK"
	// FilterLowShelf and FilterHighShelf are shelves of a Q of their own,
	// half way at their frequency.
	FilterLowShelf  = "LSC"
	FilterHighShelf = "HSC"
	// FilterLowPass and FilterHighPass take no gain.
	FilterLowPass  = "LP"
	FilterHighPass = "HP"
)

// HeadphonesFile is the ID of a correction imported from a file.
const HeadphonesFile = "file"

// Headphones is the correction of the user's headphones, or none.
type Headphones struct {
	// ID is AutoEq's folder of the profile, as
	// "oratory1990/over-ear/1MORE SonoFlow", HeadphonesFile for a file's,
	// or "" for none.
	ID string `json:"id"`
	// Name is the model's, or the file's, as the window shows it.
	Name string `json:"name"`
	// Source measured the model, on Rig, which may be "": AutoEq's
	// profiles say so.
	Source string `json:"source"`
	Rig    string `json:"rig"`
	// Preamp is in dB, from -MaxFilterGain to MaxFilterGain.
	Preamp float64 `json:"preamp"`
	// Filters play in order, up to MaxHeadphoneFilters.
	Filters []HeadphoneFilter `json:"filters"`
}

// HeadphoneFilter is a filter of a correction: of Type, at Fc, in Hz, with
// Gain, in dB, but for a low-pass or a high-pass, and Q.
type HeadphoneFilter struct {
	Type string  `json:"type"`
	Fc   float64 `json:"fc"`
	Gain float64 `json:"gain"`
	Q    float64 `json:"q"`
}

// noHeadphones is no correction.
func noHeadphones() Headphones { return Headphones{Filters: []HeadphoneFilter{}} }

// On reports whether h corrects the sound.
func (h Headphones) On() bool { return h.ID != "" }

// filterTypes are the types of filters a correction plays.
var filterTypes = []string{FilterPeak, FilterLowShelf, FilterHighShelf, FilterLowPass, FilterHighPass}

// takesGain reports whether a filter of type t has a gain.
func takesGain(t string) bool { return t != FilterLowPass && t != FilterHighPass }

// clean is h with its filters of a type it knows, each value in its
// range, up to MaxHeadphoneFilters of them: none, when no filter is left
// or h names none.
func (h Headphones) clean() Headphones {
	if h.ID == "" {
		return noHeadphones()
	}
	h.Preamp = min(max(h.Preamp, -MaxFilterGain), MaxFilterGain)
	filters := make([]HeadphoneFilter, 0, min(len(h.Filters), MaxHeadphoneFilters))
	for _, f := range h.Filters {
		if len(filters) == MaxHeadphoneFilters {
			break
		}
		known := false
		for _, t := range filterTypes {
			known = known || f.Type == t
		}
		if !known {
			continue
		}
		f.Fc = min(max(f.Fc, MinFilterHz), MaxFilterHz)
		f.Gain = min(max(f.Gain, -MaxFilterGain), MaxFilterGain)
		if !takesGain(f.Type) {
			f.Gain = 0
		}
		f.Q = min(max(f.Q, MinFilterQ), MaxFilterQ)
		filters = append(filters, f)
	}
	if len(filters) == 0 {
		return noHeadphones()
	}
	h.Filters = filters
	return h
}
