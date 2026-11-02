package ac3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// hasFFmpeg reports whether ffmpeg is on the PATH.
func hasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// haveFFmpeg skips t without ffmpeg on the PATH.
func haveFFmpeg(t testing.TB) {
	t.Helper()
	if !hasFFmpeg() {
		t.Skip("ffmpeg isn't on the PATH")
	}
}

// ffmpeg runs ffmpeg with args, and returns what it writes.
func ffmpeg(t testing.TB, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-hide_banner", "-loglevel", "error", "-nostdin"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, stderr.Bytes())
	}
	return out
}

// ffmpegDecode decodes the AC-3 file at path with ffmpeg, into float32
// samples of its channels, interleaved in ffmpeg's order.
func ffmpegDecode(t testing.TB, path string, args ...string) []float32 {
	t.Helper()
	args = append(args, "-i", path, "-f", "f32le", "-c:a", "pcm_f32le", "pipe:1")
	b := ffmpeg(t, args...)
	pcm := make([]float32, len(b)/4)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return pcm
}

// ffmpegOrder returns the channels ffmpeg decodes a stream of acmod and
// lfeon to, as those of Decode, in the order of their speaker mask; the
// single surround channel of 2/1 and 3/1, last, is -1.
func ffmpegOrder(acmod int, lfeon bool) []int {
	var order []int
	switch acmod {
	case ModeMono:
		order = []int{Center}
	case Mode30, Mode31, Mode32:
		order = []int{Left, Right, Center}
	default:
		order = []int{Left, Right}
	}
	if lfeon {
		order = append(order, LFE)
	}
	switch acmod {
	case Mode22, Mode32:
		order = append(order, LeftSurround, RightSurround)
	case Mode21, Mode31:
		order = append(order, -1)
	}
	return order
}

// frames splits an AC-3 file into its syncframes.
func frames(t testing.TB, data []byte) [][]byte {
	t.Helper()
	var out [][]byte
	for len(data) > 0 {
		var h header
		var r bitReader
		r.init(data)
		if err := h.parse(&r); err != nil {
			t.Fatalf("frame %d: %v", len(out), err)
		}
		if h.size > len(data) {
			t.Fatalf("frame %d: %d bytes of %d", len(out), len(data), h.size)
		}
		out = append(out, data[:h.size])
		data = data[h.size:]
	}
	return out
}

// toolUse counts the blocks that use each tool of the decoder.
type toolUse map[string]int

// tools are the names toolUse counts.
var tools = []string{
	"short", "dither", "cpl", "cpl-phase", "remat", "d15", "d25", "d45", "reuse",
	"cpl-d15", "cpl-d25", "cpl-d45", "delta", "cpl-delta", "dynrng", "bap1", "bap2", "bap3", "bap4", "bap5", "bap-asym", "bap16",
	"spx", "spx-notch", "aht", "vq", "gaq",
}

func (u toolUse) String() string {
	var parts []string
	for _, k := range tools {
		if u[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, u[k]))
		}
	}
	return strings.Join(parts, ", ")
}

