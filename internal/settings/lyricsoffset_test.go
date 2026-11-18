package settings

import "testing"

// TestLyricsOffset moves no lyrics at first, nor in a settings.json of
// before, which has no key of it, and keeps a timing typed up to
// MaxLyricsOffset either way, as it was: past that, it stops there.
func TestLyricsOffset(t *testing.T) {
	if Default().LyricsOffset != 0 {
		t.Errorf("a new install moves lyrics by %d ms", Default().LyricsOffset)
	}
	before, err := Decode([]byte(`{"lyricsPlain": true}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := before.Clean().LyricsOffset; got != 0 {
		t.Errorf("a file of before moves lyrics by %d ms", got)
	}
	for _, c := range []struct{ in, want int }{
		{0, 0}, {350, 350}, {-123, -123}, {10_050, 10_050},
		{120_000, 120_000}, {-120_000, -120_000}, {120_001, 120_000}, {-500_000, -120_000},
	} {
		if got := (Settings{Lyrics: Lyrics{LyricsOffset: c.in}}).Clean().LyricsOffset; got != c.want {
			t.Errorf("%d ms cleans to %d, want %d", c.in, got, c.want)
		}
	}
}
