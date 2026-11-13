package main

// Home's shelves, in groups that come at once, and the order Home shows
// them in: the user's arrangement first (homelayout.go), then the rest of
// each group in its own order.

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// readHomePlaylist and readHomeArtist read the playlist and the artist of
// a shelf the user added; tests answer for the services.
var (
	readHomePlaylist = func(s *shell, ctx context.Context, id string) (media.Playlist, error) {
		return s.svc.Catalog.PlaylistStart(ctx, id)
	}
	readHomeArtist = func(s *shell, ctx context.Context, id string) (media.Artist, error) {
		return s.svc.Catalog.Artist(ctx, id)
	}
)

// homeAddedItems is how many items a shelf the user added shows at most:
// a playlist's first tracks, an artist's newest releases.
const homeAddedItems = 40

// homeGroup is a group of Home's shelves that come at once: the user's
// own, a service's, or a shelf the user added, which comes on its own and
// shows only where they put it.
type homeGroup struct {
	// key is what it fetches: a refresh or try again asks for it again.
	key string
	// source is the pivot that keeps it, or "" for every pivot.
	source  media.Source
	done    bool
	err     error
	shelves []homeShelf
	// added is set for a shelf the user added, titled title.
	added bool
	title string
}

// homeShelf is a shelf of Home: its key, the key of its state in the
// window, and what it shows; or, as the new releases do, how it shows.
type homeShelf struct {
	key, ui, title string
	// source is the pivot that keeps it, or "" for every pivot: the new
	// releases keep each pivot's own.
	source media.Source
	shelf  media.Shelf
	// build, if set, shows it, of the pivot's source, and returns how
	// many items it shows.
	build func(c *ui.Context, key string, source media.Source) int
}

// homeGroups returns Home's groups in their own order: the user's own
// shelves, then Spotify's and YouTube Music's, then those the user added
// in layout, which show only where they put them.
func (s *shell) homeGroups(layout []settings.HomeShelf) []*homeGroup {
	yours := &homeGroup{key: "home-yours", done: true}
	local := s.localShelves()
	for i, key := range shelfKeys(local) {
		sh := local[i]
		yours.shelves = append(yours.shelves, homeShelf{key: key, ui: "home-yours-" + count(i), title: sh.Title, source: sh.Source, shelf: sh})
	}
	// The new releases of the artists followed show at once too, as the
	// last run found them, until this run's look comes.
	if len(s.newReleases()) > 0 {
		yours.shelves = append(yours.shelves, homeShelf{key: homeReleases, ui: "home-releases", title: "New from artists you follow", build: s.releasesShelf})
	}
	groups := []*homeGroup{yours}
	// Each service's shelves come on their own: one that fails leaves the
	// others, and keeps its own of before on a refresh. Each group keeps
	// keys of its own, which shelves coming or going in another don't
	// move.
	for _, sv := range []struct {
		source media.Source
		key    string
		get    func(ctx context.Context) ([]media.Shelf, error)
	}{
		{media.SourceSpotify, "home:spotify", s.svc.Catalog.SpotifyHome},
		{media.SourceNetEase, "home:netease", s.svc.Catalog.NetEaseHome},
	} {
		l := fetch(s, sv.key, sv.get)
		g := &homeGroup{key: sv.key, source: sv.source, done: l.done, err: l.err}
		for i, key := range shelfKeys(l.value) {
			sh := l.value[i]
			g.shelves = append(g.shelves, homeShelf{key: key, ui: sv.key + "-" + count(i), title: sh.Title, source: sh.Source, shelf: sh})
		}
		groups = append(groups, g)
	}
	for _, e := range layout {
		if !e.Hidden && addedKind(e.Key) {
			groups = append(groups, s.addedGroup(e))
		}
	}
	return groups
}

// homePlace is a place on Home: a shelf, or a group that failed, which
// tells why, or one that still loads, which holds back those after it as
// they'd move once it came.
type homePlace struct {
	shelf *homeShelf
	group *homeGroup
}

// homePlaces returns what Home shows, of the pivot of source ("" for
// every one), in order: the shelves layout places, where it places them,
// then the others of each group, in their own order. A group that failed
// is told of once, at its first place; one that still loads ends them.
func homePlaces(layout []settings.HomeShelf, groups []*homeGroup, source media.Source) []homePlace {
	keeps := func(of media.Source) bool { return source == "" || of == "" || of == source }
	byKey := map[string]*homeShelf{}
	for _, g := range groups {
		for i := range g.shelves {
			byKey[g.shelves[i].key] = &g.shelves[i]
		}
	}
	placed := make(map[string]bool, len(layout))
	for _, e := range layout {
		placed[e.Key] = true
	}
	var out []homePlace
	told := map[*homeGroup]bool{}
	// reach tells whether the shelves of g may show: once it came, and
	// whether it failed, which it tells of first.
	reach := func(g *homeGroup) bool {
		if !g.done {
			out = append(out, homePlace{group: g})
			return false
		}
		if g.err != nil && !told[g] {
			told[g] = true
			out = append(out, homePlace{group: g})
		}
		return true
	}
	for _, e := range layout {
		g := groupOf(groups, e.Key)
		if e.Hidden || g == nil || !keeps(g.source) {
			continue
		}
		if !reach(g) {
			return out
		}
		if sh := byKey[e.Key]; sh != nil && keeps(sh.source) {
			out = append(out, homePlace{shelf: sh})
		}
	}
	for _, g := range groups {
		if g.added || !keeps(g.source) {
			continue
		}
		if !reach(g) {
			return out
		}
		for i := range g.shelves {
			if sh := &g.shelves[i]; !placed[sh.key] && keeps(sh.source) {
				out = append(out, homePlace{shelf: sh})
			}
		}
	}
	return out
}

