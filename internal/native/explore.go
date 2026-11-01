package native

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

var exploreTabs = []struct{ id, label string }{{"new", "new"}, {"charts", "charts"}, {"moods", "moods & genres"}}

// moodPath is the page of a mood or a genre.
func moodPath(params string) string { return "/mood/" + url.PathEscape(params) }

// explorePage shows what is new on YouTube Music, its charts, and its
// moods and genres, each opening on playlists of its own.
func (s *shell) explorePage(c *ui.Context, tab string) {
	labels := make([]string, len(exploreTabs))
	selected := 0
	for i, t := range exploreTabs {
		labels[i] = t.label
		if t.id == tab {
			selected = i
		}
	}
	s.pageScroll(c, func() {
		before := selected
		s.pageHeader(c, "explore", labels, &selected)
		if selected != before {
			s.router.Replace("/explore/" + exploreTabs[selected].id)
		}
		switch exploreTabs[selected].id {
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
		s.emptyLine(c, "YouTube Music shows nothing here right now.")
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
				if s.moodTile(c, "mood-"+count(gi)+"-"+count(mi), tone, m) {
					s.navigate(moodPath(m.Params))
				}
			}
		})
	}
}

func (s *shell) moodColumns(c *ui.Context) int {
	return max(2, int(s.paneWidth(c)-2*s.lk.gutter)/180)
}

func (s *shell) moodTile(c *ui.Context, key string, tone int, m youtube.Mood) bool {
	k := &s.lk
	t := s.buttonBase(c.Key(key)).Height(96).Padding(12).AlignItems(ui.Start).Justify(ui.Start).Background(k.tile(tone)).
		Radius(k.radius).Label("Playlists for " + m.Title).Cursor(ui.CursorPointer)
	if t.Pressed() {
		t.Scale(0.97, 0.97)
	}
	t.Children(func() {
		ui.Text(c, strings.ToLower(m.Title)).FontSize(k.size(1.375)).FontWeight(300).Font(k.display).TextColor(k.tileText).MaxLines(2)
	})
	return t.Clicked()
}

// moodPage shows the playlists of a mood or a genre.
func (s *shell) moodPage(c *ui.Context, params string) {
	key := "mood:" + params
	res := fetch(s, key, func(ctx context.Context) (app.MoodPage, error) { return s.svc.Catalog.Mood(ctx, params) })
	s.pageScroll(c, func() {
		title := "mood"
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

// addToMenus are the track menu's ways to keep a track: in a mix, or in a
// YouTube Music playlist of the user's.
func (s *shell) addToMenus(m *ui.Menu, tracks []media.Track) {
	m.Submenu("Add to mix", func(m *ui.Menu) {
		if m.Item("New mix…").Chosen() {
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
	})
	if !s.accounts().YouTube.SignedIn || len(tracks) != 1 {
		return
	}
	t := tracks[0]
	m.Submenu("Add to YouTube Music playlist", func(m *ui.Menu) {
		if m.Item("New playlist…").Chosen() {
			s.promptName("New playlist on YouTube Music", "Name", t.Title, "create", func(name string) {
				s.createYouTubePlaylist(name, []media.Track{t}, nil)
			})
		}
		own := fetch(s, "youtube:playlists", s.svc.Collection.YouTubePlaylists)
		if !own.done {
			m.Item("Loading your playlists…").Disabled(true)
			return
		}
		editable := slices.DeleteFunc(slices.Clone(own.value), func(p media.Playlist) bool { return !p.Editable })
		if len(editable) > 0 {
			m.Separator()
		}
		for _, p := range editable {
			if m.Item(p.Title).Chosen() {
				go func() {
					err := s.svc.Collection.AddToYouTubePlaylist(context.Background(), p.ID, t)
					s.update(func() {
						if err != nil {
							s.notes = append(s.notes, noticeOf(err))
							return
						}
						s.notes = append(s.notes, notice("Added "+t.Title+" to "+p.Title))
						s.forget("playlist:" + p.ID)
					})
				}()
			}
		}
		if len(editable) == 0 {
			m.Item("No playlists of yours").Disabled(true)
		}
	})
}

// addToMix adds tracks to a mix.
func (s *shell) addToMix(p media.Playlist, tracks []media.Track) {
	go func() {
		err := s.svc.Collection.AddToPlaylist(context.Background(), p.ID, tracks)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			what := plural(len(tracks), "track")
			if len(tracks) == 1 {
				what = tracks[0].Title
			}
			s.notes = append(s.notes, notice("Added "+what+" to "+p.Title))
			s.forget("playlist:" + p.ID)
		})
	}()
}

// createYouTubePlaylist makes a playlist of tracks on YouTube Music, named
// title, and runs done with it, if any.
func (s *shell) createYouTubePlaylist(title string, tracks []media.Track, done func(media.Playlist)) {
	if slices.ContainsFunc(tracks, func(t media.Track) bool { return t.Source != media.SourceYouTube }) {
		s.notes = append(s.notes, notice("Finding the tracks on YouTube Music…"))
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		p, err := s.svc.Collection.CreateYouTubePlaylist(ctx, title, tracks)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: "Couldn't create " + title + ": " + err.Error(), Error: true})
				return
			}
			s.forget("youtube:playlists")
			s.notes = append(s.notes, notice("Created "+p.Title+" on YouTube Music"))
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
	s.promptName("New mix", "Name", value, "create", func(name string) {
		p, err := s.svc.Collection.CreatePlaylist(name, "", tracks)
		if err != nil {
			s.notes = append(s.notes, noticeOf(err))
			return
		}
		s.notes = append(s.notes, notice("Created "+p.Title))
	})
}
