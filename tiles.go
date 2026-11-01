package main

import (
	"context"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// tileSpan is how many columns and rows of the tile grid a tile takes.
type tileSpan struct{ cols, rows int }

var (
	tileSmall = tileSpan{1, 1}
	tileWide  = tileSpan{2, 1}
	tileTall  = tileSpan{1, 2}
	tileLarge = tileSpan{2, 2}
)

// tileGrid lays tiles on four columns of rows of at least 150px, capped at
// 1040px, two columns under a 640px pane.
func (s *shell) tileGrid(c *ui.Context, tiles func(columns int)) {
	columns := 4
	if s.paneWidth(c) < 640 {
		columns = 2
	}
	ui.Grid(c).Columns(columns).GapX(8).GapY(8).MaxWidth(1040).Margin(0, 0, 48).Children(func() { tiles(columns) })
}

// flatTile is a flat field of one tile colour with a lowercase light
// title at its top left and a small caption at its foot; art, when given,
// prints faintly under the words. It returns whether it was clicked.
func (s *shell) flatTile(c *ui.Context, key string, tone int, span tileSpan, columns int, title, caption, label string, art []media.Image) bool {
	k := &s.lk
	if columns < 4 && span.cols > 1 {
		span.cols = min(span.cols, columns)
	}
	t := s.buttonBase(c.Key(key)).ColumnSpan(span.cols).RowSpan(span.rows).MinHeight(150).Padding(16).
		AlignItems(ui.Start).Justify(ui.SpaceBetween).Column().Background(k.tile(tone)).Radius(k.radius).
		Label(label).Cursor(ui.CursorPointer).Clip()
	if t.Pressed() {
		t.Scale(0.97, 0.97)
	}
	large := span.cols > 1 && span.rows > 1
	pane := s.paneWidth(c)
	size := min(max(24, 0.034*pane), 38) * k.scale
	if large {
		size = min(max(32, 0.056*pane), 60) * k.scale
	}
	shows := len(art) > 0 && s.inView(c, t)
	t.Children(func() {
		if !shows {
			// Nothing to print under the words.
		} else if b := s.art.get(c, artworkURL(art, 600), 600); b != nil {
			ui.Image(c, b).Absolute().Top(0).Right(0).Bottom(0).Left(0).Fit(ui.Cover).Grayscale().Opacity(0.22)
		}
		if texture, ink := k.fx.texture, k.tileText; texture != "" {
			ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough().
				Draw(func(p *ui.Painter, r ui.Rect) { paintTexture(p, r, texture, ink) })
		}
		ui.Text(c, strings.ToLower(title)).FontSize(size).FontWeight(300).Font(k.display).LineHeight(0.95).
			LetterSpacing(-0.02 * size).TextColor(k.tileText)
		if caption != "" {
			ui.Text(c, caption).FontSize(k.smallSize()).TextColor(k.tileText).MaxWidthPercent(66).MaxLines(4)
		}
	})
	return t.Clicked()
}

// artTileSize is the width of an art tile, and shelfGap the gap between the
// tiles of a shelf.
const (
	artTileSize = 172
	shelfGap    = 16
)

// artTile is a square frame of artwork with its title and a subtitle that
// starts with the source mark; round for artists. Hovering shows a play
// circle. It tells whether it was opened or its play circle clicked.
func (s *shell) artTile(c *ui.Context, key string, art []media.Image, title, subtitle string, source media.Source, round, active, playable bool, menu func(m *ui.Menu)) (open, play bool) {
	k := &s.lk
	const size = artTileSize
	tile := s.buttonBase(c.Key(key)).Width(size).Shrink(0).Column().AlignItems(ui.Start).Gap(8).Label(title).Cursor(ui.CursorPointer)
	if menu != nil {
		tile.ContextMenu(menu)
	}
	open = tile.Clicked()
	hovered := tile.Hovered()
	tile.Children(func() {
		frame := ui.Box(c).Size(size, size).Shrink(0)
		frame.Children(func() {
			s.cover(c, key+"-cover", art, size, round)
			if active {
				ui.Box(c).Absolute().Top(0).Left(0).Size(size, size).InsetShadow(0, 0, 0, 3, k.accent).Radius(k.radiusSm)
			}
			if playable && hovered {
				p := s.buttonBase(c.Key(key+"-play")).Size(42, 42).Radius(21).Background(k.accent).Absolute().Right(8).Bottom(8).
					Label("Play " + title).Cursor(ui.CursorPointer)
				if p.Clicked() {
					play = true
				}
				p.Children(func() { ui.Icon(c, icon("play")).FontSize(18).TextColor(k.accentText) })
			}
		})
		ui.Column(c).Width(size).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(k.bodySize()).SingleLine()
			ui.Row(c).Gap(6).MinWidth(0).Children(func() {
				s.sourceMark(c, source, false)
				ui.Text(c, subtitle).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().MinWidth(0)
			})
		})
	})
	return open && !play, play
}

