package ac3

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writtenCase is a stream TestFFmpegWritten writes, of the blocks of an
// encoding of ffmpeg's E-AC-3 encoder, and the tool its decode must use.
type writtenCase struct {
	name   string
	source string // the name of the encoding of eac3Sources
	opts   writeOptions
	tool   string
}

// eac3Sources are the streams of ffmpeg's E-AC-3 encoder TestFFmpegWritten
// writes again: of coupling, and of none, and of white noise, whose bit
// allocation is alike in most bins.
var eac3Sources = func() []encoding {
	six := strings.Join([]string{tone(0.4, 440, 0.1, 0), tone(0.4, 550, 0.12, 0.02), tone(0.4, 660, 0.14, 0.04),
		"0.5*sin(2*PI*50*t)", tone(0.3, 770, 0.08, 0.06), tone(0.3, 880, 0.16, 0.03)}, "|")
	noisy := "+0.1*(random(0)*2-1)"
	return []encoding{
		{"5.1-cpl", "aevalsrc=" + six + ":s=48000:d=0.25", "5.1", []string{"-b:a", "256k", "-channel_coupling", "1"}},
		{"5.1", "aevalsrc=" + strings.ReplaceAll(six, "|", noisy+"|") + noisy + ":s=48000:d=0.25", "5.1", []string{"-b:a", "384k", "-channel_coupling", "0"}},
		{"stereo", "aevalsrc=0.4*sin(2*PI*440*t)" + noisy + "|0.38*sin(2*PI*440*t)" + noisy + ":s=44100:d=0.25", "stereo", []string{"-b:a", "192k", "-channel_coupling", "0"}},
		{"mono", "aevalsrc=" + tone(0.5, 330, 0.1, 0) + noisy + ":s=32000:d=0.25", "mono", []string{"-b:a", "96k"}},
		{"noise", "aevalsrc=0.3*(random(0)*2-1):s=48000:d=0.25", "mono", []string{"-b:a", "192k"}},
	}
}()

// spx is the spectral extension of the full-bandwidth channels of
// writeOptions o, of sub-bands begf to endf, copied from strtf, in bands
// of bnd sub-bands, of the notch filter of atten, or none of -1. Its
// blending, of spxblnd 31 and bands of a midpoint below 31/32 of the
// extension's end, has no noise.
func spx(o writeOptions, strtf, begf, endf, bnd, atten int) writeOptions {
	o.spx, o.spxstrtf, o.spxbegf, o.spxendf, o.spxbnd, o.spxblnd, o.atten = true, strtf, begf, endf, bnd, 31, atten
	return o
}

var writtenCases = []writtenCase{
	{"rewritten", "5.1-cpl", writeOptions{numblks: 6}, "cpl"},
	{"1 block", "5.1-cpl", writeOptions{numblks: 1}, "cpl"},
	{"2 blocks", "stereo", writeOptions{numblks: 2}, "remat"},
	{"3 blocks", "5.1", writeOptions{numblks: 3}, "d15"},
	{"bit allocation parameters", "5.1-cpl", writeOptions{numblks: 6, bamode: true}, "cpl"},
	{"fast gain codes", "5.1-cpl", writeOptions{numblks: 6, fgain: 2}, "cpl"},
	{"delta and skip", "5.1", writeOptions{numblks: 3, delta: []segment{{10, 3, 6}, {5, 2, 2}, {4, 5, 7}}, skip: true}, "delta"},
	{"short blocks", "5.1-cpl", writeOptions{numblks: 6, blksw: true}, "short"},
	{"transient pre-noise", "mono", writeOptions{numblks: 6, transproc: true}, "d15"},
	{"spx", "5.1", spx(writeOptions{numblks: 6}, 1, 3, 5, 3, -1), "spx"},
	{"spx to the end", "5.1", spx(writeOptions{numblks: 6}, 1, 3, 7, 4, -1), "spx"},
	{"spx of one band", "5.1", spx(writeOptions{numblks: 1}, 1, 7, 7, 6, -1), "spx"},
	{"spx notch", "5.1", spx(writeOptions{numblks: 6}, 0, 6, 4, 2, 7), "spx"},
	{"spx notches where it wraps", "5.1", spx(writeOptions{numblks: 3}, 0, 2, 6, 3, 20), "spx"},
	{"spx rematrixed", "stereo", spx(writeOptions{numblks: 3}, 0, 1, 4, 4, 12), "spx"},
	{"spx mono", "mono", spx(writeOptions{numblks: 2}, 2, 6, 6, 2, -1), "spx"},
	// AHT, of bit allocations of no hebap 0, whose mantissas ffmpeg's
	// decoder dithers whatever the dithflag, nor hebap 4, whose vectors it
	// takes one row off Table E.3.4.
	{"aht vq 1 3 5 7", "noise", writeOptions{numblks: 6, aht: true, csnr: 10, flatExp: 4}, "aht"},
	{"aht vq 5 7", "noise", writeOptions{numblks: 6, aht: true, csnr: 12, flatExp: 4, gaq: 3}, "aht"},
	{"aht vq 6", "noise", writeOptions{numblks: 6, aht: true, csnr: 18}, "aht"},
	{"aht gaq", "noise", writeOptions{numblks: 6, aht: true, csnr: 20}, "aht"},
	{"aht gains of 2", "noise", writeOptions{numblks: 6, aht: true, csnr: 20, gaq: 1}, "aht"},
	{"aht gains of 4", "noise", writeOptions{numblks: 6, aht: true, csnr: 24, gaq: 2}, "aht"},
	{"aht gains of 1, 2 and 4", "noise", writeOptions{numblks: 6, aht: true, csnr: 28, gaq: 3}, "aht"},
	{"aht 12 to 16 bits", "noise", writeOptions{numblks: 6, aht: true, csnr: 36}, "aht"},
}

