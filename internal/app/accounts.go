package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
)

// loginTimeout is how long signing in waits for the user in the browser.
const loginTimeout = 5 * time.Minute

// tokenSecret is the key of the Spotify token in the system's credential
// store.
const tokenSecret = "spotify-token"

// tokenStore keeps the Spotify token in the system's credential store, or
// in a file readable by the user alone where there is none.
type tokenStore struct{ file string }

func (s tokenStore) Load() (spotify.Token, error) {
	data, err := mygo.Secrets.Get(tokenSecret)
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, mygo.ErrSecretNotFound) {
		data, err = os.ReadFile(s.file)
	}
	if err != nil {
		return spotify.Token{}, err
	}
	var t spotify.Token
	return t, json.Unmarshal(data, &t)
}

func (s tokenStore) Save(t spotify.Token) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := mygo.Secrets.Set(tokenSecret, data); !errors.Is(err, errors.ErrUnsupported) {
		return err
	}
	if err := store.WriteFile(s.file, data); err != nil {
		return err
	}
	return os.Chmod(s.file, 0o600)
}

func (s tokenStore) Clear() error {
	err := mygo.Secrets.Delete(tokenSecret)
	if errors.Is(err, errors.ErrUnsupported) {
		err = nil
	}
	if rmErr := os.Remove(s.file); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = errors.Join(err, rmErr)
	}
	return err
}

// SpotifyAccount is the state of the Spotify account.
type SpotifyAccount struct {
	// Configured is set once the user entered a Client ID.
	Configured bool             `json:"configured"`
	SignedIn   bool             `json:"signedIn"`
	Profile    *spotify.Profile `json:"profile"`
	// RedirectURI is what the user registers in the developer dashboard.
	RedirectURI string `json:"redirectUri"`
	// Error tells why the profile could not be read, if it could not.
	Error string `json:"error,omitempty"`
	// Streaming is set when the sign-in lets Spotify play in MixMusic;
	// sign-ins of MixMusic 0.4.0 and before do not.
	Streaming bool `json:"streaming"`
}

// AccountsStatus is the state of the user's accounts.
type AccountsStatus struct {
	Spotify SpotifyAccount `json:"spotify"`
	YouTube YouTubeAccount `json:"youtube"`
	Lastfm  LastfmAccount  `json:"lastfm"`
}

// Accounts signs the user in and out of their music services.
type Accounts struct{ a *App }

// profileRetry is how long a profile that could not be read waits before
// it is read again. Every window asks for the accounts' state again once a
// reading tells it, so a profile read again at once would be read again
// and again, as when Spotify refuses the sign-in, and the windows told
// again and again (#50).
const profileRetry = time.Minute

// profileErrors is why the profiles read in the background could not be
// read, and when they were.
type profileErrors struct {
	spotify, youtube string
	at               time.Time
}

// waits reports whether a profile that could not be read, for err, waits
// before it is read again.
func (e profileErrors) waits(err string) bool {
	return err != "" && time.Since(e.at) < profileRetry
}

// Status returns the state of the user's accounts at once, without asking
// the services: whether the user is signed in, and the profiles read
// before. The main window waits for it before it shows anything, so the
// profiles not read yet are read in the background, and AccountsChanged
// brings them to every window. A profile that could not be read comes with
// why, and is read again after profileRetry.
func (s *Accounts) Status(ctx context.Context) AccountsStatus {
	return s.a.knownAccountsStatus()
}

// knownAccountsStatus is the state of the accounts as MixMusic knows it,
// without asking the services; it has the profiles it lacks read in the
// background.
func (a *App) knownAccountsStatus() AccountsStatus {
	a.mu.Lock()
	spotifyMe, youtubeMe, errs := a.spotifyMe, a.youtubeMe, a.profileErrs
	a.mu.Unlock()
	status := AccountsStatus{
		Spotify: a.spotifyAccount(),
		YouTube: YouTubeAccount{SignedIn: a.youtube.SignedIn(), Account: youtubeMe},
		Lastfm:  a.lastfmAccount(),
	}
	status.Spotify.Profile = spotifyMe
	read := false
	if status.Spotify.SignedIn && spotifyMe == nil {
		status.Spotify.Error = errs.spotify
		read = !errs.waits(errs.spotify)
	}
	if status.YouTube.SignedIn && youtubeMe == nil {
		status.YouTube.Error = errs.youtube
		read = read || !errs.waits(errs.youtube)
	}
	if read {
		a.readProfiles()
	}
	return status
}

