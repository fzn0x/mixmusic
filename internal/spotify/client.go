package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const apiURL = "https://api.spotify.com/v1"

// maxRetryAfter is the longest Retry-After the client waits for once;
// longer waits fail with the rate limit error.
const maxRetryAfter = 5 * time.Second

// ErrNotSignedIn is returned by calls made before the user signed in.
var ErrNotSignedIn = errors.New("spotify: not signed in")

// The refusals of an app in development mode, which an APIError is when
// Spotify answers one: of a user the app does not count among its users,
// who are added under Users Management in its settings in the developer
// dashboard, and of every request while its owner has no Premium.
var (
	ErrUnregistered        = errors.New("spotify: this Spotify account is not one of the app's users")
	ErrOwnerWithoutPremium = errors.New("spotify: the app's owner has no Spotify Premium")
)

// APIError is an error the Web API answered with.
type APIError struct {
	Status  int
	Message string
	// Reason is set for some errors, such as PREMIUM_REQUIRED or
	// QUOTA_EXCEEDED.
	Reason string
}

func (e *APIError) Error() string {
	switch {
	case e.Reason == "PREMIUM_REQUIRED":
		return "spotify: playback control needs Spotify Premium"
	case e.Reason == "NO_ACTIVE_DEVICE" || e.Status == http.StatusNotFound && strings.Contains(e.Message, "device"):
		return "spotify: no active device: open Spotify on one of your devices"
	case e.Status == http.StatusTooManyRequests:
		return "spotify: too many requests, try again in a moment"
	case e.Is(ErrUnregistered):
		return "spotify: this Spotify account is not one of the app's users: add it under Users Management, in the app's settings in the Spotify developer dashboard, then sign in again"
	case e.Is(ErrOwnerWithoutPremium):
		return "spotify: the app's owner has no Spotify Premium, which Spotify asks of apps in development mode: subscribe with the account that made the app, and sign in again in a few hours"
	case e.Message != "":
		return fmt.Sprintf("spotify: %s (HTTP %d)", e.Message, e.Status)
	case e.Status == http.StatusForbidden:
		// Spotify refuses so, without a word, the users of an app in
		// development mode it does not let in.
		return "spotify: Spotify refused (HTTP 403): an app in development mode serves only an owner with Spotify Premium, and the accounts added under Users Management in its settings"
	}
	return fmt.Sprintf("spotify: HTTP %d", e.Status)
}

// Is tells the refusals of an app in development mode by Spotify's words,
// whether in its error object or in plain text.
func (e *APIError) Is(target error) bool {
	if e.Status != http.StatusForbidden {
		return false
	}
	m := strings.ToLower(e.Message)
	switch target {
	case ErrUnregistered:
		return strings.Contains(m, "may not be registered") || strings.Contains(m, "not registered in the developer dashboard") ||
			strings.Contains(m, "not approved for app")
	case ErrOwnerWithoutPremium:
		return strings.Contains(m, "premium subscription required for the owner") || strings.Contains(m, "do not have a spotify premium subscription")
	}
	return false
}

// Client calls the Web API on behalf of the signed-in user, refreshing the
// access token as it expires. Its methods are safe for concurrent use.
type Client struct {
	Auth *Auth
	HTTP *http.Client
	// BaseURL is the API's address, replaced in tests.
	BaseURL string
	// OnToken is called with every new token, to keep it for the next run.
	OnToken func(Token)

	mu    sync.Mutex
	token Token
	// refreshing is held while a request gets its token, so that one
	// refreshes it and the others wait for the new one; mu is not held
	// meanwhile, so that SignedIn and Granted never wait for Spotify.
	refreshing sync.Mutex
}

// NewClient returns a client that refreshes tokens with auth.
func NewClient(auth *Auth) *Client {
	return &Client{Auth: auth, HTTP: &http.Client{Timeout: 20 * time.Second}, BaseURL: apiURL}
}

