package aac

import (
	"errors"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// frameWriter writes raw data blocks for tests.
type frameWriter struct{ bitWriter }

func (w *frameWriter) bits(v, n int) *frameWriter {
	w.write(uint32(v), n)
	return w
}

// codeword writes the codeword of codebook cb, 0 for scalefactors, that
// codes values.
func (w *frameWriter) codeword(cb int, values ...int) {
	for _, e := range codebooks()[cb] {
		if slices.Equal(e[2:], toInt32(values)) {
			w.bits(int(e[1]), int(e[0]))
			return
		}
	}
	panic("no codeword")
}

func toInt32(v []int) []int32 {
	out := make([]int32, len(v))
	for i, x := range v {
		out[i] = int32(x)
	}
	return out
}

// pair writes two values of the escape codebook, with their signs and
// escapes.
func (w *frameWriter) pair(a, b int) {
	abs := func(v int) int { return max(v, -v) }
	w.codeword(escapeBook, min(abs(a), 16), min(abs(b), 16))
	for _, v := range []int{a, b} {
		if v != 0 {
			w.bits(btoi(v < 0), 1)
		}
	}
	for _, v := range []int{abs(a), abs(b)} {
		if v >= 16 {
			n := bits.Len(uint(v)) - 1
			for range n - 4 {
				w.bits(1, 1)
			}
			w.bits(0, 1).bits(v-1<<n, n)
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// pulseSpec is the pulse data of a test frame: increments of the offset
// and amplitudes.
type pulseSpec struct {
	start  int
	offset []int
	amp    []int
}

// ics writes an individual_channel_stream of a long sine window whose
// first band, of 4 values, holds q, coded with the escape codebook at a
// global gain of 140, with pulses if p isn't nil.
func (w *frameWriter) ics(q [4]int, p *pulseSpec) {
	w.bits(140, 8)                                    // global_gain
	w.bits(0, 1).bits(onlyLongSequence, 2).bits(0, 1) // ics_reserved_bit, window_sequence, window_shape
	w.bits(1, 6).bits(0, 1)                           // max_sfb, predictor_data_present
	w.bits(escapeBook, 4).bits(1, 5)                  // a section of a band
	w.codeword(0, 60)                                 // its scalefactor, the global gain
	if p != nil {
		w.bits(1, 1).bits(len(p.offset)-1, 2).bits(p.start, 6)
		for i := range p.offset {
			w.bits(p.offset[i], 5).bits(p.amp[i], 4)
		}
	} else {
		w.bits(0, 1)
	}
	w.bits(0, 1).bits(0, 1) // tns_data_present, gain_control_data_present
	w.pair(q[0], q[1])
	w.pair(q[2], q[3])
}

// sce writes a single channel element of ics(q, p).
func (w *frameWriter) sce(q [4]int, p *pulseSpec) *frameWriter {
	w.bits(elemSCE, 3).bits(0, 4)
	w.ics(q, p)
	return w
}

// end writes the END element and returns the frame.
func (w *frameWriter) end() []byte {
	w.bits(elemEND, 3)
	return w.data
}

// decodeFrames decodes frames with a new decoder of cfg, and returns the
// samples, or the error of the first frame that fails.
func decodeFrames(t *testing.T, cfg Config, frames ...[]byte) ([]float32, error) {
	t.Helper()
	d, err := NewDecoder(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var out []float32
	pcm := make([]float32, 2*FrameLength)
	for _, f := range frames {
		if _, err := d.Decode(f, pcm); err != nil {
			return out, err
		}
		out = append(out, pcm...)
	}
	return out, nil
}

var mono44 = Config{ObjectType: ObjectTypeLC, SampleRate: 44100, Channels: 1}

// TestPulse checks that pulse data adds its amplitudes to the quantized
// values, away from zero, before the inverse quantization: as ffmpeg's
// aac encoder writes no pulse data, frames of it are built here.
func TestPulse(t *testing.T) {
	for _, c := range []struct {
		name   string
		q      [4]int
		pulses pulseSpec
		want   [4]int
	}{
		{"positive, zero, negative", [4]int{1, 0, 2, -1}, pulseSpec{0, []int{1, 1, 1}, []int{4, 3, 2}}, [4]int{1, -4, 5, -3}},
		{"escaped, and twice at one place", [4]int{20, 0, -40, 3}, pulseSpec{0, []int{0, 2, 0, 1}, []int{15, 1, 2, 7}}, [4]int{35, 0, -43, 10}},
		{"the most an escape codes", [4]int{8191, 0, 0, 0}, pulseSpec{0, []int{0, 0, 0, 0}, []int{15, 15, 15, 15}}, [4]int{8251, 0, 0, 0}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var a frameWriter
			pulsed := a.sce(c.q, &c.pulses).end()
			d, err := NewDecoder(mono44)
			if err != nil {
				t.Fatal(err)
			}
			d.r.init(pulsed)
			if err := d.parse(); err != nil {
				t.Fatal(err)
			}
			if got := d.ch[0].ics.quant[:4]; !slices.Equal(got, toInt32(c.want[:])) {
				t.Errorf("quantized %v, want %v", got, c.want)
			}
			got, err := decodeFrames(t, mono44, pulsed, pulsed)
			if err != nil {
				t.Fatal(err)
			}
			if c.want[0] > 8191 {
				return // no escape codes it
			}
			var b frameWriter
			direct := b.sce(c.want, nil).end()
			want, err := decodeFrames(t, mono44, direct, direct)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("pulses decode unlike the values they make")
			}

			// ffmpeg's decoder agrees.
			if !hasFFmpeg() {
				return
			}
			path := filepath.Join(t.TempDir(), "pulse.aac")
			if err := os.WriteFile(path, adts(mono44, pulsed, pulsed), 0o644); err != nil {
				t.Fatal(err)
			}
			snr, _, _ := compare(ffmpegDecode(t, path), 1, got)
			t.Logf("SNR %.1f dB against ffmpeg's decode", snr)
			if snr < 90 {
				t.Errorf("SNR %.1f dB", snr)
			}
		})
	}
}

// align pads w to a byte.
func (w *frameWriter) align() *frameWriter {
	for w.n%8 != 0 {
		w.bits(0, 1)
	}
	return w
}

// zeros writes n bytes of zeros.
func (w *frameWriter) zeros(n int) *frameWriter {
	for range n {
		w.bits(0, 8)
	}
	return w
}

// TestSkippedElements checks that data stream, fill and program config
// elements, which ffmpeg's aac encoder doesn't write but for fill data,
// change nothing of what a frame decodes to.
func TestSkippedElements(t *testing.T) {
	q := [4]int{3, -2, 20, 1}
	var plain, padded frameWriter
	want, err := decodeFrames(t, mono44, plain.sce(q, nil).end(), plain.data)
	if err != nil {
		t.Fatal(err)
	}

	// A data stream element of 300 bytes, after a byte alignment.
	padded.bits(elemDSE, 3).bits(0, 4).bits(1, 1).bits(255, 8).bits(45, 8).align().zeros(300)
	// A fill element of 100 bytes, which escapes its count.
	padded.bits(elemFIL, 3).bits(15, 4).bits(100-14, 8).zeros(100)
	// A program config element of a SCE, with a comment of 3 bytes.
	padded.bits(elemPCE, 3).bits(0, 4).bits(1, 2).bits(4, 4).bits(1, 4).bits(0, 4).bits(0, 4).bits(0, 2).bits(0, 3).bits(0, 4)
	padded.bits(0, 3).bits(0, 1).bits(0, 4).align().bits(3, 8).zeros(3)
	// A data stream element without the alignment, then the audio, then
	// fill bits.
	padded.bits(elemDSE, 3).bits(0, 4).bits(0, 1).bits(2, 8).zeros(2)
	padded.sce(q, nil).bits(elemFIL, 3).bits(3, 4).zeros(3)
	got, err := decodeFrames(t, mono44, padded.end(), padded.data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Error("the skipped elements change the decode")
	}
}

// header writes the start of a single channel element, up to its
// section data: a long window of maxSFB bands at gain.
func (w *frameWriter) header(gain, maxSFB int) *frameWriter {
	return w.bits(elemSCE, 3).bits(0, 4).bits(gain, 8).bits(0, 1).bits(onlyLongSequence, 2).bits(0, 1).bits(maxSFB, 6).bits(0, 1)
}

// TestErrors checks that the decoder refuses malformed frames, and the
// elements it doesn't decode, with the errors they deserve.
func TestErrors(t *testing.T) {
	stereo44 := Config{ObjectType: ObjectTypeLC, SampleRate: 44100, Channels: 2}
	var good frameWriter
	valid := good.sce([4]int{3, -2, 20, 1}, nil).end()
	frame := func(build func(w *frameWriter)) []byte {
		var w frameWriter
		build(&w)
		return w.end()
	}
	for _, c := range []struct {
		name  string
		cfg   Config
		frame []byte
		err   error
	}{
		{"empty frame", mono44, nil, ErrFormat},
		{"no END", mono44, valid[:len(valid)-1], ErrFormat},
		{"no channel element", mono44, frame(func(w *frameWriter) {}), ErrFormat},
		{"a SCE in a stereo stream", stereo44, valid, ErrFormat},
		{"a CPE in a mono stream", mono44, frame(func(w *frameWriter) {
			w.bits(elemCPE, 3).bits(0, 4).bits(0, 1)
		}), ErrFormat},
		{"two SCEs", mono44, frame(func(w *frameWriter) {
			w.sce([4]int{}, nil).sce([4]int{}, nil)
		}), ErrFormat},
		{"LFE", mono44, frame(func(w *frameWriter) { w.bits(elemLFE, 3) }), errors.ErrUnsupported},
		{"CCE", mono44, frame(func(w *frameWriter) { w.bits(elemCCE, 3) }), errors.ErrUnsupported},
		{"more bands than the window", mono44, frame(func(w *frameWriter) { w.header(140, 50) }), ErrFormat},
		{"prediction", mono44, frame(func(w *frameWriter) {
			w.bits(elemSCE, 3).bits(0, 4).bits(140, 8).bits(0, 4).bits(1, 6).bits(1, 1)
		}), ErrFormat},
		{"reserved codebook", mono44, frame(func(w *frameWriter) { w.header(140, 1).bits(12, 4).bits(1, 5) }), ErrFormat},
		{"section past the bands", mono44, frame(func(w *frameWriter) { w.header(140, 1).bits(escapeBook, 4).bits(2, 5) }), ErrFormat},
		{"sections of no band", mono44, frame(func(w *frameWriter) {
			w.header(140, 1)
			for range 200 {
				w.bits(escapeBook, 4).bits(0, 5)
			}
		}), ErrFormat},
		{"scalefactor past 255", mono44, frame(func(w *frameWriter) {
			w.header(255, 1).bits(escapeBook, 4).bits(1, 5)
			w.codeword(0, 120)
		}), ErrFormat},
		{"escape too long", mono44, frame(func(w *frameWriter) {
			w.header(140, 1).bits(escapeBook, 4).bits(1, 5)
			w.codeword(0, 60)
			w.bits(0, 3) // pulse, TNS, gain control
			w.codeword(escapeBook, 16, 0)
			w.bits(0, 1).bits(0x1FF, 9)
		}), ErrFormat},
		{"pulses past the bands", mono44, frame(func(w *frameWriter) {
			w.header(140, 0).bits(1, 1).bits(0, 2).bits(49, 6)
		}), ErrFormat},
		{"pulses past the spectrum", mono44, frame(func(w *frameWriter) {
			w.header(140, 0).bits(1, 1).bits(3, 2).bits(48, 6)
			for range 4 {
				w.bits(31, 5).bits(1, 4)
			}
		}), ErrFormat},
		{"pulses in short windows", mono44, frame(func(w *frameWriter) {
			w.bits(elemSCE, 3).bits(0, 4).bits(140, 8).bits(0, 1).bits(eightShortSequence, 2).bits(0, 1)
			w.bits(1, 4).bits(0x7F, 7) // max_sfb, one group of 8
			w.bits(escapeBook, 4).bits(1, 3)
			w.codeword(0, 60)
			w.bits(1, 1)
		}), ErrFormat},
		{"gain control", mono44, frame(func(w *frameWriter) { w.header(140, 0).bits(0, 2).bits(1, 1) }), errors.ErrUnsupported},
		{"reserved M/S mask", stereo44, frame(func(w *frameWriter) {
			w.bits(elemCPE, 3).bits(0, 4).bits(1, 1).bits(0, 4).bits(0, 6).bits(0, 1).bits(3, 2)
		}), ErrFormat},
		{"program config past the frame", mono44, frame(func(w *frameWriter) {
			w.bits(elemPCE, 3).bits(0, 4).bits(1, 2).bits(4, 4).bits(15, 4)
		}), ErrFormat},
		{"fill past the frame", mono44, frame(func(w *frameWriter) { w.bits(elemFIL, 3).bits(15, 4).bits(255, 8) }), ErrFormat},
	} {
		d, err := NewDecoder(c.cfg)
		if err != nil {
			t.Fatal(err)
		}
		pcm := make([]float32, 2*FrameLength)
		n, err := d.Decode(c.frame, pcm)
		if !errors.Is(err, c.err) || n != 0 {
			t.Errorf("%s: Decode = %d, %v; want %v", c.name, n, err, c.err)
		}
		t.Logf("%s: %v", c.name, err)
	}

	d, err := NewDecoder(mono44)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Decode(valid, make([]float32, 2*FrameLength-1)); err != ErrBufferTooSmall {
		t.Errorf("Decode into a short buffer: %v", err)
	}
	// Each cut of a frame short of its END fails.
	for n := range len(valid) - 1 {
		if _, err := d.Decode(valid[:n], make([]float32, 2*FrameLength)); !errors.Is(err, ErrFormat) {
			t.Errorf("Decode of %d bytes of %d: %v", n, len(valid), err)
		}
	}
}

// TestFinite checks that a frame whose samples overflow, as only a corrupt
// one does, decodes to finite samples: every band at the highest
// scalefactor, of the highest value an escape codes, through a filter of
// temporal noise shaping of the highest order and coefficients.
func TestFinite(t *testing.T) {
	var w frameWriter
	w.header(255, 49)
	w.bits(escapeBook, 4).bits(31, 5).bits(18, 5) // a section of the 49 bands
	for range 49 {
		w.codeword(0, 60)
	}
	w.bits(0, 1)                                    // pulse_data_present
	w.bits(1, 1).bits(1, 2).bits(1, 1)              // tns_data_present, n_filt, coef_res
	w.bits(49, 6).bits(12, 5).bits(0, 1).bits(0, 1) // length, order, direction, coef_compress
	for range 12 {
		w.bits(8, 4)
	}
	w.bits(0, 1) // gain_control_data_present
	for range FrameLength / 2 {
		w.pair(8191, -8191)
	}
	frame := w.end()

	d, err := NewDecoder(mono44)
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]float32, 2*FrameLength)
	overflowed := false
	for range 2 {
		if _, err := d.Decode(frame, pcm); err != nil {
			t.Fatal(err)
		}
		for i, v := range pcm {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("sample %d is %v", i, v)
			}
		}
		for _, v := range d.out[0] {
			overflowed = overflowed || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0)
		}
	}
	if !overflowed {
		t.Error("the frame doesn't overflow")
	}
}

