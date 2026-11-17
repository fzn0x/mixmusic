package hct

import "testing"

func TestCoreGreySeeds(t *testing.T) {
	chromaOf := func(a ARGB) float64 { return FromARGB(a).Chroma }
	for _, seed := range []ARGB{0xff000000, 0xff0a0a0a, 0xff808080, 0xffffffff, 0xff101012, 0xff121010} {
		for _, dark := range []bool{true, false} {
			s := Core(seed).Scheme(dark)
			for name, c := range map[string]ARGB{"primary": s.Primary, "secondary": s.Secondary, "tertiary": s.Tertiary, "surface": s.Surface} {
				if ch := chromaOf(c); ch >= 3 {
					t.Errorf("seed %08x dark=%v: %s has chroma %.1f, want a grey", uint32(seed), dark, name, ch)
				}
			}
		}
	}
	a, b := Core(0xff0d0c0c).Scheme(true), Core(0xff0d0d0d).Scheme(true)
	if a.Primary != b.Primary {
		t.Errorf("near-identical greys give accents %08x and %08x", uint32(a.Primary), uint32(b.Primary))
	}
}

func TestCoreNearGreySeedGetsCalmPrimary(t *testing.T) {
	// #7a7a82 has HCT chroma between 5 and 12.
	seed := ARGB(0xff7a7a85)
	if c := FromARGB(seed).Chroma; c < greySeed || c >= calmSeed {
		t.Fatalf("test seed has chroma %.1f, want 5 to under 12", c)
	}
	p := Core(seed)
	if p.Primary.Chroma != calmSeed || p.Primary.Hue != FromARGB(seed).Hue {
		t.Errorf("primary = %+v, want hue of the seed, chroma 12", p.Primary)
	}
}

func TestCoreVividSeedUnchanged(t *testing.T) {
	seed := ARGB(0xff0000ff)
	if p := Core(seed); p.Primary.Chroma < 48 || p.Secondary.Chroma != 16 {
		t.Errorf("vivid seed changed: %+v", p)
	}
	// Chroma 12 or more keeps the strong primary.
	seed = 0xff6a6a90
	if c := FromARGB(seed).Chroma; c < calmSeed {
		t.Fatalf("seed chroma %.1f", c)
	}
	if p := Core(seed); p.Primary.Chroma < 48 {
		t.Errorf("chroma-12 seed got primary %.1f", p.Primary.Chroma)
	}
}