// shelf is a titled row of tiles that scrolls sideways.
func (s *shell) shelf(c *ui.Context, key, title string, tiles func()) {
	s.shelfWith(c, key, title, nil, tiles)
}

// shelfWith is a shelf with links in its title's row, as to make what it
// shows. Its arrows scroll it by most of its width.
func (s *shell) shelfWith(c *ui.Context, key, title string, links, tiles func()) {
	k := &s.lk
	// A shelf of each page keeps its own place: two pages of a kind show
	// at once as one slides in.
	at := s.pagePath + " " + key
	st := s.shelfScroll[at]
	if st == nil {
		st = &ui.ScrollState{}
		s.shelfScroll[at] = st
	}
	step := 0.8 * (s.paneWidth(c) - 2*k.gutter)
	col := ui.Column(c.Key(key)).Margin(0, 0, 48)
	if h, far := s.farFromView(c, "shelf "+at, col); far {
		col.Height(h) // its tiles build once it nears the view
		return
	}
	col.Children(func() {
		s.sectionHead(c, title, func() {
			if links != nil {
				links()
			}
			if s.iconButton(c, key+"-back", "back", "Scroll "+title+" back", false, 18).Clicked() {
				st.ScrollTo(max(st.X-step, 0), 0, ui.Smooth)
			}
			if s.iconButton(c, key+"-on", "forward", "Scroll "+title+" on", false, 18).Clicked() {
				st.ScrollTo(min(st.X+step, st.MaxX), 0, ui.Smooth)
			}
		})
		ui.ScrollHorizontal(c).TrackScroll(st).Gap(shelfGap).Padding(0, 0, 12).Children(tiles)
	})
}

// itemTile shows any item of a shelf; tracks are the shelf's, which play
// in order.
func (s *shell) itemTile(c *ui.Context, key string, it media.Item, tracks []media.Track) {
	switch {
	case it.Album != nil:
		s.albumTile(c, key, *it.Album)
	case it.Playlist != nil:
		s.playlistTile(c, key, *it.Playlist)
	case it.Artist != nil:
		s.artistTile(c, key, *it.Artist)
	case it.Track != nil:
		s.trackTile(c, key, *it.Track, tracks)
	}
}

func (s *shell) albumTile(c *ui.Context, key string, a media.Album) {
	var parts []string
	if a.Kind != "" && a.Kind != media.AlbumKindAlbum {
		parts = append(parts, string(a.Kind))
	}
	if a.Year != "" {
		parts = append(parts, a.Year)
	}
	if n := names(a.Artists); n != "" {
		parts = append(parts, n)
	}
	open, play := s.artTile(c, key, a.Artwork, a.Title, strings.Join(parts, " · "), a.Source, false, false, true, func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			s.navigate(albumPath(a.ID))
		}
		if m.Item("Play").Chosen() {
			s.playAlbum(a.ID)
		}
		s.copyLinkItem(m, a.URL)
	})
	switch {
	case play:
		s.playAlbum(a.ID)
	case open:
		s.navigate(albumPath(a.ID))
	}
}

func (s *shell) playlistTile(c *ui.Context, key string, p media.Playlist) {
	var parts []string
	if p.Owner != "" {
		parts = append(parts, p.Owner)
	}
	if p.TrackCount > 0 {
		parts = append(parts, plural(p.TrackCount, "track"))
	}
	open, play := s.artTile(c, key, p.Artwork, p.Title, strings.Join(parts, " · "), p.Source, false, false, true, func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			s.navigate(playlistPath(p.ID))
		}
		if m.Item("Play").Chosen() {
			s.playPlaylist(p.ID)
		}
		s.copyLinkItem(m, p.URL)
	})
	switch {
	case play:
		s.playPlaylist(p.ID)
	case open:
		s.navigate(playlistPath(p.ID))
	}
}

