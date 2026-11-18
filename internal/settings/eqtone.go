package settings

import "math"

// The simple mode of the equalizer (#615): three sliders, Bass, Mid and
// Treble, which set the ten bands.

// toneWeights are how much of Bass, Mid and Treble each band takes: each
// band's add up to 1, so that the three at one gain make the bands flat at
// it.
var toneWeights = [3][10]float64{
	{1, 1, 0.8, 0.4, 0.1, 0, 0, 0, 0, 0},
	{0, 0, 0.2, 0.6, 0.9, 1, 0.8, 0.4, 0.1, 0},
	{0, 0, 0, 0, 0, 0, 0.2, 0.6, 0.9, 1},
}

// ToneGains are the bands' gains that Bass, Mid and Treble, in dB, set, to
// a tenth of a decibel.
func ToneGains(bass, mid, treble float64) []float64 {
	gains := make([]float64, len(Bands))
	for i := range gains {
		g := bass*toneWeights[0][i] + mid*toneWeights[1][i] + treble*toneWeights[2][i]
		gains[i] = math.Round(g*10) / 10
	}
	return gains
}

// ToneOf returns the Bass, Mid and Treble whose bands come nearest gains,
// by least squares, to the half decibel, each in its range: the three the
// simple mode shows of bands set otherwise.
func ToneOf(gains []float64) (bass, mid, treble float64) {
	if len(gains) != len(Bands) {
		return 0, 0, 0
	}
	// The normal equations, AᵀA x = Aᵀg, solved by Cramer's rule.
	var m [3][3]float64
	var v [3]float64
	for i, g := range gains {
		for r := range 3 {
			v[r] += toneWeights[r][i] * g
			for c := range 3 {
				m[r][c] += toneWeights[r][i] * toneWeights[c][i]
			}
		}
	}
	det := func(m [3][3]float64) float64 {
		return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
			m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
			m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	}
	d := det(m)
	var x [3]float64
	for c := range 3 {
		mc := m
		for r := range 3 {
			mc[r][c] = v[r]
		}
		x[c] = min(max(math.Round(det(mc)/d*2)/2, -MaxGain), MaxGain)
	}
	return x[0], x[1], x[2]
}
