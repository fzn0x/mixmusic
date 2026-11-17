// Package discord shows what MixMusic plays on the user's Discord profile,
// as Rich Presence. It talks to the Discord app running on the same
// computer, through the app's named pipe on Windows and its Unix socket
// elsewhere, and needs no network access of its own.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// callTimeout bounds a call whose context has no deadline.
const callTimeout = 5 * time.Second

// ErrNotRunning is returned when the Discord app is not running on this
// computer: none of its pipes or sockets exists.
var ErrNotRunning = errors.New("discord: Discord is not running")

// errNoAppID is returned by calls of a client with no application ID.
var errNoAppID = errors.New("discord: no application ID")

// errInterrupted is returned by a call that Close or SetAppID interrupted
// while it connected.
var errInterrupted = errors.New("discord: the connection was closed")

// Error is an error Discord answered with, such as for an unknown
// application ID or an activity it does not take.
type Error struct {
	// Code and Message are Discord's, as 4000 and "Invalid Client ID".
	Code    int
	Message string
}

// Error returns Discord's message, with its code.
func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("discord: error %d", e.Code)
	}
	return fmt.Sprintf("discord: %s (%d)", e.Message, e.Code)
}

// Client sets the user's Rich Presence through the Discord app on this
// computer. The zero Client is not usable: create one with New. Its methods
// are safe for concurrent use.
//
// A Client connects on its first call, and again on the call after a
// connection fails. Discord clears the presence when the connection closes.
type Client struct {
	// dial opens a pipe or socket of the Discord app, replaced in tests.
	dial func(ctx context.Context) (io.ReadWriteCloser, error)
	// busy is held by the call that talks to Discord, one at a time.
	busy chan struct{}

	mu    sync.Mutex
	appID string
	conn  *conn  // nil until connected, and after a failure
	shown []byte // the activity last set over conn, as JSON
	gen   int    // counts Close and SetAppID, which interrupt connecting
}

// New returns a client that shows presence as the Discord application whose
// ID, as the Discord Developer Portal shows it, is appID. Discord names the
// application in the presence, as in "Listening to MixMusic".
//
// In a test binary the client never reaches the Discord app running on the
// computer: the status it would set is the person's own, and a test's track
// would show on their profile. Tests that talk to Discord replace dial.
func New(appID string) *Client {
	d := dial
	if testing.Testing() {
		d = notRunning
	}
	return &Client{dial: d, busy: make(chan struct{}, 1), appID: appID}
}

// notRunning is the dial of a client in a test binary: as if Discord were
// not running.
func notRunning(context.Context) (io.ReadWriteCloser, error) {
	return nil, ErrNotRunning
}

// SetActivity shows a on the user's Discord profile. It sends nothing when
// a is the activity the client shows already.
func (c *Client) SetActivity(ctx context.Context, a Activity) error {
	return c.set(ctx, a.wire())
}

// Clear removes the client's activity from the user's Discord profile.
func (c *Client) Clear(ctx context.Context) error {
	return c.set(ctx, nil)
}

// SetAppID changes the Discord application the client shows presence as.
// It closes the connection, which clears the presence; the next call
// connects as appID.
func (c *Client) SetAppID(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if appID == c.appID {
		return
	}
	c.appID = appID
	c.disconnect()
}

// Close closes the connection to Discord, which clears the presence. A
// later call connects again.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnect()
}

// disconnect closes the connection, stopping the call that uses it. c.mu
// is held.
func (c *Client) disconnect() error {
	c.gen++
	cn := c.conn
	c.conn, c.shown = nil, nil
	if cn == nil {
		return nil
	}
	if err := cn.rwc.Close(); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, os.ErrClosed) {
		return fmt.Errorf("discord: %w", err)
	}
	return nil
}

// set sets the activity a, or clears it when a is nil, connecting first
// when the client has no connection.
func (c *Client) set(ctx context.Context, a *activity) error {
	body, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callTimeout)
		defer cancel()
	}
	select {
	case c.busy <- struct{}{}:
		defer func() { <-c.busy }()
	case <-ctx.Done():
		return fmt.Errorf("discord: %w", ctx.Err())
	}

	c.mu.Lock()
	cn, appID, gen := c.conn, c.appID, c.gen
	shown := []byte("null") // without a connection, nothing is shown
	if cn != nil {
		shown = c.shown
	}
	c.mu.Unlock()
	if bytes.Equal(body, shown) {
		return nil
	}

	if cn == nil {
		if appID == "" {
			return errNoAppID
		}
		if cn, err = c.connect(ctx, appID); err != nil {
			return err
		}
		c.mu.Lock()
		if c.gen != gen {
			c.mu.Unlock()
			cn.rwc.Close()
			return errInterrupted
		}
		c.conn, c.shown = cn, []byte("null")
		c.mu.Unlock()
	}

	err = cn.command(ctx, "SET_ACTIVITY", struct {
		PID      int             `json:"pid"`
		Activity json.RawMessage `json:"activity"`
	}{os.Getpid(), body})

	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case cn.dead:
		if c.conn == cn {
			c.conn, c.shown = nil, nil
		}
		cn.rwc.Close()
	case err == nil && c.conn == cn:
		c.shown = body
	}
	return err
}

// connect opens a connection to Discord and completes the handshake as
// the application appID.
func (c *Client) connect(ctx context.Context, appID string) (*conn, error) {
	rwc, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	cn := &conn{rwc: rwc}
	if err := cn.handshake(ctx, appID); err != nil {
		rwc.Close()
		return nil, err
	}
	return cn, nil
}