// watch has d count in u the tools of each block it decodes, and, when
// dither isn't nil, note there where the dithflags of each block are, in
// bits from the start of the frame, and how many.
func (u toolUse) watch(d *Decoder, dither *[]int) {
	var exps [maxStates][maxBins]uint8
	d.onBlock = func(blk int) {
		seen := map[string]bool{}
		nf := d.h.nfchans
		if dither != nil {
			*dither = append(*dither, d.ditherAt, nf)
		}
		check := func(ch *channel, state int) {
			for _, b := range ch.bap[ch.start:ch.end] {
				switch {
				case b >= 1 && b <= 5:
					seen[fmt.Sprintf("bap%d", b)] = true
				case b == 15:
					seen["bap16"] = true
				case b > 5:
					seen["bap-asym"] = true
				}
			}
			if blk > 0 && ch.exp == exps[state] {
				seen["reuse"] = true
			}
			exps[state] = ch.exp
		}
		for c := range nf {
			ch := &d.ch[c]
			if ch.blksw {
				seen["short"] = true
			}
			if ch.dithflag {
				seen["dither"] = true
			}
			if ch.delta {
				seen["delta"] = true
			}
			if e := d.expstr[c]; e > 0 {
				seen[[4]string{"", "d15", "d25", "d45"}[e]] = true
			}
			check(ch, c)
		}
		if d.h.lfeon {
			check(&d.ch[lfeIndex], lfeIndex)
		}
		if d.cplinu {
			seen["cpl"] = true
			if d.phsflginu {
				seen["cpl-phase"] = true
			}
			if d.ch[cplIndex].delta {
				seen["cpl-delta"] = true
			}
			if e := d.cplexpstr; e > 0 {
				seen[[4]string{"", "cpl-d15", "cpl-d25", "cpl-d45"}[e]] = true
			}
			check(&d.ch[cplIndex], cplIndex)
		}
		if d.h.acmod == ModeStereo {
			for _, f := range d.rematflg[:d.rematBands()] {
				if f {
					seen["remat"] = true
				}
			}
		}
		if d.dynrng != 1 {
			seen["dynrng"] = true
		}
		if d.spxinu {
			seen["spx"] = true
			for c := range nf {
				if d.chinspx[c] && d.frm.spxattencod[c] >= 0 {
					seen["spx-notch"] = true
				}
			}
		}
		for c := range nf {
			if ch := &d.ch[c]; ch.aht {
				seen["aht"] = true
				for _, b := range ch.bap[ch.start:ch.end] {
					switch {
					case b == 0:
					case b <= 7:
						seen["vq"] = true
					default:
						seen["gaq"] = true
					}
				}
			}
		}
		for k := range seen {
			u[k]++
		}
	}
}

// undither clears the dithflags of every block of a file, and sets its
// CRCs again: decoders then make the same samples of it, as none makes up
// noise of its own for mantissas of no bits. An E-AC-3 frame without
// dithflags, whose blocks all dither, gets dithflage and dithflags of 0.
func undither(t testing.TB, data []byte) []byte {
	t.Helper()
	return edit(t, data, func(s bitstring, m blockMarks) bitstring {
		if m.eac3 && !m.dithOn {
			for blk := m.numblks - 1; blk >= 0; blk-- {
				s = s.replace(m.dither[blk], append(make([]byte, m.nf), s[m.dither[blk]])...)
			}
			s[m.dithflage] = 1
			return s
		}
		for blk := range m.numblks {
			for c := range m.nf {
				s[m.dither[blk]+c] = 0
			}
		}
		return s
	})
}

// setCRC sets the CRCs of a syncframe, as an encoder does: of E-AC-3, its
// crc2, so that the CRC of all of it after the syncword is 0; of AC-3, that
// crc2, and crc1 so that the CRC of its first 5/8 is 0. crc1 comes first,
// so that it can't simply be that of what follows: as the CRC is linear,
// it solves the 16 equations of its bits.
func setCRC(f []byte) {
	if f[5]>>3 > 10 {
		binary.BigEndian.PutUint16(f[len(f)-2:], crc16(0, f[2:len(f)-2]))
		return
	}
	words := len(f) / 2
	five8 := 2 * (words>>1 + words>>3)
	crc := func(x uint16) uint16 {
		binary.BigEndian.PutUint16(f[2:], x)
		return crc16(0, f[2:five8])
	}
	// The CRC of crc1 = x is c0 xor the sum of col[b] over the bits b of x.
	c0 := crc(0)
	var col [16]uint16
	for b := range col {
		col[b] = crc(1<<b) ^ c0
	}
	// Gaussian elimination of col * x = c0, over GF(2).
	var rows [16]uint32 // the bit of each equation in the low 16 bits, its right side above
	for i := range rows {
		var v uint32
		for b := range col {
			v |= uint32(col[b]>>i&1) << b
		}
		rows[i] = v | uint32(c0>>i&1)<<16
	}
	for b := range 16 {
		p := -1
		for i := b; i < 16; i++ {
			if rows[i]>>b&1 != 0 {
				p = i
				break
			}
		}
		if p < 0 {
			panic("crc1: singular")
		}
		rows[b], rows[p] = rows[p], rows[b]
		for i := range rows {
			if i != b && rows[i]>>b&1 != 0 {
				rows[i] ^= rows[b]
			}
		}
	}
	var x uint16
	for b := range 16 {
		x |= uint16(rows[b]>>16&1) << b
	}
	binary.BigEndian.PutUint16(f[2:], x)
	binary.BigEndian.PutUint16(f[len(f)-2:], crc16(0, f[2:len(f)-2]))
}

