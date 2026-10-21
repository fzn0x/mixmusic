package native

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// headProps are what the head of an album, a playlist or an artist shows.
type headProps struct {
	art         []media.Image
	round       bool
	source      media.Source
	facts       []fact
	description string
	playLabel   string
	play        func()
	shuffle     func()
	// keep is a save or follow toggle, with its labels.
	keep      *bool
	keepLabel string
	keptLabel string
	onKeep    func(on bool)
	menu      func(m *ui.Menu)
}

// fact is one of the facts of a collection's head: a text, or artists,
// each a link to its page.
type fact struct {
	text    string
	artists []media.Ref
}

// texts are facts of texts alone.
func texts(ts ...string) []fact {
	out := make([]fact, len(ts))
	for i, t := range ts {
		out[i] = fact{text: t}
	}
	return out
}

// collectionHead pairs the artwork with the facts and the actions.
func (s *shell) collectionHead(c *ui.Context, h headProps) {
	k := &s.lk
	size := float32(208)
	if s.paneWidth(c) < 720 {
		size = 128
	}
	ui.Row(c).Gap(24).AlignItems(ui.End).Margin(0, 0, 32).Children(func() {
		s.cover(c, "head-cover", h.art, size, h.round)
		ui.Column(c).Grow(1).MinWidth(0).Gap(12).Children(func() {
			ui.Row(c).Gap(6).Wrap().Children(func() {
				s.sourceMark(c, h.source, false)
				ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Children(func() {
					ui.Text(c, sourceName(h.source))
					for _, f := range h.facts {
						switch {
						case len(f.artists) > 0:
							ui.Text(c, " · ")
							for j, a := range f.artists {
								if j > 0 {
									ui.Text(c, ", ")
								}
								if a.ID == "" {
									ui.Text(c, a.Name)
								} else {
									ui.Link(c, a.Name, artistPath(a.ID))
								}
							}
						case f.text != "":
							ui.Text(c, " · "+f.text)
						}
					}
				})
			})
			if h.description != "" {
				d := h.description
				if len([]rune(d)) > 320 {
					d = string([]rune(d)[:320]) + "…"
				}
				ui.Text(c, d).FontSize(k.smallSize()).TextColor(k.muted).MaxLines(3).MaxWidth(640)
			}
			ui.Row(c).Gap(8).Wrap().Children(func() {
				if h.play != nil {
					label := h.playLabel
					if label == "" {
						label = "play"
					}
					if s.button(c, "head-play", label, true).Clicked() {
						h.play()
					}
				}
				if h.shuffle != nil && s.button(c, "head-shuffle", "shuffle", false).Clicked() {
					h.shuffle()
				}
				if h.keep != nil {
					label := h.keepLabel
					if *h.keep {
						label = h.keptLabel
					}
					if s.button(c, "head-keep", label, *h.keep).Clicked() {
						h.onKeep(!*h.keep)
					}
				}
				if h.menu != nil {
					s.roundButton(c, "head-more", "more", "More", false, false).Menu(h.menu)
				}
			})
		})
	})
}

func total(tracks []media.Track) time.Duration {
	var d time.Duration
	for _, t := range tracks {
		d += ms(t.DurationMs)
	}
	return d
}

// shuffled plays tracks shuffled, from one at random.
func (s *shell) shuffled(tracks []media.Track) {
	if len(tracks) == 0 {
		return
	}
	s.p.SetShuffle(true)
	s.p.PlayTracks(tracks, rand.IntN(len(tracks)))
}

// pageLoading and pageProblem stand for a page whose data hasn't come.
func (s *shell) pageLoading(c *ui.Context, title string) {
	s.pageScroll(c, func() {
		s.pageHeader(c, title, nil, nil)
		s.loading(c)
	})
}

func (s *shell) pageProblem(c *ui.Context, title string, err error, key string) {
	s.pageScroll(c, func() {
		s.pageHeader(c, title, nil, nil)
		s.problem(c, err, func() { s.forget(key) })
	})
}

// signedIn reports whether the user is signed in to a service.
func (s *shell) signedIn(source media.Source) bool {
	st := s.accounts()
	switch source {
	case media.SourceSpotify:
		return st.Spotify.SignedIn
	case media.SourceYouTube:
		return st.YouTube.SignedIn
	}
	return false
}

// keep saves or follows something on its service, then shows the page
// again.
func (s *shell) keep(key string, do func(ctx context.Context) error) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := do(ctx)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
			s.forget(key)
		})
	}()
}

