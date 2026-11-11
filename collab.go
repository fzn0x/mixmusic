package main

// Collaborating on playlists. On a playlist of the user's on YouTube
// Music, Invite collaborators, in its page's More menu, turns YouTube
// Music's collaboration on and copies its invite link, and Stop
// collaboration turns it off; on one of theirs on Spotify, Make
// collaborative does as Spotify's own apps do. The Spotify playlists of
// others that the user collaborates on have a shelf of their own in
// Library › Spotify, and take tracks from a track's menu, and moves on
// their page, as the user's own do. A mix's Share makes it an unlisted
// playlist on YouTube Music, and copies its link.

import (
	"context"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// The changes of playlists on their services that collaborating asks for;
// tests answer for the services.
var (
	inviteOnYouTube = func(s *shell, ctx context.Context, id string, unlist bool) (string, error) {
		return s.svc.Collection.InviteYouTubeCollaborators(ctx, id, unlist)
	}
	stopOnYouTube = func(s *shell, ctx context.Context, id string) error {
		return s.svc.Collection.StopYouTubeCollaboration(ctx, id)
	}
	collaborativeOnSpotify = func(s *shell, ctx context.Context, id string, on bool) error {
		return s.svc.Collection.SetSpotifyCollaborative(ctx, id, on)
	}
	moveOnSpotify = func(s *shell, ctx context.Context, id string, from, to int) error {
		return s.svc.Collection.MoveInSpotifyPlaylist(ctx, id, from, to)
	}
	addOnSpotify = func(s *shell, ctx context.Context, id string, tracks []media.Track) error {
		return s.svc.Collection.AddToSpotifyPlaylist(ctx, id, tracks)
	}
	shareOnYouTube = func(s *shell, ctx context.Context, title string, tracks []media.Track) (media.Playlist, error) {
		return s.svc.Collection.ShareOnYouTube(ctx, title, tracks)
	}
)

// collabWait is how long a change of a playlist's collaboration waits for
// its service.
const collabWait = 30 * time.Second

// collabKey is the key of s.keeping that is set while the service of the
// playlist id changes its collaboration, or moves one of its tracks.
func collabKey(id string) string { return "collab:" + id }

// collaborativeFact is the fact of a playlist's head that tells others add
// to it, or "".
func collaborativeFact(p media.Playlist) string {
	if p.Collaborative {
		return "collaborative"
	}
	return ""
}

// withCollab is menu, the More menu of the page of p, with the items that
// collaborate on p, when it's a playlist the user made on a service they
// are signed in to. p is the page's own, which the items change as the
// service answers.
func (s *shell) withCollab(menu func(m *ui.Menu), p *media.Playlist) func(m *ui.Menu) {
	if !p.Mine || p.Source == media.SourceLocal || p.Source == media.SourceFile || !s.signedIn(p.Source) {
		return menu
	}
	return func(m *ui.Menu) {
		menu(m)
		m.Separator()
		busy := s.keeping[collabKey(p.ID)]
		switch p.Source {
		case media.SourceYouTube:
			if m.Item("Invite collaborators").Disabled(busy).Chosen() {
				s.inviteCollaborators(p)
			}
			if p.Collaborative && m.Item("Stop collaboration").Disabled(busy).Chosen() {
				s.stopCollaboration(p)
			}
		case media.SourceSpotify:
			label := "Make collaborative"
			if p.Collaborative {
				label = "Stop collaboration"
			}
			if m.Item(label).Disabled(busy).Chosen() {
				s.setCollaborative(p, !p.Collaborative)
			}
		}
	}
}

// changeCollab runs change, a change of p's collaboration on its service,
// beside the window, and then done with its error, while the items that
// change it are greyed out.
func (s *shell) changeCollab(p *media.Playlist, change func(ctx context.Context) error, done func(err error)) {
	key := collabKey(p.ID)
	s.keeping[key] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), collabWait)
		defer cancel()
		err := change(ctx)
		s.update(func() {
			delete(s.keeping, key)
			done(err)
		})
	}()
}

// inviteCollaborators turns YouTube Music's collaboration on for p and
// copies its invite link. Collaborators can't open a private playlist,
// which it asks to make unlisted first.
func (s *shell) inviteCollaborators(p *media.Playlist) {
	if p.Privacy != media.Private {
		s.invite(p, false)
		return
	}
	s.ask("invite collaborators", "Collaborators can't open a private playlist. Inviting them makes "+p.Title+
		" unlisted: anyone with its link can open it, and it shows on none of your pages.", "invite", func() { s.invite(p, true) })
}

