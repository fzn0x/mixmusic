package ac3

import (
	"errors"
	"math"
	"path/filepath"
	"testing"
)

// FuzzDecode decodes any bytes, its first telling whether to check CRCs,
// as the syncframes of an MP4 sample, and checks that the decoder doesn't
// panic, decodes the frames it can to FrameLength samples each, refuses
// the others with ErrFormat or errors.ErrUnsupported, and makes only
// finite samples. Without the CRCs, which would refuse nearly every
// mutation, it reads what a corrupt frame holds.
func FuzzDecode(f *testing.F) {
	for _, x := range fixtures {
		all := frames(f, readFile(f, filepath.Join("testdata", x.name+".ac3")))
		f.Add(append([]byte{0}, all[1]...))
		f.Add(append(append([]byte{1}, all[2]...), all[3]...))
	}
	f.Add([]byte{1, 0x0B, 0x77})
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength*4)
	f.Fuzz(func(t *testing.T, in []byte) {
		if len(in) < 1 {
			return
		}
		d.noCRC = in[0]&1 == 0
		data := in[1:]
		n, err := d.Decode(data, pcm)
		if err != nil && !errors.Is(err, ErrFormat) && !errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, ErrBufferTooSmall) {
			t.Fatalf("error %v", err)
		}
		if n%FrameLength != 0 {
			t.Fatalf("%d samples", n)
		}
		for i, v := range pcm[:MaxChannels*n] {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("sample %d is %v", i, v)
			}
		}
	})
}
