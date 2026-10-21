package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/store"
)

// lastfmSecret is the key, in the system's credential store, of what
// connects MixMusic to Last.fm: the API account and the user's session.
const lastfmSecret = "lastfm"

// lastfmPoll is how often connecting asks Last.fm whether the user approved.
const lastfmPoll = 2 * time.Second

// lastfmCredentials are the API account the user made on Last.fm and the
// session of their approval.
type lastfmCredentials struct {
	APIKey  string         `json:"apiKey"`
	Secret  string         `json:"secret"`
	Session lastfm.Session `json:"session"`
}

// lastfmStore keeps the credentials like the Spotify token: in the
// system's credential store, or in a file readable by the user alone.
type lastfmStore struct{ file string }

func (s lastfmStore) Load() (lastfmCredentials, error) {
	data, err := mygo.Secrets.Get(lastfmSecret)
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, mygo.ErrSecretNotFound) {
		data, err = os.ReadFile(s.file)
	}
	if err != nil {
		return lastfmCredentials{}, err
	}
	var c lastfmCredentials
	return c, json.Unmarshal(data, &c)
}

func (s lastfmStore) Save(c lastfmCredentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := mygo.Secrets.Set(lastfmSecret, data); !errors.Is(err, errors.ErrUnsupported) {
		return err
	}
	if err := store.WriteFile(s.file, data); err != nil {
		return err
	}
	return os.Chmod(s.file, 0o600)
}

func (s lastfmStore) Clear() error {
	err := mygo.Secrets.Delete(lastfmSecret)
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, mygo.ErrSecretNotFound) {
		err = nil
	}
	if rmErr := os.Remove(s.file); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = errors.Join(err, rmErr)
	}
	return err
}

// LastfmAccount is the state of the Last.fm connection.
type LastfmAccount struct {
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
	// Expired is set when Last.fm no longer accepts the session.
	Expired bool `json:"expired,omitempty"`
}

// scrobbles is a scrobbler and where its queue of plays to send is kept.
type scrobbles struct {
	client    *lastfm.Client
	scrobbler *lastfm.Scrobbler
	name      string
	expired   bool
}

// openLastfm connects to Last.fm with the credentials kept, if any.
func (a *App) openLastfm() {
	c, err := a.lastfmStore.Load()
	if err != nil || c.Session.Key == "" {
		return
	}
	a.startScrobbling(c)
}

// startScrobbling makes the scrobbler of the credentials, with the queue
// of plays not sent yet.
func (a *App) startScrobbling(c lastfmCredentials) {
	client := lastfm.NewClient(c.APIKey, c.Secret)
	s := lastfm.NewScrobbler(client, c.Session.Key)
	queue := filepath.Join(a.opts.DataDir, "lastfm-queue.json")
	if data, err := os.ReadFile(queue); err == nil {
		var plays []lastfm.Play
		if json.Unmarshal(data, &plays) == nil {
			s.SetQueue(plays)
		}
	}
	s.OnQueue = func(plays []lastfm.Play) {
		data, err := json.Marshal(plays)
		if err == nil {
			err = store.WriteFile(queue, data)
		}
		if err != nil {
			log.Printf("mixmusic: keeping the scrobbles to send: %v", err)
		}
	}
	a.mu.Lock()
	a.lastfm = &scrobbles{client: client, scrobbler: s, name: c.Session.Name}
	a.mu.Unlock()
}

func (a *App) lastfmAccount() LastfmAccount {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastfm == nil {
		return LastfmAccount{}
	}
	return LastfmAccount{Connected: true, Name: a.lastfm.name, Expired: a.lastfm.expired}
}

// scrobble feeds what plays to the scrobbler, and sends what it has to.
func (a *App) scrobble(np NowPlaying) {
	a.mu.Lock()
	l := a.lastfm
	a.mu.Unlock()
	if l == nil || !a.settings.Get().Scrobbling {
		return
	}
	var (
		id string
		t  lastfm.Track
	)
	if np.Track != nil {
		id = np.Track.ID
		t = lastfm.Track{Title: np.Track.Title, Duration: time.Duration(np.DurationMs) * time.Millisecond}
		if len(np.Track.Artists) > 0 {
			// Last.fm knows a track by its main artist.
			t.Artist = np.Track.Artists[0].Name
		}
		if np.Track.Album != nil {
			t.Album = np.Track.Album.Name
		}
		if t.Duration == 0 {
			t.Duration = time.Duration(np.Track.DurationMs) * time.Millisecond
		}
	}
	if !l.scrobbler.Observe(time.Now(), id, t, np.Playing, time.Duration(np.PositionMs)*time.Millisecond) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := l.scrobbler.Send(ctx)
		switch {
		case errors.Is(err, lastfm.ErrSessionExpired):
			a.mu.Lock()
			l.expired = true
			a.mu.Unlock()
			notify()
		case err != nil && !lastfm.IsTemporary(err):
			log.Printf("mixmusic: scrobbling: %v", err)
		}
	}()
}

// LastfmLogin connects MixMusic to the user's Last.fm account, with the
// API account they made on Last.fm: the user approves in their browser,
// and MixMusic waits for it.
func (s *Accounts) LastfmLogin(ctx context.Context, apiKey, secret string) (AccountsStatus, error) {
	apiKey, secret = strings.TrimSpace(apiKey), strings.TrimSpace(secret)
	if apiKey == "" || secret == "" {
		return AccountsStatus{}, errors.New("paste the API key and the shared secret of your Last.fm API account")
	}
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	client := lastfm.NewClient(apiKey, secret)
	token, err := client.Token(ctx)
	if err != nil {
		return AccountsStatus{}, err
	}
	if err := mygo.Shell.OpenExternal(client.AuthURL(token)); err != nil {
		return AccountsStatus{}, err
	}
	var session lastfm.Session
	for {
		session, err = client.Session(ctx, token)
		if err == nil {
			break
		}
		if !errors.Is(err, lastfm.ErrNotAuthorized) {
			return AccountsStatus{}, err
		}
		select {
		case <-ctx.Done():
			return AccountsStatus{}, errors.New("Last.fm did not hear from you in time: approve MixMusic in the browser, then connect again")
		case <-time.After(lastfmPoll):
		}
	}
	c := lastfmCredentials{APIKey: apiKey, Secret: secret, Session: session}
	if err := s.a.lastfmStore.Save(c); err != nil {
		return AccountsStatus{}, err
	}
	s.a.startScrobbling(c)
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Show() // back from the browser
	}
	status := s.a.accountsStatus(ctx)
	notify()
	return status, nil
}

// LastfmLogout disconnects MixMusic from Last.fm.
func (s *Accounts) LastfmLogout(ctx context.Context) error {
	s.a.mu.Lock()
	s.a.lastfm = nil
	s.a.mu.Unlock()
	err := s.a.lastfmStore.Clear()
	if rmErr := os.Remove(filepath.Join(s.a.opts.DataDir, "lastfm-queue.json")); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = errors.Join(err, rmErr)
	}
	notify()
	return err
}
