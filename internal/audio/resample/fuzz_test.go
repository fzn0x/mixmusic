package resample

import (
	"encoding/binary"
	"math"
	"testing"
)

// fuzzRates are the rates FuzzConverter picks from: those of AAC, and a
// few that make many phases, or too many.
var fuzzRates = []int{
	7350, 8000, 11025, 12000, 16000, 22050, 24000, 32000,
	44100, 48000, 64000, 88200, 96000, 1000, 44056, 47999,
}

// FuzzConverter converts any bytes, as floats, between two rates it picks,
// in pieces of the sizes it picks, and checks that the converter doesn't
// panic, gives the frames of one call, bit for bit, as many as the stream's
// length makes, no larger than the input allows, and the same again after
// a Reset.
func FuzzConverter(f *testing.F) {
	f.Add(byte(0x89), byte(2), byte(7), byte(5), make([]byte, 400))
	f.Add(byte(0x59), byte(1), byte(0), byte(0), noiseBytes(3000))
	f.Add(byte(0xC9), byte(3), byte(60), byte(1), noiseBytes(5000))
	f.Add(byte(0x9C), byte(8), byte(13), byte(31), noiseBytes(2000))
	f.Add(byte(0x99), byte(2), byte(1), byte(63), noiseBytes(100))
	f.Fuzz(func(t *testing.T, rates, channels, inSize, outSize byte, data []byte) {
		from, to := fuzzRates[rates>>4], fuzzRates[rates&15]
		ch := 1 + int(channels%maxChannels)
		c, err := New(from, to, ch)
		if err != nil {
			return
		}
		// Longer inputs only take longer: 1000 to 96000 Hz makes 96 frames
		// of each.
		in := make([]float32, min(len(data)/4, 4096))
		bounded := true
		for i := range in {
			in[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4*i:]))
			bounded = bounded && math.Abs(float64(in[i])) <= 1
		}
		frames := len(in) / ch
		want := convert(t, c, in)
		if n := ceilDiv(int64(frames)*int64(to), int64(from)); int64(len(want)/ch) != n {
			t.Fatalf("%d to %d Hz: %d frames gave %d; want %d", from, to, frames, len(want)/ch, n)
		}
		if bounded {
			// The coefficients of a phase sum to 1, and their magnitudes
			// to less than 2.
			for i, v := range want {
				if !(math.Abs(float64(v)) < 2) {
					t.Fatalf("%d to %d Hz: sample %d of input within ±1 is %g", from, to, i, v)
				}
			}
		}

		// Again, after the Reset that the last Flush made, in pieces.
		var got []float32
		out := make([]float32, (1+int(outSize%64))*ch)
		step := 1 + int(inSize%61)
		for rest := in[:frames*ch]; len(rest) > 0; {
			n := min(step, len(rest)/ch)
			read, written := c.Process(rest[:n*ch], out)
			if read == 0 && written == 0 {
				t.Fatalf("%d to %d Hz: Process read and wrote nothing", from, to)
			}
			got = append(got, out[:written*ch]...)
			rest = rest[read*ch:]
		}
		for {
			n := c.Flush(out)
			got = append(got, out[:n*ch]...)
			if n*ch < len(out) {
				break
			}
		}
		if i := sameFloats(got, want); i >= 0 {
			t.Fatalf("%d to %d Hz: %d samples in pieces, %d at once, differing from %d", from, to, len(got), len(want), i)
		}
	})
}

// noiseBytes returns n bytes of floats of noise, from -1 to 1.
func noiseBytes(n int) []byte {
	b := make([]byte, n/4*4)
	for i, v := range noise(9, 1, n/4) {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(v))
	}
	return b
}
