package main

// Following artists: on their service where the user signed in, else in
// MixMusic's own library, from the artist's page, its More menu and the
// menus of artists' tiles; and Library › Artists, which lists them.

import (
	"context"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// followOnService follows an artist on their service, or stops; tests
// answer for the services.
var followOnService = func(s *shell, ctx context.Context, id string, on bool) error {
	return s.svc.Collection.SetFollowing(ctx, id, on)
}

// followsArtist reports whether the user follows the artist a: in
// MixMusic, or on their service (followsOn).
func (s *shell) followsArtist(a media.Artist) bool {
	return s.svc.Collection.FollowsArtist(a.ID) || s.followsOn(a)
}

// followsOn reports whether the user follows the artist a on their
// service, signed in there: as the artist's page tells, once it came, or
// the artists followed there, as the library read them.
func (s *shell) followsOn(a media.Artist) bool {
	if !s.signedIn(a.Source) {
		return false
	}
	if page, ok := s.loads["artist:"+a.ID].(*loaded[media.Artist]); ok && page.done && page.err == nil {
		return page.value.Following
	}
	l, ok := s.loads[string(a.Source)+":artists"].(*loaded[[]media.Artist])
	return ok && l.done && l.err == nil && slices.ContainsFunc(l.value, func(x media.Artist) bool { return x.ID == a.ID })
}

// followArtist follows the artist a, or stops. Signed in to their service,
// the user follows them there; signed out, in MixMusic, which never
// follows them on the service on its own, even once the user signs in.
// Stopping stops in both, where the user follows them. The page of a
// says so at once, and goes back if the service refuses.
func (s *shell) followArtist(a media.Artist, on bool) {
	there := s.signedIn(a.Source) && (on || s.followsOn(a))
	if here := s.svc.Collection.FollowsArtist(a.ID); on && !there || !on && here {
		if err := s.svc.Collection.SetArtistFollowed(a, on); err != nil {
			s.notes = append(s.notes, noticeOf(err))
			return
		}
		s.releasesFollowed(a, on)
		if !there {
			if on {
				s.notes = append(s.notes, notice(a.Name+": following in MixMusic"))
			} else {
				s.notes = append(s.notes, notice(a.Name+": no longer following"))
			}
			return
		}
	}
	if !there {
		return
	}
	key := "artist:" + a.ID
	page, _ := s.loads[key].(*loaded[media.Artist])
	if page != nil && page.done {
		page.value.Following = on
	}
	s.keeping[key] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := followOnService(s, ctx, a.ID, on)
		s.update(func() {
			delete(s.keeping, key)
			if err != nil {
				if cur, _ := s.loads[key].(*loaded[media.Artist]); cur == page && page != nil && page.done {
					page.value.Following = !on
				}
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.followedOn(a, on)
			s.releasesFollowed(a, on)
			if on {
				s.notes = append(s.notes, notice(a.Name+": following on "+sourceName(a.Source)))
			} else {
				s.notes = append(s.notes, notice(a.Name+": no longer following"))
			}
		})
	}()
}

// followedOn writes into the artists followed on a's service, as fetched,
// that the user followed a there or stopped, rather than read the list
// again, which the service may answer as it was a moment before. A list
// still on its way, or that failed, is asked for again, once it's needed.
func (s *shell) followedOn(a media.Artist, on bool) {
	key := string(a.Source) + ":artists"
	l, ok := s.loads[key].(*loaded[[]media.Artist])
	switch {
	case !ok:
	case !l.done || l.err != nil:
		s.refetch(key)
	default:
		followed := slices.DeleteFunc(slices.Clone(l.value), func(x media.Artist) bool { return x.ID == a.ID })
		if on {
			sum := media.Artist{ID: a.ID, Source: a.Source, Name: a.Name, Artwork: a.Artwork, URL: a.URL,
				TopTracks: []media.Track{}, Albums: []media.Album{}, Singles: []media.Album{}, Related: []media.Ref{}}
			followed = slices.Insert(followed, 0, sum)
		}
		l.value = followed
	}
}

// followItem is the item of an artist's menus that follows them, or stops
// (followArtist). It's greyed out while their service answers.
func (s *shell) followItem(m *ui.Menu, a media.Artist) {
	if a.Source == media.SourceFile {
		return // no releases to follow
	}
	on := !s.followsArtist(a)
	label := "Follow"
	if !on {
		label = "Unfollow"
	}
	if m.Item(label).Disabled(s.keeping["artist:"+a.ID]).Chosen() {
		s.followArtist(a, on)
	}
}

// artistLibrary is Library › Artists: the new releases of the artists the
// user follows, then the artists they follow in MixMusic, then those they
// follow on each service they signed in to, past those of MixMusic.
func (s *shell) artistLibrary(c *ui.Context) {
	k := &s.lk
	col := s.svc.Collection
	accounts := s.accounts()
	var services []serviceArtists
	if accounts.Spotify.SignedIn {
		services = append(services, serviceArtists{"spotify", "Followed on Spotify", fetch(s, "spotify:artists", col.SpotifyArtists)})
	}
	if accounts.YouTube.SignedIn {
		services = append(services, serviceArtists{"youtube", "Followed on YouTube Music", fetch(s, "youtube:artists", col.YouTubeArtists)})
	}
	here := col.FollowedArtists()
	if len(here) == 0 && len(services) == 0 {
		ui.Text(c, k.words("Nothing followed yet")).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, "Follow an artist from their page or their menu, and they wait here, kept in MixMusic: "+
			"their new albums and singles show here and on Home. Signed in to their service, you follow them there.").
			FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640).Margin(0, 0, 48)
		return
	}
	s.releasesShelf(c, "library-releases", "")
	if len(here) > 0 {
		s.sectionTitle(c, "Followed in MixMusic")
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.tileWrap(c, func() {
				for i, a := range here {
					s.artistTile(c, "followed-"+count(i), a)
				}
			})
		})
	}
	shown := len(here)
	for _, sv := range services {
		shown += s.serviceArtistShelf(c, sv)
	}
	if shown == 0 && !slices.ContainsFunc(services, func(sv serviceArtists) bool { return !sv.artists.done }) {
		s.emptyLine(c, "Follow an artist from their page or their menu, and they wait here.")
	}
}

// serviceArtists are the artists the user follows on a service, for
// Library › Artists.
type serviceArtists struct {
	key, title string
	artists    *loaded[[]media.Artist]
}

// serviceArtistShelf is a shelf of the artists followed on a service,
// past those followed in MixMusic, which show above. It returns how many
// it shows.
func (s *shell) serviceArtistShelf(c *ui.Context, sv serviceArtists) int {
	switch l := sv.artists; {
	case !l.done:
		s.loading(c)
		return 0
	case l.err != nil:
		s.problem(c, l.err, func() { s.forget(sv.key + ":artists") })
		return 0
	}
	var shown []media.Artist
	for _, a := range sv.artists.value {
		if !s.svc.Collection.FollowsArtist(a.ID) {
			shown = append(shown, a)
		}
	}
	if len(shown) > 0 {
		s.shelf(c, sv.key+"-followed", sv.title, func() {
			for i, a := range shown {
				s.artistTile(c, sv.key+"-followed-"+count(i), a)
			}
		})
	}
	return len(shown)
}
