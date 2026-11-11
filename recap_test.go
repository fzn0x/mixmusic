package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/sharecard"
)

// recapPlays makes the history of a user who played made-up songs in 2025
// and 2026, the latest play first.
func recapPlays() []library.Play {
	song := func(id, title, artist, album string, minutes int) media.Track {
		return media.Track{ID: id, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: artist}},
			Album: &media.Ref{ID: "al-" + album, Name: album}, DurationMs: minutes * 60000}
	}
	glass := song("yt:glass", "Glass Harbor", "North Pier", "Paper Lanterns", 4)
	moss := song("yt:moss", "Moss Garden", "Quiet Tide", "Slow Orchard", 3)
	kite := song("yt:kite", "Kite Weather", "Low Orchard", "Slow Orchard", 5)
	old := song("yt:old", "Old Ferry", "North Pier", "Paper Lanterns", 2)
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.Local) }
	return []library.Play{
		{Track: kite, PlayedAt: at(2026, 12, 1)},
		{Track: glass, PlayedAt: at(2026, 7, 1)},
		{Track: moss, PlayedAt: at(2026, 3, 6)},
		{Track: glass, PlayedAt: at(2026, 3, 5)},
		{Track: glass, PlayedAt: at(2026, 3, 5)},
		{Track: old, PlayedAt: at(2025, 11, 1)},
	}
}

