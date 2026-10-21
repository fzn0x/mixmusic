package native

import (
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestThemeEffectsOptions turns the effects of Greenroom, Glasshouse
// and Anime on and off with their options.
func TestThemeEffectsOptions(t *testing.T) {
	s := newTestShell(t)
	for _, c := range []struct {
		id     string
		chosen map[string]string
		want   effects
	}{
		{"greenroom", nil, effects{wash: true}},
		{"greenroom", map[string]string{"wash": "false"}, effects{}},
		{"glasshouse", nil, effects{cover: true, blur: 20}},
		{"glasshouse", map[string]string{"cover": "false", "blur": "0"}, effects{}},
		{"anime", nil, effects{speedlines: true, screentone: true}},
		{"anime", map[string]string{"speedlines": "false"}, effects{screentone: true}},
	} {
		found := false
		for _, th := range s.inv.Themes {
			if th.ID == c.id {
				found = true
				if got := themeEffects(th, c.chosen); got != c.want {
					t.Errorf("%s with %v: %+v, want %+v", c.id, c.chosen, got, c.want)
				}
			}
		}
		if !found {
			t.Errorf("no built-in theme %s", c.id)
		}
	}
}

// TestCoverTint gives the ground the fallback's colour without a cover,
// then changes it to the cover's over tintFade, at once while the computer
// asks for less motion, and never holds it halfway when the clock goes
// back.
func TestCoverTint(t *testing.T) {
	s := newTestShell(t)
	jade := ui.RGB(0x2b, 0xe0, 0x9c)
	tone := func(c color.NRGBA) ui.Color { return col(toned(c, washTone.lo, washTone.hi, washTone.sat)) }
	var got ui.Color
	var changing bool
	tt := ui.NewTester(func(c *ui.Context) { got, changing = s.coverTint(c, jade, washTone) }, 10, 10)
	tt.Frame()
	if want := tone(nrgba(jade)); got != want || changing {
		t.Fatalf("without a cover: %v, changing %v; want %v", got, changing, want)
	}
	s.artColor = "#d8572a"
	tt.Frame()
	if !changing || got != tone(nrgba(jade)) {
		t.Errorf("as the cover comes: %v, changing %v", got, changing)
	}
	tt.Advance(tintFade)
	tt.Frame()
	cover, _ := parseColor("#d8572a")
	if want := tone(cover); got != want || changing {
		t.Errorf("after %v: %v, changing %v; want %v", tintFade, got, changing, want)
	}

	// The clock goes back: the change ends there.
	s.artColor = ""
	tt.Frame()
	s.tint.at = s.tint.at.Add(time.Hour)
	tt.Frame()
	if got != tone(nrgba(jade)) || changing {
		t.Errorf("with the clock set back: %v, changing %v", got, changing)
	}

	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.artColor = "#d8572a"
	tt.Frame()
	if got != tone(cover) || changing {
		t.Errorf("with less motion: %v, changing %v", got, changing)
	}
}

// TestToned brings a cover's colour to a ground's lightness and up to its
// saturation, and leaves a gray gray.
func TestToned(t *testing.T) {
	for _, c := range []struct {
		in             color.NRGBA
		lo, hi, sat    float64
		grayIn, grayOK bool
	}{
		{color.NRGBA{12, 10, 30, 255}, 0.28, 0.46, 0.45, false, false},
		{color.NRGBA{250, 240, 200, 255}, 0.28, 0.46, 0.45, false, false},
		{color.NRGBA{180, 120, 110, 255}, 0.68, 0.82, 0.55, false, false},
		{color.NRGBA{128, 128, 128, 255}, 0.68, 0.82, 0.55, true, true},
	} {
		_, s, l := rgbToHSL(toned(c.in, c.lo, c.hi, c.sat))
		if l < c.lo-0.01 || l > c.hi+0.01 {
			t.Errorf("%v: lightness %.2f, not within %.2f to %.2f", c.in, l, c.lo, c.hi)
		}
		if c.grayIn && s > 0.01 {
			t.Errorf("%v: a gray took a hue: saturation %.2f", c.in, s)
		}
		if !c.grayIn && s < c.sat-0.01 {
			t.Errorf("%v: saturation %.2f, under %.2f", c.in, s, c.sat)
		}
	}
}

// TestFollowArtworkForGround asks for the colour of the cover where the
// accent follows it, or where the theme's ground takes it, and not
// otherwise.
func TestFollowArtworkForGround(t *testing.T) {
	s := newTestShell(t)
	tr := testTracks[0]
	tr.Artwork = []media.Image{{URL: "https://example.com/cover.jpg", Width: 544, Height: 544}}
	s.p.PlayTracks([]media.Track{tr}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.st = s.p.State()
	s.set.DynamicAccent = false
	s.lk.fx = effects{}
	s.followArtwork()
	if s.artURL != "" {
		t.Errorf("asked for the cover's colour with no use for it: %q", s.artURL)
	}
	for _, fx := range []effects{{wash: true}, {cover: true}} {
		s.artURL = ""
		s.lk.fx = fx
		s.followArtwork()
		if s.artURL == "" {
			t.Errorf("%+v didn't ask for the cover's colour", fx)
		}
	}
}

// TestBurst bursts Anime's speed lines as a track starts, and not again
// until another does, nor while the computer asks for less motion.
func TestBurst(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	var burst float32
	tt := ui.NewTester(func(c *ui.Context) {
		s.st = s.p.State()
		burst = s.burst(c)
	}, 10, 10)
	tt.Frame()
	if burst < 0.99 {
		t.Errorf("as the track starts, the lines burst %.2f", burst)
	}
	tt.Advance(burstFor)
	tt.Frame()
	if burst != 0 {
		t.Errorf("after %v, the lines still burst %.2f", burstFor, burst)
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	if burst < 0.99 {
		t.Errorf("the next track bursts them %.2f", burst)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 2 })
	tt.Frame()
	if burst != 0 {
		t.Errorf("with less motion, the lines burst %.2f", burst)
	}
}

// TestScreentone prints its dots largest in the bottom left corner, and
// none a width away from it.
func TestScreentone(t *testing.T) {
	img := screentone(200, 200, 1, color.NRGBA{A: 255})
	ink := func(x0, y0, x1, y1 int) (sum int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				sum += int(img.NRGBAAt(x, y).A)
			}
		}
		return sum
	}
	near, middle, far := ink(0, 170, 30, 200), ink(85, 85, 115, 115), ink(170, 0, 200, 30)
	if near == 0 || middle >= near || far != 0 {
		t.Errorf("ink in the corner %d, the middle %d, the far corner %d", near, middle, far)
	}
}