func (s *shell) albumPage(c *ui.Context, id string) {
	key := "album:" + id
	res := fetch(s, key, func(ctx context.Context) (media.Album, error) { return s.svc.Catalog.Album(ctx, id) })
	switch {
	case !res.done:
		s.pageLoading(c, "album")
		return
	case res.err != nil:
		s.pageProblem(c, "album", res.err, key)
		return
	}
	a := res.value
	s.pageScroll(c, func() {
		s.pageHeader(c, a.Title, nil, nil)
		kind := string(a.Kind)
		if kind == "" {
			kind = "album"
		}
		n := a.TrackCount
		if n == 0 {
			n = len(a.Tracks)
		}
		facts := []fact{{text: kind}, {text: a.Year}, {artists: a.Artists}, {text: plural(n, "track")}}
		if len(a.Tracks) > 0 {
			facts = append(facts, fact{text: longDuration(total(a.Tracks))})
		}
		h := headProps{art: a.Artwork, source: a.Source, facts: facts,
			play:    func() { s.p.PlayTracks(a.Tracks, 0) },
			shuffle: func() { s.shuffled(a.Tracks) },
			menu:    s.collectionMenu(a.Title, a.Tracks, a.URL, a.Source),
		}
		if s.signedIn(a.Source) {
			saved := a.Saved
			h.keep, h.keepLabel, h.keptLabel = &saved, "save", "saved"
			h.onKeep = func(on bool) {
				s.keep(key, func(ctx context.Context) error {
					return s.svc.Collection.SetSaved(ctx, "album", a.ID, a.PlaylistID, on)
				})
			}
		}
		s.collectionHead(c, h)
		s.trackList(c, "album", a.Tracks, trackListOptions{})
	})
}

func (s *shell) playlistPage(c *ui.Context, id string) {
	key := "playlist:" + id
	res := fetch(s, key, func(ctx context.Context) (media.Playlist, error) { return s.svc.Catalog.Playlist(ctx, id) })
	switch {
	case !res.done:
		s.pageLoading(c, "playlist")
		return
	case res.err != nil:
		s.pageProblem(c, "playlist", res.err, key)
		return
	}
	p := res.value
	local := p.Source == media.SourceLocal
	s.pageScroll(c, func() {
		s.pageHeader(c, p.Title, nil, nil)
		owner := p.Owner
		if local {
			owner = "your mix"
		}
		n := p.TrackCount
		if n == 0 {
			n = len(p.Tracks)
		}
		facts := texts(owner, plural(n, "track"))
		if len(p.Tracks) > 0 {
			facts = append(facts, fact{text: longDuration(total(p.Tracks))})
		}
		menu := s.collectionMenu(p.Title, p.Tracks, p.URL, p.Source)
		if local {
			base := menu
			menu = func(m *ui.Menu) {
				base(m)
				m.Separator()
				if m.Item("Rename").Chosen() {
					s.renaming, s.renameTo, s.renameID = true, p.Title, p.ID
				}
				if m.Item("Delete").Chosen() {
					s.deleting, s.deleteID, s.deleteTitle = true, p.ID, p.Title
				}
			}
		}
		h := headProps{art: p.Artwork, source: p.Source, facts: facts, description: p.Description, menu: menu}
		if len(p.Tracks) > 0 {
			h.play = func() { s.p.PlayTracks(p.Tracks, 0) }
			h.shuffle = func() { s.shuffled(p.Tracks) }
		}
		if !local && !p.Editable && s.signedIn(p.Source) {
			saved := p.Saved
			h.keep, h.keepLabel, h.keptLabel = &saved, "save", "saved"
			h.onKeep = func(on bool) {
				s.keep(key, func(ctx context.Context) error { return s.svc.Collection.SetSaved(ctx, "playlist", p.ID, "", on) })
			}
		}
		s.collectionHead(c, h)
		noun := "playlist"
		if local {
			noun = "mix"
		}
		view := s.trackTools(c, key, noun, p.Tracks)
		empty := "This playlist is empty."
		switch {
		case s.searching(key) != "":
			empty = "Nothing in this " + noun + " matches “" + s.searching(key) + "”."
		case local:
			empty = "This mix is empty: add tracks of any service from their menu."
		}
		o := trackListOptions{art: true, album: true, empty: empty}
		switch {
		case local:
			o.menu = func(m *ui.Menu, i int, t media.Track) {
				m.Separator()
				if m.Item("Remove from this mix").Chosen() {
					// The list shown may be sorted or searched: the mix's own
					// place of the track is what goes.
					if err := s.svc.Collection.RemoveFromPlaylist(p.ID, view.indices[i]); err != nil {
						s.notes = append(s.notes, noticeOf(err))
					}
					s.forget(key)
				}
			}
		case p.Source == media.SourceYouTube && p.Editable && s.signedIn(media.SourceYouTube):
			o.menu = func(m *ui.Menu, i int, t media.Track) {
				m.Separator()
				if m.Item("Remove from this playlist").Chosen() {
					s.keep(key, func(ctx context.Context) error {
						if err := s.svc.Collection.RemoveFromYouTubePlaylist(ctx, p.ID, t); err != nil {
							return err
						}
						s.update(func() { s.notes = append(s.notes, notice("Removed "+t.Title+" from "+p.Title)) })
						return nil
					})
				}
			}
		}
		s.trackList(c, "playlist", view.tracks, o)
	})
	s.mixDialogs(c, key)
}

