package main

// A playlist of the user's on YouTube Music shows, below its tracks, the
// songs YouTube Music suggests adding to it, five at a time, with a
// refresh for five more. They're the page's own: playing the playlist
// leaves them out, and one the user adds joins its tracks.

import (
	"context"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// suggestOnYouTube reads the songs YouTube Music suggests for a playlist
// of the user's, and addSuggestionOnYouTube adds one to it; tests answer
// for YouTube.
var (
	suggestOnYouTube = func(s *shell, ctx context.Context, token string) (app.PlaylistSuggestions, error) {
		return s.svc.Collection.PlaylistSuggestions(ctx, token)
	}
	addSuggestionOnYouTube = func(s *shell, ctx context.Context, playlistID string, t media.Track) error {
		return s.svc.Collection.AddToYouTubePlaylist(ctx, playlistID, t)
	}
)

// suggested is what a playlist's page shows of its suggestions. The loads
// keep it under key, beside the playlist's fetch, and forget it with the
// account.
type suggested struct {
	key    string
	tracks []media.Track
	// refresh is the token of the next suggestions, "" when there are
	// none.
	refresh string
	// asking is set while suggestions are on their way; done once some
	// answer came, and err tells why the last failed, while none showed.
	asking, done bool
	err          error
	// adding holds the songs on their way into the playlist, by ID.
	adding map[string]bool
}

// suggestionsOf returns the suggestions of p, the playlist of key, and asks
// for the first ones the first time.
func (s *shell) suggestionsOf(key string, p media.Playlist) *suggested {
	key += " suggestions"
	if st, ok := s.loads[key].(*suggested); ok {
		return st
	}
	st := &suggested{key: key, adding: map[string]bool{}}
	s.loads[key] = st
	s.askSuggestions(st, p.Suggest)
	return st
}

// askSuggestions asks for the suggestions of token beside the page. Those
// shown stay until the new ones come, and stay if the ask fails, which a
// toast tells.
func (s *shell) askSuggestions(st *suggested, token string) {
	st.asking = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		got, err := suggestOnYouTube(s, ctx, token)
		s.update(func() {
			if s.loads[st.key] != st {
				return
			}
			st.asking = false
			switch {
			case err != nil && st.done && st.err == nil:
				s.notes = append(s.notes, player.Notice{Message: locales.SuggestionsRefreshFailed(err.Error()), Error: true})
			case err != nil:
				st.err, st.done = err, true
			default:
				st.tracks, st.refresh, st.err, st.done = got.Tracks, got.Refresh, nil, true
			}
		})
	}()
}

// playlistSuggestions shows the songs YouTube Music suggests adding to p,
// the playlist of key, below its tracks, once they're all read: for a
// playlist of the user's there, while they're signed in. Each plays alone,
// and Add puts it in the playlist. While the tracks are searched, or when
// YouTube Music suggests none, nothing shows.
func (s *shell) playlistSuggestions(c *ui.Context, key string, p media.Playlist) {
	if p.Source != media.SourceYouTube || !p.Editable || p.Suggest == "" || !s.signedIn(media.SourceYouTube) || s.searching(key) != "" {
		return
	}
	if r, ok := s.loads[key+" read"].(*playlistRead); ok && r.reading {
		return
	}
	st := s.suggestionsOf(key, p)
	if !st.done || st.err == nil && len(st.tracks) == 0 && st.refresh == "" && !st.asking {
		return
	}
	k := &s.lk
	ui.Column(c.Key("suggestions")).Margin(48, 0, 0).Children(func() {
		s.sectionHead(c, locales.SuggestionsTitle(), func() {
			switch {
			case st.asking:
				ui.Spinner(c).Size(16, 16).Label(locales.SuggestionsLoading())
			case st.err == nil && st.refresh != "" && s.linkOf(c.Key("suggestions-refresh"), k.sentence(locales.SuggestionsRefresh()), "").
				FontSize(k.smallSize()).Label(locales.SuggestionsRefreshLabel()).Clicked():
				s.askSuggestions(st, st.refresh)
			}
		})
		if st.err != nil {
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.SuggestionsLoadFailed(st.err.Error())).FontSize(k.bodySize()).TextColor(k.muted).Shrink(1).MinWidth(0)
				if !st.asking && s.link(c, "suggestions-retry", locales.SuggestionsTryAgain()) {
					s.askSuggestions(st, p.Suggest)
				}
			})
			return
		}
		for i, t := range st.tracks {
			if s.hides(t) {
				continue // the settings hide it (explicit.go)
			}
			ui.Row(c.Key("suggested-" + t.ID)).Gap(16).AlignItems(ui.Center).Children(func() {
				// A list of one: playing a suggestion plays it alone.
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					s.trackList(c, "suggestion-"+count(i), []media.Track{t}, trackListOptions{art: true, album: true})
				})
				add := s.button(c, "suggestion-add-"+t.ID, locales.SuggestionsAdd(), false).Label(locales.SuggestionsAddLabel(t.Title, p.Title)).
					AlignSelf(ui.Center).Disabled(st.adding[t.ID])
				if add.Clicked() {
					s.addSuggestion(key, st, p, t)
				}
			})
		}
	})
}

// addSuggestion adds t, a song suggested for p, the playlist of key, to
// it. Once YouTube Music did, the song leaves the suggestions, the next
// ones come when it was the last, and it joins the playlist's tracks.
func (s *shell) addSuggestion(key string, st *suggested, p media.Playlist, t media.Track) {
	st.adding[t.ID] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := addSuggestionOnYouTube(s, ctx, p.ID, t)
		s.update(func() {
			delete(st.adding, t.ID)
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, notice(locales.SuggestionsAdded(t.Title, p.Title)))
			if s.loads[st.key] == st {
				st.tracks = slices.DeleteFunc(slices.Clone(st.tracks), func(x media.Track) bool { return x.ID == t.ID })
				if len(st.tracks) == 0 && st.refresh != "" && !st.asking {
					s.askSuggestions(st, st.refresh)
				}
			}
			s.joinPlaylist(key, t)
		})
	}()
}

// joinPlaylist shows t, just added to the playlist of key, at its end, as
// YouTube Music adds it, while the page shows it whole, and reads the
// playlist again beside the page: the answer gives the track its place
// there, which taking it out needs.
func (s *shell) joinPlaylist(key string, t media.Track) {
	// The reading of the playlist's tracks, as readOn keeps it.
	if r, ok := s.loads[key+" read"].(*playlistRead); ok && !r.reading && r.err == nil && !r.p.Cut {
		r.p.Tracks = append(slices.Clone(r.p.Tracks), t)
		if r.p.TrackCount > 0 {
			r.p.TrackCount++
		}
		r.length += ms(t.DurationMs)
	}
	s.refetch(key)
}
