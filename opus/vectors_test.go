package opus_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/opus/internal/opustest"
)

// libopusFloat is, for each test vector, the SHA-256 of what libopus 1.6.1
// decodes it to with opus_decode_float, built as plain C in the
// configuration this port follows: the files of CELT_SOURCES, SILK_SOURCES,
// SILK_SOURCES_FLOAT, OPUS_SOURCES and OPUS_SOURCES_FLOAT, with a config.h
// that defines only OPUS_BUILD, VAR_ARRAYS, HAVE_LRINT and HAVE_LRINTF, by
// gcc -O2 -ffp-contract=off for x86-64. Each is of the float32 samples,
// little-endian, of its decodes at 48, 24, 16, 12 and 8 kHz, in stereo then
// in mono at each.
var libopusFloat = map[string]string{
	"testvector01": "16dd21307b912ac96171010bed71eb168dd20e9003c7fe1220b256e624cdd4c1",
	"testvector02": "c86e3cfc539f87c07a8ab2860f60d3cba0ed597e3e26e75ae627b00a67694244",
	"testvector03": "0603119898b0ccef1e4263893da55964d7c81d9ebc92edf3bc371ce8965f9fbb",
	"testvector04": "d59ee58b2fc8890e563d8601bdbb160b66237481d59dff83bcae5ddd628a2ce9",
	"testvector05": "433e346488e36aaceeb003ae514d64fba6518ce8b61c6d11d257942b9c05cc6d",
	"testvector06": "dbc27c2079937e91820d1c70abd7042d4fb22c88d6c420a14c9f7d9b469a935a",
	"testvector07": "e8f61641fbb683dab09775ce35dd332cc8989b675fcb20e5921a56e08982ca84",
	"testvector08": "12547054abae358f5068be3206ab277048b42e0cfda5eeddb81f76123e0d7ea0",
	"testvector09": "222324aa546bfac293c0ffafcba885657c9b826b4f78f0482f0c5c47ad2b7be5",
	"testvector10": "db74ba23be19547da6d382d35a22d034cd91b7b9cc2fe2cda4f317274b672850",
	"testvector11": "a5a97f85253ce801f733dcee6003d7014e84ef5c0ef53e531c41bff8d04e55ad",
	"testvector12": "21d405d3c6350a2f278444886569552676d0709305752469e451efa842506621",
}

// fx, fy and fz are variables, so that the compiler can't fold them.
var fx, fy, fz float32 = 1 + 1.0/(1<<12), 1 + 1.0/(1<<12), -1

// mulAdd is x*y + z, which Go may compute as one fused operation.
//
//go:noinline
func mulAdd(x, y, z float32) float32 { return x*y + z }

// fusesMultiplyAdd reports whether Go fuses a multiplication and an
// addition into one operation, with one rounding where C rounds twice, as it
// does on arm64, and on amd64 from GOAMD64=v3: then the decodes differ from
// C's in their last bits. The conversion keeps the second from fusing.
func fusesMultiplyAdd() bool { return mulAdd(fx, fy, fz) != float32(fx*fy)+fz }

// decode decodes the packets of a test vector at rate as opus_demo does,
// through opus_decode to 16 bits, and checks each packet's final range. It
// writes the samples before opus_decode clips them, as opus_decode_float
// returns them, to h.
func decode(t *testing.T, packets []opustest.Packet, rate, channels int, h hash.Hash) []int16 {
	t.Helper()
	dec, err := opus.NewDecoder(rate, channels)
	if err != nil {
		t.Fatal(err)
	}
	softclip := make([]float32, 2)
	pcm := make([]float32, rate/25*3*channels)
	var out []int16
	mismatches := 0
	lostPrev := false
	for i, p := range packets {
		lost := len(p.Data) == 0
		var n int
		if lost {
			n, err = dec.Decode(nil, pcm[:dec.LastPacketDuration()*channels], false)
		} else {
			n, err = dec.Decode(p.Data, pcm, false)
		}
		if err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		if !lost && !lostPrev && dec.FinalRange() != p.FinalRange {
			if mismatches < 5 {
				t.Errorf("packet %d (toc %#x, %d bytes): final range %#x, want %#x", i, p.Data[0], len(p.Data), dec.FinalRange(), p.FinalRange)
			}
			mismatches++
		}
		binary.Write(h, binary.LittleEndian, pcm[:n*channels])
		// opus_decode clips softly what it decodes, not what it conceals.
		if !lost {
			opustest.SoftClip(pcm, n, channels, softclip)
		}
		for _, s := range pcm[:n*channels] {
			out = append(out, opustest.Float2Int16(s))
		}
		lostPrev = lost
	}
	if mismatches > 0 {
		t.Errorf("%d of %d packets end in another range than the encoder's", mismatches, len(packets))
	}
	return out
}

