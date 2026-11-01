package aac

import (
	"errors"
	"testing"
)

// bitWriter writes bits, most significant first.
type bitWriter struct {
	data []byte
	n    int // bits written
}

func (w *bitWriter) write(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		if w.n%8 == 0 {
			w.data = append(w.data, 0)
		}
		w.data[w.n/8] |= byte(v>>i&1) << (7 - w.n%8)
		w.n++
	}
}

// asc builds an AudioSpecificConfig of fields, each a value and its width
// in bits.
func asc(fields ...[2]int) []byte {
	var w bitWriter
	for _, f := range fields {
		w.write(uint32(f[0]), f[1])
	}
	return w.data
}

func TestParseConfig(t *testing.T) {
	lc := func(rate, channels int) Config {
		return Config{ObjectType: ObjectTypeLC, SampleRate: rate, Channels: channels}
	}
	for _, c := range []struct {
		name string
		asc  []byte
		want Config
		err  error
	}{
		{"stereo 44.1 kHz", []byte{0x12, 0x10}, lc(44100, 2), nil},
		// YouTube's itag 140 pads its config with zeros.
		{"itag 140", []byte{0x12, 0x10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, lc(44100, 2), nil},
		{"mono 48 kHz", []byte{0x11, 0x88}, lc(48000, 1), nil},
		{"7350 Hz", asc([2]int{2, 5}, [2]int{12, 4}, [2]int{1, 4}, [2]int{0, 3}), lc(7350, 1), nil},
		{"explicit rate", asc([2]int{2, 5}, [2]int{15, 4}, [2]int{44056, 24}, [2]int{2, 4}, [2]int{0, 3}), lc(44056, 2), nil},
		{"escaped object type", asc([2]int{31, 5}, [2]int{0, 6}, [2]int{4, 4}, [2]int{2, 4}, [2]int{0, 3}), Config{}, errors.ErrUnsupported},
		{"core coder delay and extension flag", asc([2]int{2, 5}, [2]int{3, 4}, [2]int{2, 4}, [2]int{0, 1}, [2]int{1, 1}, [2]int{1234, 14}, [2]int{1, 1}, [2]int{0, 1}), lc(48000, 2), nil},
		// ffmpeg's -aac_pce: a program of one CPE, with ffmpeg's name as
		// its comment, then a sync extension that signals no SBR.
		{"program config", append(append([]byte{0x12, 0x00, 0x05, 0x04, 0x00, 0x00, 0x20, 0x0c}, "Lavc61.3.100"...), 0x56, 0xe5, 0x00), lc(44100, 2), nil},
		// The fields of a program config element: element_instance_tag,
		// object_type, sampling_frequency_index, the counts of the front,
		// side and back elements, of LFEs, data streams and coupling
		// elements, three mixdown flags, then each element's is_cpe and
		// tag, each LFE's tag, the byte alignment and the comment's length.
		{"program config of a SCE", asc([2]int{2, 5}, [2]int{4, 4}, [2]int{0, 4}, [2]int{0, 3},
			[2]int{0, 4}, [2]int{1, 2}, [2]int{4, 4}, [2]int{1, 4}, [2]int{0, 4}, [2]int{0, 4}, [2]int{0, 2}, [2]int{0, 3}, [2]int{0, 4}, [2]int{0, 3},
			[2]int{0, 5}, [2]int{0, 1}, [2]int{0, 8}), lc(44100, 1), nil},
		{"program config of 5.1", asc([2]int{2, 5}, [2]int{3, 4}, [2]int{0, 4}, [2]int{0, 3},
			[2]int{0, 4}, [2]int{1, 2}, [2]int{3, 4}, [2]int{2, 4}, [2]int{0, 4}, [2]int{1, 4}, [2]int{1, 2}, [2]int{0, 3}, [2]int{0, 4}, [2]int{0, 3},
			[2]int{0, 5}, [2]int{16, 5}, [2]int{16, 5}, [2]int{1, 4}, [2]int{0, 8}), Config{}, errors.ErrUnsupported},
		{"program config past the end", []byte{0x12, 0x00, 0x05, 0x04, 0x00, 0x00, 0x20, 0x0c, 'L'}, Config{}, ErrFormat},
		// HE-AAC compatible with AAC-LC: a sync extension of SBR at 44.1
		// kHz after a core at 22.05 kHz.
		{"sync extension", asc([2]int{2, 5}, [2]int{7, 4}, [2]int{2, 4}, [2]int{0, 3}, [2]int{0x2B7, 11}, [2]int{5, 5}, [2]int{1, 1}, [2]int{4, 4}),
			Config{ObjectType: ObjectTypeLC, SampleRate: 22050, Channels: 2, SBR: true}, nil},
		// YouTube's itag 139: explicit HE-AAC, of SBR at 44.1 kHz on a
		// core at 22.05 kHz.
		{"itag 139", []byte{0x2b, 0x92, 0x08, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Config{}, errors.ErrUnsupported},
		{"HE-AAC v2", asc([2]int{29, 5}, [2]int{7, 4}, [2]int{2, 4}, [2]int{4, 4}, [2]int{2, 5}, [2]int{0, 3}), Config{}, errors.ErrUnsupported},
		{"AAC Main", []byte{0x0a, 0x10}, Config{}, errors.ErrUnsupported},
		{"AAC LTP", []byte{0x22, 0x10}, Config{}, errors.ErrUnsupported},
		{"ER AAC LC", asc([2]int{17, 5}, [2]int{4, 4}, [2]int{2, 4}, [2]int{0, 3}), Config{}, errors.ErrUnsupported},
		{"960 samples", []byte{0x12, 0x14}, Config{}, errors.ErrUnsupported},
		{"5.1", []byte{0x12, 0x30}, Config{}, errors.ErrUnsupported},
		{"reserved sampling frequency index", asc([2]int{2, 5}, [2]int{13, 4}, [2]int{2, 4}, [2]int{0, 3}), Config{}, ErrFormat},
		{"explicit rate of 0", asc([2]int{2, 5}, [2]int{15, 4}, [2]int{0, 24}, [2]int{2, 4}, [2]int{0, 3}), Config{}, ErrFormat},
		{"one byte", []byte{0x12}, Config{}, ErrFormat},
		{"empty", nil, Config{}, ErrFormat},
	} {
		got, err := ParseConfig(c.asc)
		if !errors.Is(err, c.err) || err == nil && c.err != nil || got != c.want {
			t.Errorf("%s: ParseConfig(% x) = %+v, %v; want %+v, %v", c.name, c.asc, got, err, c.want, c.err)
		}
	}
}

// TestRateIndex checks that each rate of an index has the index's tables,
// and that rates between take the nearest's.
func TestRateIndex(t *testing.T) {
	for i, rate := range sampleRates {
		if got := rateIndex(rate); got != min(i, 11) {
			t.Errorf("rateIndex(%d) = %d, want %d", rate, got, min(i, 11))
		}
	}
	for rate, want := range map[int]int{100000: 0, 92016: 1, 50000: 3, 44056: 4, 46009: 3, 46008: 4, 9391: 10, 9390: 11, 1: 11} {
		if got := rateIndex(rate); got != want {
			t.Errorf("rateIndex(%d) = %d, want %d", rate, got, want)
		}
	}
}
