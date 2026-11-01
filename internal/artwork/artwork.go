// Package artwork downloads artwork, keeps it on disk, and picks its
// color, which themes use as the accent of what plays.
package artwork

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // artwork is mostly JPEG
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "golang.org/x/image/webp" // and sometimes WebP
)

// maxSize bounds the artwork downloaded, in bytes.
const maxSize = 8 << 20

// Client fetches artwork: from Store when it is set, which keeps it.
type Client struct {
	HTTP  *http.Client
	Store *Store
}

// New returns a client.
func New() *Client { return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}} }

// Color returns the accent color of the artwork at rawURL, as "#rrggbb".
func (c *Client) Color(ctx context.Context, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return "", errors.New("artwork: only https artwork is read")
	}
	if c.Store != nil {
		img, err := c.Store.Image(ctx, u.String())
		if err != nil {
			return "", err
		}
		return Hex(Pick(img)), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("artwork: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("artwork: HTTP %d", res.StatusCode)
	}
	img, _, err := image.Decode(io.LimitReader(res.Body, maxSize))
	if err != nil {
		return "", fmt.Errorf("artwork: %w", err)
	}
	return Hex(Pick(img)), nil
}

// Pick returns the color that best represents an image as an accent: the
// most common vivid hue, weighted by how vivid it is, or the image's
// average when nothing in it is vivid.
func Pick(img image.Image) color.RGBA {
	const buckets = 36 // hues of 10 degrees
	var (
		weight  [buckets]float64
		r, g, b [buckets]float64
		avg     [3]float64
		count   float64
		bounds  = img.Bounds()
		stepX   = max(bounds.Dx()/48, 1)
		stepY   = max(bounds.Dy()/48, 1)
	)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			fr, fg, fb := float64(cr>>8), float64(cg>>8), float64(cb>>8)
			avg[0], avg[1], avg[2] = avg[0]+fr, avg[1]+fg, avg[2]+fb
			count++
			h, s, v := hsv(fr, fg, fb)
			if s < 0.25 || v < 0.2 || v > 0.98 {
				continue // greys, near blacks and near whites make no accent
			}
			i := int(h/10) % buckets
			w := s * s * v
			weight[i] += w
			r[i], g[i], b[i] = r[i]+fr*w, g[i]+fg*w, b[i]+fb*w
		}
	}
	// Smooth over neighbouring hues, so one hue split in two still wins.
	best, bestWeight := -1, 0.0
	for i := range buckets {
		w := weight[i] + 0.5*(weight[(i+buckets-1)%buckets]+weight[(i+1)%buckets])
		if w > bestWeight {
			best, bestWeight = i, w
		}
	}
	if best < 0 || weight[best] == 0 || bestWeight < count*0.002 {
		if count == 0 {
			return color.RGBA{0x7b, 0x5c, 0xff, 0xff}
		}
		return color.RGBA{uint8(avg[0] / count), uint8(avg[1] / count), uint8(avg[2] / count), 0xff}
	}
	w := weight[best]
	return color.RGBA{uint8(math.Round(r[best] / w)), uint8(math.Round(g[best] / w)), uint8(math.Round(b[best] / w)), 0xff}
}

// hsv converts a color of 0-255 channels to its hue (degrees), saturation
// and value (0-1).
func hsv(r, g, b float64) (h, s, v float64) {
	r, g, b = r/255, g/255, b/255
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	v = mx
	d := mx - mn
	if mx == 0 || d == 0 {
		return 0, 0, v
	}
	s = d / mx
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// Hex formats a color as "#rrggbb".
func Hex(c color.RGBA) string {
	return strings.ToLower(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
}
