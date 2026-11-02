package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// librarySections are the library's pivots, by their paths' last part.
var librarySections = []struct{ id, label string }{
	{"liked", "Liked"}, {"mixes", "Mixes"}, {"history", "History"}, {"disliked", "Disliked"}, {"spotify", "Spotify"},
	{"youtube", "YouTube Music"},
}

// libraryPage shows liked songs, the user's mixes, the history, the songs
// they dislike once there are any, and the libraries of the services the
// user signed in to.
func (s *shell) libraryPage(c *ui.Context, tab string) {
	accounts := s.accounts()
	disliked := s.svc.Collection.Disliked()
	var ids, labels []string
	for _, sec := range librarySections {
		if (sec.id == "spotify" && !accounts.Spotify.SignedIn) || (sec.id == "youtube" && !accounts.YouTube.SignedIn) ||
			(sec.id == "disliked" && len(disliked) == 0) {
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
				counts[i] = len(s.svc.Collection.Liked())
			case "mixes":
				counts[i] = len(s.svc.Collection.Playlists())
			case "disliked":
				counts[i] = len(disliked)
			default:
				counts[i] = -1
			}
		}
		s.pageHeaderCounts(c, "Library", labels, counts, &selected)
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
		case "disliked":
			s.dislikedSongs(c, disliked)
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
		ui.Text(c, k.words("Nothing liked yet")).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, "Tap the heart on any track, from Spotify or YouTube Music: they all gather here, in one list.").
			FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640)
		return
	}
	view := s.trackTools(c, "liked", "list", liked)
	s.trackList(c, "liked", view.tracks, trackListOptions{art: true, album: true,
		empty: "No liked song matches “" + s.searching("liked") + "”."})
}

// dislikedSongs lists the songs the user dislikes, whose menus take the
// dislike back.
func (s *shell) dislikedSongs(c *ui.Context, disliked []media.Track) {
	view := s.trackTools(c, "disliked", "list", disliked)
	s.trackList(c, "disliked", view.tracks, trackListOptions{art: true, album: true,
		empty: "No disliked song matches “" + s.searching("disliked") + "”."})
}

func (s *shell) mixTiles(c *ui.Context) {
	s.tileWrap(c, func() {
		if s.newMixTile(c) {
			s.newMix()
		}
		for i, p := range s.svc.Collection.Playlists() {
			s.playlistTile(c, "mix-"+count(i), p)
		}
	})
}

// newMixTile makes a new mix when clicked: a flat tile in the Tiles
// layout, and in the layouts of cards a card as the mixes' beside it, a
// plus where their covers are. It returns whether it was clicked.
func (s *shell) newMixTile(c *ui.Context) bool {
	k := &s.lk
	about := k.sentence("tracks of every service, together")
	t := s.buttonBase(c.Key("new-mix-tile")).Width(k.tileWidth()).Shrink(0).Column().AlignItems(ui.Start).Label("New mix").Cursor(ui.CursorPointer)
	if !k.cards {
		t.Height(k.tileWidth()).Justify(ui.SpaceBetween).Padding(16).Background(k.tile(2)).Radius(k.radius).Children(func() {
			ui.Text(c, k.words("New mix")).FontSize(k.size(1.5)).FontWeight(k.titleWeight()).Font(k.display).TextColor(k.tileText)
			ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.tileText)
		})
		return t.Clicked()
	}
	fill := k.surface
	if t.Hovered() {
		fill = k.surface2
	}
	pad := k.cardPad()
	t.Gap(8).Padding(pad, pad, pad+4).Radius(k.radiusSm + pad).Background(fill).Children(func() {
		ui.Box(c).Size(k.cover, k.cover).Radius(k.radiusSm).Background(k.tile(2)).Center().Children(func() {
			ui.Icon(c, icon("plus")).FontSize(k.cover / 4).TextColor(k.tileText)
		})
		ui.Column(c).Width(k.cover).Gap(2).Children(func() {
			k.labelWeight(ui.Text(c, "New mix").FontSize(k.bodySize()).SingleLine())
			ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
		})
	})
	return t.Clicked()
}

