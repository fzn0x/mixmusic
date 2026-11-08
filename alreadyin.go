package main

// A track a playlist holds already: YouTube Music refuses to add it again,
// and a mix leaves it out. The toast says "Already in" the playlist, with
// Add anyway, which adds it a second time, as YouTube Music's own does.

import (
	"context"
	"errors"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// addOnYouTube adds a track to a playlist of the user's on YouTube Music,
// and addAnywayOnYouTube adds one the playlist holds already a second
// time; tests answer for YouTube.
var (
	addOnYouTube = func(s *shell, ctx context.Context, playlistID string, t media.Track) error {
		return s.svc.Collection.AddToYouTubePlaylist(ctx, playlistID, t)
	}
	addAnywayOnYouTube = func(s *shell, ctx context.Context, playlistID string, t media.Track) error {
		return s.svc.Collection.AddToYouTubePlaylistAnyway(ctx, playlistID, t)
	}
)

// addWait is how long an add to a playlist on YouTube Music waits for it,
// which may first find a Spotify track there.
const addWait = time.Minute

// addToYouTubePlaylist adds t to p, a playlist of the user's on YouTube
// Music. One p holds already, the toast says so, with Add anyway.
func (s *shell) addToYouTubePlaylist(p media.Playlist, t media.Track) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), addWait)
		defer cancel()
		err := addOnYouTube(s, ctx, p.ID, t)
		s.update(func() {
			switch {
			case errors.Is(err, app.ErrAlreadyInPlaylist):
				s.alreadyIn(p, func() { s.addToYouTubePlaylistAnyway(p, t) })
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			default:
				s.added(p, t.Title, false)
			}
		})
	}()
}

// addToYouTubePlaylistAnyway adds t to p, a playlist of the user's on
// YouTube Music that holds it already, a second time.
func (s *shell) addToYouTubePlaylistAnyway(p media.Playlist, t media.Track) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), addWait)
		defer cancel()
		err := addAnywayOnYouTube(s, ctx, p.ID, t)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.added(p, t.Title, true)
		})
	}()
}

// toldMixAdd tells what an add of tracks to the mix p came to, as AddToMix
// answered: held are those it held already, which stayed out, and which
// the toast offers to add anyway, all of them when err says it held them
// all.
func (s *shell) toldMixAdd(p media.Playlist, tracks, held []media.Track, err error) {
	switch {
	case errors.Is(err, app.ErrAlreadyInPlaylist):
		s.alreadyIn(p, func() { s.addToMixAnyway(p, tracks) })
	case err != nil:
		s.notes = append(s.notes, noticeOf(err))
	case len(held) > 0:
		were := " were"
		if len(held) == 1 {
			were = " was"
		}
		s.actionToasts = append(s.actionToasts, actionToast{
			message: "Added " + plural(len(tracks)-len(held), "track") + " to " + p.Title + ": " + count(len(held)) + were + " in it already",
			action:  "add anyway",
			run:     func() { s.addToMixAnyway(p, held) },
		})
		s.forget("playlist:" + p.ID)
	default:
		what := plural(len(tracks), "track")
		if len(tracks) == 1 {
			what = tracks[0].Title
		}
		s.added(p, what, false)
	}
}

// addToMixAnyway adds tracks to the mix p, those it holds already too.
func (s *shell) addToMixAnyway(p media.Playlist, tracks []media.Track) {
	if err := s.svc.Collection.AddToMixAnyway(p.ID, tracks); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	what := plural(len(tracks), "track")
	if len(tracks) == 1 {
		what = tracks[0].Title
	}
	s.added(p, what, true)
}

// alreadyIn tells that p holds what was added already, with Add anyway,
// which anyway does.
func (s *shell) alreadyIn(p media.Playlist, anyway func()) {
	s.actionToasts = append(s.actionToasts, actionToast{message: "Already in " + p.Title, action: "add anyway", run: anyway})
}

// added tells that what went into p, again when p held it already, and
// has p's page show it the next time.
func (s *shell) added(p media.Playlist, what string, again bool) {
	message := "Added " + what + " to " + p.Title
	if again {
		message += " again"
	}
	s.notes = append(s.notes, notice(message))
	s.forget("playlist:" + p.ID)
}