func (s *shell) artistTile(c *ui.Context, key string, a media.Artist) {
	sub := a.Subtitle
	if sub == "" {
		sub = "Artist"
	}
	open, _ := s.artTile(c, key, a.Artwork, a.Name, sub, a.Source, true, false, false, func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			s.navigate(artistPath(a.ID))
		}
		s.copyLinkItem(m, a.URL)
	})
	if open {
		s.navigate(artistPath(a.ID))
	}
}

func (s *shell) trackTile(c *ui.Context, key string, t media.Track, queue []media.Track) {
	cur, ok := s.st.Current()
	active := ok && cur.Track.ID == t.ID
	open, play := s.artTile(c, key, t.Artwork, t.Title, names(t.Artists), t.Source, false, active, true, func(m *ui.Menu) { s.trackMenu(m, t) })
	switch {
	case play:
		s.playFrom(queue, t)
	case open && t.Album != nil && t.Album.ID != "":
		s.navigate(albumPath(t.Album.ID))
	case open:
		s.playFrom(queue, t)
	}
}

// playFrom plays tracks from t.
func (s *shell) playFrom(tracks []media.Track, t media.Track) {
	for i, tr := range tracks {
		if tr.ID == t.ID {
			s.p.PlayTracks(tracks, i)
			return
		}
	}
	s.p.PlayTracks([]media.Track{t}, 0)
}

// playAlbum and playPlaylist fetch what a collection holds, and play it.
func (s *shell) playAlbum(id string) {
	s.playCollection(func(ctx context.Context) ([]media.Track, error) {
		a, err := s.svc.Catalog.Album(ctx, id)
		return a.Tracks, err
	})
}

func (s *shell) playPlaylist(id string) {
	s.playCollection(func(ctx context.Context) ([]media.Track, error) {
		p, err := s.svc.Catalog.Playlist(ctx, id)
		return p.Tracks, err
	})
}

func (s *shell) playCollection(load func(ctx context.Context) ([]media.Track, error)) {
	go func() {
		tracks, err := load(context.Background())
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case len(tracks) == 0:
				s.notes = append(s.notes, player.Notice{Message: "Nothing to play here"})
			default:
				s.p.PlayTracks(tracks, 0)
			}
		})
	}()
}

// shelves shows shelves, each a row of tiles; a shelf's tracks play as one
// queue.
func (s *shell) shelves(c *ui.Context, key string, shelves []media.Shelf) {
	for i, sh := range shelves {
		var tracks []media.Track
		for _, it := range sh.Items {
			if it.Track != nil {
				tracks = append(tracks, *it.Track)
			}
		}
		s.shelf(c, key+"-"+count(i), sh.Title, func() {
			// The tiles far to either side take their room, and build
			// nothing.
			n := len(sh.Items)
			from, to := s.nearTiles(c, key+"-"+count(i), n)
			if from > 0 {
				ui.Box(c).Width(float32(from)*(artTileSize+shelfGap) - shelfGap).Shrink(0)
			}
			for j := from; j < to; j++ {
				s.itemTile(c, key+"-"+count(i)+"-"+count(j), sh.Items[j], tracks)
			}
			if to < n {
				ui.Box(c).Width(float32(n-to)*(artTileSize+shelfGap) - shelfGap).Shrink(0)
			}
		})
	}
}

// nearTiles returns the tiles, of the n art tiles of the shelf keyed key,
// that showed in the frame before, or within the pane's width of them. A
// shelf that scrolled since asks for another frame, as followMoves does.
func (s *shell) nearTiles(c *ui.Context, key string, n int) (from, to int) {
	at := s.pagePath + " " + key
	x := float32(0)
	if st := s.shelfScroll[at]; st != nil {
		x = st.X
	}
	s.moved(c, "tiles "+at, x)
	w, step := s.paneWidth(c), float32(artTileSize+shelfGap)
	from = min(max(int((x-w)/step), 0), n)
	to = min(max(int(math.Ceil(float64((x+2*w)/step))), from), n)
	return from, to
}
