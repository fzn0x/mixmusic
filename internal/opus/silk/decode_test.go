package silk

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/fzn0x/mixmusic/internal/opus/opustest"
	"github.com/fzn0x/mixmusic/internal/opus/rangedec"
)

// TestRandomPackets decodes random payloads in every configuration, with
// packet loss (PLC) and FEC decoding, at every API rate: a decoder must not
// fail or panic on any input. No vector covers LBRR, FEC or PLC, so this
// only checks that those paths run.
func TestRandomPackets(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	rates := []int32{8000, 12000, 16000, 24000, 48000}
	payload := make([]byte, 400)
	out := make([]float32, 2*960)
	for run := 0; run < 40; run++ {
		var d Decoder
		if err := d.Init(); err != nil {
			t.Fatal(err)
		}
		ctl := DecControl{ChannelsAPI: int32(1 + run%2), APISampleRate: rates[run%len(rates)]}
		for p := 0; p < 300; p++ {
			ms := []int{10, 20, 40, 60}[r.IntN(4)]
			if r.IntN(4) != 0 && p > 0 {
				ms = ctl.PayloadSizeMs // mostly keep the configuration
			} else {
				ctl.ChannelsInternal = int32(1 + r.IntN(2))
				ctl.InternalSampleRate = []int32{8000, 12000, 16000}[r.IntN(3)]
			}
			ctl.PayloadSizeMs = ms
			n := 1 + r.IntN(len(payload))
			for i := range payload[:n] {
				payload[i] = byte(r.Uint32())
			}
			var rd rangedec.Decoder
			rd.Init(payload[:n])
			lostFlag := 0
			switch x := r.IntN(10); {
			case x == 0:
				lostFlag = flagPacketLost
			case x == 1:
				lostFlag = flagDecodeLBRR
			}
			if lostFlag == flagPacketLost {
				// opus_decode_frame conceals at most 20 ms per call.
				ctl.PayloadSizeMs = min(ms, 20)
			}
			want := int(ctl.APISampleRate) / 1000 * ctl.PayloadSizeMs
			for got, first := 0, true; got < want; first = false {
				k, err := d.Decode(&ctl, lostFlag, first, &rd, out)
				if err != nil {
					t.Fatalf("run %d, packet %d: %v", run, p, err)
				}
				for _, y := range out[:k*int(ctl.ChannelsAPI)] {
					if !(y >= -1 && y < 1) {
						t.Fatalf("run %d, packet %d: sample %v", run, p, y)
					}
				}
				got += k
			}
		}
	}
}

// TestPacketLoss conceals every seventh packet of the SILK-only vectors, once
// with the PLC and once with the FEC of the next packet, as opus_decode_frame
// does: the other packets must still decode to the encoder's final range,
// and the concealed audio must stay close to the reference.
func TestPacketLoss(t *testing.T) {
	if opustest.Vectors() == "" {
		t.Skip("OPUS_TESTVECTORS names the directory of the RFC 8251 test vectors")
	}
	for _, v := range silkVectors {
		name := "testvector" + v
		packets := readBitstream(t, name+".bit")
		ref := readPCM(t, name+"m.dec")
		for _, fec := range []bool{false, true} {
			var d Decoder
			if err := d.Init(); err != nil {
				t.Fatal(err)
			}
			ctl := DecControl{ChannelsAPI: 2, APISampleRate: 48000}
			out := make([]float32, 2*2880)
			var pcm []int16
			var rd rangedec.Decoder
			decode := func(pkt []byte, lostFlag int) {
				toc := pkt[0]
				config := toc >> 3
				ms := []int{10, 20, 40, 60}[config&3]
				if lostFlag != flagPacketLost {
					rd.Init(pkt[1:])
					ctl.ChannelsInternal = 1 + int32(toc>>2&1)
					ctl.InternalSampleRate = []int32{8000, 12000, 16000}[config/4]
				}
				for done := 0; done < ms; {
					chunk := ms - done
					if lostFlag == flagPacketLost {
						chunk = min(chunk, 20) // opus_decode_frame conceals 20 ms at most at once
					}
					ctl.PayloadSizeMs = max(10, chunk)
					for got := 0; got < 48*chunk; {
						k, err := d.Decode(&ctl, lostFlag, got == 0, &rd, out[2*got:])
						if err != nil {
							t.Fatalf("%s: %v", name, err)
						}
						got += k
					}
					for _, y := range out[:2*48*chunk] {
						pcm = append(pcm, int16(math.Round(float64(y)*32768)))
					}
					done += chunk
				}
			}
			lost, bad := 0, 0
			for p, pkt := range packets {
				if p%7 == 3 {
					lost++
					if fec && p+1 < len(packets) && packets[p+1].Data[0]&3 == 0 &&
						packets[p+1].Data[0]>>3&3 == pkt.Data[0]>>3&3 {
						// The FEC of the next packet, of the same duration.
						decode(packets[p+1].Data, flagDecodeLBRR)
					} else {
						decode(pkt.Data, flagPacketLost)
					}
					continue
				}
				decode(pkt.Data, flagDecodeNormal)
				if rd.Range() != pkt.FinalRange {
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("%s (fec %v): %d final ranges differ after losses", name, fec, bad)
			}
			if len(pcm) != len(ref) {
				t.Fatalf("%s (fec %v): %d samples, want %d", name, fec, len(pcm), len(ref))
			}
			var sig, noise float64
			for i, s := range pcm {
				e := float64(s) - float64(ref[i])
				sig += float64(ref[i]) * float64(ref[i])
				noise += e * e
			}
			snr := 10 * math.Log10(sig/max(noise, 1))
			t.Logf("%s (fec %v): %d of %d packets lost, SNR %.1f dB against the lossless decode", name, fec, lost, len(packets), snr)
			if snr < 3 {
				t.Errorf("%s (fec %v): SNR %.1f dB, want >= 3 dB", name, fec, snr)
			}
		}
	}
}