// TestFFmpegWritten writes the blocks of streams of ffmpeg's E-AC-3
// encoder again, of the syntax and the tools the encoder doesn't use, and
// checks the decodes of the streams written against ffmpeg's. Spectral
// extension scales the high frequencies up to 28 times (29 dB), and the
// difference of the decodes with them: it is checked to 95 dB, and -100
// dBFS.
func TestFFmpegWritten(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	sources := map[string]*capturedStream{}
	for _, e := range eac3Sources {
		data, err := os.ReadFile(e.encodeAs(t, dir, "eac3"))
		if err != nil {
			t.Fatal(err)
		}
		sources[e.name] = capture(t, undither(t, data))
	}
	for _, c := range writtenCases {
		t.Run(c.name, func(t *testing.T) {
			data := writeEAC3(t, sources[c.source], c.opts)
			path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".eac3")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			checkEAC3(t, path, data, c.tool)
		})
	}
}

// checkEAC3 decodes an E-AC-3 file with ffmpeg and with the decoder, and
// checks that they agree, and that its blocks use tool.
func checkEAC3(t *testing.T, path string, data []byte, tool string) {
	t.Helper()
	d := NewDecoder()
	use := toolUse{}
	use.watch(d, nil)
	var hebaps [20]int // of the channels of AHT
	watch := d.onBlock
	d.onBlock = func(blk int) {
		watch(blk)
		for c := range d.h.nfchans {
			if ch := &d.ch[c]; ch.aht && blk == 0 {
				for _, b := range ch.bap[ch.start:ch.end] {
					hebaps[b]++
				}
			}
		}
	}
	pcm := make([]float32, MaxChannels*FrameLength)
	var ours []float32
	for i, f := range frames(t, data) {
		n, err := d.Decode(f, pcm)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		ours = append(ours, pcm[:MaxChannels*n]...)
	}
	if use[tool] == 0 {
		t.Fatalf("no block uses %s: %v", tool, use)
	}
	if tool == "aht" {
		t.Logf("hebaps %v", hebaps)
		if hebaps[0] != 0 || hebaps[4] != 0 {
			t.Fatalf("hebaps 0 or 4: %v", hebaps)
		}
	}
	diffs, ok := compare(t, ffmpegDecode(t, path), ours, d.h.acmod, d.h.lfeon)
	t.Logf("%d blocks: %v; %v", d.h.numblks, diffs, use)
	if tool == "spx" {
		ok = true
		for _, d := range diffs {
			ok = ok && d.noise <= -100 && (d.level < -30 || d.snr >= 95)
		}
	}
	if !ok {
		t.Errorf("the decodes differ: %v", diffs)
	}
}
