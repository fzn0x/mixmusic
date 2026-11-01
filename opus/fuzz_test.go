package opus_test

import (
	"math"
	"testing"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/opus/internal/opustest"
)

// fuzzPacket is a packet of an input of FuzzDecode, and how to decode it:
// bit 0 of ctl loses it, bit 1 decodes it with FEC, bits 2-4 pick the room
// the decoder has.
type fuzzPacket struct {
	ctl  byte
	data []byte
}

// fuzzInput builds an input of FuzzDecode: the decoder's settings, then
// each packet after its control byte and its length, 16 bits.
func fuzzInput(settings byte, packets ...fuzzPacket) []byte {
	in := []byte{settings}
	for _, p := range packets {
		in = append(in, p.ctl, byte(len(p.data)>>8), byte(len(p.data)))
		in = append(in, p.data...)
	}
	return in
}

// frame is a packet of a TOC byte and n bytes of a fixed pseudo-random
// sequence.
func frame(toc byte, n int) []byte {
	b := []byte{toc}
	seed := uint32(toc)<<16 + uint32(n)
	for range n {
		seed = seed*1664525 + 1013904223
		b = append(b, byte(seed>>24))
	}
	return b
}

// FuzzDecode decodes any bytes as a sequence of packets, lost packets and
// FEC, and checks that the decoder doesn't panic, decodes a packet to its
// duration, returns no more than it was asked for, and only finite samples.
func FuzzDecode(f *testing.F) {
	// A frame of each mode, bandwidth and frame code.
	for _, toc := range []byte{
		0x08, 0x48, 0x58, // SILK NB 20 ms, WB 20 ms, WB 60 ms
		0x78, 0x7C, 0x60, // hybrid FB 20 ms, its stereo, SWB 10 ms
		0x80, 0xF8, 0xFC, // CELT NB 2.5 ms, FB 20 ms, its stereo
	} {
		for _, n := range []int{0, 1, 2, 17, 60, 300} {
			for settings := byte(0); settings < 10; settings += 3 {
				f.Add(fuzzInput(settings, fuzzPacket{0, frame(toc, n)}))
			}
		}
	}
	// Two frames of the same size, two not; three with padding.
	f.Add(fuzzInput(1, fuzzPacket{0, frame(0xFD, 80)}, fuzzPacket{0, append([]byte{0x7E, 20}, frame(0, 50)...)}))
	f.Add(fuzzInput(0, fuzzPacket{0, append([]byte{0x4B, 0xC3, 3, 10, 12}, frame(0, 60)...)}))
	// Switches between modes, a loss, and FEC.
	f.Add(fuzzInput(1,
		fuzzPacket{0, frame(0x4C, 70)},
		fuzzPacket{0, frame(0xFC, 90)},
		fuzzPacket{0, frame(0x7C, 90)},
		fuzzPacket{1, frame(0x7C, 90)},
		fuzzPacket{2, frame(0x4C, 70)},
		fuzzPacket{0, frame(0x4C, 70)}))
	// Real packets, when the test vectors are at hand.
	if opustest.Vectors() != "" {
		for _, name := range []string{"02", "05", "07", "08", "10", "12"} {
			packets, err := opustest.ReadBit(opustest.Path("testvector" + name + ".bit"))
			if err != nil {
				f.Fatal(err)
			}
			for i := 0; i+3 < len(packets); i += max(1, len(packets)/8) {
				f.Add(fuzzInput(byte(i),
					fuzzPacket{0, packets[i].Data},
					fuzzPacket{byte(i), packets[i+1].Data},
					fuzzPacket{0, packets[i+2].Data},
					fuzzPacket{2, packets[i+3].Data}))
			}
		}
	}

	rates := []int{48000, 24000, 16000, 12000, 8000}
	f.Fuzz(func(t *testing.T, in []byte) {
		if len(in) < 1 {
			return
		}
		rate := rates[int(in[0]>>1)%len(rates)]
		channels := int(in[0]&1) + 1
		dec, err := opus.NewDecoder(rate, channels)
		if err != nil {
			t.Fatal(err)
		}
		pcm := make([]float32, rate/25*3*channels)
		in = in[1:]
		for len(in) >= 3 {
			ctl := in[0]
			n := min(int(in[1])<<8|int(in[2]), len(in)-3)
			packet := in[3 : 3+n]
			in = in[3+n:]
			if ctl&1 != 0 {
				packet = nil
			}
			fec := ctl&2 != 0
			frameSize := []int{rate / 25 * 3, rate / 400, rate / 200, rate / 100, rate / 50, rate / 25,
				rate * 3 / 50, dec.LastPacketDuration()}[ctl>>2&7]
			got, err := dec.Decode(packet, pcm[:frameSize*channels], fec)
			if err != nil {
				continue
			}
			if got <= 0 || got > frameSize {
				t.Fatalf("decoded %d samples into room for %d", got, frameSize)
			}
			if len(packet) > 0 && !fec {
				if want, err := opus.PacketSamples(packet, rate); err != nil || got != want {
					t.Fatalf("decoded %d samples of a packet of %d (%v)", got, want, err)
				}
			}
			for i, v := range pcm[:got*channels] {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatalf("sample %d is %v", i, v)
				}
			}
		}
	})
}
