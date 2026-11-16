// Package listenbrainz scrobbles what MixMusic plays to ListenBrainz, or to
// a server that speaks its API, as a ListenBrainz of the user's own or
// Maloja.
//
// Client calls the API with the user's token: it checks the token,
// submits listens and now-playing updates, the latter giving the
// recording's MessyBrainz ID, and sends loves as recording feedback. It
// is the scrobble.Service of a scrobble.Scrobbler, which follows what the
// player does and decides which plays count as listens. It makes one call
// a second at most, as ListenBrainz asks of its clients.
package listenbrainz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// DefaultServer is ListenBrainz's own API.
const DefaultServer = "https://api.listenbrainz.org"

// minGap is the least time from one call to the next.
const minGap = time.Second

// ErrTokenRefused is returned when the server doesn't accept the token,
// such as after the user made a new one. It is scrobble.ErrKeyRefused.
var ErrTokenRefused error = tokenRefused{}

type tokenRefused struct{}

func (tokenRefused) Error() string        { return "listenbrainz: the token isn't accepted" }
func (tokenRefused) Is(target error) bool { return target == scrobble.ErrKeyRefused }

// APIError is an error the server answered with.
type APIError struct {
	// Status is the HTTP status of the answer.
	Status int
	// Message is the server's description of the error, if any.
	Message string
	// Wait is how long the server asked to wait before the next call, as
	// X-RateLimit-Reset-In tells with a 429.
	Wait time.Duration
}

func (e *APIError) Error() string {
	switch {
	case e.Status == http.StatusTooManyRequests:
		return "listenbrainz: too many requests, try again in a moment"
	case e.Message != "":
		return fmt.Sprintf("listenbrainz: %s (HTTP %d)", e.Message, e.Status)
	}
	return fmt.Sprintf("listenbrainz: HTTP %d", e.Status)
}

// Rejected reports whether the server refused what was sent for what it
// holds, which sending it again won't change: a 400, which ListenBrainz
// answers to a listen it can't take. Any other status keeps the listens
// for later, as a server of the user's that is down or moved does.
func (e *APIError) Rejected() bool { return e.Status == http.StatusBadRequest }

// RetryAfter is how long the server asked to wait.
func (e *APIError) RetryAfter() time.Duration { return e.Wait }

// netError is a call that got no answer.
type netError struct{ err error }

func (e *netError) Error() string { return "listenbrainz: " + e.err.Error() }
func (e *netError) Unwrap() error { return e.err }

// Client calls the API of a ListenBrainz server. Its methods are safe for
// concurrent use.
type Client struct {
	// Server is the API's address, without the /1/ of its version:
	// DefaultServer, or a server of the user's.
	Server string
	// UserAgent introduces MixMusic, and Version is its version, which
	// the listens tell.
	UserAgent string
	Version   string
	// HTTP sends the requests.
	HTTP *http.Client
	// OnRecording hears the MessyBrainz ID that the server gave the
	// recording of a track announced as playing, when it gave one.
	OnRecording func(t Track, msid string)
	// Gap is the least time from the start of a call to the next: a
	// second, which tests shorten.
	Gap time.Duration

	mu sync.Mutex
	// next is when the next call may start.
	next time.Time
}

// New returns a client of the server at the address server, as
// ParseServer gives it, which introduces itself as userAgent and tells
// MixMusic's version.
func New(server, userAgent, version string) *Client {
	return &Client{Server: server, UserAgent: userAgent, Version: version, HTTP: &http.Client{Timeout: 20 * time.Second}, Gap: minGap}
}

// turn waits until the client may make a call: a second after the call
// before started, or once the wait the server asked for is over.
func (c *Client) turn(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	at := c.next
	if at.Before(now) {
		at = now
	}
	c.next = at.Add(c.Gap)
	c.mu.Unlock()
	wait := at.Sub(now)
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// hold has no call start before d passed.
func (c *Client) hold(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if at := time.Now().Add(d); at.After(c.next) {
		c.next = at
	}
}

// call makes a request of the API's path with the user's token, sending
// body as JSON when it isn't nil, and decodes the answer into out when out
// isn't nil.
func (c *Client) call(ctx context.Context, method, path, token string, query url.Values, body, out any) error {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(data)
	}
	u := strings.TrimRight(c.Server, "/") + "/1/" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Token "+token)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if err := c.turn(ctx); err != nil {
		return &netError{err}
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return &netError{err}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	res.Body.Close()
	if err != nil {
		return &netError{err}
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return ErrTokenRefused
	case res.StatusCode == http.StatusTooManyRequests:
		wait := resetIn(res.Header)
		c.hold(max(wait, c.Gap))
		return &APIError{Status: res.StatusCode, Message: message(data), Wait: wait}
	case res.StatusCode >= 300:
		return &APIError{Status: res.StatusCode, Message: message(data)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("listenbrainz: decoding %s: %w", path, err)
	}
	return nil
}

// resetIn is the wait the X-RateLimit-Reset-In header of a 429 asks for,
// in seconds, or 0 when it has none.
func resetIn(h http.Header) time.Duration {
	s, err := strconv.ParseFloat(strings.TrimSpace(h.Get("X-RateLimit-Reset-In")), 64)
	if err != nil || !(s > 0) || s > 3600 {
		return 0
	}
	return time.Duration(s * float64(time.Second))
}

// message is the description of the error that the server answered with:
// ListenBrainz says it as {"code": 400, "error": "…"}.
func message(data []byte) string {
	var answer struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &answer) != nil {
		return ""
	}
	return answer.Error
}

// ValidateToken returns the name of the user of token, or ErrTokenRefused
// when the server doesn't accept it.
func (c *Client) ValidateToken(ctx context.Context, token string) (string, error) {
	var answer struct {
		Valid    bool   `json:"valid"`
		UserName string `json:"user_name"`
	}
	err := c.call(ctx, http.MethodGet, "validate-token", token, nil, nil, &answer)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
		return "", ErrTokenRefused
	}
	if err != nil {
		return "", err
	}
	if !answer.Valid {
		return "", ErrTokenRefused
	}
	return answer.UserName, nil
}