// invite is inviteCollaborators, which makes p unlisted first when unlist
// is set.
func (s *shell) invite(p *media.Playlist, unlist bool) {
	id, title := p.ID, p.Title
	var link string
	s.changeCollab(p, func(ctx context.Context) (err error) {
		link, err = inviteOnYouTube(s, ctx, id, unlist)
		return err
	}, func(err error) {
		if err != nil {
			s.notes = append(s.notes, player.Notice{Message: "Couldn't invite collaborators to " + title + ": " + err.Error(), Error: true})
			if unlist {
				// It may be unlisted now: its page tells.
				s.refetch("playlist:" + id)
			}
			return
		}
		p.Collaborative = true
		if unlist {
			p.Privacy = media.Unlisted
		}
		copyText(link)
		s.notes = append(s.notes, notice("Copied the invite link of "+title+": whoever opens it can add to it"))
	})
}

// stopCollaboration turns YouTube Music's collaboration off for p.
func (s *shell) stopCollaboration(p *media.Playlist) {
	id, title := p.ID, p.Title
	s.changeCollab(p, func(ctx context.Context) error { return stopOnYouTube(s, ctx, id) }, func(err error) {
		if err != nil {
			s.notes = append(s.notes, player.Notice{Message: "Couldn't stop collaboration on " + title + ": " + err.Error(), Error: true})
			return
		}
		p.Collaborative = false
		s.notes = append(s.notes, notice("Stopped collaboration on "+title))
	})
}

// setCollaborative makes p, a playlist of the user's on Spotify,
// collaborative, and copies its link, or makes it theirs alone again.
// Spotify keeps collaborative playlists off their owner's profile: for a
// playlist on it, it asks first.
func (s *shell) setCollaborative(p *media.Playlist, on bool) {
	id, title, link := p.ID, p.Title, p.URL
	run := func() {
		s.changeCollab(p, func(ctx context.Context) error { return collaborativeOnSpotify(s, ctx, id, on) }, func(err error) {
			switch {
			case err != nil && on:
				s.notes = append(s.notes, player.Notice{Message: "Couldn't make " + title + " collaborative: " + err.Error(), Error: true})
				return
			case err != nil:
				s.notes = append(s.notes, player.Notice{Message: "Couldn't stop collaboration on " + title + ": " + err.Error(), Error: true})
				return
			}
			p.Collaborative = on
			s.refetch("spotify:playlists")
			switch {
			case !on:
				s.notes = append(s.notes, notice("Stopped collaboration on "+title))
			case link != "":
				p.Privacy = media.Private
				copyText(link)
				s.notes = append(s.notes, notice("Made "+title+" collaborative: copied its link, for those who'll add to it"))
			default:
				p.Privacy = media.Private
				s.notes = append(s.notes, notice("Made "+title+" collaborative"))
			}
		})
	}
	if on && p.Privacy == media.Public {
		s.ask("collaborative playlist", "Spotify keeps collaborative playlists off your profile: "+title+
			" leaves it. Those you send its link to can then add to it and put it in order.", "make collaborative", run)
		return
	}
	run()
}

// collaboration reports whether p is a playlist of another account's that
// the user collaborates on.
func collaboration(p media.Playlist) bool { return p.Collaborative && !p.Mine && p.Editable }

// collaborationsShelf is the shelf of the playlists of others, among
// playlists, that the user collaborates on, in the library of the service
// of key, when there are any; more ends their tiles' menus, if set.
func (s *shell) collaborationsShelf(c *ui.Context, key string, playlists []media.Playlist, more func(m *ui.Menu, p media.Playlist)) {
	if !slices.ContainsFunc(playlists, collaboration) {
		return
	}
	s.shelf(c, key+"-collaborations", "Playlists you collaborate on", func() {
		for i, p := range playlists {
			if collaboration(p) {
				s.playlistTileWith(c, key+"-playlist-"+count(i), p, more)
			}
		}
	})
}

// spotifyPlaylists returns the playlists of the user's library on Spotify,
// which the library shows and the track menu adds to.
func (s *shell) spotifyPlaylists() *loaded[[]media.Playlist] {
	return fetch(s, "spotify:playlists", s.svc.Collection.SpotifyPlaylists)
}

// keepSpotifyPlaylists fetches the user's playlists on Spotify once they're
// signed in, before any track menu opens, as keepYouTubePlaylists does.
func (s *shell) keepSpotifyPlaylists() {
	if s.accounts().Spotify.SignedIn {
		s.spotifyPlaylists()
	}
}

