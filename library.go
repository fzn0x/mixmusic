package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// librarySections are the library's pivots, by their paths' last part.
var librarySections = []string{"liked", "mixes", "podcasts", "artists", "downloads", "computer", "history", "disliked", "spotify", "netease"}

// librarySectionLabel is the label of the pivot id of the library.
func librarySectionLabel(id string) string {
	switch id {
	case "liked":
		return locales.LibraryLiked()
	case "mixes":
		return locales.LibraryMixes()
	case "podcasts":
		return locales.LibraryPodcasts()
	case "artists":
		return locales.LibraryArtists()
	case "downloads":
		return locales.RailDownloads()
	case "computer":
		return locales.LibraryOnThisComputer()
	case "history":
		return locales.LibraryHistory()
	case "disliked":
		return locales.LibraryDisliked()
	case "spotify":
		return "Spotify"
	case "netease":
		return "网易云音乐"
	}
	return id
}

// libraryPage shows liked songs, the user's mixes, the history, the songs
// they dislike once there are any, and the libraries of the services the
// user signed in to.
func (s *shell) libraryPage(c *ui.Context, tab string) {
	accounts := s.accounts()
	disliked := s.svc.Collection.Disliked()
	var ids, labels []string
	for _, sec := range librarySections {
		if (sec == "spotify" && !accounts.Spotify.SignedIn) || (sec == "netease" && !accounts.NetEase.SignedIn) ||
			(sec == "disliked" && len(disliked) == 0) {
			continue
		}
		ids = append(ids, sec)
		labels = append(labels, librarySectionLabel(sec))
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
			case "podcasts":
				counts[i] = len(s.svc.Collection.Podcasts())
			case "artists":
				counts[i] = len(s.svc.Collection.FollowedArtists())
			case "downloads":
				counts[i] = s.kept.Status().Tracks
			case "computer":
				counts[i] = len(s.svc.LocalFiles.Tracks())
			case "disliked":
				counts[i] = len(disliked)
			default:
				counts[i] = -1
			}
		}
		s.pageHeaderCounts(c, locales.RailLibrary(), labels, counts, &selected)
		if selected != before {
			s.router.Replace("/library/" + ids[selected])
		}
		switch ids[selected] {
		case "liked":
			s.likedSongs(c)
		case "mixes":
			s.mixTiles(c)
		case "podcasts":
			s.podcastLibrary(c)
		case "artists":
			s.artistLibrary(c)
		case "downloads":
			s.downloadsLibrary(c)
		case "computer":
			s.computerLibrary(c)
		case "history":
			s.history(c)
		case "disliked":
			s.dislikedSongs(c, disliked)
		case "spotify":
			s.spotifyLibrary(c)
		case "netease":
			s.neteaseLibrary(c)
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
		ui.Text(c, k.words(locales.LibraryNothingLiked())).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, locales.LibraryLikedHint()).
			FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640)
		return
	}
	s.likedHead(c, liked)
	view := s.trackTools(c, "liked", "list", liked)
	s.trackList(c, "liked", view.tracks, trackListOptions{art: true, album: true,
		empty: locales.LibraryNoLikedMatch(s.searching("liked"))})
}

// likedHead plays, shuffles and downloads the liked songs, as a
// collection's head does, and tells the top bar of it (headed).
func (s *shell) likedHead(c *ui.Context, liked []media.Track) {
	d := likedDownload(liked)
	h := barHead{title: locales.RailLikedSongs(), glyph: solidIcons["heart"], download: d,
		play:    func() { s.p.PlayAll(liked, false) },
		shuffle: func() { s.shuffled(liked) },
	}
	row := ui.Row(c).Gap(8).Wrap().Margin(0, 0, 16)
	s.headed(row, h)
	row.Children(func() {
		if s.button(c, "liked-play", locales.LibraryPlay(), true).Clicked() {
			h.play()
		}
		if s.button(c, "liked-shuffle", locales.LibraryShuffle(), false).Clicked() {
			h.shuffle()
		}
		s.downloadButton(c, d)
	})
}

