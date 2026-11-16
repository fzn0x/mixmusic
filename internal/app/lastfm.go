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
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/scrobble"
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
// system's credential store, sealed for the Windows user where the
// Credential Manager isn't available, or in a file readable by the user
// alone where there is no store.
type lastfmStore = jsonSecret[lastfmCredentials]

// LastfmAccount is the state of the Last.fm connection.
type LastfmAccount struct {
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
	// Expired is set when Last.fm no longer accepts the session.
	Expired bool `json:"expired,omitempty"`
	// Waiting is how many scrobbles wait to be sent.
	Waiting int `json:"waiting,omitempty"`
	// Other is the name of another Last.fm user who approved a new
	// connection while scrobbles wait: they go to that account, or are
	// dropped, as the user chooses.
	Other string `json:"other,omitempty"`
}

// scrobbles is the Last.fm connection: its client and its scrobbler.
type scrobbles struct {
	scrobbling
	client *lastfm.Client
	// other is the connection another user approved, which waits for the
	// user to say what becomes of the waiting scrobbles.
	other *lastfmCredentials
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
	s := scrobble.New(client, c.Session.Key, scrobble.Options{})
	queue := filepath.Join(a.opts.DataDir, "lastfm-queue.json")
	if data, err := os.ReadFile(queue); err == nil {
		var plays []lastfm.Play
		if json.Unmarshal(data, &plays) == nil {
			s.SetQueue(plays)
		}
	}
	s.OnQueue = a.lastfmQueueSaver(queue)
	a.mu.Lock()
	old := a.lastfm
	a.lastfm = &scrobbles{scrobbling: scrobbling{scrobbler: s, name: c.Session.Name, service: "Last.fm"}, client: client}
	a.mu.Unlock()
	if old != nil {
		old.sends.stop()
	}
}

// lastfmQueueSaver returns what keeps the scrobbles waiting in the file.
func (a *App) lastfmQueueSaver(file string) func([]lastfm.Play) {
	return func(plays []lastfm.Play) {
		data, err := json.Marshal(plays)
		if err == nil {
			err = store.WriteFile(file, data)
		}
		if err != nil {
			log.Printf("mixmusic: keeping the scrobbles to send: %v", err)
		}
	}
}

func (a *App) lastfmAccount() LastfmAccount {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastfm == nil {
		return LastfmAccount{}
	}
	acc := LastfmAccount{Connected: true, Name: a.lastfm.name, Expired: a.lastfm.expired, Waiting: len(a.lastfm.scrobbler.Queue())}
	if a.lastfm.other != nil {
		acc.Other = a.lastfm.other.Session.Name
	}
	return acc
}

// scrobble feeds what plays to the scrobbler, which sends what it has to
// beside the app.
func (a *App) scrobble(np NowPlaying) {
	a.mu.Lock()
	l := a.lastfm
	a.mu.Unlock()
	if l == nil || !a.settings.Get().Scrobbling {
		return
	}
	id, t := lastfmTrack(np)
	l.scrobbler.Observe(time.Now(), id, t, np.Playing, time.Duration(np.PositionMs)*time.Millisecond, np.Speed)
	a.sendScrobbles(&l.scrobbling)
	a.tellWaiting(&l.scrobbling)
}

// lastfmTrack is what plays as the scrobbler knows it, and its ID, "" when
// nothing does. A live stream has no length, which the scrobbler never
// scrobbles: Last.fm hears only that it plays.
func lastfmTrack(np NowPlaying) (string, lastfm.Track) {
	// Last.fm keeps music: a podcast's episode is none.
	if np.Track == nil || np.Track.IsEpisode() {
		return "", lastfm.Track{}
	}
	// A track on this computer whose file names no artist isn't one
	// Last.fm can count.
	if localfiles.NoArtist(*np.Track) {
		return "", lastfm.Track{}
	}
	t := lastfm.Track{Title: np.Track.Title, Duration: time.Duration(np.DurationMs) * time.Millisecond}
	if len(np.Track.Artists) > 0 {
		// Last.fm knows a track by its main artist.
		t.Artist = np.Track.Artists[0].Name
	}
	if np.Track.Album != nil {
		t.Album = np.Track.Album.Name
	}
	switch {
	case np.Live:
		t.Duration = 0
	case t.Duration == 0:
		t.Duration = time.Duration(np.Track.DurationMs) * time.Millisecond
	}
	// The parts the player passes over count for nothing: a scrobble takes
	// half of what is left.
	if skipped := time.Duration(np.SkippedMs) * time.Millisecond; t.Duration > skipped {
		t.Duration -= skipped
	}
	return np.Track.ID, t
}