// recapShell is a test shell whose history is recapPlays, on the day now.
func recapShell(t *testing.T, now time.Time) *shell {
	t.Helper()
	was := recapClock
	recapClock = func() time.Time { return now }
	t.Cleanup(func() { recapClock = was })
	return newTestShellOf(t, func(o *app.Options) {
		data, err := json.Marshal(library.Data{History: recapPlays()})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(o.DataDir, "library.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	})
}

// sizeName names a card's size in a file name.
func sizeName(s sharecard.Size) string { return fmt.Sprintf("%dx%d", s.W, s.H) }

// TestRecapYears offers Your year for the past years with history, and
// for the current year from December.
func TestRecapYears(t *testing.T) {
	for _, tc := range []struct {
		now  time.Time
		want []int
	}{
		{time.Date(2026, 11, 30, 12, 0, 0, 0, time.Local), []int{2025}},
		{time.Date(2026, 12, 1, 12, 0, 0, 0, time.Local), []int{2026, 2025}},
		{time.Date(2027, 1, 5, 12, 0, 0, 0, time.Local), []int{2026, 2025}},
		{time.Date(2025, 6, 1, 12, 0, 0, 0, time.Local), nil},
	} {
		s := recapShell(t, tc.now)
		got := s.recapYears()
		if fmt.Sprint(got) != fmt.Sprint(tc.want) && len(got)+len(tc.want) > 0 {
			t.Errorf("on %v: years %v, want %v", tc.now.Format(time.DateOnly), got, tc.want)
		}
	}
}

// TestRecapPage shows the recap under Your year, and no tab before there
// is a year to look back at.
func TestRecapPage(t *testing.T) {
	s := recapShell(t, time.Date(2026, 11, 20, 12, 0, 0, 0, time.Local))
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/stats")
	if !tt.HasText("your year") {
		t.Fatalf("no Your year tab for 2025: %q", tt.Texts())
	}
	click(t, s, tt, "your year")
	for _, want := range []string{"Old Ferry", "North Pier", "Paper Lanterns", "top songs", "top artists", "top albums"} {
		if !tt.HasText(want) {
			t.Errorf("2025's recap lacks %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("Glass Harbor") {
		t.Error("2025's recap shows a song of 2026")
	}

	// From December the year in course has its own, and the years switch.
	s = recapShell(t, time.Date(2026, 12, 2, 12, 0, 0, 0, time.Local))
	tt = testerOf(t, s, 1280, 1800)
	show(s, tt, "/stats")
	click(t, s, tt, "your year")
	for _, want := range []string{"Glass Harbor", "You listened most in March", "You found 2 new artists."} {
		if !tt.HasText(want) {
			t.Errorf("2026's recap lacks %q: %q", want, tt.Texts())
		}
	}
	click(t, s, tt, "2025")
	if !tt.HasText("Old Ferry") || tt.HasText("Glass Harbor") {
		t.Errorf("2025 isn't shown after choosing it: %q", tt.Texts())
	}

	// Before the history has a past year, the tab isn't there.
	s = recapShell(t, time.Date(2025, 6, 1, 12, 0, 0, 0, time.Local))
	tt = testerOf(t, s, 1280, 1800)
	show(s, tt, "/stats")
	if tt.HasText("your year") {
		t.Error("Your year shows with no year to look back at")
	}
}

// TestRecapSave saves the recap as a square and as a story, in the
// theme's colours.
func TestRecapSave(t *testing.T) {
	s := recapShell(t, time.Date(2026, 12, 2, 12, 0, 0, 0, time.Local))
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/stats")
	click(t, s, tt, "your year")
	dir := t.TempDir()
	for _, tc := range []struct {
		button string
		size   sharecard.Size
		name   string
	}{{"save image", sharecard.Square, "MixMusic 2026.png"}, {"save image for a story", sharecard.Story, "MixMusic 2026 story.png"}} {
		path := filepath.Join(dir, sizeName(tc.size)+".png")
		asked := savesTo(t, path)
		click(t, s, tt, tc.button)
		select {
		case opts := <-asked:
			if opts.DefaultPath != tc.name {
				t.Errorf("%s: the dialog proposes %q, want %q", tc.button, opts.DefaultPath, tc.name)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s opened no save dialog", tc.button)
		}
		img := decodedCard(t, s, path)
		if got := img.Bounds().Size(); got != image.Pt(tc.size.W, tc.size.H) {
			t.Errorf("%s: the card is %v", tc.button, got)
		}
		want := s.lk.cardColors().Background
		r, g, b, _ := img.At(tc.size.W-5, tc.size.H-5).RGBA()
		if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
			t.Errorf("%s: the card's ground isn't the theme's background %v", tc.button, want)
		}
	}
}

var (
	recapDark  = sharecard.Colors{Background: color.NRGBA{18, 18, 22, 255}, Text: color.NRGBA{245, 245, 247, 255}, Muted: color.NRGBA{160, 160, 170, 255}, Accent: color.NRGBA{0, 189, 227, 255}}
	recapLight = sharecard.Colors{Background: color.NRGBA{250, 247, 240, 255}, Text: color.NRGBA{28, 26, 24, 255}, Muted: color.NRGBA{110, 104, 98, 255}, Accent: color.NRGBA{200, 60, 20, 255}}
)

// recapLong is a recap whose titles are far too long for a card's width.
func recapLong() library.Recap {
	long := "The Extraordinarily Long Name Of A Song That Goes On And On Past Any Edge, Part Two"
	r := library.Recap{Year: 2026, Plays: 12345, Minutes: 98765}
	for i := range 5 {
		tr := media.Track{ID: "t" + count(i), Title: long, Artists: []media.Ref{{Name: "A Band With A Remarkably Long Name Indeed " + count(i)}}}
		r.TopTracks = append(r.TopTracks, library.TrackPlays{Track: tr, Plays: 50 - i})
		r.TopArtists = append(r.TopArtists, library.ArtistPlays{Artist: media.Ref{Name: long}, Plays: 50 - i})
		r.TopAlbums = append(r.TopAlbums, library.AlbumPlays{Album: media.Ref{Name: long}, Artist: long, Plays: 50 - i})
	}
	return r
}

// TestRecapCardMargins keeps all of a card's ink inside its margins, on
// both sizes, in a light and a dark theme, with short and long titles, and
// starts its text at the left margin. RECAP_OUT names a folder to save the
// cards in, to look at them.
func TestRecapCardMargins(t *testing.T) {
	s := newTestShell(t)
	fam := s.lk.cardFamily()
	short := library.Recap{Year: 2026, Plays: 321, Minutes: 1234}
	for _, p := range recapPlays()[:5] {
		short.TopTracks = append(short.TopTracks, library.TrackPlays{Track: p.Track, Plays: 3})
		short.TopArtists = append(short.TopArtists, library.ArtistPlays{Artist: p.Track.Artists[0], Plays: 3})
		short.TopAlbums = append(short.TopAlbums, library.AlbumPlays{Album: *p.Track.Album, Artist: p.Track.Artists[0].Name, Plays: 3})
	}
	out := os.Getenv("RECAP_OUT")
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, r := range map[string]library.Recap{"short": short, "long": recapLong()} {
		for cname, colors := range map[string]sharecard.Colors{"dark": recapDark, "light": recapLight} {
			for _, size := range []sharecard.Size{sharecard.Square, sharecard.Story} {
				img, err := recapCardOf(r, colors)(nil, size).Render(fam)
				if err != nil {
					t.Fatal(err)
				}
				ground, _ := sharecard.Card{Size: size, Colors: colors}.Render(fam)
				label := name + "-" + cname + "-" + sizeName(size)
				if out != "" {
					f, err := os.Create(filepath.Join(out, label+".png"))
					if err != nil {
						t.Fatal(err)
					}
					png.Encode(f, img)
					f.Close()
				}
				margin := int(float64(size.W)*0.09 + 0.5)
				left, top, right, bottom := size.W, size.H, 0, 0
				for y := range size.H {
					for x := range size.W {
						if img.NRGBAAt(x, y) != ground.NRGBAAt(x, y) {
							left, right = min(left, x), max(right, x)
							top, bottom = min(top, y), max(bottom, y)
						}
					}
				}
				if left < margin-2 || right > size.W-margin+2 || top < margin-2 || bottom > size.H-margin+2 {
					t.Errorf("%s: the ink spans x %d-%d, y %d-%d, outside the margin of %d", label, left, right, top, bottom, margin)
				}
				// The text starts at the left margin, and the foot ends
				// at the bottom one.
				if left > margin+12 {
					t.Errorf("%s: the ink starts at x %d, the margin is %d", label, left, margin)
				}
				if bottom < size.H-margin-30 {
					t.Errorf("%s: the ink ends at y %d, the margin is at %d", label, bottom, size.H-margin)
				}
			}
		}
	}
}
