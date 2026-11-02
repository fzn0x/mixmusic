package ac3

import (
	"math"
	"path/filepath"
	"testing"
)

// TestFFmpegDownmix checks Info().Downmix against ffmpeg's decoder's own
// downmix to stereo, of the fixtures of more than two channels: the LoRo
// downmix of their mix levels, scaled down by the sum of the gains of an
// output.
func TestFFmpegDownmix(t *testing.T) {
	haveFFmpeg(t)
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			file := filepath.Join("testdata", f.name+".ac3")
			data := readFile(t, file)
			ours, d, _ := decodeAll(t, data)
			if d.h.nfchans+b2i(d.h.lfeon) <= 2 {
				t.Skip("two channels or one")
			}
			m := d.Info().Downmix
			ref := ffmpegDecode(t, file, "-downmix", "stereo")
			if len(ref) != 2*len(ours)/MaxChannels {
				t.Skipf("ffmpeg's downmix is of %d channels", len(ref)*MaxChannels/len(ours))
			}
			var sig, noise float64
			for i := range len(ours) / MaxChannels {
				for o := range 2 {
					var v float64
					for c := range MaxChannels {
						v += float64(m[o][c]) * float64(ours[i*MaxChannels+c])
					}
					w := float64(ref[2*i+o])
					sig += w * w
					noise += (v - w) * (v - w)
				}
			}
			snr := 10 * math.Log10(sig/noise)
			t.Logf("%v: SNR %.1f dB", m, snr)
			if snr < 95 {
				t.Errorf("SNR %.1f dB", snr)
			}
		})
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