// TestVectors decodes the test vectors of RFC 8251 at each rate the decoder
// supports, in stereo and in mono. A decoder conforms when each decode is
// close enough, as opus_compare measures, to one of the two references of
// the vector: testvectorNN.dec, decoded with stereo phase inversion, or
// testvectorNNm.dec, decoded without, as libopus decodes to mono. The decodes
// must also equal libopus's, bit for bit, where Go rounds as C does.
func TestVectors(t *testing.T) {
	if opustest.Vectors() == "" {
		t.Skip("OPUS_TESTVECTORS names the directory of the RFC 8251 test vectors")
	}
	for v := 1; v <= 12; v++ {
		name := fmt.Sprintf("testvector%02d", v)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			packets, err := opustest.ReadBit(opustest.Path(name + ".bit"))
			if err != nil {
				t.Fatal(err)
			}
			refs := map[string]*opustest.Reference{}
			reference := func(suffix string) *opustest.Reference {
				if refs[suffix] == nil {
					r, err := opustest.ReadReference(opustest.Path(name + suffix))
					if err != nil {
						t.Fatal(err)
					}
					refs[suffix] = r
				}
				return refs[suffix]
			}
			h := sha256.New()
			for _, rate := range []int{48000, 24000, 16000, 12000, 8000} {
				for _, channels := range []int{2, 1} {
					out := decode(t, packets, rate, channels, h)
					// The reference the decode should match first, the other
					// only if it fails.
					order := []string{".dec", "m.dec"}
					if channels == 1 {
						order = []string{"m.dec", ".dec"}
					}
					var scores []string
					pass := false
					for _, suffix := range order {
						q, e, err := reference(suffix).Compare(out, channels, rate)
						if err != nil {
							t.Fatalf("%d Hz, %d channels, against %s: %v", rate, channels, suffix, err)
						}
						scores = append(scores, fmt.Sprintf("%.1f%% (error %f) against %s", q, e, suffix))
						if q >= 0 {
							pass = true
							break
						}
					}
					t.Logf("%d Hz, %d channels: quality %s", rate, channels, strings.Join(scores, ", "))
					if !pass {
						t.Errorf("%d Hz, %d channels: the vector fails", rate, channels)
					}
				}
			}
			if got := hex.EncodeToString(h.Sum(nil)); got != libopusFloat[name] {
				if fusesMultiplyAdd() {
					t.Logf("the decodes differ from libopus's in their last bits, as Go fuses multiply-adds here")
				} else {
					t.Errorf("the decodes differ from libopus's: SHA-256 %s, want %s", got, libopusFloat[name])
				}
			}
		})
	}
}

// BenchmarkDecode decodes each test vector in stereo, and reports how many
// times faster than real time.
func BenchmarkDecode(b *testing.B) {
	if opustest.Vectors() == "" {
		b.Skip("OPUS_TESTVECTORS names the directory of the RFC 8251 test vectors")
	}
	for v := 1; v <= 12; v++ {
		name := fmt.Sprintf("testvector%02d", v)
		b.Run(name, func(b *testing.B) {
			packets, err := opustest.ReadBit(opustest.Path(name + ".bit"))
			if err != nil {
				b.Fatal(err)
			}
			pcm := make([]float32, 5760*2)
			samples := 0
			for b.Loop() {
				dec, err := opus.NewDecoder(48000, 2)
				if err != nil {
					b.Fatal(err)
				}
				samples = 0
				for _, p := range packets {
					n, err := dec.Decode(p.Data, pcm, false)
					if err != nil {
						b.Fatal(err)
					}
					samples += n
				}
			}
			b.ReportMetric(float64(samples)/48000*float64(b.N)/b.Elapsed().Seconds(), "x-realtime")
		})
	}
}
