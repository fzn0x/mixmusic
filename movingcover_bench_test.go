package main

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// largeGIF is a GIF of 600 pixels square, a mix's cover at its largest,
// of 60 frames of a tenth of a second each, each of them noise over all
// of it: 23 MB decoded, just under movingCoverMax.
func largeGIF(b testing.TB) []byte {
	b.Helper()
	pal := make(color.Palette, 256)
	for i := range pal {
		pal[i] = color.RGBA{uint8(i), uint8(255 - i), uint8(i * 7), 255}
	}
	g := &gif.GIF{Config: image.Config{Width: 600, Height: 600, ColorModel: pal}}
	seed := uint32(1)
	for range 60 {
		f := image.NewPaletted(image.Rect(0, 0, 600, 600), pal)
		for i := range f.Pix {
			seed = seed*1664525 + 1013904223
			f.Pix[i] = uint8(seed >> 24)
		}
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 10)
	}
	var out bytes.Buffer
	if err := gif.EncodeAll(&out, g); err != nil {
		b.Fatal(err)
	}
	return out.Bytes()
}

// BenchmarkMovingCover measures a large GIF cover: what decoding it takes
// beside the main thread, the memory it holds, and the main thread's time
// for each frame it shows, which it draws over the one before and hands
// its bitmap (gifCover.next), with what that comes to at its 10 frames a
// second. The painter's own drawing of the bitmap is the GPU's.
func BenchmarkMovingCover(b *testing.B) {
	path := filepath.Join(b.TempDir(), "large.gif")
	if err := os.WriteFile(path, largeGIF(b), 0o644); err != nil {
		b.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	dec, first, err := openGIF(path)
	if err != nil {
		b.Fatal(err)
	}
	decode := time.Since(start)
	g := &gifCover{dec: dec, bmp: ui.NewBitmap(first.Image), end: first.Duration}
	runtime.GC()
	runtime.ReadMemStats(&after)
	b.ResetTimer()
	for range b.N {
		g.next()
	}
	b.StopTimer()
	perFrame := b.Elapsed() / time.Duration(b.N)
	// Reported after the timer's reset, which drops metrics.
	b.ReportMetric(float64(decode.Milliseconds()), "decode-ms")
	b.ReportMetric(float64(after.HeapAlloc-before.HeapAlloc)/(1<<20), "held-MB")
	b.ReportMetric(100*float64(perFrame*10)/float64(time.Second), "cpu-%-at-10fps")
	runtime.KeepAlive(g)
}
