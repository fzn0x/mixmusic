package native

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

var statsRanges = []struct {
	label string
	days  int
}{{"this week", 7}, {"this month", 30}, {"all time", 0}}

// statsPage shows what the user listened to: artists rank as circles on
// one fixed size ramp, and each service's share shows as two rings.
func (s *shell) statsPage(c *ui.Context) {
	k := &s.lk
	labels := make([]string, len(statsRanges))
	for i, r := range statsRanges {
		labels[i] = r.label
	}
	s.pageScroll(c, func() {
		s.pageHeader(c, "stats", labels, &s.statsRange)
		st := s.svc.Collection.Stats(statsRanges[s.statsRange].days)
		if st.Plays == 0 {
			ui.Text(c, "nothing counted yet").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
			ui.Text(c, "Tracks count once they played for thirty seconds.").FontSize(k.subtitleSize()).FontWeight(300)
			return
		}
		hours := st.Minutes / 60
		spent := fmt.Sprintf("%d min", st.Minutes)
		if hours > 0 {
			spent = fmt.Sprintf("%d hr %d min", hours, st.Minutes%60)
		}
		size := k.subtitleSize()
		ui.RichText(c,
			ui.Span{Text: "You played ", Size: size, Weight: 300},
			ui.Span{Text: plural(st.Plays, "track"), Size: size, Weight: 400},
			ui.Span{Text: " for ", Size: size, Weight: 300},
			ui.Span{Text: spent, Size: size, Weight: 400},
			ui.Span{Text: ": ", Size: size, Weight: 300},
			ui.Span{Text: fmt.Sprintf("%d from Spotify", st.Spotify), Size: size, Weight: 300, Color: k.spotify},
			ui.Span{Text: " and ", Size: size, Weight: 300},
			ui.Span{Text: fmt.Sprintf("%d from YouTube Music", st.YouTube), Size: size, Weight: 300, Color: k.youtube},
			ui.Span{Text: ".", Size: size, Weight: 300},
		).MaxWidth(760).Margin(0, 0, 48)
		s.dayChart(c, st.Daily)
		ui.Row(c).Gap(48).Wrap().AlignItems(ui.Start).Margin(0, 0, 48).Children(func() {
			ui.Column(c).Width(320).Children(func() {
				s.sectionTitle(c, "the mix")
				s.venn(c, st.Spotify, st.YouTube)
			})
			ui.Column(c).Grow(1).Basis(320).Children(func() {
				s.sectionTitle(c, "top artists")
				s.artistCircles(c, st.TopArtists)
			})
		})
		s.sectionTitle(c, "top tracks")
		tracks := make([]media.Track, len(st.TopTracks))
		for i, t := range st.TopTracks {
			tracks[i] = t.Track
		}
		s.trackList(c, "top-tracks", tracks, trackListOptions{art: true, album: true})
	})
}