// SetToken sets the token of the signed-in user, or signs them out with
// the zero Token.
func (c *Client) SetToken(t Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = t
}

// SignedIn reports whether a user is signed in.
func (c *Client) SignedIn() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token.RefreshToken != "" || c.token.AccessToken != ""
}

// Granted reports whether the user's sign-in grants scope. Sign-ins made
// before MixMusic asked for a scope do not.
func (c *Client) Granted(scope string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Contains(strings.Fields(c.token.Scope), scope)
}

// AccessToken returns a valid access token of the signed-in user, for
// Spotify's own code, such as the Web Playback SDK.
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	return c.accessToken(ctx, false)
}

// accessToken returns a valid access token, refreshing it when it expires
// or when force is set.
func (c *Client) accessToken(ctx context.Context, force bool) (string, error) {
	c.refreshing.Lock()
	defer c.refreshing.Unlock()
	c.mu.Lock()
	old := c.token
	c.mu.Unlock()
	if old.AccessToken == "" && old.RefreshToken == "" {
		return "", ErrNotSignedIn
	}
	if old.Valid() && !force {
		return old.AccessToken, nil
	}
	if old.RefreshToken == "" {
		return "", ErrSignInExpired
	}
	t, err := c.Auth.Refresh(ctx, old.RefreshToken)
	if err != nil {
		return "", err
	}
	if t.Scope == "" {
		t.Scope = old.Scope // a refresh keeps the scopes granted
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token.RefreshToken != old.RefreshToken {
		// The user signed out, or in again, while it refreshed: that stands.
		if c.token.AccessToken == "" {
			return "", ErrNotSignedIn
		}
		return c.token.AccessToken, nil
	}
	c.token = t
	if c.OnToken != nil {
		go c.OnToken(t)
	}
	return t.AccessToken, nil
}

// get calls GET path and decodes the response into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// do calls the API. body, when not nil, is sent as JSON; the response, if
// out is not nil, is decoded into it.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	u := path
	if !strings.HasPrefix(path, "https://") && !strings.HasPrefix(path, "http://") {
		u = strings.TrimRight(c.BaseURL, "/") + path
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	refreshed, waited := false, false
	for {
		token, err := c.accessToken(ctx, false)
		if err != nil {
			return err
		}
		var r io.Reader
		if payload != nil {
			r = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, r)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("spotify: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("spotify: %w", err)
		}
		switch {
		case res.StatusCode == http.StatusUnauthorized && !refreshed:
			refreshed = true
			if _, err := c.accessToken(ctx, true); err != nil {
				return err
			}
			continue
		case res.StatusCode == http.StatusTooManyRequests && !waited:
			waited = true
			wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			if d := time.Duration(wait) * time.Second; d <= maxRetryAfter {
				select {
				case <-time.After(max(d, 500*time.Millisecond)):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		if res.StatusCode >= 300 {
			return apiError(res.StatusCode, data)
		}
		if out == nil || len(bytes.TrimSpace(data)) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("spotify: decoding %s: %w", path, err)
		}
		return nil
	}
}

// apiError reads the error of a failed response: the Web API's error
// object, or the plain text some of its refusals are, as that of a user an
// app in development mode does not count among its users.
func apiError(status int, data []byte) error {
	var body struct {
		Error struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
			Reason  string `json:"reason"`
		} `json:"error"`
		// Quota errors put these at the top level.
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return &APIError{Status: status, Message: plainText(data)}
	}
	e := &APIError{Status: status, Message: body.Error.Message, Reason: body.Error.Reason}
	if e.Message == "" {
		e.Message = body.Message
	}
	if e.Reason == "" {
		e.Reason = body.Reason
	}
	return e
}

// plainText is the text of a body that is not JSON, on one line and cut to
// a sentence's length, or "" for a page of HTML, which is no message.
func plainText(data []byte) string {
	s := strings.Join(strings.Fields(strings.ToValidUTF8(string(data), "")), " ")
	if strings.HasPrefix(s, "<") {
		return ""
	}
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}