// dislikedSongs lists the songs the user dislikes, whose menus take the
// dislike back.
func (s *shell) dislikedSongs(c *ui.Context, disliked []media.Track) {
	view := s.trackTools(c, "disliked", "list", disliked)
	s.trackList(c, "disliked", view.tracks, trackListOptions{art: true, album: true,
		empty: locales.LibraryNoDislikedMatch(s.searching("disliked"))})
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
	about := k.sentence(locales.LibraryNewMixAbout())
	t := s.buttonBase(c.Key("new-mix-tile")).Width(k.tileWidth()).Shrink(0).Column().AlignItems(ui.Start).Label(locales.RailNewMix()).Cursor(ui.CursorPointer)
	if !k.cards {
		t.Height(k.tileWidth()).Justify(ui.SpaceBetween).Padding(16).Background(k.tile(2)).Radius(k.radius).Children(func() {
			ui.Text(c, k.words(locales.RailNewMix())).FontSize(k.size(1.5)).FontWeight(k.titleWeight()).Font(k.display).TextColor(k.tileText)
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
			k.labelWeight(ui.Text(c, locales.RailNewMix()).FontSize(k.bodySize()).SingleLine())
			ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
		})
	})
	return t.Clicked()
}

func (s *shell) history(c *ui.Context) {
	k := &s.lk
	plays := s.svc.Collection.History(300)
	if len(plays) == 0 {
		ui.Text(c, k.words(locales.LibraryNoHistory())).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, locales.LibraryHistoryHint()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		return
	}
	tracks := make([]media.Track, len(plays))
	for i, p := range plays {
		tracks[i] = p.Track
	}
	ui.Row(c).Margin(0, 0, 16).Children(func() {
		if s.button(c, "clear-history", locales.LibraryClearHistory(), false).Clicked() {
			s.clearingHistory = true
		}
	})
	if s.confirm(c, "clear-history", &s.clearingHistory, locales.LibraryClearHistory(), locales.LibraryClearHistoryQuestion(), locales.LibraryClear()) {
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
			if s.link(c, "new-youtube-playlist", locales.LibraryNewPlaylist()) {
				s.promptName(locales.LibraryNewYouTubePlaylist(), locales.RailName(), "", locales.RailCreate(), func(name string) {
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
	// The playlists of others the user collaborates on have a shelf of
	// their own.
	if slices.ContainsFunc(playlists.value, func(p media.Playlist) bool { return !collaboration(p) }) || links != nil {
		s.shelfWith(c, key+"-playlists", locales.LibraryPlaylists(), links, func() {
			for i, p := range playlists.value {
				if !collaboration(p) {
					s.playlistTileWith(c, key+"-playlist-"+count(i), p, more)
				}
			}
		})
	}
	s.collaborationsShelf(c, key, playlists.value, more)
	if len(albums.value) > 0 {
		s.shelf(c, key+"-albums", locales.LibraryAlbums(), func() { s.albumTiles(c, key+"-album", albums.value) })
	}
	if len(artists.value) > 0 {
		s.shelf(c, key+"-artists", locales.LibraryArtistsFollowed(), func() {
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
			s.sectionTitle(c, locales.LibraryLikedOnSpotify())
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
			s.trackList(c, "spotify-liked", tracks, trackListOptions{art: true, album: true, empty: locales.LibraryNoSpotifyLiked()})
			if len(tracks) < first.value.Total {
				label := locales.LibraryLoadMore(count(len(tracks)), count(first.value.Total))
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
		s.youtubePlaylists(),
		fetch(s, "youtube:albums", col.YouTubeAlbums),
		fetch(s, "youtube:artists", col.YouTubeArtists),
		func(c *ui.Context) {
			s.youtubeSongsList(c)
			s.sectionTitle(c, locales.LibraryLikedOnYouTube())
			liked := fetch(s, "youtube:liked", col.YouTubeLiked)
			switch {
			case !liked.done:
				s.loading(c)
			case liked.err != nil:
				s.problem(c, liked.err, func() { s.forget("youtube:liked") })
			default:
				// The first page shows at once, and the rest as they come.
				r := s.readOn("youtube:liked", liked)
				s.readLine(c, r, "list")
				if len(r.p.Tracks) > 0 || !r.reading {
					s.trackList(c, "youtube-liked", r.p.Tracks, trackListOptions{art: true, album: true, empty: locales.LibraryNoYouTubeLiked()})
				}
			}
		})
}
