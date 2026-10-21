package silk

import (
	"fmt"
	"math"
	"testing"

	"github.com/fzn0x/mixmusic/internal/opus/opustest"
	"github.com/fzn0x/mixmusic/internal/opus/rangedec"
)

// The SILK-only test vectors of RFC 8251: every packet has a TOC config of
// 0-11 and code 0 (one frame), switching between mono and stereo streams.
var silkVectors = []string{"02", "03", "04"}

// readBitstream reads a .bit file of the test vectors.
func readBitstream(t *testing.T, name string) []opustest.Packet {
	t.Helper()
	packets, err := opustest.ReadBit(opustest.Path(name))
	if err != nil {
		t.Fatal(err)
	}
	return packets
}

// readPCM reads a reference decode of the test vectors.
func readPCM(t *testing.T, name string) []int16 {
	t.Helper()
	pcm, err := opustest.ReadPCM16(opustest.Path(name))
	if err != nil {
		t.Fatal(err)
	}
	return pcm
}

// packetResult is what decoding one packet gave.
type packetResult struct {
	offset     int  // first sample per channel of the packet in the output
	samples    int  // samples per channel
	stereo     bool // stream channels 2
	redundancy bool // opus_decode_frame would read a redundant CELT frame
	rangeOK    bool // final range equals the encoder's
	gotRange   uint32
}

// decodeSILKOnly decodes SILK-only packets at 48 kHz into channels output
// channels, as opus_decode_frame does for them (no FEC, no packet loss), and
// returns the output converted to int16.
func decodeSILKOnly(t *testing.T, packets []opustest.Packet, channels int) ([]int16, []packetResult) {
	t.Helper()
	var d Decoder
	if err := d.Init(); err != nil {
		t.Fatal(err)
	}
	return decodeSILKOnlyWith(t, &d, packets, channels)
}

func decodeSILKOnlyWith(t *testing.T, d *Decoder, packets []opustest.Packet, channels int) ([]int16, []packetResult) {
	t.Helper()
	ctl := DecControl{ChannelsAPI: int32(channels), APISampleRate: 48000}
	var pcm []int16
	frame := make([]float32, 2880*channels)
	results := make([]packetResult, len(packets))
	var rd rangedec.Decoder
	for p, pkt := range packets {
		toc := pkt.Data[0]
		config := toc >> 3
		if config > 11 || toc&3 != 0 {
			t.Fatalf("packet %d: TOC %#x is not a single SILK-only frame", p, toc)
		}
		payload := pkt.Data[1:]
		if len(payload) <= 1 {
			t.Fatalf("packet %d: a payload of %d bytes would need the PLC", p, len(payload))
		}
		ms := []int{10, 20, 40, 60}[config&3]
		frameSize := 48 * ms
		res := &results[p]
		res.offset = len(pcm) / channels
		res.samples = frameSize
		res.stereo = toc&4 != 0

		rd.Init(payload)
		ctl.PayloadSizeMs = max(10, ms)
		ctl.ChannelsInternal = 1
		if res.stereo {
			ctl.ChannelsInternal = 2
		}
		ctl.InternalSampleRate = []int32{8000, 12000, 16000}[config/4]
		decoded := 0
		for decoded < frameSize {
			n, err := d.Decode(&ctl, 0, decoded == 0, &rd, frame[decoded*channels:])
			if err != nil {
				t.Fatalf("packet %d: %v", p, err)
			}
			if n <= 0 {
				t.Fatalf("packet %d: decoded %d samples", p, n)
			}
			decoded += n
		}
		if decoded != frameSize {
			t.Fatalf("packet %d: decoded %d samples, want %d", p, decoded, frameSize)
		}
		// opus_decode_frame: a SILK-only frame with 17 bits left holds a
		// redundant CELT frame, whose bits also enter the final range.
		res.redundancy = rd.Tell()+17 <= 8*len(payload)
		res.gotRange = rd.Range()
		res.rangeOK = res.gotRange == pkt.FinalRange
		for _, y := range frame[:frameSize*channels] {
			pcm = append(pcm, int16(math.Round(float64(y)*32768)))
		}
	}
	return pcm, results
}

// checkRanges checks the final range of every packet without redundancy.
func checkRanges(t *testing.T, packets []opustest.Packet, results []packetResult) {
	t.Helper()
	redundant, checked, bad := 0, 0, 0
	for p, r := range results {
		if r.redundancy {
			redundant++
			continue
		}
		checked++
		if !r.rangeOK {
			if bad == 0 {
				t.Errorf("packet %d: final range %#x, want %#x", p, r.gotRange, packets[p].FinalRange)
			}
			bad++
		}
	}
	if redundant > 0 {
		t.Logf("%d packets have a redundant CELT frame: their final range is not checked", redundant)
	}
	if bad > 0 {
		t.Errorf("%d of %d final ranges differ", bad, checked)
	} else {
		t.Logf("%d packets: all %d final ranges match", len(results), checked)
	}
}