// TestAfterError checks that a frame the decoder refuses leaves it as it
// was.
func TestAfterError(t *testing.T) {
	var a, b frameWriter
	frames := [][]byte{a.sce([4]int{3, -2, 20, 1}, nil).end(), b.sce([4]int{-7, 0, 1, 100}, nil).end()}
	want, err := decodeFrames(t, mono44, frames[0], frames[1], frames[0])
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDecoder(mono44)
	if err != nil {
		t.Fatal(err)
	}
	var got []float32
	pcm := make([]float32, 2*FrameLength)
	for i, f := range [][]byte{frames[0], frames[1][:len(frames[1])/2], frames[1], frames[0]} {
		if _, err := d.Decode(f, pcm); err != nil {
			if i != 1 {
				t.Fatal(err)
			}
			continue
		}
		got = append(got, pcm...)
	}
	if !slices.Equal(got, want) {
		t.Error("a refused frame changes the frames after it")
	}
}

// adts wraps frames, raw data blocks of cfg, in ADTS headers.
func adts(cfg Config, frames ...[]byte) []byte {
	index := slices.Index(sampleRates[:], cfg.SampleRate)
	var out []byte
	for _, f := range frames {
		n := 7 + len(f)
		var w bitWriter
		w.write(0xFFF, 12)
		w.write(0, 1)                        // MPEG-4
		w.write(0, 2)                        // layer
		w.write(1, 1)                        // protection_absent
		w.write(uint32(cfg.ObjectType-1), 2) // profile
		w.write(uint32(index), 4)            // sampling_frequency_index
		w.write(0, 1)                        // private_bit
		w.write(uint32(cfg.Channels), 3)     // channel_configuration
		w.write(0, 4)                        // original_copy, home, copyright bits
		w.write(uint32(n), 13)               // aac_frame_length
		w.write(0x7FF, 11)                   // adts_buffer_fullness
		w.write(0, 2)                        // number_of_raw_data_blocks_in_frame
		out = append(append(out, w.data...), f...)
	}
	return out
}