// addToSpotifyMenu is the track menu's way to add Spotify tracks to a
// playlist on Spotify, the user's or one they collaborate on, which tells
// whose it is. Spotify's playlists hold Spotify's tracks alone.
func (s *shell) addToSpotifyMenu(m *ui.Menu, tracks []media.Track) {
	if len(tracks) == 0 || !s.accounts().Spotify.SignedIn ||
		slices.ContainsFunc(tracks, func(t media.Track) bool { return t.Source != media.SourceSpotify || t.IsEpisode() }) {
		return
	}
	m.Submenu("Add to Spotify playlist", func(m *ui.Menu) {
		own := s.spotifyPlaylists()
		switch {
		case !own.done:
			m.Item("Loading your playlists…").Disabled(true)
			return
		case own.err != nil:
			own.refresh()
			m.Item("Couldn't load your playlists").Disabled(true)
			return
		}
		some := false
		for _, p := range own.value {
			if !p.Editable {
				continue
			}
			some = true
			label := p.Title
			if !p.Mine && p.Owner != "" {
				label += " · " + p.Owner
			}
			if m.Item(label).Chosen() {
				s.addToSpotifyPlaylist(p, tracks)
			}
		}
		if !some {
			m.Item("No playlists you can add to").Disabled(true)
		}
	})
}

// addToSpotifyPlaylist adds tracks to p, a playlist on Spotify the user
// may change.
func (s *shell) addToSpotifyPlaylist(p media.Playlist, tracks []media.Track) {
	what := tracks[0].Title
	if len(tracks) > 1 {
		what = plural(len(tracks), "track")
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), collabWait)
		defer cancel()
		err := addOnSpotify(s, ctx, p.ID, tracks)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.added(p, what, false)
		})
	}()
}

// spotifyMovable reports whether the page moves the tracks of p, as read
// in r, on Spotify: a playlist the user may change, signed in, read whole,
// each of whose items the page shows, as Spotify counts their places, and
// none of which moves meanwhile.
func (s *shell) spotifyMovable(r *playlistRead, p media.Playlist) bool {
	return p.Source == media.SourceSpotify && p.Editable && s.signedIn(media.SourceSpotify) &&
		len(r.p.Tracks) == r.p.TrackCount && !s.keeping[collabKey(p.ID)]
}

// moveOnSpotifyPlaylist moves the track at from of p, a playlist on
// Spotify the user may change, which the page of key shows as read in r,
// to to: the page shows it there at once, and Spotify hears of it. Until
// it answers, the page moves no other. Refused, the page reads the
// playlist again, as Spotify keeps it.
func (s *shell) moveOnSpotifyPlaylist(key string, r *playlistRead, p media.Playlist, from, to int) {
	if from < 0 || from >= len(r.p.Tracks) || to < 0 || to >= len(r.p.Tracks) {
		return
	}
	r.p.Tracks = movedTo(r.p.Tracks, from, to)
	busy := collabKey(p.ID)
	s.keeping[busy] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), collabWait)
		defer cancel()
		err := moveOnSpotify(s, ctx, p.ID, from, to)
		s.update(func() {
			delete(s.keeping, busy)
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				s.refetch(key)
			}
		})
	}()
}

// mixShare is the share of the mix or the playlist p, whose tracks are
// tracks once they're all read: a mix's makes it a playlist on YouTube
// Music, while the user is signed in there, to share its link.
func (s *shell) mixShare(p media.Playlist, tracks []media.Track) shareTarget {
	t := playlistShare(p)
	if p.Source == media.SourceLocal && len(tracks) > 0 && s.signedIn(media.SourceYouTube) {
		t.onService = func(m *ui.Menu) {
			if m.Item("As a YouTube Music playlist…").Chosen() {
				s.promptName("Share as a YouTube Music playlist", "Name", p.Title, "share", func(name string) {
					s.shareMixOnYouTube(name, tracks)
				})
			}
		}
	}
	return t
}

// shareMixOnYouTube makes a playlist of tracks on YouTube Music named
// title, which anyone with its link can open, copies its link, and opens
// its page, where Invite collaborators lets others add to it.
func (s *shell) shareMixOnYouTube(title string, tracks []media.Track) {
	if slices.ContainsFunc(tracks, func(t media.Track) bool { return t.Source != media.SourceYouTube }) {
		s.notes = append(s.notes, notice("Finding the tracks on YouTube Music…"))
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		p, err := shareOnYouTube(s, ctx, title, tracks)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: "Couldn't share " + title + ": " + err.Error(), Error: true})
				return
			}
			s.keepPlaylist("youtube:playlists", p)
			copyText(p.URL)
			s.notes = append(s.notes, notice("Made "+p.Title+" on YouTube Music: copied its link, which anyone can open"))
			s.navigate(playlistPath(p.ID))
		})
	}()
}
