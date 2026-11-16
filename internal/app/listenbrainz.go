package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// listenbrainzSecret is the key, in the system's credential store, of the
// ListenBrainz connection: the server, the user's token and their name.
const listenbrainzSecret = "listenbrainz"

// The files of the data folder the ListenBrainz connection keeps, which
// disconnecting deletes.
const (
	listenbrainzQueueFile      = "listenbrainz-queue.json"
	listenbrainzRecordingsFile = "listenbrainz-recordings.json"
)

// errListenBrainzToken is what connecting says of a token the server
// refuses, or that isn't one of ListenBrainz's own.
var errListenBrainzToken = errors.New("ListenBrainz doesn't accept this token")

// errPlainRemote is what connecting says of a plain http address that
// isn't on the local network.
var errPlainRemote = errors.New("plain http is for a server on your local network alone, as the token would travel unencrypted: use the server's https address")

// listenbrainzCredentials connect MixMusic to the user's account on a
// server of ListenBrainz's API.
type listenbrainzCredentials struct {
	// Server is the address of the API, "" for ListenBrainz's own.
	Server string `json:"server,omitempty"`
	Token  string `json:"token"`
	// Name is the user's name there, as the server told it.
	Name string `json:"name"`
}

// server is the address of the API the credentials are of.
func (c listenbrainzCredentials) server() string {
	if c.Server == "" {
		return listenbrainz.DefaultServer
	}
	return c.Server
}

// listenbrainzStore keeps the credentials as Last.fm's are kept: in the
// system's credential store, or where there is none, as secretStore does.
type listenbrainzStore = jsonSecret[listenbrainzCredentials]

// listenbrainzState is what the App keeps of ListenBrainz: where the
// credentials are kept, and the connection, nil until the user connects,
// which a.mu guards.
type listenbrainzState struct {
	store listenbrainzStore
	conn  *brainzScrobbles
	// resolve and validate stand in for the network in tests: when nil,
	// connecting asks the system's resolver and the server.
	resolve  listenbrainz.Resolver
	validate func(ctx context.Context, server, token string) (string, error)
}

// brainzScrobbles is the ListenBrainz connection: its client, its
// scrobbler, and the MessyBrainz IDs of the recordings it announced.
type brainzScrobbles struct {
	scrobbling
	client     *listenbrainz.Client
	creds      listenbrainzCredentials
	recordings *recordingBook
	// other is the connection of another user, which waits for the user
	// to say what becomes of the listens waiting.
	other *listenbrainzCredentials
}

// ListenBrainzAccount is the state of the ListenBrainz connection.
type ListenBrainzAccount struct {
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
	// Server is the address of the user's own server, "" for
	// ListenBrainz's.
	Server string `json:"server,omitempty"`
	// Plain is set when the server is reached over plain http, where the
	// token travels unencrypted.
	Plain bool `json:"plain,omitempty"`
	// Expired is set when the server no longer accepts the token.
	Expired bool `json:"expired,omitempty"`
	// Waiting is how many listens wait to be sent.
	Waiting int `json:"waiting,omitempty"`
	// Other is the name of another user, whose token the user gave while
	// listens wait: they go to that account, or are dropped, as the user
	// chooses.
	Other string `json:"other,omitempty"`
}

// openListenBrainz connects to ListenBrainz with the credentials kept, if
// any.
func (a *App) openListenBrainz() {
	c, err := a.listenbrainz.store.Load()
	if err != nil || c.Token == "" {
		return
	}
	a.startListenBrainz(c)
}

// startListenBrainz makes the scrobbler of the credentials, with the
// listens not sent yet.
func (a *App) startListenBrainz(c listenbrainzCredentials) {
	client := listenbrainz.New(c.server(), a.userAgent(), a.opts.Version)
	book := openRecordings(filepath.Join(a.opts.DataDir, listenbrainzRecordingsFile))
	client.OnRecording = book.keep
	s := scrobble.New(client, c.Token, scrobble.Options{Batch: listenbrainz.MaxListens})
	queue := filepath.Join(a.opts.DataDir, listenbrainzQueueFile)
	if plays := loadQueue(queue); plays != nil {
		s.SetQueue(plays)
	}
	s.OnQueue = queueSaver(queue)
	conn := &brainzScrobbles{scrobbling: scrobbling{scrobbler: s, name: c.Name, service: "ListenBrainz"}, client: client, creds: c, recordings: book}
	a.mu.Lock()
	old := a.listenbrainz.conn
	a.listenbrainz.conn = conn
	a.mu.Unlock()
	if old != nil {
		old.sends.stop()
	}
}

func (a *App) listenbrainzAccount() ListenBrainzAccount {
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.listenbrainz.conn
	if l == nil {
		return ListenBrainzAccount{}
	}
	acc := ListenBrainzAccount{Connected: true, Name: l.name, Server: l.creds.Server, Expired: l.expired, Waiting: l.scrobbler.Waiting()}
	acc.Plain = strings.HasPrefix(l.creds.server(), "http:")
	if l.other != nil {
		acc.Other = l.other.Name
	}
	return acc
}