func (s *shell) history(c *ui.Context) {
	k := &s.lk
	plays := s.svc.Collection.History(300)
	if len(plays) == 0 {
		ui.Text(c, k.words("No history yet")).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, "Tracks count once they played for thirty seconds.").FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
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
	if s.confirm(c, "clear-history", &s.clearingHistory, "clear history", "Forget every track you played? Your stats start over.", "clear") {
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
	// The tiles of YouTube Music's playlists delete those the user made,
	// or take out of the library those they saved: of its shelves, only
	// this one, which shows them gone at once.
	var more func(m *ui.Menu, p media.Playlist)
	if key == "youtube" {
		more = s.youtubeLibraryItems
	}
	if len(playlists.value) > 0 || links != nil {
		s.shelfWith(c, key+"-playlists", "Playlists", links, func() {
			for i, p := range playlists.value {
				s.playlistTileWith(c, key+"-playlist-"+count(i), p, more)
			}
		})
	}
	if len(albums.value) > 0 {
		s.shelf(c, key+"-albums", "Albums", func() {
			for i, a := range albums.value {
				s.albumTile(c, key+"-album-"+count(i), a)
			}
		})
	}
	if len(artists.value) > 0 {
		s.shelf(c, key+"-artists", "Artists you follow", func() {
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
			s.sectionTitle(c, "Liked on Spotify")
			first := fetch(s, "spotify:liked", func(ctx context.Context) (app.TrackPage, error) { return col.SpotifyLikedTracks(ctx, 0) })
			switch {
			case !first.done:
				s.loading(c)
				return
			case first.err != nil:
				s.problem(c, first.err, func() { s.forget("spotify:liked") })
				return
			}
			// The pages loaded past the first follow the first page that
			// came last: one of a refresh, a try again or another account
			// starts the list again.
			if s.spotifyLikedOf != first || s.spotifyLikedAt != first.answers {
				s.spotifyLiked, s.spotifyLikedOf, s.spotifyLikedAt = slices.Clip(first.value.Tracks), first, first.answers
			}
			tracks := s.spotifyLiked
			s.trackList(c, "spotify-liked", tracks, trackListOptions{art: true, album: true, empty: "No liked songs on Spotify."})
			if len(tracks) < first.value.Total {
				label := "load more · " + count(len(tracks)) + " of " + count(first.value.Total)
				if s.button(c, "spotify-more", label, false).Disabled(s.loadingMore).Clicked() {
					s.loadingMore = true
					go func(at, offset int) {
						page, err := col.SpotifyLikedTracks(context.Background(), offset)
						s.update(func() { s.tookSpotifyLiked(first, at, page, err) })
					}(first.answers, len(tracks))
				}
			}
		})
}

// tookSpotifyLiked adds a page of liked songs on Spotify that load more
// brought, unless the first page it came after is gone: answer at of
// first.
func (s *shell) tookSpotifyLiked(first *loaded[app.TrackPage], at int, page app.TrackPage, err error) {
	s.loadingMore = false
	switch {
	case err != nil:
		s.notes = append(s.notes, noticeOf(err))
	case s.spotifyLikedOf == first && s.spotifyLikedAt == at:
		s.spotifyLiked = append(s.spotifyLiked, page.Tracks...)
	}
}

func (s *shell) youtubeLibrary(c *ui.Context) {
	col := s.svc.Collection
	s.serviceLibrary(c, "youtube",
		fetch(s, "youtube:playlists", col.YouTubePlaylists),
		fetch(s, "youtube:albums", col.YouTubeAlbums),
		fetch(s, "youtube:artists", col.YouTubeArtists),
		func(c *ui.Context) {
			s.sectionTitle(c, "Liked on YouTube Music")
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