// readProfiles reads the profiles of the accounts in the background, one
// reading at a time, and tells every window the accounts' state once read.
func (a *App) readProfiles() {
	if !a.readingProfiles.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer a.readingProfiles.Store(false)
		// Reading the status reads the profiles, and keeps them; it says
		// why of those it could not read.
		status := a.accountsStatus(context.Background())
		a.mu.Lock()
		a.profileErrs = profileErrors{spotify: status.Spotify.Error, youtube: status.YouTube.Error, at: time.Now()}
		a.mu.Unlock()
		notify()
	}()
}

// spotifyAccount is the state of the Spotify account, without its profile.
func (a *App) spotifyAccount() SpotifyAccount {
	return SpotifyAccount{
		Configured:  a.auth.ClientID() != "",
		SignedIn:    a.spotify.SignedIn(),
		RedirectURI: spotify.RedirectURI,
		Streaming:   a.spotify.Granted("streaming"),
	}
}

// accountsStatus is the state of the accounts with their profiles, which it
// asks the services for when it has not read them yet.
func (a *App) accountsStatus(ctx context.Context) AccountsStatus {
	sp := a.spotifyAccount()
	if sp.SignedIn {
		me, err := a.spotifyProfile(ctx)
		if err != nil {
			sp.Error = err.Error()
		} else {
			sp.Profile = &me
		}
	}
	return AccountsStatus{Spotify: sp, YouTube: a.youtubeAccount(ctx), Lastfm: a.lastfmAccount()}
}

// spotifyProfile returns the signed-in Spotify user, asking Spotify once.
func (a *App) spotifyProfile(ctx context.Context) (spotify.Profile, error) {
	a.mu.Lock()
	me := a.spotifyMe
	a.mu.Unlock()
	if me != nil {
		return *me, nil
	}
	p, err := a.spotify.Me(ctx)
	if err != nil {
		if errors.Is(err, spotify.ErrSignInExpired) {
			a.signOutSpotify()
		}
		return spotify.Profile{}, err
	}
	a.mu.Lock()
	a.spotifyMe = &p
	a.mu.Unlock()
	return p, nil
}

// spotifyUserID returns the ID of the signed-in Spotify user, or "".
func (a *App) spotifyUserID(ctx context.Context) string {
	if !a.spotify.SignedIn() {
		return ""
	}
	me, err := a.spotifyProfile(ctx)
	if err != nil {
		return ""
	}
	return me.ID
}

// SpotifyLogin signs the user in to Spotify in their browser, with the
// Client ID of Settings › Accounts.
func (s *Accounts) SpotifyLogin(ctx context.Context) (AccountsStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	t, err := s.a.auth.Login(ctx, mygo.Shell.OpenExternal)
	if err != nil {
		return AccountsStatus{}, err
	}
	s.a.spotify.SetToken(t)
	s.a.mu.Lock()
	s.a.spotifyMe, s.a.profileErrs.spotify = nil, ""
	s.a.mu.Unlock()
	// MixMusic's own device connects again, with the new sign-in.
	s.a.here.close()
	if err := s.a.tokens.Save(t); err != nil {
		return AccountsStatus{}, err
	}
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Show() // back from the browser
	}
	status := s.a.accountsStatus(ctx)
	notify()
	return status, nil
}

// SpotifyLogout signs the user out of Spotify.
func (s *Accounts) SpotifyLogout() error {
	return s.a.signOutSpotify()
}

func (a *App) signOutSpotify() error {
	a.spotify.SetToken(spotify.Token{})
	a.here.close()
	a.mu.Lock()
	a.spotifyMe, a.profileErrs.spotify = nil, ""
	a.mu.Unlock()
	err := a.tokens.Clear()
	notify()
	return err
}