// decodeAll decodes the frames of an AC-3 file, one at a time, with a new
// Decoder, and counts the tools they use.
func decodeAll(t testing.TB, data []byte) ([]float32, *Decoder, toolUse) {
	t.Helper()
	d := NewDecoder()
	use := toolUse{}
	use.watch(d, nil)
	pcm := make([]float32, MaxChannels*FrameLength)
	var all []float32
	for i, f := range frames(t, data) {
		n, err := d.Decode(f, pcm)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		all = append(all, pcm[:MaxChannels*n]...)
	}
	return all, d, use
}

// difference is how a decode of a channel differs from ffmpeg's: its
// SNR, the level of the difference, and that of ffmpeg's decode, in dB of
// full scale.
type difference struct{ snr, noise, level float64 }

func (d difference) String() string {
	return fmt.Sprintf("%.1f dB (%.0f dBFS)", d.snr, d.noise)
}

// ok reports whether the decode is ffmpeg's but for the differences of
// their arithmetic: whose level is under -110 dBFS, and 100 dB or more
// under that of a channel of -30 dBFS or more.
func (d difference) ok() bool {
	return d.noise <= -110 && (d.level < -30 || d.snr >= 100)
}

// compare returns how ours, of Decode's six channels, differs from ref,
// ffmpeg's decode of a stream of acmod and lfeon: of each of ffmpeg's
// channels, and whether all are ok.
func compare(t testing.TB, ref, ours []float32, acmod int, lfeon bool) ([]difference, bool) {
	t.Helper()
	order := ffmpegOrder(acmod, lfeon)
	n := len(order)
	if len(ref)/n != len(ours)/MaxChannels {
		t.Fatalf("%d samples, ffmpeg's %d", len(ours)/MaxChannels, len(ref)/n)
	}
	diffs := make([]difference, n)
	ok := true
	samples := float64(len(ref) / n)
	for i, c := range order {
		var sig, noise float64
		for f := range len(ref) / n {
			want := float64(ref[f*n+i])
			var got float64
			if c < 0 {
				got = float64(ours[f*MaxChannels+LeftSurround]) / minus3dB
				if ours[f*MaxChannels+RightSurround] != ours[f*MaxChannels+LeftSurround] {
					t.Fatalf("sample %d: the surround outputs differ", f)
				}
			} else {
				got = float64(ours[f*MaxChannels+c])
			}
			sig += want * want
			noise += (got - want) * (got - want)
		}
		diffs[i] = difference{
			snr:   10 * math.Log10(sig/noise),
			noise: 10 * math.Log10(noise/samples),
			level: 10 * math.Log10(sig/samples),
		}
		ok = ok && diffs[i].ok()
	}
	return diffs, ok
}

// encoding is a stream to encode with ffmpeg's AC-3 encoder: the source of
// lavfi it encodes, its layout, and the encoder's arguments.
type encoding struct {
	name   string
	src    string
	layout string
	args   []string
}

// tone returns an aevalsrc expression of a tone of freq Hz at amp, which
// fades in and out, and has a click every period seconds, from offset.
func tone(amp float64, freq int, period, offset float64) string {
	return fmt.Sprintf("%g*sin(2*PI*%d*t)*min(1\\,4*t)+%g*lt(mod(t+%g\\,%g)\\,0.002)", amp, freq, amp/2, offset, period)
}

