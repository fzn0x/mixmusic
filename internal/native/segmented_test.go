package native

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSegmentLabelsWhole keeps a segment's label whole: in Mochi's face,
// "best" laid out a hair wider than the box measured for it, and its
// segment showed "be…". The ink of the label in its segment, on the
// settings, is that of the label drawn whole, as text that can't be cut.
func TestSegmentLabelsWhole(t *testing.T) {
	s := newTestShell(t)
	wear(s, "mochi", nil)
	k := &s.lk
	ink := func(img *image.RGBA, r ui.Rect) int {
		n := 0
		want := nrgba(k.accentText)
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				c := img.RGBAAt(x, y)
				d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
				if d(c.R, want.R)+d(c.G, want.G)+d(c.B, want.B) < 120 {
					n++
				}
			}
		}
		return n
	}
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/playback")
	r, ok := tt.Find("best")
	if !ok {
		t.Fatalf("no segment best: %q", tt.Texts())
	}
	inSegment := ink(tt.Image(), r)
	whole := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(&k.theme)
		ui.Row(c).Padding(20).Children(func() {
			ui.Box(c).Background(k.accent).PaddingX(40).Height(34).Center().Label("best").Children(func() {
				ui.Text(c, "best").FontSize(k.smallSize()).TextColor(k.accentText)
			})
		})
	}, 400, 120)
	whole.Frame()
	rw, _ := whole.Find("best")
	want := ink(whole.Image(), rw)
	if inSegment < want*9/10 {
		t.Errorf("the segment cuts its label: %d pixels of ink, of %d", inSegment, want)
	}
}
