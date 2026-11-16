package align

import (
	"math"
	"time"
)

// Apart is how far from the best match the next best is looked for: a
// match nearer would keep time all the same.
const Apart = 100 * time.Millisecond

// match is where a stretch of a recording best matches another recording.
type match struct {
	// at is the time of the other recording the stretch's first frame
	// matches best, between frames as the matches beside it tell.
	at time.Duration
	// score is the normalized correlation of the envelopes there, each
	// band's mean taken away, from -1 to 1, and next that of the next best
	// match, more than Apart away; 0 when there is none.
	score, next float64
	// edge is set when the best match is at the edge of the times
	// searched: a better one may lie beyond it.
	edge bool
}

// searched is an envelope the stretches of another recording are looked
// for in: its levels less each band's mean over it all, which changes no
// correlation, but keeps the sums of float32s small.
type searched struct {
	at time.Duration
	v  []float32
}

func searchIn(e Envelope) searched {
	frames := e.Frames()
	var mean [Bands]float64
	for i, v := range e.v {
		mean[i%Bands] += float64(v)
	}
	v := make([]float32, len(e.v))
	for i, x := range e.v {
		v[i] = float32(float64(x) - mean[i%Bands]/float64(frames))
	}
	return searched{at: e.At, v: v}
}

// find finds where part best matches w, of the times of w's recording from
// from to to that part's first frame may match, so that the whole of part
// matches frames of w. It reports false when there are fewer than three
// such times, or part doesn't change.
func (w searched) find(part Envelope, from, to time.Duration) (match, bool) {
	n, frames := part.Frames(), len(w.v)/Bands
	lo := 0
	if from > w.at {
		lo = int((from - w.at + Hop - 1) / Hop)
	}
	hi := frames - n
	if to < w.at {
		hi = -1
	} else {
		hi = min(hi, int((to-w.at)/Hop))
	}
	if n == 0 || hi-lo < 2 {
		return match{}, false
	}
	// p is part less each band's mean, and norm its norm.
	p := make([]float32, len(part.v))
	var mean [Bands]float64
	for i, v := range part.v {
		mean[i%Bands] += float64(v)
	}
	var norm float64
	for i, v := range part.v {
		p[i] = float32(float64(v) - mean[i%Bands]/float64(n))
		norm += float64(p[i]) * float64(p[i])
	}
	if norm < 1e-9 {
		return match{}, false
	}
	norm = math.Sqrt(norm)
	// sums are of w's levels under part by band, and squares of their
	// squares, at the time k searched, moved along a frame at a time: of
	// them, the norm of w's levels under part, less each band's mean.
	var sums [Bands]float64
	var squares float64
	add := func(f int, sign float64) {
		for b, v := range w.v[f*Bands : (f+1)*Bands] {
			sums[b] += sign * float64(v)
			squares += sign * float64(v) * float64(v)
		}
	}
	for f := lo; f < lo+n; f++ {
		add(f, 1)
	}
	scores := make([]float64, hi-lo+1)
	for k := lo; k <= hi; k++ {
		if k > lo {
			add(k-1, -1)
			add(k+n-1, 1)
		}
		spread := squares
		for _, s := range sums {
			spread -= s * s / float64(n)
		}
		if spread > 1e-9 {
			scores[k-lo] = float64(dot(p, w.v[k*Bands:])) / (norm * math.Sqrt(spread))
		}
	}
	best := 0
	for i, s := range scores {
		if s > scores[best] {
			best = i
		}
	}
	m := match{score: scores[best], edge: best <= 1 || best >= len(scores)-2}
	apart := int(Apart / Hop)
	for i, s := range scores {
		peak := (i == 0 || s >= scores[i-1]) && (i == len(scores)-1 || s >= scores[i+1])
		if peak && (i < best-apart || i > best+apart) && s > m.next {
			m.next = s
		}
	}
	// Between frames, at the top of the parabola through the best and the
	// matches beside it.
	at := float64(best)
	if best > 0 && best < len(scores)-1 {
		a, b, c := scores[best-1], scores[best], scores[best+1]
		if d := a - 2*b + c; d < 0 {
			at += max(-0.5, min(0.5, (a-c)/(2*d)))
		}
	}
	m.at = w.at + time.Duration((float64(lo)+at)*float64(Hop))
	return m, true
}

// dot returns the dot product of a and the start of b.
func dot(a, b []float32) float32 {
	b = b[:len(a)]
	var s0, s1, s2, s3 float32
	i := 0
	for ; i+4 <= len(a); i += 4 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
	}
	for ; i < len(a); i++ {
		s0 += a[i] * b[i]
	}
	return s0 + s1 + s2 + s3
}
