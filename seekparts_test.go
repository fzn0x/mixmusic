package main

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// SetSkips has the test engine take the parts to pass over, as the engine
// does, so that the player marks them.
func (s *testStream) SetSkips(string, []audio.Skip) {}

// markedParts are SponsorBlock's parts of every stream of markedPlayer: a
// sponsor read from 20 s to 60 s, and the highlight at 2:30.
var markedParts = []sponsorblock.Segment{
	{Kind: sponsorblock.Sponsor, From: 20 * time.Second, To: 60 * time.Second},
	{Kind: sponsorblock.Highlight, From: 150 * time.Second, To: 150 * time.Second},
}

// markedPlayer is the test app knowing SponsorBlock's parts of every
// stream, while its settings skip them.
type markedPlayer struct{ testPlayer }

func (markedPlayer) Settings() settings.Settings {
	s := settings.Default()
	s.SkipParts.On = true
	return s
}

func (markedPlayer) Marks(context.Context, player.Source) ([]sponsorblock.Segment, error) {
	return markedParts, nil
}

func (markedPlayer) KnownMarks(player.Source) ([]sponsorblock.Segment, bool) {
	return markedParts, true
}

// markedShell is a test shell playing the first test track, at 100 s of
// its 211, with a sponsor read and a highlight marked.
func markedShell(t *testing.T) *shell {
	t.Helper()
	s := newTestShellWith(t, nil, func(a *app.App) player.Services { return markedPlayer{testPlayer{a: a}} }, player.Saved{})
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.p.Time(100*time.Second, 211*time.Second)
	s.settle(t, func(st player.State) bool {
		return st.Position >= 100*time.Second && len(st.Parts) == 1 && st.Highlight > 0
	})
	return s
}

// nearInk reports whether c is within 40 of want in each channel.
func nearInk(c color.Color, want ui.Color) bool {
	r, g, b, _ := c.RGBA()
	d := func(x uint32, y uint8) bool { return int(x>>8)-int(y) <= 40 && int(y)-int(x>>8) <= 40 }
	return d(r, want.R) && d(g, want.G) && d(b, want.B)
}

// inked counts the pixels of img near col, of the columns from x0 to x1.
func inked(img image.Image, col ui.Color, x0, x1 int) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := max(x0, b.Min.X); x < min(x1, b.Max.X); x++ {
			if nearInk(img.At(x, y), col) {
				n++
			}
		}
	}
	return n
}

// TestSeekBarMarksParts marks the sponsor read on the player bar's seek
// bar, in its kind's colour, where it is, and the highlight.
func TestSeekBarMarksParts(t *testing.T) {
	s := markedShell(t)
	tt := ui.NewTester(s.view, 1280, 820)
	tt.Frame()
	tt.Advance(time.Second)
	bar := seekPixels(t, tt)
	w := bar.Bounds().Dx()
	// Half the knob, 5 spaces of 4 DIPs wide, within the bar on each side.
	pad := 10
	at := func(t time.Duration) int { return pad + int(float64(w-2*pad)*t.Seconds()/211) }
	sponsor := s.lk.partColor(sponsorblock.Sponsor)
	if n := inked(bar, sponsor, at(25*time.Second), at(55*time.Second)); n == 0 {
		t.Error("the sponsor read isn't marked where it is")
	}
	if n := inked(bar, sponsor, at(70*time.Second), at(200*time.Second)); n != 0 {
		t.Errorf("%d pixels of the sponsor read's colour past it", n)
	}
	if n := inked(bar, s.lk.danger, at(148*time.Second), at(152*time.Second)); n == 0 {
		t.Error("the highlight isn't marked")
	}
}

// TestSeekBarTellsTheParts tells a part's kind and times, and the
// highlight's, under the pointer, as the mini player's progress does.
func TestSeekBarTellsTheParts(t *testing.T) {
	s := markedShell(t)
	s.st = s.p.State()
	if tip, ok := s.markAt(40.0/211, 211*time.Second, 0.01); !ok || tip != "Sponsor, 0:20 to 1:00" {
		t.Errorf("over the sponsor read: %q, %v", tip, ok)
	}
	if tip, ok := s.markAt(151.0/211, 211*time.Second, 0.01); !ok || tip != "Highlight, 2:30" {
		t.Errorf("by the highlight: %q, %v", tip, ok)
	}
	if _, ok := s.markAt(0.9, 211*time.Second, 0.01); ok {
		t.Error("a mark where there is none")
	}
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	r, ok := tt.Find("Seek")
	if !ok {
		t.Fatalf("no progress: %q", tt.Texts())
	}
	tt.Move(r.X+r.W*40/211, r.Y+r.H/2)
	tt.Frame()
	if !tt.HasText("Sponsor, 0:20 to 1:00") {
		t.Errorf("hovering the sponsor read in the mini player doesn't tell it: %q", tt.Texts())
	}
}

// TestNotchMarksParts marks the sponsor read on the notch's progress.
func TestNotchMarksParts(t *testing.T) {
	s := markedShell(t)
	x := newNotchTest(t, s, testHousing)
	x.openPanel()
	r, ok := x.tt.Find("Seek")
	if !ok {
		t.Fatalf("no progress in the notch: %q", x.tt.Texts())
	}
	// The bar runs 4 DIPs within the progress on each side.
	line, span := float64(r.X+4), float64(r.W-8)
	from, to := int(line+span*25/211), int(line+span*55/211)
	sub := x.tt.Image().SubImage(image.Rect(from, int(r.Y), to, int(r.Y+r.H)))
	if n := inked(sub, s.lk.partColor(sponsorblock.Sponsor), from, to); n == 0 {
		t.Error("the notch's progress doesn't mark the sponsor read")
	}
}
