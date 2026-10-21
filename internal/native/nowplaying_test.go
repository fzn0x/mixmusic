package native

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestVisualizerOfSilence draws nothing while MixMusic hears nothing, as
// of a Spotify device elsewhere: no flat line of bars, which would look
// broken.
func TestVisualizerOfSilence(t *testing.T) {
	s := newTestShell(t)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(240, 240).Background(ui.RGB(0, 0, 0)).Children(func() { s.visualizer(c, 240) })
	}, 240, 240)
	tt.Frame()
	img := tt.Image()
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if r, g, b, _ := img.At(x, y).RGBA(); r|g|b != 0 {
				t.Fatalf("silence drew at (%d, %d)", x, y)
			}
		}
	}
}
