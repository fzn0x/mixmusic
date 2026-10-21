package celt_test

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/opus"
	"github.com/fzn0x/mixmusic/internal/opus/celt"
	"github.com/fzn0x/mixmusic/internal/opus/opustest"
	"github.com/fzn0x/mixmusic/internal/opus/rangedec"
)

// endBand is the last CELT band of a bandwidth.
func endBand(bandwidth int) int {
	switch bandwidth {
	case opus.BandwidthNarrowband:
		return 13
	case opus.BandwidthMediumband, opus.BandwidthWideband:
		return 17
	case opus.BandwidthSuperwideband:
		return 19
	}
	return 21
}

// decodeCELT decodes the CELT-only packets of a test vector as
// opus_decode_frame does, checks each packet's final range, and returns the
// number of samples per channel.
func decodeCELT(t *testing.T, packets []opustest.Packet, channels int) int {
	t.Helper()
	dec, err := celt.NewDecoder(48000, channels)
	if err != nil {
		t.Fatal(err)
	}
	var rd rangedec.Decoder
	pcm := make([]float32, 5760*channels)
	samples := 0
	mismatches := 0
	for i, p := range packets {
		toc, frames, err := opus.ParsePacket(p.Data)
		if err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		if opus.PacketMode(toc) != opus.ModeCELTOnly {
			t.Fatalf("packet %d is not CELT-only", i)
		}
		dec.SetStartBand(0)
		dec.SetEndBand(endBand(opus.PacketBandwidth(toc)))
		dec.SetStreamChannels(opus.PacketChannels(toc))
		fs := opus.SamplesPerFrame(toc, 48000)
		n := 0
		var rng uint32
		for _, f := range frames {
			rd.Init(f)
			k, err := dec.Decode(f, pcm[n*channels:], fs, &rd, false)
			if err != nil {
				t.Fatalf("packet %d: %v", i, err)
			}
			n += k
			rng = dec.FinalRange()
			if len(f) <= 1 {
				rng = 0
			}
		}
		if rng != p.FinalRange {
			if mismatches < 5 {
				t.Errorf("packet %d (toc %#x, %d frames): final range %#x, want %#x", i, toc, len(frames), rng, p.FinalRange)
			}
			mismatches++
		}
		samples += n
	}
	if mismatches > 0 {
		t.Errorf("%d of %d packets end in another range than the encoder's", mismatches, len(packets))
	}
	return samples
}

// TestVectors decodes the test vectors that are CELT-only throughout, in
// stereo and in mono, and checks the final range of every packet; the test
// of package opus compares what they decode to with the references.
func TestVectors(t *testing.T) {
	if opustest.Vectors() == "" {
		t.Skip("OPUS_TESTVECTORS names the directory of the RFC 8251 test vectors")
	}
	for _, v := range []string{"01", "07", "11"} {
		t.Run(v, func(t *testing.T) {
			packets, err := opustest.ReadBit(opustest.Path("testvector" + v + ".bit"))
			if err != nil {
				t.Fatal(err)
			}
			ref, err := opustest.ReadPCM16(opustest.Path("testvector" + v + ".dec"))
			if err != nil {
				t.Fatal(err)
			}
			for _, channels := range []int{2, 1} {
				if n := decodeCELT(t, packets, channels); n != len(ref)/2 {
					t.Errorf("%d channels: %d samples, want %d", channels, n, len(ref)/2)
				}
			}
		})
	}
}
