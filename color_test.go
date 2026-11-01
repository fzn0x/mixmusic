package main

import (
	"image/color"
	"testing"

	"github.com/fzn0x/mixmusic/internal/colorfile"
)

func TestParseColor(t *testing.T) {
	for in, want := range map[string]color.NRGBA{
		"#000":                        {0, 0, 0, 255},
		"#fff8":                       {255, 255, 255, 0x88},
		"#00bde3":                     {0x00, 0xbd, 0xe3, 255},
		"#FF5A4F80":                   {0xff, 0x5a, 0x4f, 0x80},
		"transparent":                 {},
		"rgba(255, 255, 255, 0.16)":   {255, 255, 255, 41},
		"rgb(0 128 255)":              {0, 128, 255, 255},
		"rgb(0 128 255 / 50%)":        {0, 128, 255, 128},
		"rgb(100%, 0%, 0%)":           {255, 0, 0, 255},
		"hsl(0 100% 50%)":             {255, 0, 0, 255},
		"hsl(120, 100%, 25%)":         {0, 128, 0, 255},
		"hsla(240deg 100% 50% / 0.5)": {0, 0, 255, 128},
		"hsl(0.5turn 100% 50%)":       {0, 255, 255, 255},
		"hsl(200grad 100% 50%)":       {0, 255, 255, 255},
		// White and black, and Oklab's red, as CSS converts them.
		"oklch(1 0 0)":                    {255, 255, 255, 255},
		"oklch(0% 0 0)":                   {0, 0, 0, 255},
		"oklab(0.628 0.2249 0.1258)":      {255, 0, 0, 255},
		"oklch(62.8% 0.2577 29.23 / 25%)": {255, 0, 0, 64},
	} {
		got, ok := parseColor(in)
		if !ok {
			t.Errorf("%q didn't parse", in)
			continue
		}
		if !near(got, want) {
			t.Errorf("%q is %v, want %v", in, got, want)
		}
	}
	for _, bad := range []string{"", "#12", "#12345", "red", "rgb(1, 2)", "hsl(x 1% 1%)", "rgb(1 2 3 / x)", "url(x)"} {
		if _, ok := parseColor(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

// near tells colours apart by more than rounding.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return int(x)-int(y) <= 1 && int(y)-int(x) <= 1 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}

func TestMix(t *testing.T) {
	black, white := color.NRGBA{A: 255}, color.NRGBA{255, 255, 255, 255}
	if got := mix(black, white, 0.07); got != (color.NRGBA{18, 18, 18, 255}) {
		t.Errorf("7%% of white into black: %v", got)
	}
	if got := over(alpha(white, 0.045), black); got != (color.NRGBA{11, 11, 11, 255}) {
		t.Errorf("a 4.5%% white surface over black: %v", got)
	}
	if inkOn(color.NRGBA{0x00, 0xbd, 0xe3, 255}) != black || inkOn(black) != white {
		t.Error("ink on cyan should be black, on black white")
	}
}

func TestReadableOn(t *testing.T) {
	black, white := color.NRGBA{A: 255}, color.NRGBA{255, 255, 255, 255}
	navy := color.NRGBA{0x1a, 0x1f, 0x4d, 255}
	for _, c := range []struct {
		accent, ground color.NRGBA
	}{{navy, black}, {color.NRGBA{0xff, 0xf0, 0x60, 255}, white}, {black, black}, {white, white}} {
		got := readableOn(c.accent, c.ground)
		if contrast(got, c.ground) < 3 {
			t.Errorf("%v on %v gave %v, at %.2f:1", c.accent, c.ground, got, contrast(got, c.ground))
		}
	}
	// An accent that reads already stays.
	cyan := color.NRGBA{0x00, 0xbd, 0xe3, 255}
	if got := readableOn(cyan, black); got != cyan {
		t.Errorf("cyan on black became %v", got)
	}
}

// TestReadableAt reaches its ratio on any ground: on grounds of a middle
// tone too, where white reads less than black, and from a colour on
// either side of the ground.
func TestReadableAt(t *testing.T) {
	black, white := color.NRGBA{A: 255}, color.NRGBA{255, 255, 255, 255}
	grey77, grey95 := color.NRGBA{0x77, 0x77, 0x77, 255}, color.NRGBA{0x95, 0x95, 0x95, 255}
	coral := color.NRGBA{0xd0, 0x60, 0x60, 255}
	for _, c := range []struct {
		c, ground color.NRGBA
		ratio     float64
	}{
		{coral, grey95, 3}, {coral, grey77, 4.5}, {white, grey77, 4.5}, {black, grey77, 4.5},
		{color.NRGBA{0xff, 0xf0, 0x60, 255}, color.NRGBA{0x40, 0x40, 0x40, 255}, 4.5},
		{color.NRGBA{0x30, 0x30, 0x80, 255}, color.NRGBA{0x20, 0x20, 0x20, 255}, 4.5},
		{grey95, grey95, 4.5}, {white, white, 4.5}, {black, black, 4.5},
		// A translucent colour reads as it shows on the ground.
		{color.NRGBA{255, 255, 255, 40}, black, 4.5},
	} {
		got := readableAt(c.c, c.ground, c.ratio)
		if got.A != 255 || contrast(got, c.ground) < c.ratio {
			t.Errorf("%v on %v at %.1f:1 gave %v, at %.2f:1", c.c, c.ground, c.ratio, got, contrast(got, c.ground))
		}
	}
	// A light colour on a mid-tone ground that white reads on stays light.
	if got := readableAt(color.NRGBA{0xf0, 0xe0, 0x90, 255}, color.NRGBA{0x60, 0x60, 0x60, 255}, 3); luminance(got) < 0.5 {
		t.Errorf("a light yellow on grey went dark: %v", got)
	}
}

// TestReadableOnThemes gives the accents the same colours on the grounds of
// the built-in themes as readableOn did before it reached any ground.
func TestReadableOnThemes(t *testing.T) {
	before := func(accent, ground color.NRGBA) color.NRGBA {
		toward := color.NRGBA{A: 255}
		if luminance(ground) < 0.5 {
			toward = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		c := accent
		c.A = 255
		for i := 0; i < 10 && contrast(c, ground) < 3; i++ {
			c = mix(c, toward, 0.15)
		}
		return c
	}
	s := newTestShell(t)
	for _, th := range s.inv.Themes {
		bg, _ := parseColor(th.Colors["bg"])
		bg.A = 255
		for r := 0; r < 256; r += 17 {
			for g := 0; g < 256; g += 51 {
				for b := 0; b < 256; b += 51 {
					a := color.NRGBA{uint8(r), uint8(g), uint8(b), 255}
					if got, want := readableOn(a, bg), before(a, bg); got != want {
						t.Fatalf("%s: %v became %v, not %v", th.ID, a, got, want)
					}
				}
			}
		}
	}
}

// TestRaised gives every built-in theme a solid face for the menus that
// float over the page: its surface-2 where that's solid, and Aurora's glass
// over its ground.
func TestRaised(t *testing.T) {
	s := newTestShell(t)
	n := 0
	for _, th := range s.inv.Themes {
		if !th.Builtin {
			continue
		}
		n++
		set := s.set
		set.FollowSystem, set.Theme = false, th.ID
		k := newLook(set, s.inv.Themes, true, "", colorfile.Colors{})
		if k.raised.A != 255 {
			t.Errorf("%s: the menus' face %+v shows the page through", th.ID, k.raised)
		}
		if c, ok := parseColor(th.Colors["surface-2"]); ok && c.A == 255 && k.raised != col(c) {
			t.Errorf("%s: the menus' face is %+v, not surface-2's %+v", th.ID, k.raised, c)
		}
	}
	if n == 0 {
		t.Fatal("no built-in themes")
	}
}

func TestCSSLength(t *testing.T) {
	for in, want := range map[string]float32{
		"12px": 12, "0": 0, "0px": 0, " 4px ": 4, "0.75rem": 12, "1em": 16, "1.5": 1.5, "": 0, "-2px": 0, "big": 0,
	} {
		if got := cssLength(in); got != want {
			t.Errorf("cssLength(%q) = %v, want %v", in, got, want)
		}
	}
}
