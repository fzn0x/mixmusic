// Package spotify reads the Spotify catalog and the user's library through
// the Spotify Web API, and plays tracks on Spotify Connect devices.
//
// Spotify lets an app in development mode serve its owner and a few users
// they invite, so each MixMusic user signs in with a Client ID of their own,
// which they create in the Spotify developer dashboard.
package spotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	authorizeURL = "https://accounts.spotify.com/authorize"
	tokenURL     = "https://accounts.spotify.com/api/token"

	// RedirectPort is the port of the loopback address Spotify sends the
	// user back to after they sign in. Apps list the redirect URI in the
	// developer dashboard, so it never changes.
	RedirectPort = 47823
	// RedirectURI is the address Spotify sends the user back to.
	RedirectURI = "http://127.0.0.1:47823/callback"
)

// Scopes are the permissions MixMusic asks for.
var Scopes = []string{
	"user-library-read",
	"user-library-modify",
	"user-follow-read",
	"user-follow-modify",
	"playlist-read-private",
	"playlist-read-collaborative",
	"playlist-modify-private",
	"playlist-modify-public",
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"user-read-recently-played",
	"user-top-read",
	// Where the user left a podcast's episode, which it resumes from.
	"user-read-playback-position",
	// The Web Playback SDK plays in MixMusic with these.
	"streaming",
	"user-read-email",
	"user-read-private",
}

// Token grants access to the API on the user's behalf.
type Token struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope,omitempty"`
}

// Valid reports whether the access token can still be used for a while.
func (t Token) Valid() bool {
	return t.AccessToken != "" && time.Until(t.Expiry) > time.Minute
}

// Auth signs users in with the authorization code flow and PKCE, which
// needs no client secret.
type Auth struct {
	HTTP *http.Client
	// TokenURL is the token endpoint, replaced in tests.
	TokenURL string

	mu       sync.Mutex
	clientID string
}

// NewAuth returns an Auth for the app of clientID.
func NewAuth(clientID string) *Auth {
	a := &Auth{HTTP: &http.Client{Timeout: 30 * time.Second}, TokenURL: tokenURL}
	a.SetClientID(clientID)
	return a
}

// ClientID returns the Client ID of the app users sign in to.
func (a *Auth) ClientID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.clientID
}

// SetClientID changes the Client ID of the app users sign in to. Tokens of
// the former app no longer refresh.
func (a *Auth) SetClientID(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clientID = strings.TrimSpace(id)
}

// ErrNoClientID is returned when signing in without a Client ID.
var ErrNoClientID = errors.New("spotify: set your Client ID in Settings › Accounts first")

// Login signs the user in: it calls open with the address of Spotify's
// sign-in page, which the app opens in the browser, waits for Spotify to
// send the user back, and returns the token. It gives up when ctx is done.
func (a *Auth) Login(ctx context.Context, open func(url string) error) (Token, error) {
	clientID := a.ClientID()
	if clientID == "" {
		return Token{}, ErrNoClientID
	}
	verifier := randomString(64)
	state := randomString(24)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", RedirectPort))
	if err != nil {
		return Token{}, fmt.Errorf("spotify: cannot listen for the sign-in callback on port %d: %w", RedirectPort, err)
	}
	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		var res result
		switch {
		case q.Get("state") != state:
			res.err = errors.New("spotify: the sign-in response does not match the request")
		case q.Get("error") != "":
			res.err = fmt.Errorf("spotify: sign-in failed: %s", q.Get("error"))
		case q.Get("code") == "":
			res.err = errors.New("spotify: the sign-in response has no code")
		default:
			res.code = q.Get("code")
		}
		writeCallbackPage(w, res.err)
		select {
		case results <- res:
		default:
		}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	if err := open(a.AuthorizeURL(state, challenge(verifier))); err != nil {
		return Token{}, err
	}
	select {
	case <-ctx.Done():
		return Token{}, ctx.Err()
	case res := <-results:
		if res.err != nil {
			return Token{}, res.err
		}
		return a.exchange(ctx, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {res.code},
			"redirect_uri":  {RedirectURI},
			"client_id":     {clientID},
			"code_verifier": {verifier},
		})
	}
}

// AuthorizeURL returns the address of Spotify's sign-in page.
func (a *Auth) AuthorizeURL(state, codeChallenge string) string {
	clientID := a.ClientID()
	q := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {RedirectURI},
		"state":                 {state},
		"scope":                 {strings.Join(Scopes, " ")},
		"code_challenge_method": {"S256"},
		"code_challenge":        {codeChallenge},
	}
	return authorizeURL + "?" + q.Encode()
}

// Refresh exchanges a refresh token for a new access token.
func (a *Auth) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	clientID := a.ClientID()
	t, err := a.exchange(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	})
	if err == nil && t.RefreshToken == "" {
		t.RefreshToken = refreshToken // Spotify may keep the old one
	}
	return t, err
}

// ErrSignInExpired is returned when the user must sign in again.
var ErrSignInExpired = errors.New("spotify: sign in again")

func (a *Auth) exchange(ctx context.Context, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.HTTP.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("spotify: %w", err)
	}
	defer res.Body.Close()
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return Token{}, fmt.Errorf("spotify: token response: %w", err)
	}
	if res.StatusCode != http.StatusOK || body.AccessToken == "" {
		if body.Error == "invalid_grant" {
			return Token{}, ErrSignInExpired
		}
		msg := body.Description
		if msg == "" {
			msg = body.Error
		}
		return Token{}, fmt.Errorf("spotify: token request failed: %s (HTTP %d)", msg, res.StatusCode)
	}
	return Token{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
		Scope:        body.Scope,
	}, nil
}

// challenge returns the PKCE code challenge of a verifier.
func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randomString returns n random characters allowed in a PKCE verifier.
func randomString(n int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// writeCallbackPage tells the user, in the browser, how signing in went.
func writeCallbackPage(w http.ResponseWriter, err error) {
	title, msg := "Signed in to Spotify", "You can close this tab and go back to MixMusic."
	if err != nil {
		title, msg = "Could not sign in", err.Error()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>MixMusic</title>
<body style="font-family:system-ui,sans-serif;display:grid;place-items:center;height:100vh;margin:0;background:#0e0f13;color:#f2f2f5">
<div style="text-align:center"><h1>%s</h1><p style="color:#a3a3b2">%s</p></div></body>`, html.EscapeString(title), html.EscapeString(msg))
}