// listenbrainzCheck returns the credentials of token on the server at the
// address server, as the user wrote it, once the server accepted it.
// Plain http goes only to a server on the local network.
func (a *App) listenbrainzCheck(ctx context.Context, token, server string) (listenbrainzCredentials, error) {
	token = strings.TrimSpace(token)
	addr, plain, err := listenbrainz.ParseServer(server)
	if err != nil {
		return listenbrainzCredentials{}, err
	}
	if plain && !listenbrainz.Local(ctx, addr, a.listenbrainz.resolve) {
		return listenbrainzCredentials{}, errPlainRemote
	}
	if !listenbrainz.Plausible(addr, token) {
		return listenbrainzCredentials{}, errListenBrainzToken
	}
	validate := a.listenbrainz.validate
	if validate == nil {
		validate = func(ctx context.Context, server, token string) (string, error) {
			return listenbrainz.New(server, a.userAgent(), a.opts.Version).ValidateToken(ctx, token)
		}
	}
	name, err := validate(ctx, addr, token)
	switch {
	case errors.Is(err, listenbrainz.ErrTokenRefused):
		return listenbrainzCredentials{}, errListenBrainzToken
	case err != nil:
		return listenbrainzCredentials{}, fmt.Errorf("MixMusic couldn't check the token with the server, so nothing was kept: check its address and your connection (%v)", err)
	}
	c := listenbrainzCredentials{Token: token, Name: name}
	if addr != listenbrainz.DefaultServer {
		c.Server = addr
	}
	return c, nil
}

// ListenBrainzLogin connects MixMusic to the user's account on
// ListenBrainz, or on the server at the address server, "" for
// ListenBrainz's own, with their user token: the server checks it, and
// MixMusic keeps it in the system's credential store until the user
// disconnects. A token the server refuses keeps nothing.
func (s *Accounts) ListenBrainzLogin(ctx context.Context, token, server string) (AccountsStatus, error) {
	c, err := s.a.listenbrainzCheck(ctx, token, server)
	if err != nil {
		return AccountsStatus{}, err
	}
	if err := s.a.listenbrainz.store.Save(c); err != nil {
		return AccountsStatus{}, err
	}
	s.a.startListenBrainz(c)
	status := s.a.accountsStatus(ctx)
	notify()
	return status, nil
}

// ListenBrainzReconnect gives the running scrobbler a new token of the
// same server, once the server refused the one before: the listens
// waiting go out with it. When the token is another user's while listens
// wait, nothing changes until ListenBrainzResolve.
func (s *Accounts) ListenBrainzReconnect(ctx context.Context, token string) error {
	s.a.mu.Lock()
	l := s.a.listenbrainz.conn
	s.a.mu.Unlock()
	if l == nil {
		return errors.New("connect to ListenBrainz first")
	}
	c, err := s.a.listenbrainzCheck(ctx, token, l.creds.server())
	if err != nil {
		return err
	}
	s.a.mu.Lock()
	name := l.name
	s.a.mu.Unlock()
	if c.Name != name && l.scrobbler.Waiting() > 0 {
		s.a.mu.Lock()
		l.other = &c
		s.a.mu.Unlock()
		notify()
		return nil
	}
	return s.a.listenbrainzSwitch(l, c)
}

// listenbrainzSwitch makes c's token the one l scrobbles with.
func (a *App) listenbrainzSwitch(l *brainzScrobbles, c listenbrainzCredentials) error {
	if err := a.listenbrainz.store.Save(c); err != nil {
		return err
	}
	l.scrobbler.SetSession(c.Token)
	a.mu.Lock()
	l.name, l.creds, l.expired, l.other = c.Name, c, false, nil
	a.mu.Unlock()
	notify()
	return nil
}

// ListenBrainzResolve settles what the token of another user does to the
// listens waiting: send them to that account, or drop them.
func (s *Accounts) ListenBrainzResolve(ctx context.Context, send bool) error {
	s.a.mu.Lock()
	l := s.a.listenbrainz.conn
	var c *listenbrainzCredentials
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
	return s.a.listenbrainzSwitch(l, *c)
}

// ListenBrainzLogout disconnects MixMusic from ListenBrainz: it forgets
// the token, and drops the listens that wait to be sent, and the IDs of
// the recordings kept for loves.
func (s *Accounts) ListenBrainzLogout(ctx context.Context) error {
	s.a.mu.Lock()
	l := s.a.listenbrainz.conn
	s.a.listenbrainz.conn = nil
	s.a.mu.Unlock()
	if l != nil {
		l.sends.stop() // before its files go
	}
	err := s.a.listenbrainz.store.Clear()
	for _, name := range []string{listenbrainzQueueFile, listenbrainzRecordingsFile} {
		if rmErr := os.Remove(filepath.Join(s.a.opts.DataDir, name)); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}
	notify()
	return err
}
