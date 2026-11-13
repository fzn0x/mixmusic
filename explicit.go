package main

// Hide explicit content, in Settings › Playback › Explicit content (#427).
// The lists the services make leave out the songs, videos and albums
// marked explicit, one filter for each kind of list, as the pages read
// them: what a search found and what the search box suggests, the shelves
// (of Home, Explore, the moods and genres and the now playing page), an
// artist's page, the playlists and mixes that aren't the user's own, the
// new releases of the artists they follow and the songs suggested for a
// playlist. The user's own lists, the library and the queue list them,
// dimmed, with an "e" and a note, and the player passes over them
// (internal/player/explicit.go). Radios leave them out in the app.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// hides reports whether the settings hide the track t: an explicit song or
// video, while they hide explicit content.
func (s *shell) hides(t media.Track) bool { return s.set.HideExplicit && t.ExplicitSong() }

// hidesAlbum reports whether the settings hide the album a.
func (s *shell) hidesAlbum(a media.Album) bool { return s.set.HideExplicit && a.ExplicitAlbum() }

// shownResults are what a search found, without what the settings hide.
func (s *shell) shownResults(r media.SearchResults) media.SearchResults {
	if !s.set.HideExplicit {
		return r
	}
	return r.WithoutExplicit()
}

// shownItems are the items of a shelf, or suggested, without what the
// settings hide.
func (s *shell) shownItems(items []media.Item) []media.Item {
	if !s.set.HideExplicit {
		return items
	}
	return media.WithoutExplicitItems(items)
}

// shownShelf is the shelf sh without what the settings hide, and whether
// it still shows: one of nothing else doesn't.
func (s *shell) shownShelf(sh media.Shelf) (media.Shelf, bool) {
	items := s.shownItems(sh.Items)
	if len(items) == len(sh.Items) {
		return sh, true
	}
	sh.Items = items
	return sh, len(items) > 0
}

// shownArtist is an artist's page without what the settings hide.
func (s *shell) shownArtist(a media.Artist) media.Artist {
	if !s.set.HideExplicit {
		return a
	}
	return a.WithoutExplicit()
}

// shownList is a playlist's tracks without those the settings hide, kept
// by the loads beside the page's own: the same list from frame to frame,
// which the track tools' view keeps, made again once the tracks change.
type shownList struct{ of, shown []media.Track }

// shownPlaylist returns tracks, of the playlist p of key, without those
// the settings hide, unless p is the user's own: a mix of theirs, or a
// playlist they may change, which lists them dimmed.
func (s *shell) shownPlaylist(key string, p media.Playlist, tracks []media.Track) []media.Track {
	if !s.set.HideExplicit || p.Source == media.SourceLocal || p.Editable {
		return tracks
	}
	m, _ := s.loads[key+" shown"].(*shownList)
	if m == nil || len(m.of) != len(tracks) || len(tracks) > 0 && &m.of[0] != &tracks[0] {
		m = &shownList{of: tracks, shown: media.WithoutExplicit(tracks)}
		s.loads[key+" shown"] = m
	}
	return m.shown
}

// shownVersions are the versions of an album that its page's switch
// offers, of g as their list gave them, and versions as their pages tell
// them where read: those the settings don't hide.
func (s *shell) shownVersions(g, versions []media.Album) ([]media.Album, []media.Album) {
	if !s.set.HideExplicit {
		return g, versions
	}
	explicit := media.ExplicitVersions(versions)
	var keptG, keptVersions []media.Album
	for i := range g {
		if !explicit[i] {
			keptG, keptVersions = append(keptG, g[i]), append(keptVersions, versions[i])
		}
	}
	return keptG, keptVersions
}

// shownGroup is g, the versions of an album as their list gave them,
// without those the settings hide.
func (s *shell) shownGroup(g []media.Album) []media.Album {
	shown, _ := s.shownVersions(g, g)
	return shown
}

// explicitBadge is the "e" of an explicit track, in ink.
func (s *shell) explicitBadge(c *ui.Context, ink ui.Color) {
	ui.Text(c, "e").FontSize(s.lk.captionSize()).TextColor(ink).Padding(0, 4).Border(1, ink).
		Radius(2).Label(locales.ExplicitTagLabel()).Shrink(0)
}

// hiddenNote is the note of a row of a track the settings hide.
func hiddenNote() string { return locales.ExplicitTagHidden() }

// explicitMark marks the row of an explicit track, after its title: its
// "e".
func (s *shell) explicitMark(c *ui.Context, t media.Track, ink ui.Color) {
	if t.Explicit {
		s.explicitBadge(c, ink)
	}
}

// artistLine is the line under the title of a track's row, of key: its
// artists, and while the settings hide it, a note that they do.
func (s *shell) artistLine(c *ui.Context, key string, t media.Track, ink ui.Color) {
	if !s.hides(t) {
		s.artistLinks(c, key, t.Artists, ink)
		return
	}
	k := &s.lk
	ui.Row(c).MinWidth(0).Gap(k.smallSize() / 4).Children(func() {
		s.artistLinks(c, key, t.Artists, ink)
		ui.Text(c, "·").FontSize(k.smallSize()).TextColor(ink).Shrink(0)
		ui.Text(c, hiddenNote()).FontSize(k.smallSize()).TextColor(ink).SingleLine().Shrink(0)
	})
}

// queueTitle is the title of an entry of the queue, with its "e" while the
// settings hide it.
func (s *shell) queueTitle(c *ui.Context, t media.Track, hidden bool, ink, sub ui.Color) {
	k := &s.lk
	if !hidden {
		ui.Text(c, t.Title).FontSize(k.bodySize()).TextColor(ink).SingleLine()
		return
	}
	ui.Row(c).Gap(6).MinWidth(0).Children(func() {
		ui.Text(c, t.Title).FontSize(k.bodySize()).TextColor(ink).SingleLine().MinWidth(0)
		s.explicitBadge(c, sub)
	})
}