// firstMismatch returns the first packet and sample (per channel) where got
// and want differ, or -1.
func firstMismatch(got, want []int16, channels int, results []packetResult) (packet, sample, ch int) {
	for i := range got {
		if i >= len(want) || got[i] != want[i] {
			s := i / channels
			for p := len(results) - 1; p >= 0; p-- {
				if results[p].offset <= s {
					return p, s - results[p].offset, i % channels
				}
			}
			return 0, s, i % channels
		}
	}
	return -1, -1, -1
}

func TestVectors(t *testing.T) {
	if opustest.Vectors() == "" {
		t.Skip("OPUS_TESTVECTORS names the directory of the RFC 8251 test vectors")
	}
	for _, v := range silkVectors {
		name := "testvector" + v
		t.Run(name, func(t *testing.T) {
			packets := readBitstream(t, name+".bit")
			refM := readPCM(t, name+"m.dec")
			ref := readPCM(t, name+".dec")

			t.Run("stereo", func(t *testing.T) {
				pcm, results := decodeSILKOnly(t, packets, 2)
				checkRanges(t, packets, results)
				if len(pcm) != len(refM) {
					t.Errorf("%d samples, want %d", len(pcm), len(refM))
				}
				if p, s, c := firstMismatch(pcm, refM, 2, results); p >= 0 {
					i := (results[p].offset+s)*2 + c
					t.Errorf("first mismatch with %sm.dec: packet %d, sample %d, channel %d: %s",
						name, p, s, c, describe(pcm, refM, i))
				} else {
					t.Logf("PCM equals %sm.dec exactly (%d samples per channel)", name, len(pcm)/2)
				}
				if p, _, _ := firstMismatch(pcm, ref, 2, results); p >= 0 || len(pcm) != len(ref) {
					t.Logf("PCM differs from %s.dec, first at packet %d", name, p)
				} else {
					t.Logf("PCM equals %s.dec exactly too", name)
				}
			})

			t.Run("reset", func(t *testing.T) {
				// Reset after decoding garbage gives a fresh decoder back.
				var d Decoder
				if err := d.Init(); err != nil {
					t.Fatal(err)
				}
				var rd rangedec.Decoder
				out := make([]float32, 2*960)
				garbage := []byte{0x5a, 0xc3, 0x99, 0x17, 0xe8, 0x42, 0x0f, 0xb6, 0x71, 0x2d}
				for i := 0; i < 20; i++ {
					rd.Init(garbage)
					ctl := DecControl{ChannelsAPI: 2, ChannelsInternal: 2, APISampleRate: 48000,
						InternalSampleRate: []int32{8000, 12000, 16000}[i%3], PayloadSizeMs: 20}
					if _, err := d.Decode(&ctl, i%3, true, &rd, out); err != nil {
						t.Fatal(err)
					}
				}
				if err := d.Reset(); err != nil {
					t.Fatal(err)
				}
				pcm, results := decodeSILKOnlyWith(t, &d, packets, 2)
				checkRanges(t, packets, results)
				if p, s, c := firstMismatch(pcm, refM, 2, results); p >= 0 || len(pcm) != len(refM) {
					t.Errorf("after Reset: first mismatch at packet %d, sample %d, channel %d", p, s, c)
				}
			})

			t.Run("mono", func(t *testing.T) {
				pcm, results := decodeSILKOnly(t, packets, 1)
				checkRanges(t, packets, results)
				if 2*len(pcm) != len(refM) {
					t.Errorf("%d samples, want %d", len(pcm), len(refM)/2)
				}
				// Until the first stereo packet, a mono decode is exactly the
				// left channel of the stereo one.
				exact := 0
				for _, r := range results {
					if r.stereo {
						break
					}
					exact = r.offset + r.samples
				}
				for i := 0; i < exact && i < len(pcm); i++ {
					if pcm[i] != refM[2*i] {
						t.Errorf("sample %d of the leading mono packets: %d, want %d", i, pcm[i], refM[2*i])
						break
					}
				}
				// Elsewhere there is no exact reference: the official check
				// scores against the downmix (L+R)/2. A rough sanity check.
				var sig, noise float64
				for i := 0; i < len(pcm) && 2*i+1 < len(refM); i++ {
					x := (float64(refM[2*i]) + float64(refM[2*i+1])) / 2
					e := float64(pcm[i]) - x
					sig += x * x
					noise += e * e
				}
				snr := 10 * math.Log10(sig/max(noise, 1))
				t.Logf("leading mono samples exact: %d; SNR against the (L+R)/2 downmix: %.1f dB", exact, snr)
				if snr < 20 {
					t.Errorf("SNR against the (L+R)/2 downmix %.1f dB, want >= 20 dB", snr)
				}
			})
		})
	}
}

func describe(got, want []int16, i int) string {
	if i >= len(want) {
		return "past the reference's end"
	}
	return fmt.Sprintf("got %d, want %d", got[i], want[i])
}
