package main

// Arranging Home: the user moves its shelves, hides them and adds shelves
// of their own, and settings.json keeps how, as homeShelves
// (settings.HomeShelf). Home tells its shelves apart by keys that stay the
// same in every language and from day to day: a service's own name for a
// shelf where it gives one (media.Shelf.Key), else its title. The shelves
// the arrangement doesn't name show after those it does, in their own
// order, as a service's new shelves do.

import (
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The keys of the shelves Home shows of its own, and of those the user
// adds: a playlist's and an artist's go on with their IDs, as
// "playlist:yt:PL…".
const (
	homeRecent    = "local:recent"
	homeMixes     = "local:mixes"
	homeReleases  = "local:releases"
	homeTopMonth  = "local:top-month"
	homeDownloads = "local:downloads"
	homePlaylist  = "playlist:"
	homeArtist    = "artist:"
)

// shelfKeys returns the keys Home tells shelves apart by, after their
// source: each one's name on its service, or else its title. A shelf
// whose key one before it took is told by its place among them, as "#2".
func shelfKeys(shelves []media.Shelf) []string {
	keys := make([]string, len(shelves))
	seen := map[string]int{}
	for i, sh := range shelves {
		key := string(sh.Source) + ":" + sh.Key
		if sh.Key == "" {
			key = string(sh.Source) + ":title:" + strings.TrimSpace(sh.Title)
		}
		seen[key]++
		if n := seen[key]; n > 1 {
			key += "#" + count(n)
		}
		keys[i] = key
	}
	return keys
}

// homeRow is a shelf of Home, as arranging it lists it: its key and its
// title.
type homeRow struct{ key, title string }

// arranged returns layout with the shelves of rows it doesn't place after
// those it does, in the order of rows, and the titles of those it places
// as rows have them now: what an arrangement of Home starts from, so that
// every shelf keeps its place once one moved. A shelf it places, unless
// hidden, goes once gone tells that its service's answer no longer has it.
func arranged(layout []settings.HomeShelf, rows []homeRow, gone func(key string) bool) []settings.HomeShelf {
	titles := make(map[string]string, len(rows))
	for _, r := range rows {
		titles[r.key] = r.title
	}
	out := make([]settings.HomeShelf, 0, len(layout)+len(rows))
	placed := make(map[string]bool, len(layout))
	for _, e := range layout {
		if title, ok := titles[e.Key]; ok {
			e.Title = title
		} else if !e.Hidden && gone(e.Key) {
			continue
		}
		placed[e.Key] = true
		out = append(out, e)
	}
	for _, r := range rows {
		if !placed[r.key] {
			placed[r.key] = true
			out = append(out, settings.HomeShelf{Key: r.key, Title: r.title})
		}
	}
	return out
}

// shelfAt returns where layout has the shelf of key, or -1.
func shelfAt(layout []settings.HomeShelf, key string) int {
	return slices.IndexFunc(layout, func(e settings.HomeShelf) bool { return e.Key == key })
}

// movedOnto returns layout with the shelf of key moved onto the shelf of
// onto, as a drag drops it: after it, from above it, and before it, from
// below.
func movedOnto(layout []settings.HomeShelf, key, onto string) []settings.HomeShelf {
	from, to := shelfAt(layout, key), shelfAt(layout, onto)
	if from < 0 || to < 0 || from == to {
		return layout
	}
	e := layout[from]
	out := slices.Delete(slices.Clone(layout), from, from+1)
	at := shelfAt(out, onto)
	if from < to {
		at++
	}
	return slices.Insert(out, at, e)
}

// withHidden returns layout with the shelf of key hidden, or shown again
// where it was.
func withHidden(layout []settings.HomeShelf, key string, hidden bool) []settings.HomeShelf {
	out := slices.Clone(layout)
	if i := shelfAt(out, key); i >= 0 {
		out[i].Hidden = hidden
	}
	return out
}

// withoutShelf returns layout without the shelf of key, which the user
// added.
func withoutShelf(layout []settings.HomeShelf, key string) []settings.HomeShelf {
	return slices.DeleteFunc(slices.Clone(layout), func(e settings.HomeShelf) bool { return e.Key == key })
}

// withAdded returns layout with the shelf of key, titled title, first and
// shown, where the user who added it sees it.
func withAdded(layout []settings.HomeShelf, key, title string) []settings.HomeShelf {
	out := withoutShelf(layout, key)
	return slices.Insert(out, 0, settings.HomeShelf{Key: key, Title: title})
}

// addedKind reports whether key names a shelf only the user adds, which
// shows only where they put it, and which they remove rather than hide.
func addedKind(key string) bool {
	return key == homeTopMonth || key == homeDownloads || strings.HasPrefix(key, homePlaylist) || strings.HasPrefix(key, homeArtist)
}

// keySource returns the source of the shelves key names: a service's, or
// MixMusic's own for the user's shelves.
func keySource(key string) media.Source {
	switch prefix, rest, _ := strings.Cut(key, ":"); prefix {
	case string(media.SourceSpotify):
		return media.SourceSpotify
	case string(media.SourceYouTube):
		return media.SourceYouTube
	case "playlist", "artist":
		if source, _, err := media.ParseID(rest); err == nil {
			return source
		}
	}
	return media.SourceLocal
}
