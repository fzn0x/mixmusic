package settings

import (
	"slices"
	"testing"
)

// TestLyricSizeSetting starts a new install at full size, keeps the sizes
// the settings allow, rounded to their step, and puts the others back to
// full size.
func TestLyricSizeSetting(t *testing.T) {
	if got := Default().LyricsSize; got != 1 {
		t.Errorf("a new install's lyrics size is %v, not 1", got)
	}
	for _, c := range []struct{ in, want float64 }{
		{MinLyricsSize, MinLyricsSize}, {MaxLyricsSize, MaxLyricsSize}, {1.3, 1.3}, {1.34, 1.3}, {1.36, 1.4},
		{0, 1}, {0.2, 1}, {2.5, 1}, {-1, 1},
	} {
		s := Default()
		s.LyricsSize = c.in
		if got := s.Clean().LyricsSize; got != c.want {
			t.Errorf("Clean makes the lyrics size %v %v, not %v", c.in, got, c.want)
		}
	}
}

// TestLyricBallMotionSetting starts a new install with the motion that mixes
// them, keeps the motions the settings know, and puts the others back to the
// first.
func TestLyricBallMotionSetting(t *testing.T) {
	if got := Default().LyricsBallMotion; got != LyricBallMotions[0] || got != LyricBallMix {
		t.Errorf("a new install's ball motion is %q, not the first, %q, which mixes them", got, LyricBallMotions[0])
	}
	for _, m := range LyricBallMotions {
		s := Default()
		s.LyricsBallMotion = m
		if got := s.Clean().LyricsBallMotion; got != m {
			t.Errorf("Clean changes the ball motion %q to %q", m, got)
		}
	}
	for _, bad := range []LyricBallMotion{"", "Hop", "spin", "hop ", "mix\n"} {
		s := Default()
		s.LyricsBallMotion = bad
		if got := s.Clean().LyricsBallMotion; got != LyricBallMotions[0] {
			t.Errorf("Clean makes the ball motion %q %q, not the first", bad, got)
		}
	}
	if len(slices.Compact(slices.Sorted(slices.Values(LyricBallMotions)))) != len(LyricBallMotions) {
		t.Error("the ball motions repeat")
	}
}

// TestLyricStyleSetting starts a new install in the first style, keeps a
// style the settings know, and puts a style they don't back to the first.
func TestLyricStyleSetting(t *testing.T) {
	if got := Default().LyricsStyle; got != LyricStyles[0] || got != LyricStyleSpotlight {
		t.Errorf("a new install's style is %q, not the first, %q, which is Spotlight", got, LyricStyles[0])
	}
	if !Default().LyricsBall {
		t.Error("a new install has the ball off")
	}
	if len(LyricStyles) < 2 {
		t.Fatal("the arrows have no style to go to")
	}
	for _, id := range LyricStyles {
		s := Default()
		s.LyricsStyle = id
		if got := s.Clean().LyricsStyle; got != id {
			t.Errorf("Clean changes the style %q to %q", id, got)
		}
	}
	for _, bad := range []LyricStyle{"", "Glow", "nonesuch", "glow ", "halo"} {
		s := Default()
		s.LyricsStyle = bad
		if got := s.Clean().LyricsStyle; got != LyricStyles[0] {
			t.Errorf("Clean makes the style %q %q, not the first", bad, got)
		}
	}
	if len(slices.Compact(slices.Sorted(slices.Values(LyricStyles)))) != len(LyricStyles) {
		t.Error("the styles repeat")
	}
}