// niceCeil rounds a value up to a clean tick: 1, 2, 2.5 or 5 times a
// power of ten.
func niceCeil(v float64) float64 {
	if v <= 0 {
		return 0
	}
	p := math.Pow(10, math.Floor(math.Log10(v)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if m*p >= v {
			return m * p
		}
	}
	return 10 * p
}

// dayChart draws the minutes listened each day as columns in the accent.
func (s *shell) dayChart(c *ui.Context, days []library.DayPlays) {
	k := &s.lk
	n := len(days)
	if n == 0 {
		return
	}
	peak, most := 0, 0
	for i, d := range days {
		if d.Minutes > most {
			peak, most = i, d.Minutes
		}
	}
	top := niceCeil(float64(most))
	every := 1
	switch {
	case n > 31:
		every = 15
	case n > 7:
		every = 7
	}
	s.sectionTitle(c, "day by day")
	ui.Text(c, "Minutes listened each day.").FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 12)
	ui.Text(c, fmt.Sprintf("%.0f min", top)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
	ui.Box(c).Height(160).FillWidth().Margin(4, 0, 0).BorderWidth(0, 0, 1, 0).BorderColor(k.border).Label("Minutes listened each day").
		Draw(func(p *ui.Painter, r ui.Rect) {
			w := r.W / float32(n)
			for i, d := range days {
				h := float32(0)
				if top > 0 {
					h = float32(float64(d.Minutes)/top) * r.H
				}
				if h > 0 {
					p.Fill(ui.Rect{X: r.X + float32(i)*w + max(1, w*0.15), Y: r.Y + r.H - h, W: max(1, w*0.7), H: h}, k.accent, 0)
				}
				if i == peak && d.Minutes > 0 {
					p.Text(r.X+float32(i)*w, r.Y+r.H-h-16, fmt.Sprintf("%d min", d.Minutes), k.captionSize(), k.text)
				}
			}
		})
	ui.Row(c).FillWidth().Margin(4, 0, 48).Children(func() {
		for i, d := range days {
			label := ""
			switch {
			case i == n-1:
				label = "today"
			case (n-1-i)%every == 0:
				if t, err := time.ParseInLocation("2006-01-02", d.Date, time.Local); err == nil {
					if n <= 7 {
						label = t.Format("Mon")
					} else {
						label = t.Format("2 Jan")
					}
				}
			}
			ui.Text(c, label).FontSize(k.captionSize()).TextColor(k.muted).Grow(1).Basis(0).MinWidth(0).NoWrap()
		}
	})
}

// venn draws the plays of each service as rings whose areas match,
// overlapping as the icon's.
func (s *shell) venn(c *ui.Context, spotify, youtube int) {
	k := &s.lk
	most := float64(max(spotify, youtube, 1))
	rs := float32(20 + 70*math.Sqrt(float64(spotify)/most))
	ry := float32(20 + 70*math.Sqrt(float64(youtube)/most))
	ui.Box(c).Size(300, 200).Label(fmt.Sprintf("%d plays from Spotify, %d from YouTube Music", spotify, youtube)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			cy := r.Y + 100
			sx, yx := r.X+150-rs*0.55, r.X+150+ry*0.55
			var a, b ui.Path
			a.Circle(sx, cy, rs)
			b.Circle(yx, cy, ry)
			p.StrokePath(&a, 2, k.spotify)
			p.StrokePath(&b, 2, k.youtube)
			label := func(x float32, n int) {
				t := fmt.Sprint(n)
				w, h := p.MeasureText(0, ui.Span{Text: t, Size: k.subtitleSize(), Weight: 300})
				p.Text(x-w/2, cy-h/2, t, k.subtitleSize(), k.text)
			}
			label(sx, spotify)
			label(yx, youtube)
		})
}

// artistCircles ranks the top artists as circles whose areas grow with
// their plays.
func (s *shell) artistCircles(c *ui.Context, artists []library.ArtistPlays) {
	k := &s.lk
	if len(artists) == 0 {
		return
	}
	top := float64(max(artists[0].Plays, 1))
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		for i, a := range artists {
			r := float32(14 + 46*math.Sqrt(float64(a.Plays)/top))
			b := ui.ButtonBase(c.Key("stat-artist-"+count(i))).Size(2*r, 2*r).Radius(r).Background(k.tile(i)).Column().Center().
				Padding(4).Label(fmt.Sprintf("%s: %s", a.Artist.Name, plural(a.Plays, "play"))).Tooltip(a.Artist.Name).
				Disabled(a.Artist.ID == "")
			if a.Artist.ID != "" {
				b.Cursor(ui.CursorPointer)
				if b.Clicked() {
					s.navigate(artistPath(a.Artist.ID))
				}
			}
			b.Children(func() {
				if r >= 30 {
					ui.Text(c, a.Artist.Name).FontSize(k.captionSize()).TextColor(k.tileText).SingleLine().MaxWidth(2*r - 12)
				}
				ui.Text(c, fmt.Sprint(a.Plays)).FontSize(k.captionSize()).TextColor(k.tileText).FontFeatures("tnum")
			})
		}
	})
}
