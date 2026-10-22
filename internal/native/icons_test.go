package native

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSettingsIconIsCentred checks that the settings gear draws the same
// mirrored across and up and down, as a gear centred on its grid does. It
// drew squashed above the centre, its hole running into its bottom tooth
// (#40).
func TestSettingsIconIsCentred(t *testing.T) {
	const n = 48
	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Size(n, n).Background(ui.RGB(0, 0, 0)).Children(func() {
			ui.Icon(c, icon("settings")).FontSize(n).TextColor(ui.RGB(255, 255, 255))
		})
	}, n, n, 1)
	off := 0
	for y := range n {
		for x := range n {
			ink := int(img.RGBAAt(x, y).R)
			for _, mirror := range []int{int(img.RGBAAt(n-1-x, y).R), int(img.RGBAAt(x, n-1-y).R)} {
				if d := ink - mirror; d > 64 || d < -64 {
					off++
				}
			}
		}
	}
	if off > n {
		t.Errorf("%d pixels of the settings icon differ from their mirror images", off)
	}
}