// encodings are the streams TestFFmpeg encodes, of every audio coding
// mode, sample rate and tool ffmpeg's encoder can make.
var encodings = func() []encoding {
	six := strings.Join([]string{tone(0.4, 440, 0.25, 0), tone(0.4, 550, 0.3, 0.05), tone(0.4, 660, 0.35, 0.1),
		"0.5*sin(2*PI*50*t)", tone(0.3, 770, 0.2, 0.15), tone(0.3, 880, 0.4, 0.2)}, "|")
	noise := "0.3*(random(0)*2-1)*gt(mod(t\\,0.25)\\,0.12)"
	return []encoding{
		{"5.1-48k-384", "aevalsrc=" + six + ":s=48000:d=0.5", "5.1", []string{"-b:a", "384k"}},
		{"5.1-48k-192-cpl", "aevalsrc=" + six + ":s=48000:d=0.5", "5.1", []string{"-b:a", "192k", "-channel_coupling", "1"}},
		{"5.1-44k-448", "aevalsrc=" + six + ":s=44100:d=0.5", "5.1", []string{"-b:a", "448k"}},
		{"5.1-32k-256", "aevalsrc=" + six + ":s=32000:d=0.5", "5.1", []string{"-b:a", "256k"}},
		{"5.1-48k-loro", "aevalsrc=" + six + ":s=48000:d=0.3", "5.1", []string{"-b:a", "384k", "-dmix_mode", "loro", "-loro_cmixlev", "0.841", "-loro_surmixlev", "0.5"}},
		{"5.0-48k-noise", "aevalsrc=" + strings.Join([]string{noise, tone(0.3, 300, 0.3, 0), noise, noise, tone(0.2, 900, 0.2, 0)}, "|") + ":s=48000:d=0.5", "5.0", []string{"-b:a", "256k"}},
		{"stereo-48k-remat", "aevalsrc=0.4*sin(2*PI*440*t)+0.05*(random(0)*2-1)|0.38*sin(2*PI*440*t)+0.05*(random(0)*2-1):s=48000:d=0.5", "stereo", []string{"-b:a", "192k"}},
		{"stereo-44k-cpl", "aevalsrc=" + tone(0.4, 500, 0.3, 0) + "+0.1*(random(0)*2-1)|" + tone(0.3, 700, 0.25, 0.1) + "+0.1*(random(0)*2-1):s=44100:d=0.5", "stereo", []string{"-b:a", "96k", "-channel_coupling", "1"}},
		{"stereo-32k-no-remat", "aevalsrc=" + tone(0.4, 500, 0.3, 0) + "|" + tone(0.3, 700, 0.25, 0.1) + ":s=32000:d=0.5", "stereo", []string{"-b:a", "128k", "-stereo_rematrixing", "0"}},
		{"mono-48k", "aevalsrc=" + tone(0.5, 330, 0.2, 0) + ":s=48000:d=0.5", "mono", []string{"-b:a", "96k"}},
		{"mono-32k-64", "aevalsrc=" + tone(0.5, 330, 0.2, 0) + "+0.2*(random(0)*2-1):s=32000:d=0.5", "mono", []string{"-b:a", "64k"}},
		{"3.0-48k", "aevalsrc=" + tone(0.4, 440, 0.3, 0) + "|" + tone(0.4, 550, 0.2, 0) + "|" + tone(0.4, 660, 0.25, 0) + ":s=48000:d=0.4", "3.0", []string{"-b:a", "192k"}},
		{"2.1-48k", "aevalsrc=" + tone(0.4, 440, 0.3, 0) + "|" + tone(0.4, 550, 0.2, 0) + "|" + tone(0.4, 990, 0.25, 0) + ":s=48000:d=0.4", "3.0(back)", []string{"-b:a", "192k"}},
		{"3.1-44k", "aevalsrc=" + tone(0.4, 440, 0.3, 0) + "|" + tone(0.4, 550, 0.2, 0) + "|" + tone(0.4, 660, 0.25, 0) + "|" + tone(0.3, 990, 0.35, 0) + ":s=44100:d=0.4", "4.0", []string{"-b:a", "256k"}},
		{"quad-48k", "aevalsrc=" + tone(0.4, 440, 0.3, 0) + "|" + tone(0.4, 550, 0.2, 0) + "|" + tone(0.3, 660, 0.25, 0) + "|" + tone(0.3, 990, 0.35, 0) + ":s=48000:d=0.4", "quad", []string{"-b:a", "256k"}},
		{"2.1-lfe-32k", "aevalsrc=" + tone(0.4, 440, 0.3, 0) + "|" + tone(0.4, 550, 0.2, 0) + "|0.5*sin(2*PI*40*t)|" + tone(0.3, 990, 0.35, 0) + ":s=32000:d=0.4", "FL+FR+LFE+BC", []string{"-b:a", "192k"}},
		{"mono-lfe-44k", "aevalsrc=" + tone(0.4, 440, 0.3, 0) + "|0.5*sin(2*PI*40*t):s=44100:d=0.4", "FC+LFE", []string{"-b:a", "128k"}},
		{"5.1-48k-quiet", "aevalsrc=" + strings.Join([]string{"0.001*sin(2*PI*440*t)", "0.0001*sin(2*PI*550*t)", "0", "0", "0.00001*(random(0)*2-1)", "0.5*sin(2*PI*3000*t)"}, "|") + ":s=48000:d=0.3", "5.1", []string{"-b:a", "320k"}},
		{"5.1-48k-loud", "aevalsrc=" + strings.Join([]string{"1.2*sin(2*PI*200*t)", "sin(2*PI*210*t)", "0.9*(random(0)*2-1)", "0.99*sin(2*PI*80*t)", "sgn(sin(2*PI*100*t))", "0.7*sin(2*PI*5000*t)"}, "|") + ":s=48000:d=0.3", "5.1", []string{"-b:a", "640k"}},
	}
}()

