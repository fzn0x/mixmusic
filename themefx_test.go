package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestThemeEffectsOptions turns the effects of Greenroom, Glasshouse,
// Anime and Mochi on and off with their options; the Material themes make
// their colours of a seed, and Crate has none.
func TestThemeEffectsOptions(t *testing.T) {
	s := newTestShell(t)
	for _, c := range []struct {
		id     string
		chosen map[string]string
		want   effects
	}{
		{"greenroom", nil, effects{wash: true}},
		{"greenroom", map[string]string{"wash": "false"}, effects{}},
		{"glasshouse", nil, effects{cover: true, blur: 20, white: true}},
		{"glasshouse", map[string]string{"cover": "false", "blur": "0"}, effects{white: true}},
		{"anime", nil, effects{speedlines: true, screentone: true}},
		{"anime", map[string]string{"speedlines": "false"}, effects{screentone: true}},
		{"mochi", nil, effects{sparkles: true}},
		{"mochi", map[string]string{"sparkles": "false"}, effects{}},
		{"material", nil, effects{material: true}},
		{"material-dark", nil, effects{material: true}},
		{"crate", nil, effects{}},
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
	tt := ui.NewTester(func(c *ui.Context) {
		tn := s.coverTint(c, jade, washTone)
		got, changing = tn.color(c.Now()), tn.changing(c.Now())
	}, 10, 10)
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

// TestFollowArtworkAsksAgain asks again for the colour of a cover that
// couldn't load, a while after, and takes it once it loads; a cover whose
// address gives no image isn't asked for again so soon (#73).
func TestFollowArtworkAsksAgain(t *testing.T) {
	was := artColorRetry
	artColorRetry = 20 * time.Millisecond
	t.Cleanup(func() { artColorRetry = was })
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = []byte{0x20, 0x60, 0xe0, 0xff}[i%4]
	}
	var cover bytes.Buffer
	if err := png.Encode(&cover, img); err != nil {
		t.Fatal(err)
	}
	var busy atomic.Bool
	var asked, gone atomic.Int32
	busy.Store(true)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/gone.png":
			gone.Add(1)
			w.WriteHeader(http.StatusNotFound)
		case busy.Swap(false):
			asked.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			asked.Add(1)
			w.Write(cover.Bytes())
		}
	}))
	t.Cleanup(srv.Close)
	s := newTestShell(t)
	s.svc.System.Artwork().HTTP = srv.Client()
	s.set.DynamicAccent = true
	play := func(path string) {
		tr := testTracks[0]
		tr.ID = "yt:" + path
		tr.Artwork = []media.Image{{URL: srv.URL + path, Width: 544, Height: 544}}
		s.p.PlayTracks([]media.Track{tr}, 0)
		s.settle(t, func(st player.State) bool {
			cur, ok := st.Current()
			return ok && cur.Track.ID == tr.ID && st.Playing
		})
		s.st = s.p.State()
	}
	// frame asks for the colour as each frame does.
	frame := func() {
		s.followArtwork()
		time.Sleep(5 * time.Millisecond)
		s.drain()
	}

	play("/cover.png")
	for deadline := time.Now().Add(5 * time.Second); s.artColor == ""; frame() {
		if time.Now().After(deadline) {
			t.Fatalf("the accent didn't take the cover's colour once it loaded: asked %d times", asked.Load())
		}
	}
	if n := asked.Load(); n != 2 {
		t.Errorf("the cover was asked for %d times", n)
	}

	play("/gone.png")
	for end := time.Now().Add(10 * artColorRetry); time.Now().Before(end); {
		frame()
	}
	if n := gone.Load(); n != 1 || s.artColor != "" {
		t.Errorf("a cover that isn't there was asked for %d times, the accent %q", n, s.artColor)
	}
}

// TestBurst bursts Anime's speed lines as a track starts to play, not as
// the window opens on one paused, and not again until another does, nor
// while the computer asks for less motion.
func TestBurst(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	var b float32
	tt := ui.NewTester(func(c *ui.Context) {
		s.st = s.p.State()
		b = burst(s.burstAt(c), c.Now())
	}, 10, 10)
	tt.Frame()
	if b != 0 {
		t.Errorf("as the window opens on a paused track, the lines burst %.2f", b)
	}
	tt.Advance(time.Second)
	s.p.Play()
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	if b < 0.99 {
		t.Errorf("as the track starts to play, the lines burst %.2f", b)
	}
	tt.Advance(burstFor)
	tt.Frame()
	if b != 0 {
		t.Errorf("after %v, the lines still burst %.2f", burstFor, b)
	}
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	s.p.Play()
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	if b != 0 {
		t.Errorf("playing the same track again bursts the lines %.2f", b)
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	if b < 0.99 {
		t.Errorf("the next track bursts them %.2f", b)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 2 })
	tt.Frame()
	if b != 0 {
		t.Errorf("with less motion, the lines burst %.2f", b)
	}
}

