package lastfm

import (
	"context"
	"errors"
	"net/url"
)

// authURL is the page where the user approves a token.
const authURL = "https://www.last.fm/api/auth/"

// Session is the user's permission for the app to scrobble to their
// account. Its key does not expire, so the app keeps it until the user
// disconnects or Last.fm answers with ErrSessionExpired.
type Session struct {
	// Name is the user's Last.fm username.
	Name string `json:"name"`
	// Key signs the calls made for the user.
	Key string `json:"key"`
}

// Token returns a new request token, which the user approves on the page of
// AuthURL. A token is valid for 60 minutes and can be exchanged for a
// session once.
func (c *Client) Token(ctx context.Context) (string, error) {
	var answer struct {
		Token string `json:"token"`
	}
	if err := c.call(ctx, "auth.getToken", nil, &answer); err != nil {
		return "", err
	}
	if answer.Token == "" {
		return "", errors.New(serviceName(c.Name) + ": the token response has no token")
	}
	return answer.Token, nil
}

// AuthURL returns the address of the page where the user approves token, to
// open in their browser: Last.fm's, or the client's AuthPage.
func (c *Client) AuthURL(token string) string {
	page := c.AuthPage
	if page == "" {
		page = authURL
	}
	return page + "?api_key=" + url.QueryEscape(c.APIKey) + "&token=" + url.QueryEscape(token)
}

// Session exchanges a token the user approved for a session. While they
// have not approved it yet, it returns ErrNotAuthorized, so the app can
// call it again every few seconds until they do; any other error means the
// token cannot be used.
func (c *Client) Session(ctx context.Context, token string) (Session, error) {
	var answer struct {
		Session Session `json:"session"`
	}
	if err := c.call(ctx, "auth.getSession", url.Values{"token": {token}}, &answer); err != nil {
		return Session{}, err
	}
	if answer.Session.Key == "" {
		return Session{}, errors.New(serviceName(c.Name) + ": the session response has no key")
	}
	return answer.Session, nil
}
