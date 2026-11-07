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
