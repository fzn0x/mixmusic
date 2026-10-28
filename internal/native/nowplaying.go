package native

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// nowPlayingPage gives the track playing the whole pane: its sleeve under
// a visualizer, its credits, the transport, its lyrics, and what relates
// to it.
func (s *shell) nowPlayingPage(c *ui.Context) {
	k := &s.lk
	cur, ok := s.st.Current()
	s.pageScroll(c, func() {
		if !ok {
			s.pageHeader(c, "now playing", nil, nil)
			ui.Text(c, "nothing playing").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
			ui.Text(c, "Play a track and it fills this page, with its lyrics.").FontSize(k.subtitleSize()).FontWeight(300)
			return
		}
		t := cur.Track
		s.pageHeader(c, t.Title, nil, nil)
		pane := s.paneWidth(c) - 2*k.gutter
		sleeve := min(max(260, pane*0.4), 420)
		stack := pane < 1100-s.railWidth()
		grid := ui.Row(c).Gap(48).AlignItems(ui.Start)
		if stack {
			grid.Column()
		}
		grid.Children(func() {
			ui.Box(c).Size(sleeve, sleeve).Shrink(0).Children(func() {
				s.cover(c, "sleeve", t.Artwork, sleeve, false)
				if s.st.Playing {
					s.visualizer(c, sleeve)
				}
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(24).Children(func() {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					s.sourceMark(c, t.Source, false)
					s.artistLinks(c, "now-artists", t.Artists, k.text)
					if t.Album != nil && t.Album.Name != "" {
						ui.Text(c, "·").FontSize(k.smallSize()).TextColor(k.muted)
						if t.Album.ID == "" {
							ui.Text(c, t.Album.Name).FontSize(k.smallSize()).TextColor(k.muted)
						} else {
							link := ui.ButtonBase(c.Key("now-album")).Label(t.Album.Name).Cursor(ui.CursorPointer)
							if link.Clicked() {
								s.navigate(albumPath(t.Album.ID))
							}
							link.Children(func() { ui.Text(c, t.Album.Name).FontSize(k.smallSize()).TextColor(k.muted) })
						}
					}
				})
				s.progress(c)
				ui.Row(c).Gap(12).Children(func() {
					name, label := "play", "Play"
					switch {
					case s.st.Loading:
						label = "Loading"
					case s.st.Playing:
						name, label = "pause", "Pause"
					}
					if s.roundButton(c, "now-toggle", name, label, true, s.st.Loading).Clicked() {
						s.p.Toggle()
					}
					if s.roundButton(c, "now-next", "next", "Next", false, false).Clicked() {
						s.p.Next()
					}
					s.likeButton(c, "now-like", t, 18)
				})
				ui.Column(c).Height(420).Children(func() { s.lyrics(c, true) })
				s.pluginWidgets(c)
			})
		})
		s.related(c, t)
	})
}

// related shows what YouTube Music relates to the track, under the player:
// songs, playlists and artists like it. When it fails, the page goes
// without it.
func (s *shell) related(c *ui.Context, t media.Track) {
	res := fetch(s, "related-"+t.ID, func(ctx context.Context) ([]media.Shelf, error) {
		return s.svc.Catalog.Related(ctx, t)
	})
	if !res.done || res.err != nil || len(res.value) == 0 {
		return
	}
	ui.Column(c).Margin(48, 0, 0).Children(func() { s.shelves(c, "now-related", res.value) })
}

// visualizer draws the spectrum of what plays over the foot of the sleeve,
// in the accent, at every frame while it plays. Where MixMusic hears
// nothing, as of a Spotify device elsewhere, it draws nothing.
func (s *shell) visualizer(c *ui.Context, size float32) {
	k := &s.lk
	ui.Box(c).Absolute().Left(0).Bottom(0).Size(size, size/3).PassThrough().Label("Visualizer").Draw(func(p *ui.Painter, r ui.Rect) {
		p.AnimationFrame()
		bins := s.svc.Player.NativeAnalyser(false, false).Bytes
		const bars = 48
		if len(bins) < bars {
			return
		}
		w := r.W / bars
		for i := range bars {
			// The lower bins carry the music: spread them over the bars.
			v := float32(bins[i*len(bins)/(bars*2)]) / 255
			if v == 0 {
				// Silence: a flat line of bars would look broken.
				continue
			}
			h := max(2, v*r.H)
			p.Fill(ui.Rect{X: r.X + float32(i)*w + 1, Y: r.Y + r.H - h, W: w - 2, H: h}, k.accent.Alpha(0.85), 0)
		}
	})
}
