package app

import (
	"context"
	"fmt"
	"log"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

// Ordering the library (library/order.go): the mixes list as the user
// sorted them in the Library, pinned ones first, wherever they're listed:
// in the window, to plugins and to the assistants. Pins and hidden items
// of a service go once its whole library, listed under the account they
// were seen under, lacks them.

// sortedMixes returns the mixes, without their tracks, as the user sorted
// them, the pinned ones first.
func (a *App) sortedMixes() []media.Playlist {
	set := a.settings.Get()
	return a.library.SortedPlaylists(set.MixesSort, set.MixesDescending)
}

// MovePlaylist moves the mix id onto the mix onto, in the user's own
// order, among the mixes of among, which a search shows: every other mix
// keeps its place.
func (c *Collection) MovePlaylist(id, onto string, among []string) error {
	from, err := localID(id)
	if err != nil {
		return err
	}
	to, err := localID(onto)
	if err != nil {
		return err
	}
	var natives []string
	for _, m := range among {
		if n, err := localID(m); err == nil {
			natives = append(natives, n)
		}
	}
	if among == nil {
		natives = nil
	}
	return c.a.library.MovePlaylist(from, to, natives)
}

// Pins returns what the user pinned, in the order they put it.
func (c *Collection) Pins() []library.Mark { return c.a.library.Pins() }

// PinAt returns where the item id is among the pins, or -1.
func (c *Collection) PinAt(id string) int { return c.a.library.PinAt(id) }

// SetPinned pins an item, a mix or a playlist, album or artist of a
// service, first among the pins, or unpins it.
func (c *Collection) SetPinned(m library.Mark, pinned bool) error {
	m.Account = c.a.markAccount(m.ID)
	return c.a.library.SetPinned(m, pinned)
}

// MovePin moves the pin of id onto the pin of onto, among the pins of
// among, which a search shows: every other pin keeps its place.
func (c *Collection) MovePin(id, onto string, among []string) error {
	return c.a.library.MovePin(id, onto, among)
}

// Hidden returns the items of the services' libraries the user hid in
// MixMusic's, the last hidden first.
func (c *Collection) Hidden() []library.Mark { return c.a.library.Hidden() }

// IsHidden reports whether the user hid the item id from MixMusic's
// library.
func (c *Collection) IsHidden(id string) bool { return c.a.library.IsHidden(id) }

// SetHidden hides a playlist or an album of a service's library from
// MixMusic's, which unpins it, or shows it again: nothing changes on the
// service.
func (c *Collection) SetHidden(m library.Mark, hidden bool) error {
	m.Account = c.a.markAccount(m.ID)
	return c.a.library.SetHidden(m, hidden)
}

// markAccount names the account of the service of the item id the user
// is signed in to, as known without asking the service: "" when not
// known, which the next whole list that has the item fills in.
func (a *App) markAccount(id string) string {
	source, _, _ := media.ParseID(id)
	a.mu.Lock()
	spotifyMe, youtubeMe := a.spotifyMe, a.youtubeMe
	a.mu.Unlock()
	switch {
	case source == media.SourceSpotify && spotifyMe != nil && a.spotify.SignedIn():
		return "spotify:" + spotifyMe.ID
	case source == media.SourceYouTube && youtubeMe != nil && a.youtube.SignedIn():
		return youtubeKey(a.youtube.AuthUser(), a.youtube.Channel(), youtubeMe.Handle, youtubeMe.Name)
	}
	return ""
}

// listAccount names the account the library of source lists, asking the
// service once if need be: "" when signed out or not known.
func (a *App) listAccount(ctx context.Context, source media.Source) string {
	switch source {
	case media.SourceSpotify:
		if id := a.spotifyUserID(ctx); id != "" {
			return "spotify:" + id
		}
	case media.SourceYouTube:
		if me := a.youtubeAccount(ctx).Account; me != nil {
			return youtubeKey(a.youtube.AuthUser(), a.youtube.Channel(), me.Handle, me.Name)
		}
	}
	return ""
}

// youtubeKey names an account of YouTube Music: the Google account of the
// session, the channel chosen of it, and its handle, or its name.
func youtubeKey(user int, page, handle, name string) string {
	if handle == "" {
		handle = name
	}
	return fmt.Sprintf("youtube:%d/%s/%s", user, page, handle)
}

// keepListed has the pins and hidden items of kind of source follow a
// list of the library of source, which list read under the account
// before: only one that came whole, under the same account to its end,
// drops those it lacks. An app without a library, as some tests make,
// keeps none.
func keepListed[T any](a *App, ctx context.Context, source media.Source, kind, before string, list []T, err error, mark func(T) library.Mark) {
	if err != nil || before == "" || a.library == nil || a.listAccount(ctx, source) != before {
		return
	}
	listed := make([]library.Mark, len(list))
	for i, it := range list {
		listed[i] = mark(it)
	}
	var keep func(id string) bool
	if kind == library.MarkArtist {
		keep = a.library.FollowsArtist
	}
	if err := a.library.KeepListed(source, kind, before, listed, keep); err != nil {
		log.Printf("mixmusic: keeping the pins of %s's %ss: %v", source, kind, err)
	}
}

func playlistMark(p media.Playlist) library.Mark { return library.Mark{ID: p.ID, Title: p.Title} }
func albumMark(al media.Album) library.Mark      { return library.Mark{ID: al.ID, Title: al.Title} }
func artistMark(ar media.Artist) library.Mark    { return library.Mark{ID: ar.ID, Title: ar.Name} }
