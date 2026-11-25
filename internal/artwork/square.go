package artwork

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

// Square returns the centre square of an encoded image, as the windows
// show a cover, at most px across, for what takes a picture as its bytes,
// such as a notification: a music video's 16:9 thumbnail gives its middle.
// It is a JPEG, or a PNG when it isn't opaque; a square JPEG or PNG no
// larger than px comes back as it is.
func Square(data []byte, px int) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("artwork: %w", err)
	}
	if cfg.Width == cfg.Height && cfg.Width <= px && (format == "jpeg" || format == "png") {
		return data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("artwork: %w", err)
	}
	return squareOf(img, px)
}

// squareOf encodes the centre square of img, at most px across, as Square
// does.
func squareOf(img image.Image, px int) ([]byte, error) {
	img = trimBars(img)
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	if side <= 0 || px <= 0 {
		return nil, errors.New("artwork: no picture to square")
	}
	at := b.Min.Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	crop := image.Rectangle{Min: at, Max: at.Add(image.Pt(side, side))}
	n := min(side, px)
	dst := image.NewRGBA(image.Rect(0, 0, n, n))
	if n == side {
		draw.Copy(dst, image.Point{}, img, crop, draw.Src, nil)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)
	}
	var out bytes.Buffer
	var err error
	if dst.Opaque() {
		err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(&out, dst)
	}
	if err != nil {
		return nil, fmt.Errorf("artwork: %w", err)
	}
	return out.Bytes(), nil
}

// barLevel is the value of each of a pixel's channels under which it counts
// as black: a still's bars are black to the eye, and a picture's darkest
// corner rarely is.
const barLevel = 16

// trimBars returns img without the black bars along one pair of its
// edges, as the 4:3 stills of 16:9 videos have above and below their
// picture: YouTube keeps such a still at the size of its frame, with the
// picture inside it. Bars on both pairs, or bars of uneven depth, are a
// picture's own dark edges rather than a frame, and come back as they are,
// as does an image whose bars are under 1% of the side or would leave
// under 40% of its area.
func trimBars(img image.Image) image.Image {
	b := img.Bounds()
	if b.Dx() < 8 || b.Dy() < 8 {
		return img
	}
	black := func(x, y int) bool {
		r, g, bl, _ := img.At(x, y).RGBA()
		return r>>8 < barLevel && g>>8 < barLevel && bl>>8 < barLevel
	}
	rows := func(y int) bool {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !black(x, y) {
				return false
			}
		}
		return true
	}
	cols := func(x int) bool {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if !black(x, y) {
				return false
			}
		}
		return true
	}
	top, bottom := b.Min.Y, b.Max.Y
	for top < bottom && rows(top) {
		top++
	}
	for bottom > top && rows(bottom-1) {
		bottom--
	}
	left, right := b.Min.X, b.Max.X
	for left < right && cols(left) {
		left++
	}
	for right > left && cols(right-1) {
		right--
	}
	up, down := top-b.Min.Y, b.Max.Y-bottom
	before, after := left-b.Min.X, b.Max.X-right
	crop := b
	switch {
	case up+down == 0 && before+after == 0:
		return img
	case up+down >= before+after:
		// A letterbox: the bars above and below are the same depth, as a
		// picture centred in its frame's.
		if abs(up-down) > 2 {
			return img
		}
		crop = image.Rect(b.Min.X, top, b.Max.X, bottom)
	default:
		if abs(before-after) > 2 {
			return img
		}
		crop = image.Rect(left, b.Min.Y, right, b.Max.Y)
	}
	// Bars under 1% of a side are a picture's own dark edge.
	if float64(crop.Dx()) >= 0.99*float64(b.Dx()) && float64(crop.Dy()) >= 0.99*float64(b.Dy()) {
		return img
	}
	// A trim that leaves little is a black picture, which is shown as it
	// is.
	if float64(crop.Dx())*float64(crop.Dy()) < 0.40*float64(b.Dx())*float64(b.Dy()) {
		return img
	}
	return cropped{img, crop}
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// cropped is a rectangle of an image, in its own coordinates.
type cropped struct {
	image.Image
	r image.Rectangle
}

func (c cropped) Bounds() image.Rectangle { return c.r }
