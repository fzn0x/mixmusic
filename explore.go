package main

import (
	"context"
	"net/url"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// exploreTabs are the tabs of Explore, by their ids; exploreTabLabels are
// their names, in the language the app shows.
var exploreTabs = []string{"new", "charts", "moods"}

func exploreTabLabels() []string {
	return []string{locales.ExploreTabNew(), locales.ExploreTabCharts(), locales.ExploreTabMoods()}
}

// moodPath is the page of a mood or a genre.
func moodPath(params string) string { return "/mood/" + url.PathEscape(params) }

// explorePage shows what is new on YouTube Music, its charts, and its
// moods and genres, each opening on playlists of its own.
func (s *shell) explorePage(c *ui.Context, tab string) {
	selected := max(slices.Index(exploreTabs, tab), 0)
	s.pageScroll(c, func() {
		before := selected
		s.pageHeader(c, locales.ExploreTitle(), exploreTabLabels(), &selected)
		if selected != before {
			s.router.Replace("/explore/" + exploreTabs[selected])
		}
		switch exploreTabs[selected] {
		case "new":
			s.shelvesOf(c, "explore", "what is new", s.svc.Catalog.Explore)
		case "charts":
			s.shelvesOf(c, "charts", "the charts", s.svc.Catalog.Charts)
		case "moods":
			s.moods(c)
		}
	})
}

// shelvesOf shows the shelves load gives.
func (s *shell) shelvesOf(c *ui.Context, key, what string, load func(ctx context.Context) ([]media.Shelf, error)) {
	res := fetch(s, key, load)
	switch {
	case !res.done:
		s.loading(c)
	case res.err != nil:
		s.problem(c, res.err, func() { s.forget(key) })
	case len(res.value) == 0:
		s.emptyLine(c, locales.ExploreNothingHere())
	default:
		s.shelves(c, key, res.value)
	}
	_ = what
}

// moods shows the moods and genres as flat tiles, by group.
func (s *shell) moods(c *ui.Context) {
	res := fetch(s, "moods", s.svc.Catalog.Moods)
	switch {
	case !res.done:
		s.loading(c)
		return
	case res.err != nil:
		s.problem(c, res.err, func() { s.forget("moods") })
		return
	}
	n := 0
	for gi, g := range res.value {
		s.sectionTitle(c, g.Title)
		ui.Grid(c.Key("moods-"+count(gi))).Columns(s.moodColumns(c)).GapX(8).GapY(8).Margin(0, 0, 48).Children(func() {
			for mi, m := range g.Moods {
				tone := n % 4
				n++
				if s.moodTile(c, "mood-"+count(gi)+"-"+count(mi), tone, m.Title, locales.ExploreMoodPlaylists(m.Title)) {
					s.navigate(moodPath(m.Params))
				}
			}
		})
	}
}

func (s *shell) moodColumns(c *ui.Context) int {
	return max(2, int(s.paneWidth(c)-2*s.lk.gutter)/180)
}

// moodTile is a flat tile of title, label telling what it opens.
func (s *shell) moodTile(c *ui.Context, key string, tone int, title, label string) bool {
	k := &s.lk
	t := s.buttonBase(c.Key(key)).Height(96).Padding(12).AlignItems(ui.Start).Justify(ui.Start).Background(k.tile(tone)).
		Radius(k.radius).Label(label).Cursor(ui.CursorPointer)
	if k.moves() {
		s.pressMotion(t, 0.97)
	} else if t.Pressed() {
		t.Scale(0.97, 0.97)
	}
	t.Children(func() {
		ui.Text(c, k.words(title)).FontSize(k.size(1.375)).FontWeight(k.titleWeight()).Font(k.display).TextColor(k.tileText).MaxLines(2)
	})
	return t.Clicked()
}

// moodPage shows the playlists of a mood or a genre.
func (s *shell) moodPage(c *ui.Context, params string) {
	key := "mood:" + params
	res := fetch(s, key, func(ctx context.Context) (app.MoodPage, error) { return s.svc.Catalog.Mood(ctx, params) })
	s.pageScroll(c, func() {
		title := locales.ExploreMood()
		if res.done && res.err == nil && res.value.Title != "" {
			title = res.value.Title
		}
		s.pageHeader(c, title, nil, nil)
		switch {
		case !res.done:
			s.loading(c)
		case res.err != nil:
			s.problem(c, res.err, func() { s.forget(key) })
		default:
			s.shelves(c, key, res.value.Shelves)
		}
	})
}

// listOnYouTube lists the playlists of the user's library on YouTube
// Music; tests answer for YouTube.
var listOnYouTube = func(s *shell, ctx context.Context) ([]media.Playlist, error) {
	return s.svc.Collection.YouTubePlaylists(ctx)
}

// youtubePlaylists returns the playlists of the user's library on YouTube
// Music, which the library shows and the track menu adds to.
func (s *shell) youtubePlaylists() *loaded[[]media.Playlist] {
	list := listOnYouTube
	return fetch(s, "youtube:playlists", func(ctx context.Context) ([]media.Playlist, error) { return list(s, ctx) })
}

// keepYouTubePlaylists fetches the user's playlists on YouTube Music once
// they're signed in, before any track menu opens: the system's menus show
// what they held as they opened, so a menu that asked for them itself
// said "Loading your playlists…" until it opened again (#260). Signed
// out, nothing is asked; signing in or out, or choosing another channel,
// forgets them, and they're asked for again.
func (s *shell) keepYouTubePlaylists() {
	if s.accounts().YouTube.SignedIn {
		s.youtubePlaylists()
	}
}

// addToMenus are the track menu's ways to keep a track: in a mix, or in a
// YouTube Music playlist of the user's.
func (s *shell) addToMenus(m *ui.Menu, tracks []media.Track) {
	m.Submenu(locales.ExploreAddToMix(), func(m *ui.Menu) { s.mixItems(m, tracks) })
	if !s.accounts().YouTube.SignedIn || len(tracks) != 1 || tracks[0].Source == media.SourceFile {
		return
	}
	t := tracks[0]
	m.Submenu(locales.ExploreAddToYouTube(), func(m *ui.Menu) {
		if m.Item(locales.ExploreNewPlaylist()).Chosen() {
			s.promptName(locales.ExploreNewYouTubePlaylist(), locales.RailName(), t.Title, locales.RailCreate(), func(name string) {
				s.createYouTubePlaylist(name, []media.Track{t}, nil)
			})
		}
		own := s.youtubePlaylists()
		switch {
		case !own.done:
			// Only until the answer the window asked for ahead comes.
			m.Item(locales.ExploreLoadingPlaylists()).Disabled(true)
			return
		case own.err != nil:
			// Asked again, as a refresh asks, unless they already are:
			// they may be there the next time the menu opens.
			own.refresh()
			m.Item(locales.ExploreLoadPlaylistsFailed()).Disabled(true)
			return
		}
		editable := slices.DeleteFunc(slices.Clone(own.value), func(p media.Playlist) bool { return !p.Editable })
		if len(editable) > 0 {
			m.Separator()
		}
		for _, p := range editable {
			// One the user collaborates on tells whose it is.
			label := p.Title
			if p.Collaborative && !p.Mine && p.Owner != "" {
				label += " · " + p.Owner
			}
			if m.Item(label).Chosen() {
				s.addToYouTubePlaylist(p, t)
			}
		}
		if len(editable) == 0 {
			m.Item(locales.ExploreNoPlaylists()).Disabled(true)
		}
	})
}

// mixItems are the items of Add to mix, of a track's menu or of the bar
// of the tracks selected: New mix…, then the user's mixes.
func (s *shell) mixItems(m *ui.Menu, tracks []media.Track) {
	if m.Item(locales.ExploreNewMixItem()).Chosen() {
		s.newMixOf(tracks)
	}
	mixes := s.svc.Collection.Playlists()
	if len(mixes) > 0 {
		m.Separator()
	}
	for _, p := range mixes {
		if m.Item(p.Title).Chosen() {
			s.addToMix(p, tracks)
		}
	}
}

// addToMix adds tracks to a mix, but those it holds already, which the
// toast offers to add anyway (toldMixAdd).
func (s *shell) addToMix(p media.Playlist, tracks []media.Track) {
	go func() {
		held, err := s.svc.Collection.AddToMix(p.ID, tracks)
		s.update(func() { s.toldMixAdd(p, tracks, held, err) })
	}()
}

// createYouTubePlaylist makes a playlist of tracks on YouTube Music, named
// title, and runs done with it, if any.
func (s *shell) createYouTubePlaylist(title string, tracks []media.Track, done func(media.Playlist)) {
	if slices.ContainsFunc(tracks, func(t media.Track) bool { return t.Source != media.SourceYouTube }) {
		s.notes = append(s.notes, notice(locales.ExploreFinding()))
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		p, err := s.svc.Collection.CreateYouTubePlaylist(ctx, title, tracks)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.ExploreCouldntCreate(title, err.Error()), Error: true})
				return
			}
			s.keepPlaylist("youtube:playlists", p)
			s.notes = append(s.notes, notice(locales.ExploreCreatedOnYouTube(p.Title)))
			if done != nil {
				done(p)
			}
		})
	}()
}

// newMixOf asks for the name of a mix of tracks, the first's title to
// start with, and makes it, staying where the user is.
func (s *shell) newMixOf(tracks []media.Track) {
	value := ""
	if len(tracks) > 0 {
		value = tracks[0].Title
	}
	s.promptName(locales.RailNewMix(), locales.RailName(), value, locales.RailCreate(), func(name string) {
		p, err := s.svc.Collection.CreatePlaylist(name, "", tracks)
		if err != nil {
			s.notes = append(s.notes, noticeOf(err))
			return
		}
		s.notes = append(s.notes, notice(locales.ExploreCreated(p.Title)))
	})
}
