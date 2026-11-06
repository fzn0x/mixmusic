package media

import "time"

// Release is an album, a single or an EP of an artist, with when it came
// out, as far as its service tells.
type Release struct {
	Album Album `json:"album"`
	// Released is the day it came out, as "2026-10-31", or, where the
	// service tells no more, its month, "2026-10", or its year, "2026".
	Released string `json:"released,omitempty"`
	// Of is the artist whose release it is, as a list of the releases of
	// several artists holds it.
	Of Ref `json:"of,omitzero"`
}

// Span returns the first and the last day the release may have come out
// on, as Released tells: its day twice, or the first and the last day of
// its month or its year. ok is false when Released tells none.
func (r Release) Span() (first, last time.Time, ok bool) {
	var err error
	switch len(r.Released) {
	case len("2006-01-02"):
		first, err = time.Parse("2006-01-02", r.Released)
		last = first
	case len("2006-01"):
		first, err = time.Parse("2006-01", r.Released)
		last = first.AddDate(0, 1, -1)
	case len("2006"):
		first, err = time.Parse("2006", r.Released)
		last = first.AddDate(1, 0, -1)
	default:
		return time.Time{}, time.Time{}, false
	}
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return first, last, true
}
