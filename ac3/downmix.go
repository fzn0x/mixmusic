package ac3

import "math"

// The mix levels of the stream, as gains.
var (
	// centerLevels are clev of cmixlev, of Table 5.9: its reserved code
	// takes the middle level, as section 7.8.2 says.
	centerLevels = [4]float64{math.Sqrt2 / 2, 0.5946035575013605, 0.5, 0.5946035575013605}
	// surroundLevels are slev of surmixlev, of Table 5.10, its reserved
	// code -6 dB.
	surroundLevels = [4]float64{math.Sqrt2 / 2, 0.5, 0, 0.5}
	// loroLevels are those of lorocmixlev and lorosurmixlev, of Tables D2.5
	// and D2.6: +3 dB to -6 dB in steps of 1.5 dB, then none. The first
	// three of lorosurmixlev are reserved, and take -1.5 dB.
	loroLevels = [8]float64{math.Sqrt2, 1.189207115002721, 1, 0.8408964152537145, math.Sqrt2 / 2, 0.5946035575013605, 0.5, 0}
)

// downmix returns the LoRo downmix of the six channels Decode writes of a
// stream of h, of section 7.8: L, and C at clev and the left surround at
// slev, to the left output; R, C and the right surround to the right. A
// single surround channel, in both surround channels at -3 dB, so mixes at
// 0.707 slev, as the standard's equations of 3/1 have it. The gains are
// scaled down so that the gains of the stream's channels in neither output
// sum to more than 1. The LFE is left out, as the standard allows, but of
// an E-AC-3 stream that tells its mix level. Of E-AC-3 without mixing
// metadata, C mixes at -4.5 dB and the surrounds at -6 dB, the middle
// levels of AC-3.
func downmix(h *header) [2][MaxChannels]float32 {
	var m [2][MaxChannels]float64
	m[0][Left], m[1][Right] = 1, 1
	switch h.acmod {
	case ModeMono:
		m[0][Left], m[1][Right] = 0, 0
		m[0][Center], m[1][Center] = math.Sqrt2/2, math.Sqrt2/2
	case Mode30, Mode31, Mode32:
		clev := centerLevels[h.cmixlev&3]
		if h.loroCmixlev >= 0 {
			clev = loroLevels[h.loroCmixlev]
		}
		m[0][Center], m[1][Center] = clev, clev
	}
	// share is what of a channel of the six is of the stream's channel
	// it holds, for the sums of the gains of the stream's channels: the
	// single surround channel of 2/1 and 3/1 is in both surround channels
	// at -3 dB.
	share := [MaxChannels]float64{1, 1, 1, 1, 1, 1}
	if h.acmod >= Mode21 {
		slev := surroundLevels[h.surmixlev&3]
		if s := h.loroSurmixlev; s >= 0 {
			slev = loroLevels[max(s, 3)]
		}
		m[0][LeftSurround], m[1][RightSurround] = slev, slev
		if h.acmod == Mode21 || h.acmod == Mode31 {
			share[LeftSurround], share[RightSurround] = minus3dB, minus3dB
		}
	}
	// E-AC-3's LFE mix level, which mixes the LFE into both outputs at
	// its level less 4.5 dB, as section E.2.9 does.
	if h.lfeon && h.lfemixlevcod >= 0 {
		g := math.Pow(10, float64(10-h.lfemixlevcod)/20-4.5/20)
		m[0][LFE], m[1][LFE] = g, g
	}
	var sum float64
	for _, row := range m {
		var s float64
		for c, v := range row {
			s += v * share[c]
		}
		sum = max(sum, s)
	}
	var out [2][MaxChannels]float32
	for o, row := range m {
		for c, v := range row {
			if sum > 1 {
				v /= sum
			}
			out[o][c] = float32(v)
		}
	}
	return out
}

// setInfo sets the Info of the syncframe decoded last.
func (d *Decoder) setInfo() {
	h := &d.h
	rate := bitrates[h.frmsizecod>>1] * 1000
	if h.eac3 {
		rate = h.size * 8 * h.rate / (h.numblks * BlockLength)
	}
	d.info = Info{
		SampleRate: h.rate,
		Mode:       h.acmod,
		LFE:        h.lfeon,
		Bitrate:    rate,
		Downmix:    downmix(h),
	}
}
