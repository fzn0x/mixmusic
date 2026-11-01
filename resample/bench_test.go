package resample

import (
	"strconv"
	"testing"
)

// BenchmarkProcess converts stereo to 48 kHz, 1024 frames at a time as
// AAC decodes them, and reports how many times faster than they play.
func BenchmarkProcess(b *testing.B) {
	for _, from := range []int{44100, 32000, 24000, 22050} {
		b.Run(strconv.Itoa(from), func(b *testing.B) {
			c, err := New(from, 48000, 2)
			if err != nil {
				b.Fatal(err)
			}
			in := noise(8, 2, 1024)
			out := make([]float32, 2*c.MaxOutput(1024))
			b.SetBytes(int64(len(in) * 4))
			for b.Loop() {
				c.Process(in, out)
			}
			b.ReportMetric(float64(b.N)*1024/float64(from)/b.Elapsed().Seconds(), "x-realtime")
		})
	}
}