// wear dresses s in the built-in theme id, with options.
func wear(s *shell, id string, options map[string]string) {
	s.set.FollowSystem, s.set.Theme = false, id
	s.set.ThemeOptions = map[string]map[string]string{id: options}
	s.restyle()
}

// TestGroundSettles moves the grounds of Greenroom, Glasshouse and Anime
// in the frames they ask for, which paint again without building the
// view, and has them ask for none once they are still.
func TestGroundSettles(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	for _, c := range []struct {
		theme string
		move  func()
		long  time.Duration
	}{
		{"greenroom", func() { s.artColor = "#d8572a" }, tintFade},
		{"glasshouse", func() { s.artColor = "#2a6fd8" }, tintFade},
		{"anime", func() {
			i := s.p.State().Index
			s.p.Next()
			s.settle(t, func(st player.State) bool { return st.Index == i+1 })
		}, burstFor},
	} {
		wear(s, c.theme, nil)
		s.artColor = ""
		tt := ui.NewTester(func(ctx *ui.Context) {
			s.st = s.p.State()
			s.ground(ctx, ui.Column(ctx).Fill())
		}, 200, 120)
		tt.Frame()
		tt.Advance(c.long + time.Second)
		if tt.FramePending() {
			t.Fatalf("%s: the ground still asks for frames before anything moves", c.theme)
		}
		c.move()
		tt.Frame()
		before := append([]byte(nil), tt.Image().Pix...)
		tt.Advance(c.long / 2)
		if bytes.Equal(before, tt.Image().Pix) {
			t.Errorf("%s: the ground stood still halfway through", c.theme)
		}
		tt.Advance(c.long)
		if tt.FramePending() {
			t.Errorf("%s: the ground still asks for frames %v after it moved", c.theme, c.long)
		}
	}
}

// TestSparklesRise has Mochi's sparkles rise while something plays, hold
// still where they are while nothing does, or while the computer asks for
// less motion, and go on from there.
func TestSparklesRise(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	wear(s, "mochi", nil)
	tt := ui.NewTester(func(ctx *ui.Context) {
		s.st = s.p.State()
		s.ground(ctx, ui.Column(ctx).Fill())
	}, 200, 160)
	frame := func() []byte { return append([]byte(nil), tt.Image().Pix...) }
	tt.Frame()
	start := frame()
	tt.Advance(time.Second)
	risen := frame()
	if bytes.Equal(start, risen) {
		t.Fatal("while something plays, the sparkles stood still")
	}
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	if !bytes.Equal(risen, frame()) {
		t.Error("pausing moved the sparkles")
	}
	tt.Advance(time.Second)
	if !bytes.Equal(risen, frame()) {
		t.Error("while nothing plays, the sparkles rose")
	}
	s.p.Play()
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	if !bytes.Equal(risen, frame()) {
		t.Error("playing again, the sparkles didn't go on from where they were")
	}
	tt.Advance(time.Second)
	if bytes.Equal(risen, frame()) {
		t.Error("playing again, the sparkles stood still")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	still := frame()
	tt.Advance(time.Second)
	if !bytes.Equal(still, frame()) {
		t.Error("with less motion, the sparkles rose")
	}
}

// TestDialogFace gives a dialog a face its ink reads on over a vivid
// tile: Glasshouse's white glass whatever its options, and, without a
// blur, a face that doesn't show the page through, though the theme's
// surface would.
func TestDialogFace(t *testing.T) {
	s := newTestShell(t)
	for _, c := range []struct {
		theme   string
		options map[string]string
	}{
		{"glasshouse", nil},
		{"glasshouse", map[string]string{"cover": "false"}},
		{"glasshouse", map[string]string{"blur": "0"}},
		{"glasshouse", map[string]string{"cover": "false", "blur": "0"}},
		{"aurora", map[string]string{"blur": "0"}},
		{"mix", nil},
	} {
		wear(s, c.theme, c.options)
		face := func(under ui.Color) color.NRGBA {
			tt := ui.NewTester(func(ctx *ui.Context) {
				ui.Column(ctx).Fill().Background(under).Children(func() {
					s.frosted(ui.Column(ctx)).Width(120).Height(80)
				})
			}, 200, 120)
			tt.Frame()
			return color.NRGBAModel.Convert(tt.Image().At(60, 40)).(color.NRGBA)
		}
		magenta, blue := face(ui.RGB(0xe1, 0x1d, 0x48)), face(ui.RGB(0x2a, 0x4b, 0xb8))
		for _, f := range []color.NRGBA{magenta, blue} {
			if r := contrast(nrgba(s.lk.text), f); r < 4.5 {
				t.Errorf("%s with %v: ink on the dialog's face %v reads at %.2f:1", c.theme, c.options, f, r)
			}
		}
		if s.lk.fx.blur == 0 && magenta != blue {
			t.Errorf("%s with %v: the dialog shows the page through: %v over one tile, %v over another", c.theme, c.options, magenta, blue)
		}
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