// lastfmApprove has the user approve MixMusic on Last.fm, in their browser,
// with the API account of apiKey and secret, and returns the session.
func lastfmApprove(ctx context.Context, apiKey, secret string) (lastfm.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	client := lastfm.NewClient(apiKey, secret)
	token, err := client.Token(ctx)
	if err != nil {
		return lastfm.Session{}, err
	}
	if err := mygo.Shell.OpenExternal(client.AuthURL(token)); err != nil {
		return lastfm.Session{}, err
	}
	for {
		session, err := client.Session(ctx, token)
		if err == nil {
			return session, nil
		}
		if !errors.Is(err, lastfm.ErrNotAuthorized) {
			return lastfm.Session{}, err
		}
		select {
		case <-ctx.Done():
			return lastfm.Session{}, errors.New("Last.fm did not hear from you in time: approve MixMusic in the browser, then connect again")
		case <-time.After(lastfmPoll):
		}
	}
}

// LastfmLogin connects MixMusic to the user's Last.fm account, with the
// API account they made on Last.fm: the user approves in their browser,
// and MixMusic waits for it.
func (s *Accounts) LastfmLogin(ctx context.Context, apiKey, secret string) (AccountsStatus, error) {
	apiKey, secret = strings.TrimSpace(apiKey), strings.TrimSpace(secret)
	if apiKey == "" || secret == "" {
		return AccountsStatus{}, errors.New("paste the API key and the shared secret of your Last.fm API account")
	}
	session, err := lastfmApprove(ctx, apiKey, secret)
	if err != nil {
		return AccountsStatus{}, err
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

// LastfmReconnect has the user approve MixMusic on Last.fm again, with the
// API account kept, and gives the new session to the running scrobbler,
// which keeps the scrobbles waiting and sends them. When another user
// approves while scrobbles wait, nothing changes until LastfmResolve.
func (s *Accounts) LastfmReconnect(ctx context.Context) error {
	c, err := s.a.lastfmStore.Load()
	if err != nil || c.APIKey == "" {
		return errors.New("connect to Last.fm first")
	}
	session, err := lastfmApprove(ctx, c.APIKey, c.Secret)
	if err != nil {
		return err
	}
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Show() // back from the browser
	}
	return s.a.lastfmNewSession(c, session)
}

// lastfmNewSession gives the running scrobbler the session the user
// approved, or holds it when it is another user's and scrobbles wait.
func (a *App) lastfmNewSession(c lastfmCredentials, session lastfm.Session) error {
	a.mu.Lock()
	l := a.lastfm
	a.mu.Unlock()
	if l == nil {
		return errors.New("connect to Last.fm first")
	}
	c.Session = session
	a.mu.Lock()
	name := l.name
	a.mu.Unlock()
	if session.Name != name && len(l.scrobbler.Queue()) > 0 {
		a.mu.Lock()
		l.other = &c
		a.mu.Unlock()
		notify()
		return nil
	}
	return a.lastfmSwitch(l, c)
}

// lastfmSwitch makes c's session the one l scrobbles with.
func (a *App) lastfmSwitch(l *scrobbles, c lastfmCredentials) error {
	if err := a.lastfmStore.Save(c); err != nil {
		return err
	}
	l.scrobbler.SetSession(c.Session.Key)
	a.mu.Lock()
	l.name, l.expired, l.other = c.Session.Name, false, nil
	a.mu.Unlock()
	notify()
	return nil
}

// LastfmResolve settles what a connection of another Last.fm user does to
// the scrobbles waiting: send them to that account, or drop them.
func (s *Accounts) LastfmResolve(ctx context.Context, send bool) error {
	s.a.mu.Lock()
	l := s.a.lastfm
	var c *lastfmCredentials
	if l != nil {
		c = l.other
	}
	s.a.mu.Unlock()
	if c == nil {
		return nil
	}
	if !send {
		l.scrobbler.Discard()
	}
	return s.a.lastfmSwitch(l, *c)
}

// LastfmLogout disconnects MixMusic from Last.fm, and drops the scrobbles
// that wait to be sent.
func (s *Accounts) LastfmLogout(ctx context.Context) error {
	s.a.mu.Lock()
	l := s.a.lastfm
	s.a.lastfm = nil
	s.a.mu.Unlock()
	if l != nil {
		l.sends.stop() // before the queue's file goes
	}
	err := s.a.lastfmStore.Clear()
	if rmErr := os.Remove(filepath.Join(s.a.opts.DataDir, "lastfm-queue.json")); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = errors.Join(err, rmErr)
	}
	notify()
	return err
}
