package app

import (
	"context"
	"errors"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// YouTubeChannels returns the channels the YouTube session can use YouTube
// Music as: each Google account it is signed in to, and the brand channels
// they manage. Current marks the one MixMusic uses; none is current when
// that one is no longer of the session.
func (s *Accounts) YouTubeChannels(ctx context.Context) ([]youtube.AccountChannel, error) {
	channels, err := s.a.youtube.Channels(ctx)
	if err != nil {
		return nil, err
	}
	page, user := s.a.youtube.Channel(), s.a.youtube.AuthUser()
	for i := range channels {
		channels[i].Current = channels[i].PageID == page && channels[i].AuthUser == user
	}
	return channels, nil
}

// SetYouTubeChannel makes MixMusic use YouTube Music as a Google account of
// the session, by its index (youtube.AccountChannel.AuthUser), and as a
// brand channel of it, by its page ID, or as the account itself with "":
// the home, library and likes are theirs. The account stays beside the
// session's cookies, the channel in the settings.
func (s *Accounts) SetYouTubeChannel(ctx context.Context, authUser int, pageID string) (AccountsStatus, error) {
	if !s.a.youtube.SignedIn() {
		return AccountsStatus{}, errNotSignedIn
	}
	set := s.a.settings.Get()
	set.YouTubeChannel = pageID
	if set = set.Clean(); set.YouTubeChannel != pageID || authUser < 0 {
		return AccountsStatus{}, errors.New("that is not a channel of the account")
	}
	// The client takes the account and its channel at once, as a brand
	// channel is of one account, and the session keeps the account.
	wasUser, wasPage := s.a.youtube.AuthUser(), s.a.youtube.Channel()
	s.a.youtube.SetAccount(authUser, pageID)
	if authUser != wasUser {
		if err := s.a.youtubeStore.Save(youtubeSession{s.a.youtube.Cookies(), authUser}); err != nil {
			s.a.youtube.SetAccount(wasUser, wasPage)
			return AccountsStatus{}, err
		}
	}
	// As an edit of settings.json would: the settings keep the channel the
	// client uses already, and the profile of the former one goes.
	if _, err := (&Preferences{s.a}).Update(func(set *settings.Settings) { set.YouTubeChannel = pageID }); err != nil {
		s.a.youtube.SetAccount(wasUser, wasPage)
		if authUser != wasUser {
			s.a.youtubeStore.Save(youtubeSession{s.a.youtube.Cookies(), wasUser})
		}
		return AccountsStatus{}, err
	}
	// The profile was the former account's, though the channel may be the
	// same: each account's own.
	s.a.mu.Lock()
	s.a.youtubeMe, s.a.profileErrs.youtube = nil, ""
	s.a.mu.Unlock()
	status := s.a.accountsStatus(ctx)
	notify()
	return status, nil
}
