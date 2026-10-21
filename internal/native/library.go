package native

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// librarySections are the library's pivots, by their paths' last part.
var librarySections = []struct{ id, label string }{
	{"liked", "liked"}, {"mixes", "mixes"}, {"history", "history"}, {"spotify", "spotify"}, {"youtube", "youtube music"},
}

// libraryPage shows liked songs, the user's mixes, the history, and the
// libraries of the services the user signed in to.
func (s *shell) libraryPage(c *ui.Context, tab string) {
	accounts := s.accounts()
	var ids, labels []string
	for _, sec := range librarySections {
		if (sec.id == "spotify" && !accounts.Spotify.SignedIn) || (sec.id == "youtube" && !accounts.YouTube.SignedIn) {
			continue
		}
		ids = append(ids, sec.id)
		labels = append(labels, sec.label)
	}
	selected := 0
	for i, id := range ids {
		if id == tab {
			selected = i
		}
	}
	s.pageScroll(c, func() {
		before := selected
		counts := make([]int, len(ids))
		for i, id := range ids {
			switch id {
			case "liked":
				counts[i] = len(s.svc.Collection.LikedIDs())
			case "mixes":
				counts[i] = len(s.svc.Collection.Playlists())
			default:
				counts[i] = -1
			}
		}
		s.pageHeaderCounts(c, "library", labels, counts, &selected)
		if selected != before {
			s.router.Replace("/library/" + ids[selected])
		}
		switch ids[selected] {
		case "liked":
			s.likedSongs(c)
		case "mixes":
			s.mixTiles(c)
		case "history":
			s.history(c)
		case "spotify":
			s.spotifyLibrary(c)
		case "youtube":
			s.youtubeLibrary(c)
		}
	})
}

// accounts returns the accounts' state, kept until they change.
func (s *shell) accounts() app.AccountsStatus {
	return fetch(s, "accounts", func(ctx context.Context) (app.AccountsStatus, error) { return s.svc.Accounts.Status(ctx), nil }).value
}

func (s *shell) likedSongs(c *ui.Context) {
	k := &s.lk
	liked := s.svc.Collection.Liked()
	if len(liked) == 0 {
		ui.Text(c, "nothing liked yet").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
		ui.Text(c, "Tap the heart on any track, from Spotify or YouTube Music: they all gather here, in one list.").
			FontSize(k.subtitleSize()).FontWeight(300).MaxWidth(640)
		return
	}
	view := s.trackTools(c, "liked", "list", liked)
	s.trackList(c, "liked", view.tracks, trackListOptions{art: true, album: true,
		empty: "No liked song matches “" + s.searching("liked") + "”."})
}

func (s *shell) mixTiles(c *ui.Context) {
	s.tileWrap(c, func() {
		new := ui.ButtonBase(c.Key("new-mix-tile")).Size(172, 172).Shrink(0).Column().AlignItems(ui.Start).Justify(ui.SpaceBetween).
			Padding(16).Background(s.lk.tile(2)).Radius(s.lk.radius).Label("New mix").Cursor(ui.CursorPointer)
		if new.Clicked() {
			s.newMix()
		}
		new.Children(func() {
			ui.Text(c, "new mix").FontSize(s.lk.size(1.5)).FontWeight(300).Font(s.lk.display).TextColor(s.lk.tileText)
			ui.Text(c, "tracks of every service, together").FontSize(s.lk.smallSize()).TextColor(s.lk.tileText)
		})
		for i, p := range s.svc.Collection.Playlists() {
			s.playlistTile(c, "mix-"+count(i), p)
		}
	})
}

