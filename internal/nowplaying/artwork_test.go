package nowplaying

import "testing"

// readingPlatform is a fakePlatform that tells back a cover, as Now
// Playing does.
type readingPlatform struct {
	fakePlatform
	w, h float64
}

func (r *readingPlatform) heldArtwork() (float64, float64, bool) { return r.w, r.h, r.w > 0 }

// TestArtwork tells back the cover the system's controls hold, where they
// tell it, and says so where they don't.
func TestArtwork(t *testing.T) {
	c := newControls()
	c.start(&readingPlatform{w: 400, h: 400}, func(Request) {})
	defer c.Close()
	if w, h, ok, told := c.Artwork(); !told || !ok || w != 400 || h != 400 {
		t.Errorf("Artwork() = %v, %v, %v, %v", w, h, ok, told)
	}
	c2 := newControls()
	c2.start(&fakePlatform{}, func(Request) {})
	defer c2.Close()
	if _, _, ok, told := c2.Artwork(); told || ok {
		t.Error("controls that tell nothing back told a cover")
	}
}
