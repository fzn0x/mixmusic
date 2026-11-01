// Package opustest checks decoders against the Opus test vectors of RFC
// 8251: their files, and the quality metric of libopus's opus_compare,
// which decides whether a decoder conforms. It is for tests only. Its
// ports of opus_compare and of libopus's soft clipping are under the BSD
// license of LICENSE.libopus in internal/opus.
package opustest

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// Packet is a packet of a test vector, with the final range of the
// encoder's range coder, which the decoder's must equal.
type Packet struct {
	Data       []byte
	FinalRange uint32
}

// ReadBit reads a .bit file of the test vectors: for each packet, its
// length and its encoder's final range, both 32-bit big-endian, then its
// bytes.
func ReadBit(path string) ([]Packet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Packet
	for len(data) > 0 {
		if len(data) < 8 {
			return nil, errors.New("opustest: truncated .bit file")
		}
		n := binary.BigEndian.Uint32(data)
		rng := binary.BigEndian.Uint32(data[4:])
		data = data[8:]
		if uint32(len(data)) < n {
			return nil, errors.New("opustest: truncated packet")
		}
		out = append(out, Packet{Data: data[:n:n], FinalRange: rng})
		data = data[n:]
	}
	return out, nil
}

// ReadPCM16 reads 16-bit little-endian samples.
func ReadPCM16(path string) ([]int16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := make([]int16, len(data)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i:]))
	}
	return out, nil
}

// Vectors returns the directory of the test vectors, from the environment
// variable OPUS_TESTVECTORS, or "" when it is not set.
func Vectors() string { return os.Getenv("OPUS_TESTVECTORS") }

// Path is the path of a file of the test vectors, as "testvector01.bit".
func Path(name string) string { return filepath.Join(Vectors(), name) }

// SoftClip is opus_pcm_soft_clip_impl: it bends samples beyond [-1, 1]
// back into it, smoothly, carrying the curve over frames in mem.
func SoftClip(x []float32, n, c int, mem []float32) {
	if c < 1 || n < 1 {
		return
	}
	// Clamp to [-2, 2], the domain of the non-linearity.
	for i := 0; i < n*c; i++ {
		x[i] = max(-2, min(2, x[i]))
	}
	for ch := 0; ch < c; ch++ {
		at := func(i int) *float32 { return &x[i*c+ch] }
		a := mem[ch]
		// Continue the non-linearity of the previous frame.
		for i := 0; i < n; i++ {
			v := at(i)
			if *v*a >= 0 {
				break
			}
			*v = *v + a**v**v
		}
		curr := 0
		x0 := *at(0)
		for {
			i := curr
			for ; i < n; i++ {
				if v := *at(i); v > 1 || v < -1 {
					break
				}
			}
			if i == n {
				a = 0
				break
			}
			peakPos := i
			start, end := i, i
			maxval := float32(math.Abs(float64(*at(i))))
			// The zero crossings around the clipping.
			for start > 0 && *at(i)**at(start - 1) >= 0 {
				start--
			}
			for end < n && *at(i)**at(end) >= 0 {
				// Other peaks before the next zero crossing.
				if v := float32(math.Abs(float64(*at(end)))); v > maxval {
					maxval = v
					peakPos = end
				}
				end++
			}
			// Clipping before the first zero crossing.
			special := start == 0 && *at(i)**at(0) >= 0
			// a such that maxval + a*maxval^2 = 1.
			a = (maxval - 1) / (maxval * maxval)
			a += a * 2.4e-7
			if *at(i) > 0 {
				a = -a
			}
			for k := start; k < end; k++ {
				v := at(k)
				*v = *v + a**v**v
			}
			if special && peakPos >= 2 {
				// A ramp from the first sample to the peak avoids a
				// discontinuity at the start of the frame.
				offset := x0 - *at(0)
				delta := offset / float32(peakPos)
				for k := curr; k < peakPos; k++ {
					offset -= delta
					v := at(k)
					*v += offset
					*v = max(-1, min(1, *v))
				}
			}
			curr = end
			if curr == n {
				break
			}
		}
		mem[ch] = a
	}
}

// Float2Int16 is FLOAT2INT16: a sample of [-1, 1] as 16 bits, rounded to
// the nearest, ties to even, as lrintf.
func Float2Int16(x float32) int16 {
	x *= 32768
	x = max(x, -32768)
	x = min(x, 32767)
	return int16(math.RoundToEven(float64(x)))
}

const (
	nbands      = 21
	nfreqs      = 240
	testWinSize = 480
	testWinStep = 120
)

var bands = [nbands + 1]int{0, 2, 4, 6, 8, 10, 12, 14, 16, 20, 24, 28, 32, 40, 48, 56, 68, 80, 96, 120, 156, 200}