func (s *shell) history(c *ui.Context) {
	k := &s.lk
	plays := s.svc.Collection.History(300)
	if len(plays) == 0 {
		ui.Text(c, "no history yet").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
		ui.Text(c, "Tracks count once they played for thirty seconds.").FontSize(k.subtitleSize()).FontWeight(300)
		return
	}
	tracks := make([]media.Track, len(plays))
	for i, p := range plays {
		tracks[i] = p.Track
	}
	ui.Row(c).Margin(0, 0, 16).Children(func() {
		if s.button(c, "clear-history", "clear history", false).Clicked() {
			s.clearingHistory = true
		}
	})
	if ui.AlertDialog(c, &s.clearingHistory, "Clear history", "Forget every track you played? Your stats start over.", "Cancel", "Clear") == 1 {
		if err := s.svc.Collection.ClearHistory(); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
	s.trackList(c, "history", tracks, trackListOptions{art: true, album: true})
}

// serviceLibrary shows a service's playlists, albums and followed artists,
// then its liked songs.
func (s *shell) serviceLibrary(c *ui.Context, key string,
	playlists *loaded[[]media.Playlist], albums *loaded[[]media.Album], artists *loaded[[]media.Artist],
	liked func(c *ui.Context)) {
	for _, l := range []error{playlists.err, albums.err, artists.err} {
		if l != nil {
			s.problem(c, l, func() {
				s.forget(key + ":playlists")
				s.forget(key + ":albums")
				s.forget(key + ":artists")
			})
			break
		}
	}
	if !playlists.done || !albums.done || !artists.done {
		s.loading(c)
	}
	// The user makes playlists on YouTube Music: theirs show, if none yet.
	var links func()
	if key == "youtube" && playlists.done && playlists.err == nil {
		links = func() {
			if s.link(c, "new-youtube-playlist", "new playlist") {
				s.promptName("New playlist on YouTube Music", "Name", "", "create", func(name string) {
					s.createYouTubePlaylist(name, nil, func(p media.Playlist) { s.navigate(playlistPath(p.ID)) })
				})
			}
		}
	}
	if len(playlists.value) > 0 || links != nil {
		s.shelfWith(c, key+"-playlists", "playlists", links, func() {
			for i, p := range playlists.value {
				s.playlistTile(c, key+"-playlist-"+count(i), p)
			}
		})
	}
	if len(albums.value) > 0 {
		s.shelf(c, key+"-albums", "albums", func() {
			for i, a := range albums.value {
				s.albumTile(c, key+"-album-"+count(i), a)
			}
		})
	}
	if len(artists.value) > 0 {
		s.shelf(c, key+"-artists", "artists you follow", func() {
			for i, a := range artists.value {
				s.artistTile(c, key+"-artist-"+count(i), a)
			}
		})
	}
	liked(c)
}

func (s *shell) spotifyLibrary(c *ui.Context) {
	col := s.svc.Collection
	s.serviceLibrary(c, "spotify",
		fetch(s, "spotify:playlists", col.SpotifyPlaylists),
		fetch(s, "spotify:albums", col.SpotifyAlbums),
		fetch(s, "spotify:artists", col.SpotifyArtists),
		func(c *ui.Context) {
			s.sectionTitle(c, "liked on spotify")
			first := fetch(s, "spotify:liked", func(ctx context.Context) (app.TrackPage, error) { return col.SpotifyLikedTracks(ctx, 0) })
			switch {
			case !first.done:
				s.loading(c)
				return
			case first.err != nil:
				s.problem(c, first.err, func() { s.forget("spotify:liked") })
				return
			}
			tracks := append(first.value.Tracks, s.spotifyMore...)
			s.trackList(c, "spotify-liked", tracks, trackListOptions{art: true, album: true, empty: "No liked songs on Spotify."})
			if len(tracks) < first.value.Total {
				label := "load more · " + count(len(tracks)) + " of " + count(first.value.Total)
				if s.button(c, "spotify-more", label, false).Disabled(s.loadingMore).Clicked() {
					s.loadingMore = true
					go func(offset int) {
						page, err := col.SpotifyLikedTracks(context.Background(), offset)
						s.update(func() {
							s.loadingMore = false
							if err != nil {
								s.notes = append(s.notes, noticeOf(err))
								return
							}
							s.spotifyMore = append(s.spotifyMore, page.Tracks...)
						})
					}(len(tracks))
				}
			}
		})
}

func (s *shell) youtubeLibrary(c *ui.Context) {
	col := s.svc.Collection
	s.serviceLibrary(c, "youtube",
		fetch(s, "youtube:playlists", col.YouTubePlaylists),
		fetch(s, "youtube:albums", col.YouTubeAlbums),
		fetch(s, "youtube:artists", col.YouTubeArtists),
		func(c *ui.Context) {
			s.sectionTitle(c, "liked on youtube music")
			liked := fetch(s, "youtube:liked", col.YouTubeLikedTracks)
			switch {
			case !liked.done:
				s.loading(c)
			case liked.err != nil:
				s.problem(c, liked.err, func() { s.forget("youtube:liked") })
			default:
				s.trackList(c, "youtube-liked", liked.value, trackListOptions{art: true, album: true, empty: "No liked songs on YouTube Music."})
			}
		})
}