// mixDialogs rename and delete mixes.
func (s *shell) mixDialogs(c *ui.Context, key string) {
	k := &s.lk
	ui.Modal(c, &s.renaming, func() {
		ui.Text(c, "rename mix").FontSize(k.titleSize()).FontWeight(300).Font(k.display)
		in := ui.TextInput(c.Key("rename"), &s.renameTo).AutoFocus().Label("Name").Width(360)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if s.button(c, "rename-cancel", "cancel", false).Clicked() {
				s.renaming = false
			}
			if s.button(c, "rename-ok", "rename", true).Clicked() || in.Submitted() {
				if t := strings.TrimSpace(s.renameTo); t != "" {
					if err := s.svc.Collection.UpdatePlaylist(s.renameID, t, ""); err != nil {
						s.notes = append(s.notes, noticeOf(err))
					}
					s.forget(key)
				}
				s.renaming = false
			}
		})
	}).Padding(32).Gap(16)
	if ui.AlertDialog(c, &s.deleting, "Delete mix", "Delete "+s.deleteTitle+"? Its tracks stay where they came from.", "Cancel", "Delete") == 1 {
		if err := s.svc.Collection.DeletePlaylist(s.deleteID); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		} else {
			s.notes = append(s.notes, player.Notice{Message: "Deleted " + s.deleteTitle})
			s.navigate("/library/mixes")
		}
	}
}

// collectionMenu is the menu of an album or a playlist: play it, queue
// it, open it on its service.
func (s *shell) collectionMenu(title string, tracks []media.Track, rawURL string, source media.Source) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if len(tracks) > 0 {
			if m.Item("Play next").Chosen() {
				s.p.PlayNext(tracks)
				s.notes = append(s.notes, player.Notice{Message: title + " plays next"})
			}
			if m.Item("Add to queue").Chosen() {
				s.p.Enqueue(tracks)
				s.notes = append(s.notes, player.Notice{Message: "Added " + title + " to the queue"})
			}
			if m.Item("Start radio").Chosen() {
				s.startRadio(tracks[0])
			}
		}
		if rawURL != "" {
			if m.Item("Open on " + sourceName(source)).Chosen() {
				go s.svc.System.OpenExternal(rawURL)
			}
		}
	}
}

func (s *shell) artistPage(c *ui.Context, id string) {
	key := "artist:" + id
	res := fetch(s, key, func(ctx context.Context) (media.Artist, error) { return s.svc.Catalog.Artist(ctx, id) })
	switch {
	case !res.done:
		s.pageLoading(c, "artist")
		return
	case res.err != nil:
		s.pageProblem(c, "artist", res.err, key)
		return
	}
	a := res.value
	s.pageScroll(c, func() {
		s.pageHeader(c, a.Name, nil, nil)
		h := headProps{art: a.Artwork, round: true, source: a.Source, facts: texts(a.Subtitle), description: a.Description,
			playLabel: "play top songs",
			menu: func(m *ui.Menu) {
				if len(a.TopTracks) > 0 && m.Item("Start radio").Chosen() {
					s.startRadio(a.TopTracks[0])
				}
				if a.URL != "" && m.Item("Open on "+sourceName(a.Source)).Chosen() {
					go s.svc.System.OpenExternal(a.URL)
				}
			},
		}
		if len(a.TopTracks) > 0 {
			h.play = func() { s.p.PlayTracks(a.TopTracks, 0) }
		}
		if s.signedIn(a.Source) {
			following := a.Following
			h.keep, h.keepLabel, h.keptLabel = &following, "follow", "following"
			h.onKeep = func(on bool) {
				s.keep(key, func(ctx context.Context) error { return s.svc.Collection.SetFollowing(ctx, a.ID, on) })
			}
		}
		s.collectionHead(c, h)
		s.sectionTitle(c, "top songs")
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.trackList(c, "top", a.TopTracks, trackListOptions{art: true, album: true, empty: "No songs listed for this artist."})
		})
		if len(a.Albums) > 0 {
			s.shelf(c, "albums", "albums", func() {
				for i, al := range a.Albums {
					s.albumTile(c, "album-"+count(i), al)
				}
			})
		}
		if len(a.Singles) > 0 {
			s.shelf(c, "singles", "singles & eps", func() {
				for i, al := range a.Singles {
					s.albumTile(c, "single-"+count(i), al)
				}
			})
		}
		if len(a.Related) > 0 {
			s.sectionTitle(c, "fans also like")
			ui.Row(c).Wrap().Gap(8).Children(func() {
				for i, r := range a.Related {
					if r.ID == "" {
						continue
					}
					chip := ui.ButtonBase(c.Key("related-"+count(i))).Height(34).PaddingX(14).Border(2, s.lk.outline).
						Radius(s.lk.radiusSm).Label(r.Name).Cursor(ui.CursorPointer)
					if chip.Hovered() {
						chip.Background(s.lk.hover)
					}
					if chip.Clicked() {
						s.navigate(artistPath(r.ID))
					}
					chip.Children(func() { ui.Text(c, r.Name).FontSize(s.lk.smallSize()).SingleLine() })
				}
			})
		}
	})
}