func bandEnergy(out, ps []float32, nb int, in []float32, nchannels, nframes, windowSz, step, downsample int) {
	const pi = float32(3.14159265)
	window := make([]float32, windowSz)
	c := make([]float32, windowSz)
	s := make([]float32, windowSz)
	x := make([]float32, nchannels*windowSz)
	psSz := windowSz / 2
	for j := 0; j < windowSz; j++ {
		window[j] = 0.5 - 0.5*float32(math.Cos(float64((2*pi/float32(windowSz-1))*float32(j))))
	}
	for j := 0; j < windowSz; j++ {
		c[j] = float32(math.Cos(float64((2 * pi / float32(windowSz)) * float32(j))))
	}
	for j := 0; j < windowSz; j++ {
		s[j] = float32(math.Sin(float64((2 * pi / float32(windowSz)) * float32(j))))
	}
	for xi := 0; xi < nframes; xi++ {
		for ci := 0; ci < nchannels; ci++ {
			for k := 0; k < windowSz; k++ {
				x[ci*windowSz+k] = window[k] * in[(xi*step+k)*nchannels+ci]
			}
		}
		j := 0
		for bi := 0; bi < nb; bi++ {
			var p [2]float32
			for ; j < bands[bi+1]; j++ {
				for ci := 0; ci < nchannels; ci++ {
					var re, im float32
					ti := 0
					for k := 0; k < windowSz; k++ {
						re += c[ti] * x[ci*windowSz+k]
						im -= s[ti] * x[ci*windowSz+k]
						ti += j
						if ti >= windowSz {
							ti -= windowSz
						}
					}
					re *= float32(downsample)
					im *= float32(downsample)
					ps[(xi*psSz+j)*nchannels+ci] = re*re + im*im + 100000
					p[ci] += ps[(xi*psSz+j)*nchannels+ci]
				}
			}
			if out != nil {
				out[(xi*nb+bi)*nchannels] = p[0] / float32(bands[bi+1]-bands[bi])
				if nchannels == 2 {
					out[(xi*nb+bi)*nchannels+1] = p[1] / float32(bands[bi+1]-bands[bi])
				}
			}
		}
	}
}

// Reference is the reference decode of a test vector, stereo 16-bit
// samples at 48 kHz, which Compare scores decodes against as opus_compare
// does. It keeps the spectrum of the reference for the next comparisons.
type Reference struct {
	samples []int16
	// spectra is by number of channels, 1 or 2: the band energies (xb) and
	// the power spectrum (X) of the reference, before the masking.
	spectra [3]*spectrum
}

type spectrum struct {
	nframes int
	xb, X   []float32
}

// ReadReference reads a reference decode, a .dec file.
func ReadReference(path string) (*Reference, error) {
	samples, err := ReadPCM16(path)
	if err != nil {
		return nil, err
	}
	return &Reference{samples: samples}, nil
}

// spectrum computes, once, the spectrum of the reference for nchannels
// channels: the mean of its two channels for 1.
func (r *Reference) spectrum(nchannels int) (*spectrum, error) {
	if s := r.spectra[nchannels]; s != nil {
		return s, nil
	}
	xlength := len(r.samples) / 2
	x := make([]float32, len(r.samples))
	for i, v := range r.samples {
		x[i] = float32(v)
	}
	if nchannels == 1 {
		for i := 0; i < xlength; i++ {
			x[i] = float32(.5 * (float64(x[2*i]) + float64(x[2*i+1])))
		}
	}
	if xlength < testWinSize {
		return nil, fmt.Errorf("insufficient sample data (%d < %d)", xlength, testWinSize)
	}
	nframes := (xlength - testWinSize + testWinStep) / testWinStep
	s := &spectrum{
		nframes: nframes,
		xb:      make([]float32, nframes*nbands*nchannels),
		X:       make([]float32, nframes*nfreqs*nchannels),
	}
	bandEnergy(s.xb, s.X, nbands, x, nchannels, nframes, testWinSize, testWinStep, 1)
	r.spectra[nchannels] = s
	return s, nil
}

