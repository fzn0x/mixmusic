package native

import (
	"image/color"
	"testing"
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

func TestCSSLength(t *testing.T) {
	for in, want := range map[string]float32{
		"12px": 12, "0": 0, "0px": 0, " 4px ": 4, "0.75rem": 12, "1em": 16, "1.5": 1.5, "": 0, "-2px": 0, "big": 0,
	} {
		if got := cssLength(in); got != want {
			t.Errorf("cssLength(%q) = %v, want %v", in, got, want)
		}
	}
}