// encode encodes e into a raw AC-3 file in dir, and returns its path.
func (e encoding) encode(t testing.TB, dir string) string {
	return e.encodeAs(t, dir, "ac3")
}

// encodeAs encodes e with ffmpeg's encoder of codec, ac3 or eac3, into a
// raw file of it in dir, and returns its path.
func (e encoding) encodeAs(t testing.TB, dir, codec string) string {
	t.Helper()
	path := filepath.Join(dir, e.name+"."+codec)
	args := append([]string{"-y", "-f", "lavfi", "-i", e.src + ":c=" + e.layout, "-c:a", codec}, e.args...)
	ffmpeg(t, append(args, "-f", codec, path)...)
	return path
}

// TestFFmpeg encodes streams of every audio coding mode, sample rate and
// tool ffmpeg's encoder makes, and compares their decodes to ffmpeg's:
// with their dithflags cleared, the decodes are the same to float32's
// precision, and with them, they differ by the dither of mantissas of no
// bits.
func TestFFmpeg(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	all := toolUse{}
	for _, e := range encodings {
		t.Run(e.name, func(t *testing.T) {
			path := e.encode(t, dir)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			plain := filepath.Join(dir, e.name+"-undithered.ac3")
			if err := os.WriteFile(plain, undither(t, data), 0o644); err != nil {
				t.Fatal(err)
			}
			pdata, _ := os.ReadFile(plain)
			ours, d, use := decodeAll(t, pdata)
			diffs, ok := compare(t, ffmpegDecode(t, plain), ours, d.h.acmod, d.h.lfeon)
			t.Logf("acmod %d, LFE %v: %v; %v", d.h.acmod, d.h.lfeon, diffs, use)
			if !ok {
				t.Errorf("the decodes differ: %v", diffs)
			}
			// As encoded, the decodes differ by their dither.
			ours, _, use = decodeAll(t, data)
			diffs, _ = compare(t, ffmpegDecode(t, path), ours, d.h.acmod, d.h.lfeon)
			t.Logf("dithered: %v", diffs)
			for k, v := range use {
				all[k] += v
			}
		})
	}
	t.Logf("tools used: %v", all)
	var unused []string
	for _, k := range tools {
		if all[k] == 0 {
			unused = append(unused, k)
		}
	}
	sort.Strings(unused)
	t.Logf("tools ffmpeg's encoder never used: %v", unused)
}
