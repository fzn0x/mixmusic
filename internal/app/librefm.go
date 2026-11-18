package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/librefm"
	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/scrobble"
	"github.com/fzn0x/mixmusic/locales"
)

// librefmSecret is the key, in the system's credential store, of the
// connection to Libre.fm or another GNU FM server: the server, the API
// account when it is the user's own, and the session.
const librefmSecret = "librefm"

// librefmQueueFile is the file of the data folder where the scrobbles not
// sent yet wait, which disconnecting deletes.
const librefmQueueFile = "librefm-queue.json"

// errPlainGNUFM is what connecting says of a plain http address that
// isn't on the local network: the password typed on the server's page,
// and the session, would travel unencrypted.
var errPlainGNUFM = errors.New("plain http is for a server on your local network alone, as your password and session would travel unencrypted: use the server's https address")

// librefmCredentials connect MixMusic to the user's account on Libre.fm
// or another GNU FM server. The server stays with the session: another
// server means connecting again.
type librefmCredentials struct {
	// Server is the server's address, "" for Libre.fm.
	Server string `json:"server,omitempty"`
	// APIKey and Secret are the user's own API account on the server, ""
	// for MixMusic's.
	APIKey  string         `json:"apiKey,omitempty"`
	Secret  string         `json:"secret,omitempty"`
	Session lastfm.Session `json:"session"`
}

// server is the address of the server the credentials are of.
func (c librefmCredentials) server() string {
	if c.Server == "" {
		return librefm.DefaultServer
	}
	return c.Server
}

// librefmStore keeps the credentials as Last.fm's are kept: in the
// system's credential store, or where there is none, as secretStore does.
type librefmStore = jsonSecret[librefmCredentials]

// librefmState is what the App keeps of Libre.fm: where the credentials
// are kept, and the connection, nil until the user connects, which a.mu
// guards.
type librefmState struct {
	store librefmStore
	conn  *gnufmScrobbles
	// resolve, open and poll stand in for the network, the browser and
	// the wait between two asks in tests: when nil or 0, connecting asks
	// the system's resolver, opens the user's browser, and asks every
	// lastfmPoll.
	resolve listenbrainz.Resolver
	open    func(url string) error
	poll    time.Duration
}

// gnufmScrobbles is the connection to Libre.fm or another GNU FM server:
// its client and its scrobbler.
type gnufmScrobbles struct {
	scrobbling
	client *lastfm.Client
	creds  librefmCredentials
	// other is the connection another user approved, which waits for the
	// user to say what becomes of the waiting scrobbles.
	other *librefmCredentials
}

// LibrefmAccount is the state of the connection to Libre.fm or another
// GNU FM server.
type LibrefmAccount struct {
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
	// Server is the address of another GNU FM server, "" for Libre.fm.
	Server string `json:"server,omitempty"`
	// Plain is set when the server is reached over plain http, where the
	// session travels unencrypted.
	Plain bool `json:"plain,omitempty"`
	// Expired is set when the server no longer accepts the session.
	Expired bool `json:"expired,omitempty"`
	// Waiting is how many scrobbles wait to be sent.
	Waiting int `json:"waiting,omitempty"`
	// Other is the name of another user who approved a new connection
	// while scrobbles wait: they go to that account, or are dropped, as
	// the user chooses.
	Other string `json:"other,omitempty"`
}

// openLibrefm connects to Libre.fm with the credentials kept, if any.
func (a *App) openLibrefm() {
	c, err := a.librefm.store.Load()
	if err != nil || c.Session.Key == "" {
		return
	}
	a.startLibrefm(c)
}

// librefmClient is a client of the server of c, with c's API account.
func (a *App) librefmClient(c librefmCredentials) *lastfm.Client {
	return librefm.New(c.server(), c.APIKey, c.Secret, a.userAgent())
}

// startLibrefm makes the scrobbler of the credentials, with the queue of
// plays not sent yet.
func (a *App) startLibrefm(c librefmCredentials) {
	client := a.librefmClient(c)
	s := scrobble.New(client, c.Session.Key, scrobble.Options{})
	queue := filepath.Join(a.opts.DataDir, librefmQueueFile)
	if plays := loadQueue(queue); plays != nil {
		s.SetQueue(plays)
	}
	s.OnQueue = queueSaver(queue)
	conn := &gnufmScrobbles{scrobbling: scrobbling{scrobbler: s, name: c.Session.Name, service: "Libre.fm"}, client: client, creds: c}
	a.mu.Lock()
	old := a.librefm.conn
	a.librefm.conn = conn
	a.mu.Unlock()
	if old != nil {
		old.sends.stop()
	}
}

func (a *App) librefmAccount() LibrefmAccount {
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.librefm.conn
	if l == nil {
		return LibrefmAccount{}
	}
	acc := LibrefmAccount{Connected: true, Name: l.name, Server: l.creds.Server, Expired: l.expired, Waiting: l.scrobbler.Waiting()}
	acc.Plain = strings.HasPrefix(l.creds.server(), "http:")
	if l.other != nil {
		acc.Other = l.other.Session.Name
	}
	return acc
}

// gnufmHost names the server at server as the windows do: Libre.fm, or
// its host.
func gnufmHost(server string) string {
	if server == "" || server == librefm.DefaultServer {
		return "Libre.fm"
	}
	if u, err := url.Parse(server); err == nil && u.Host != "" {
		return u.Host
	}
	return server
}

