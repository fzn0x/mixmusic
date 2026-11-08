// Command discordbadges draws the badges the Discord status can show at the
// corner of the cover (#352): brand/discord's PNGs, which Discord reads
// from GitHub. Each is a mark of MixMusic's own on a dark ground, as the
// app tells the services apart beside their tracks: a disc for Spotify, a
// square for YouTube Music, a record for a file on the computer, and two
// bars for a paused track. Discord shows them round, so each mark keeps to
// the middle.
//
//	go run ./tools/discordbadges
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

// size is the badges' width and height, in pixels: Discord shows them
// small, at twice that on the densest screens.
const size = 256

// samples is how many samples a pixel takes across and down, for smooth
// edges.
const samples = 4

// The colours of the badges: a dark grey ground, which shows on Discord's
// dark cards and light ones, and the marks, the services' in the Mix
// theme's colours.
var (
	ground  = color.NRGBA{0x30, 0x30, 0x30, 0xff}
	spotify = color.NRGBA{0x1e, 0xd7, 0x60, 0xff}
	youtube = color.NRGBA{0xff, 0x40, 0x40, 0xff}
	white   = color.NRGBA{0xff, 0xff, 0xff, 0xff}
)

// A shape tells whether a point of the badge, from (0, 0) at its top left
// to (1, 1), is in its mark.
type shape func(x, y float64) bool

// badges are the badges, by their files' names, and their marks.
var badges = map[string]struct {
	ink  color.NRGBA
	mark shape
}{
	"spotify":       {spotify, disc(0.5, 0.5, 0.25)},
	"youtube-music": {youtube, rect(0.29, 0.29, 0.71, 0.71)},
	"computer": {white, func(x, y float64) bool {
		r := math.Hypot(x-0.5, y-0.5)
		return (r >= 0.19 && r <= 0.28) || r <= 0.06
	}},
	"paused": {white, func(x, y float64) bool {
		return rect(0.35, 0.29, 0.45, 0.71)(x, y) || rect(0.55, 0.29, 0.65, 0.71)(x, y)
	}},
}

// disc is the disc of radius r around (cx, cy).
func disc(cx, cy, r float64) shape {
	return func(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= r }
}

// rect is the rectangle from (x0, y0) to (x1, y1).
func rect(x0, y0, x1, y1 float64) shape {
	return func(x, y float64) bool { return x >= x0 && x <= x1 && y >= y0 && y <= y1 }
}

func main() {
	out := flag.String("o", filepath.Join("brand", "discord"), "the folder to write the badges in")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for name, b := range badges {
		if err := write(filepath.Join(*out, name+".png"), draw(b.mark, b.ink)); err != nil {
			log.Fatal(err)
		}
	}
}

// draw draws mark in ink on the ground.
func draw(mark shape, ink color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := range size {
		for px := range size {
			in := 0
			for sy := range samples {
				for sx := range samples {
					x := (float64(px) + (float64(sx)+0.5)/samples) / size
					y := (float64(py) + (float64(sy)+0.5)/samples) / size
					if mark(x, y) {
						in++
					}
				}
			}
			img.SetNRGBA(px, py, mix(ground, ink, float64(in)/(samples*samples)))
		}
	}
	return img
}

// mix is a from a to b, by f from 0 to 1.
func mix(a, b color.NRGBA, f float64) color.NRGBA {
	at := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*f)) }
	return color.NRGBA{at(a.R, b.R), at(a.G, b.G), at(a.B, b.B), 0xff}
}

// write writes img to path as a PNG.
func write(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