// groupOf returns the group of groups the shelf of key comes in, or nil:
// the shelf the user added of key, or else the group of its source.
func groupOf(groups []*homeGroup, key string) *homeGroup {
	if addedKind(key) {
		for _, g := range groups {
			if g.added && g.key == key {
				return g
			}
		}
		return nil
	}
	source := keySource(key)
	for _, g := range groups {
		switch {
		case g.added:
		case source == media.SourceLocal && g.source == "", g.source == source:
			return g
		}
	}
	return nil
}

// addedGroup is the shelf the user added that e names, as it comes: a
// playlist's tracks, or a mix's, an artist's newest albums and singles,
// the tracks played most this month, or the downloads. A playlist's and
// an artist's are fetched by the keys of their pages, "playlist:" and
// "artist:" and their IDs, which their keys on Home are too.
func (s *shell) addedGroup(e settings.HomeShelf) *homeGroup {
	g := &homeGroup{key: e.Key, source: keySource(e.Key), done: true, added: true}
	title := e.Title
	var items []media.Item
	switch {
	case e.Key == homeTopMonth:
		title, items = "Most played this month", s.topMonthItems()
	case e.Key == homeDownloads:
		title, items = "Downloads", s.downloadedItems()
	case strings.HasPrefix(e.Key, homePlaylist):
		id := strings.TrimPrefix(e.Key, homePlaylist)
		if g.source == media.SourceLocal {
			var ok bool
			if title, items, ok = s.mixShelf(id); !ok {
				g.title = e.Title // a mix deleted since
				return g
			}
			break
		}
		l := fetch(s, e.Key, func(ctx context.Context) (media.Playlist, error) { return readHomePlaylist(s, ctx, id) })
		g.done, g.err = l.done, l.err
		if l.done && l.err == nil {
			title, items = l.value.Title, trackItems(l.value.Tracks)
		}
	case strings.HasPrefix(e.Key, homeArtist):
		id := strings.TrimPrefix(e.Key, homeArtist)
		l := fetch(s, e.Key, func(ctx context.Context) (media.Artist, error) { return readHomeArtist(s, ctx, id) })
		g.done, g.err = l.done, l.err
		if l.done && l.err == nil {
			title, items = "New from "+l.value.Name, newestItems(l.value)
		}
	}
	g.title = title
	if len(items) > homeAddedItems {
		items = items[:homeAddedItems]
	}
	if len(items) > 0 {
		g.shelves = []homeShelf{{key: e.Key, ui: "home-added-" + e.Key, title: title, source: g.source,
			shelf: media.Shelf{Title: title, Source: g.source, Items: items}}}
	}
	return g
}

// trackItems are tracks as the items of a shelf.
func trackItems(tracks []media.Track) []media.Item {
	items := make([]media.Item, len(tracks))
	for i := range tracks {
		items[i] = media.Item{Kind: media.ItemTrack, Track: &tracks[i]}
	}
	return items
}

// newestItems are the albums and singles of the artist a, the newest
// first, as the years their service tells have them.
func newestItems(a media.Artist) []media.Item {
	albums := append(slices.Clone(a.Singles), a.Albums...)
	slices.SortStableFunc(albums, func(x, y media.Album) int { return cmp.Compare(y.Year, x.Year) })
	items := make([]media.Item, len(albums))
	for i := range albums {
		items[i] = media.Item{Kind: media.ItemAlbum, Album: &albums[i]}
	}
	return items
}

// homeMemo is the title and the items of a shelf made of what changes
// seldom, and whether they could be made: they're made again once their
// version changes, or an hour passed.
type homeMemo struct {
	version uint64
	at      time.Time
	title   string
	items   []media.Item
	ok      bool
}

// memo returns the memo of key, made again by build once version changed
// or an hour passed.
func (s *shell) memo(key string, version uint64, build func() homeMemo) homeMemo {
	if s.home.memos == nil {
		s.home.memos = map[string]*homeMemo{}
	}
	m := s.home.memos[key]
	if m == nil || m.version != version || time.Since(m.at) > time.Hour {
		made := build()
		made.version, made.at = version, time.Now()
		m = &made
		s.home.memos[key] = m
	}
	return *m
}

// topMonthItems are the tracks played most these 30 days, as the stats
// count them, the most first.
func (s *shell) topMonthItems() []media.Item {
	return s.memo(homeTopMonth, s.svc.Collection.Version(), func() homeMemo {
		var tracks []media.Track
		for _, tp := range s.svc.Collection.Stats(30).TopTracks {
			tracks = append(tracks, tp.Track)
		}
		return homeMemo{items: trackItems(tracks), ok: true}
	}).items
}

// downloadedItems are the tracks downloaded, the last first.
func (s *shell) downloadedItems() []media.Item {
	return s.memo(homeDownloads, s.kept.Version(), func() homeMemo {
		return homeMemo{items: trackItems(slices.Clone(s.kept.Tracks())), ok: true}
	}).items
}

// mixShelf returns the title and the tracks of the mix of id, as the
// library has them, and whether it still has it.
func (s *shell) mixShelf(id string) (string, []media.Item, bool) {
	m := s.memo(homePlaylist+id, s.svc.Collection.Version(), func() homeMemo {
		p, err := s.svc.Catalog.PlaylistStart(context.Background(), id)
		return homeMemo{title: p.Title, items: trackItems(p.Tracks), ok: err == nil}
	})
	return m.title, m.items, m.ok
}