// librefmCheck returns the credentials, without a session, of the server
// at the address server, as the user wrote it, with the API account of
// apiKey and secret, or MixMusic's own when own is false. Plain http
// goes only to a server on the local network.
func (a *App) librefmCheck(ctx context.Context, server string, own bool, apiKey, secret string) (librefmCredentials, error) {
	addr, plain, err := librefm.ParseServer(server)
	if err != nil {
		return librefmCredentials{}, err
	}
	if plain && !listenbrainz.Local(ctx, addr, a.librefm.resolve) {
		return librefmCredentials{}, errPlainGNUFM
	}
	var c librefmCredentials
	if addr != librefm.DefaultServer {
		c.Server = addr
	}
	if own {
		c.APIKey, c.Secret = strings.TrimSpace(apiKey), strings.TrimSpace(secret)
		if c.APIKey == "" || c.Secret == "" {
			return librefmCredentials{}, worded("paste the API key and the shared secret of your API account on the server", locales.LibrefmSettingsPasteKeys(), nil)
		}
	}
	return c, nil
}

// librefmApprove has the user approve MixMusic on the server of c, in
// their browser, where they sign in, and returns the session.
func (a *App) librefmApprove(ctx context.Context, c librefmCredentials) (lastfm.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	client := a.librefmClient(c)
	host := gnufmHost(c.Server)
	token, err := client.Token(ctx)
	if err != nil {
		return lastfm.Session{}, wordedLater(fmt.Sprintf("MixMusic couldn't ask %s for a sign-in: check its address and your connection (%v)", host, err),
			func() string { return locales.LibrefmSettingsAskFailed(host, ErrorText(err)) }, err)
	}
	open, poll := a.librefm.open, a.librefm.poll
	if open == nil {
		open = mygo.Shell.OpenExternal
	}
	if poll == 0 {
		poll = lastfmPoll
	}
	if err := open(client.AuthURL(token)); err != nil {
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
			return lastfm.Session{}, wordedLater(host+" did not hear from you in time: approve MixMusic in the browser, then connect again",
				func() string { return locales.LibrefmSettingsTooLong(host) }, nil)
		case <-time.After(poll):
		}
	}
}

// LibrefmLogin connects MixMusic to the user's account on Libre.fm, or
// on the GNU FM server at the address server, "" for Libre.fm: the user
// signs in and approves MixMusic on the server's page in their browser,
// and MixMusic waits for it. It signs calls with MixMusic's own API key,
// which GNU FM takes, or with the user's API account of apiKey and
// secret when own is set, for a server that wants one.
func (s *Accounts) LibrefmLogin(ctx context.Context, server string, own bool, apiKey, secret string) (AccountsStatus, error) {
	c, err := s.a.librefmCheck(ctx, server, own, apiKey, secret)
	if err != nil {
		return AccountsStatus{}, err
	}
	session, err := s.a.librefmApprove(ctx, c)
	if err != nil {
		return AccountsStatus{}, err
	}
	c.Session = session
	if err := s.a.librefm.store.Save(c); err != nil {
		return AccountsStatus{}, err
	}
	s.a.startLibrefm(c)
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Show() // back from the browser
	}
	status := s.a.accountsStatus(ctx)
	notify()
	return status, nil
}

// LibrefmReconnect has the user approve MixMusic on the server again,
// with the server and the API account kept, and gives the new session to
// the running scrobbler, which keeps the scrobbles waiting and sends
// them. When another user approves while scrobbles wait, nothing changes
// until LibrefmResolve.
func (s *Accounts) LibrefmReconnect(ctx context.Context) error {
	s.a.mu.Lock()
	l := s.a.librefm.conn
	s.a.mu.Unlock()
	if l == nil {
		return worded("connect to Libre.fm first", locales.LibrefmSettingsConnectFirst(), nil)
	}
	s.a.mu.Lock()
	c := l.creds
	s.a.mu.Unlock()
	session, err := s.a.librefmApprove(ctx, c)
	if err != nil {
		return err
	}
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Show() // back from the browser
	}
	c.Session = session
	s.a.mu.Lock()
	name := l.name
	s.a.mu.Unlock()
	if session.Name != name && l.scrobbler.Waiting() > 0 {
		s.a.mu.Lock()
		l.other = &c
		s.a.mu.Unlock()
		notify()
		return nil
	}
	return s.a.librefmSwitch(l, c)
}

// librefmSwitch makes c's session the one l scrobbles with.
func (a *App) librefmSwitch(l *gnufmScrobbles, c librefmCredentials) error {
	if err := a.librefm.store.Save(c); err != nil {
		return err
	}
	l.scrobbler.SetSession(c.Session.Key)
	a.mu.Lock()
	l.name, l.creds, l.expired, l.other = c.Session.Name, c, false, nil
	a.mu.Unlock()
	notify()
	return nil
}

// LibrefmResolve settles what a connection of another user does to the
// scrobbles waiting: send them to that account, or drop them.
func (s *Accounts) LibrefmResolve(ctx context.Context, send bool) error {
	s.a.mu.Lock()
	l := s.a.librefm.conn
	var c *librefmCredentials
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
	return s.a.librefmSwitch(l, *c)
}

// LibrefmLogout disconnects MixMusic from Libre.fm or the GNU FM server:
// it forgets the session, and drops the scrobbles that wait to be sent.
func (s *Accounts) LibrefmLogout(ctx context.Context) error {
	s.a.mu.Lock()
	l := s.a.librefm.conn
	s.a.librefm.conn = nil
	s.a.mu.Unlock()
	if l != nil {
		l.sends.stop() // before the queue's file goes
	}
	err := s.a.librefm.store.Clear()
	if rmErr := os.Remove(filepath.Join(s.a.opts.DataDir, librefmQueueFile)); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = errors.Join(err, rmErr)
	}
	notify()
	return err
}