// Compare is opus_compare [-s] -r rate: the quality of decoded, of
// nchannels channels at rate (8000, 12000, 16000, 24000 or 48000 Hz),
// against the reference. A quality below 0 fails the vector.
func (r *Reference) Compare(decoded []int16, nchannels, rate int) (quality, weightedErr float64, err error) {
	ybands := nbands
	switch rate {
	case 8000:
		ybands = 13
	case 12000:
		ybands = 15
	case 16000:
		ybands = 17
	case 24000:
		ybands = 19
	case 48000:
	default:
		return 0, 0, fmt.Errorf("sampling rate must be 8000, 12000, 16000, 24000, or 48000, not %d", rate)
	}
	if nchannels != 1 && nchannels != 2 {
		return 0, 0, fmt.Errorf("%d channels, not 1 or 2", nchannels)
	}
	downsample := 48000 / rate
	yfreqs := nfreqs / downsample
	xlength := len(r.samples) / 2
	ylength := len(decoded) / nchannels
	if xlength != ylength*downsample {
		return 0, 0, fmt.Errorf("sample counts do not match (%d != %d)", xlength, ylength*downsample)
	}
	ref, err := r.spectrum(nchannels)
	if err != nil {
		return 0, 0, err
	}
	nframes := ref.nframes
	// The masking changes the spectrum of the reference: a copy.
	xb := append([]float32(nil), ref.xb...)
	X := append([]float32(nil), ref.X...)
	y := make([]float32, len(decoded))
	for i, v := range decoded {
		y[i] = float32(v)
	}
	Y := make([]float32, nframes*yfreqs*nchannels)
	bandEnergy(nil, Y, ybands, y, nchannels, nframes, testWinSize/downsample, testWinStep/downsample, downsample)
	for xi := 0; xi < nframes; xi++ {
		// Frequency masking, low to high: 10 dB/Bark.
		for bi := 1; bi < nbands; bi++ {
			for ci := 0; ci < nchannels; ci++ {
				xb[(xi*nbands+bi)*nchannels+ci] += 0.1 * xb[(xi*nbands+bi-1)*nchannels+ci]
			}
		}
		// High to low: 15 dB/Bark.
		for bi := nbands - 2; bi >= 0; bi-- {
			for ci := 0; ci < nchannels; ci++ {
				xb[(xi*nbands+bi)*nchannels+ci] += 0.03 * xb[(xi*nbands+bi+1)*nchannels+ci]
			}
		}
		if xi > 0 {
			// Temporal masking: -3 dB/2.5 ms.
			for bi := 0; bi < nbands; bi++ {
				for ci := 0; ci < nchannels; ci++ {
					xb[(xi*nbands+bi)*nchannels+ci] += 0.5 * xb[((xi-1)*nbands+bi)*nchannels+ci]
				}
			}
		}
		// Some cross-talk.
		if nchannels == 2 {
			for bi := 0; bi < nbands; bi++ {
				l := xb[(xi*nbands+bi)*nchannels+0]
				r := xb[(xi*nbands+bi)*nchannels+1]
				xb[(xi*nbands+bi)*nchannels+0] += 0.01 * r
				xb[(xi*nbands+bi)*nchannels+1] += 0.01 * l
			}
		}
		// The masking.
		for bi := 0; bi < ybands; bi++ {
			for j := bands[bi]; j < bands[bi+1]; j++ {
				for ci := 0; ci < nchannels; ci++ {
					X[(xi*nfreqs+j)*nchannels+ci] += 0.1 * xb[(xi*nbands+bi)*nchannels+ci]
					Y[(xi*yfreqs+j)*nchannels+ci] += 0.1 * xb[(xi*nbands+bi)*nchannels+ci]
				}
			}
		}
	}
	// Consecutive frames averaged, to be slightly less sensitive.
	for bi := 0; bi < ybands; bi++ {
		for j := bands[bi]; j < bands[bi+1]; j++ {
			for ci := 0; ci < nchannels; ci++ {
				xtmp := X[j*nchannels+ci]
				ytmp := Y[j*nchannels+ci]
				for xi := 1; xi < nframes; xi++ {
					xtmp2 := X[(xi*nfreqs+j)*nchannels+ci]
					ytmp2 := Y[(xi*yfreqs+j)*nchannels+ci]
					X[(xi*nfreqs+j)*nchannels+ci] += xtmp
					Y[(xi*yfreqs+j)*nchannels+ci] += ytmp
					xtmp = xtmp2
					ytmp = ytmp2
				}
			}
		}
	}
	// Below 48 kHz, the last 300 Hz don't count, to allow for different
	// transition bands; at 12 kHz the last band already skips 400 Hz.
	maxCompare := bands[nbands]
	switch rate {
	case 48000:
	case 12000:
		maxCompare = bands[ybands]
	default:
		maxCompare = bands[ybands] - 3
	}
	var e float64
	for xi := 0; xi < nframes; xi++ {
		var ef float64
		for bi := 0; bi < ybands; bi++ {
			var eb float64
			for j := bands[bi]; j < bands[bi+1] && j < maxCompare; j++ {
				for ci := 0; ci < nchannels; ci++ {
					re := Y[(xi*yfreqs+j)*nchannels+ci] / X[(xi*nfreqs+j)*nchannels+ci]
					im := float32(float64(re) - math.Log(float64(re)) - 1)
					// Less sensitive around the SILK/CELT crossover.
					if j >= 79 && j <= 81 {
						im *= 0.1
					}
					if j == 80 {
						im *= 0.1
					}
					eb += float64(im)
				}
			}
			eb /= float64((bands[bi+1] - bands[bi]) * nchannels)
			ef += eb * eb
		}
		// A fixed normalization: lower rates may have a slightly lower
		// quality.
		ef /= nbands
		ef *= ef
		e += ef * ef
	}
	e = math.Pow(e/float64(nframes), 1.0/16)
	q := 100 * (1 - 0.5*math.Log(1+e)/math.Log(1.13))
	return q, e, nil
}
