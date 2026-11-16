package main

// One tile for each album, however many versions of it its service lists
// (#345): its explicit and clean versions, and its editions, between which
// its page switches. media.GroupVersions tells which albums are versions
// of one.

import (
	"context"
	"slices"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// albumVersions is what the lists of albums found of their versions, and
// the version each album's page shows.
type albumVersions struct {
	// of are the versions of an album that a list showed as one tile, by
	// the ID of each version.
	of map[string][]media.Album
	// shown is the version the page of an album shows, by the album's
	// ID, once its switch or its versions' pages chose one: else, the
	// album itself.
	shown map[string]string
	// choosing holds the albums whose page opens on their preferred
	// version once their versions' pages told which is explicit: their
	// tiles opened them, of lists that didn't tell.
	choosing map[string]bool
	// lists are the albums of the lists shown, by page and list, with
	// their groups, made again once a list changes.
	lists map[string]groupedAlbums
}

// groupedAlbums are the IDs of a list's albums, and the albums grouped by
// album.
type groupedAlbums struct {
	ids    []string
	groups [][]media.Album
}

// albumGroups returns the albums of the list keyed key on the page being
// built, grouped by album as media.GroupVersions groups them. The versions
// of an album of several are kept by the ID of each, for its tile and its
// page.
func (s *shell) albumGroups(key string, albums []media.Album) [][]media.Album {
	v := &s.versions
	at := s.pagePath + " " + key
	if l, ok := v.lists[at]; ok && slices.EqualFunc(l.ids, albums, func(id string, a media.Album) bool { return id == a.ID }) {
		return l.groups
	}
	groups := media.GroupVersions(albums)
	ids := make([]string, len(albums))
	for i, a := range albums {
		ids[i] = a.ID
	}
	// The lists of the pages left are grouped again if they come back.
	if v.lists == nil || len(v.lists) >= 64 {
		v.lists = map[string]groupedAlbums{}
	}
	v.lists[at] = groupedAlbums{ids, groups}
	if v.of == nil {
		v.of = map[string][]media.Album{}
	}
	for _, g := range groups {
		if len(g) > 1 {
			for _, a := range g {
				v.of[a.ID] = g
			}
		}
	}
	return groups
}

// albumTiles are the tiles of a list of albums, keyed key and their
// place: one for each album, which shows the version it opens on.
func (s *shell) albumTiles(c *ui.Context, key string, albums []media.Album) {
	for i, g := range s.albumGroups(key, albums) {
		s.albumTile(c, key+"-"+count(i), s.preferredVersion(g))
	}
}

// preferredVersion is the version of an album that its tile shows and
// opens on, of its versions: the explicit one, or the clean one where
// Settings › Playback prefers clean versions; or the first, while they
// don't tell which is explicit.
func (s *shell) preferredVersion(versions []media.Album) media.Album {
	i, _ := media.PreferredVersion(versions, s.set.PrefersClean())
	return versions[i]
}

// versionsLabel tells how many versions the album id has, for its tile:
// "" for one.
func (s *shell) versionsLabel(id string) string {
	// Those the settings hide don't count (explicit.go).
	if n := len(s.shownGroup(s.versions.of[id])); n > 1 {
		return locales.VersionsCount(n)
	}
	return ""
}

// openAlbum opens the page of the album id, on the version its tile shows,
// or, when its list didn't tell which of its versions is explicit, on the
// one their pages tell.
func (s *shell) openAlbum(id string) {
	v := &s.versions
	delete(v.shown, id)
	delete(v.choosing, id)
	if g := v.of[id]; len(g) > 1 && !media.VersionsTold(g) {
		if v.choosing == nil {
			v.choosing = map[string]bool{}
		}
		v.choosing[id] = true
	}
	s.navigate(albumPath(id))
}

// albumLoad fetches the album id, with its tracks, as its page shows it.
func (s *shell) albumLoad(id string) *loaded[media.Album] {
	return fetch(s, "album:"+id, func(ctx context.Context) (media.Album, error) { return s.svc.Catalog.Album(ctx, id) })
}

// versionShown returns the version that the page of the album id shows:
// the one its switch chose, else the one it opens on. waiting is set while
// that waits for the pages of its versions.
func (s *shell) versionShown(id string) (shown string, waiting bool) {
	v := &s.versions
	if sh := v.shown[id]; sh != "" {
		return sh, false
	}
	g := v.of[id]
	if !v.choosing[id] || len(g) < 2 {
		return id, false
	}
	versions, all := s.loadedVersions(g, true)
	if !all {
		return "", true
	}
	i, _ := media.PreferredVersion(versions, s.set.PrefersClean())
	delete(v.choosing, id)
	if v.shown == nil {
		v.shown = map[string]string{}
	}
	v.shown[id] = versions[i].ID
	return versions[i].ID, false
}

// loadedVersions returns versions as their pages tell them, where they
// came, and whether all of them came, or failed; load asks for those not
// asked for yet.
func (s *shell) loadedVersions(versions []media.Album, load bool) ([]media.Album, bool) {
	out, all := slices.Clone(versions), true
	for i, a := range versions {
		l, _ := s.loads["album:"+a.ID].(*loaded[media.Album])
		if load {
			l = s.albumLoad(a.ID)
		}
		switch {
		case l == nil || !l.done:
			all = false
		case l.err == nil:
			out[i] = l.value
		}
	}
	return out, all
}

// versionSwitch switches the page of the album id between its versions:
// shown is the one it shows. Versions whose list didn't tell which is
// explicit show it once their pages came.
func (s *shell) versionSwitch(c *ui.Context, id, shown string) {
	g := s.versions.of[id]
	if len(g) < 2 {
		return
	}
	told := media.VersionsTold(g)
	versions, all := s.loadedVersions(g, !told)
	if !told && !all {
		return
	}
	// The settings hiding explicit content, it leaves out the explicit
	// versions (explicit.go).
	if g, versions = s.shownVersions(g, versions); len(g) < 2 {
		return
	}
	at := slices.IndexFunc(g, func(a media.Album) bool { return a.ID == shown })
	labels := media.VersionLabels(versions)
	ui.Row(c).Margin(0, 0, 24).Children(func() {
		if s.segmented(c, "versions", locales.VersionsSwitch(), &at, labels...) && at >= 0 {
			if s.versions.shown == nil {
				s.versions.shown = map[string]string{}
			}
			s.versions.shown[id] = g[at].ID
		}
	})
}

// albumToPlay returns what reads the album id to play, as its tile's play
// button plays it: the album, or, when its list didn't tell which of its
// versions is explicit, the one their pages tell, as its page opens on. A
// page read already plays as it is.
func (s *shell) albumToPlay(id string) func(ctx context.Context) (media.Album, error) {
	g, clean, album := s.versions.of[id], s.set.PrefersClean(), s.svc.Catalog.Album
	if len(g) > 1 && !media.VersionsTold(g) {
		versions, all := s.loadedVersions(g, false)
		if !all {
			return s.preferredAlbum(g, clean)
		}
		i, _ := media.PreferredVersion(versions, clean)
		id = g[i].ID
	}
	if l, ok := s.loads["album:"+id].(*loaded[media.Album]); ok && l.done && l.err == nil {
		a := l.value
		return func(context.Context) (media.Album, error) { return a, nil }
	}
	return func(ctx context.Context) (media.Album, error) { return album(ctx, id) }
}

// preferredAlbum returns what reads the pages of versions, beside the main
// thread, and gives the one to open, as their tracks tell which is
// explicit: the clean one when clean is set.
func (s *shell) preferredAlbum(versions []media.Album, clean bool) func(ctx context.Context) (media.Album, error) {
	album := s.svc.Catalog.Album
	return func(ctx context.Context) (media.Album, error) {
		read, errs := slices.Clone(versions), make([]error, len(versions))
		var wg sync.WaitGroup
		for i, a := range versions {
			wg.Go(func() {
				if got, err := album(ctx, a.ID); err != nil {
					errs[i] = err
				} else {
					read[i] = got
				}
			})
		}
		wg.Wait()
		i, _ := media.PreferredVersion(read, clean)
		if errs[i] != nil {
			return media.Album{}, errs[i]
		}
		return read[i], nil
	}
}
