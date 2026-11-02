package app

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// errUnswitchable refuses a brand channel the list of the session's
// channels tells no page ID of.
var errUnswitchable = errors.New("YouTube doesn't let MixMusic switch to this channel")

// YouTubeChannels returns the channels the YouTube session can use YouTube
// Music as: each Google account it is signed in to, and the brand channels
// they manage. Current marks the one MixMusic uses; none is current when
// that one is no longer of the session. A channel MixMusic can't switch to
// is never current.
func (s *Accounts) YouTubeChannels(ctx context.Context) ([]youtube.AccountChannel, error) {
	channels, err := s.a.youtube.Channels(ctx)
	if err != nil {
		return nil, err
	}
	page, user := s.a.youtube.Channel(), s.a.youtube.AuthUser()
	for i, ch := range channels {
		channels[i].Current = !ch.Unswitchable && ch.PageID == page && ch.AuthUser == user
	}
	return channels, nil
}

// SetYouTubeChannel makes MixMusic use YouTube Music as ch, one of
// YouTubeChannels: a Google account of the session, by its index
// (AuthUser), and a brand channel of it, by its page ID, or the account
// itself without one. The home, library and likes are theirs. The account
// stays beside the session's cookies, the channel in the settings.
//
// Then YouTube Music must answer as ch, by its handle or its name: else
// MixMusic goes back to the account and channel it used before, and says
// whom YouTube Music answered as. A channel of no name isn't checked.
func (s *Accounts) SetYouTubeChannel(ctx context.Context, ch youtube.AccountChannel) (AccountsStatus, error) {
	if !s.a.youtube.SignedIn() {
		return AccountsStatus{}, errNotSignedIn
	}
	if ch.Unswitchable {
		return AccountsStatus{}, errUnswitchable
	}
	set := s.a.settings.Get()
	set.YouTubeChannel = ch.PageID
	if set = set.Clean(); set.YouTubeChannel != ch.PageID || ch.AuthUser < 0 {
		return AccountsStatus{}, errors.New("that is not a channel of the account")
	}
	wasUser, wasPage := s.a.youtube.AuthUser(), s.a.youtube.Channel()
	if err := s.a.useYouTubeAccount(ch.AuthUser, ch.PageID); err != nil {
		return AccountsStatus{}, err
	}
	status := s.a.accountsStatus(ctx)
	if me := status.YouTube.Account; me != nil && ch.Name != "" && !ch.Is(*me) {
		if err := s.a.useYouTubeAccount(wasUser, wasPage); err != nil {
			log.Printf("mixmusic: going back to the YouTube Music account used before: %v", err)
		}
		notify()
		return AccountsStatus{}, fmt.Errorf("YouTube Music answered as %s, not as %s: MixMusic goes on as before", me.Name, ch.Name)
	}
	notify()
	return status, nil
}

// useYouTubeAccount makes the client use YouTube Music as the Google
// account of index authUser of the session, and as its brand channel of
// pageID, or the account itself with "", and keeps them: the account
// beside the session, the channel in the settings. The profile read of the
// one before goes. On a failure, nothing changed.
func (a *App) useYouTubeAccount(authUser int, pageID string) error {
	// The client takes the account and its channel at once, as a brand
	// channel is of one account, and the session keeps the account.
	wasUser, wasPage := a.youtube.AuthUser(), a.youtube.Channel()
	a.youtube.SetAccount(authUser, pageID)
	if authUser != wasUser {
		if err := a.youtubeStore.Save(youtubeSession{a.youtube.Cookies(), authUser}); err != nil {
			a.youtube.SetAccount(wasUser, wasPage)
			return err
		}
	}
	// As an edit of settings.json would: the settings keep the channel the
	// client uses already, and the profile of the former one goes.
	if _, err := (&Preferences{a}).Update(func(set *settings.Settings) { set.YouTubeChannel = pageID }); err != nil {
		a.youtube.SetAccount(wasUser, wasPage)
		if authUser != wasUser {
			a.youtubeStore.Save(youtubeSession{a.youtube.Cookies(), wasUser})
		}
		return err
	}
	// The profile was the former account's, though the channel may be the
	// same: each account's own.
	a.mu.Lock()
	a.youtubeMe, a.profileErrs.youtube = nil, ""
	a.mu.Unlock()
	return nil
}
