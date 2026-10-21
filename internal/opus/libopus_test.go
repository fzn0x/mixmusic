package opus_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/opus"
	"github.com/fzn0x/mixmusic/internal/opus/opustest"
	"github.com/fzn0x/mixmusic/internal/webm"
)

// hasLBRR is opus_packet_has_lbrr: whether the first SILK frame of a
// packet carries in-band FEC (LBRR) data, in either channel.
func hasLBRR(packet []byte) bool {
	toc, frames, err := opus.ParsePacket(packet)
	if err != nil || opus.PacketMode(toc) == opus.ModeCELTOnly || len(frames[0]) == 0 {
		return false
	}
	n := max(1, opus.SamplesPerFrame(toc, 48000)/960)
	b := frames[0][0]
	return b>>(7-n)&1 != 0 || opus.PacketChannels(toc) == 2 && b>>(6-2*n)&1 != 0
}

// TestLibopusDecodes decodes streams of SILK with in-band FEC data, which
// none of the test vectors has, as libopus decodes them: bit for bit, as
// SILK is fixed-point. The streams in testdata are 4 seconds of a speech-like
// signal, which ffmpeg 7.0 and libopus encoded with
//
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=exprs='0.35*sin(2*PI*(140+60*sin(2*PI*0.4*t))*t)*(0.55+0.45*sin(2*PI*3.1*t))+0.25*sin(2*PI*(280+120*sin(2*PI*0.4*t))*t)*(0.5+0.5*sin(2*PI*3.1*t))+0.04*(random(0)-0.5)':s=48000:d=4" -ac 2 -c:a libopus OPTIONS -flags:a +bitexact -f webm NAME.webm
//
// with the options of each below, and the SHA-256 is of the 16-bit decode
// of libopus, through ffmpeg:
//
//	ffmpeg -c:a libopus -i NAME.webm -f s16le NAME.s16
func TestLibopusDecodes(t *testing.T) {
	for _, c := range []struct {
		name, options, sha256 string
	}{
		{"fec-stereo-40", "-application voip -b:a 22k -fec 1 -packet_loss 20 -frame_duration 40",
			"4e10a7df63fdd97ac32a3d4cbe91df922ab7179bdbc646bd976b0d19a6834402"},
		{"fec-stereo-80", "-application audio -b:a 24k -fec 1 -packet_loss 10 -frame_duration 80",
			"e762250f9e6109f2473388372b31cde443f4dceb4c5e7a3ccc15c4870de2a90e"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", c.name+".webm"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r, err := webm.NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			tr := r.Track()
			head := tr.CodecPrivate
			if len(head) < 19 || string(head[:8]) != "OpusHead" {
				t.Fatalf("CodecPrivate %q is not an OpusHead", head)
			}
			channels := tr.Channels
			dec, err := opus.NewDecoder(48000, channels)
			if err != nil {
				t.Fatal(err)
			}
			softclip := make([]float32, 2)
			pcm := make([]float32, 5760*channels)
			var out []int16
			packets, lbrr := 0, 0
			var padding time.Duration
			for {
				p, err := r.ReadPacket()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				packets++
				if hasLBRR(p.Data) {
					lbrr++
				}
				// As opus_decode does it, to 16 bits.
				n, err := dec.Decode(p.Data, pcm, false)
				if err != nil {
					t.Fatalf("packet %d: %v", packets-1, err)
				}
				opustest.SoftClip(pcm, n, channels, softclip)
				for _, s := range pcm[:n*channels] {
					out = append(out, opustest.Float2Int16(s))
				}
				padding = p.DiscardPadding
			}
			if lbrr < packets/2 {
				t.Errorf("only %d of %d packets have FEC data", lbrr, packets)
			}
			// ffmpeg drops the pre-skip of the OpusHead and the padding of
			// the last packet.
			preSkip := int(binary.LittleEndian.Uint16(head[10:]))
			end := len(out) - int(padding*48000/time.Second)*channels
			if preSkip*channels > end {
				t.Fatalf("%d samples, pre-skip %d, padding %v", len(out), preSkip, padding)
			}
			h := sha256.New()
			binary.Write(h, binary.LittleEndian, out[preSkip*channels:end])
			if got := hex.EncodeToString(h.Sum(nil)); got != c.sha256 {
				t.Errorf("the decode's SHA-256 is %s, libopus's %s", got, c.sha256)
			}
		})
	}
}
