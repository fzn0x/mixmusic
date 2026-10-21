package native

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestProto draws Selawik and the icons, for a look at how MyGo renders
// them, when MIXMUSIC_RENDER names a folder.
func TestProto(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	registerFonts()
	bg, ink, accent := ui.Hex("#000000"), ui.Hex("#ffffff"), ui.Hex("#00bde3")
	view := func(c *ui.Context) {
		th := *ui.DarkTheme()
		th.Background, th.Text, th.Accent, th.Font = bg, ink, accent, face
		c.SetTheme(&th)
		ui.Column(c).Fill().Background(bg).Padding(24).Gap(16).Children(func() {
			ui.Text(c, "home").FontSize(96).FontWeight(300).LetterSpacing(-0.045 * 96).TextColor(ink)
			ui.Row(c).Gap(24).Children(func() {
				for _, w := range []int{300, 400, 600, 700} {
					ui.Textf(c, "Selawik %d", w).FontSize(22).FontWeight(w).TextColor(ink)
				}
				ui.Text(c, "Semilight").Font(faceSemilight).FontSize(22).TextColor(ink)
			})
			ui.Row(c).Gap(12).Children(func() {
				for _, n := range []string{"home", "search", "compass", "library", "stats", "store", "settings", "play", "pause", "shuffle", "repeat", "heart", "queue", "lyrics", "volume"} {
					ui.Box(c).Size(42, 42).Border(2, ink.Alpha(0.85)).Center().Children(func() {
						ui.Icon(c, icon(n)).FontSize(20).TextColor(ink)
					})
				}
			})
			ui.Row(c).Gap(12).Children(func() {
				ui.Box(c).Size(42, 42).Background(accent).Center().Children(func() {
					ui.Icon(c, icon("home")).FontSize(20).TextColor(bg)
				})
				ui.Text(c, "home").FontSize(17).Font(faceSemilight).TextColor(accent)
				ui.Text(c, "Ünïcødé — 日本語 — 3:28").FontSize(15).TextColor(ink)
			})
		})
	}
	img := ui.Render(view, 900, 320, 1)
	f, err := os.Create(filepath.Join(dir, "proto.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
