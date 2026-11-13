// Package netease reads NetEase Cloud Music (网易云音乐) through the web
// player's weapi: the signed-in user's playlists and liked songs, search,
// albums, artists, lyrics, and the audio of songs.
package netease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	origin    = "https://music.163.com"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	// CookieName is the cookie of a NetEase Cloud Music session.
	CookieName = "MUSIC_U"
	// SignInURL is the web player's sign-in page, which shows the QR code
	// the NetEase Cloud Music app scans.
	SignInURL = origin + "/#/login"
)

var (
	// ErrSignedOut is returned for what needs a session while there is
	// none, or NetEase no longer takes it.
	ErrSignedOut = errors.New("netease: not signed in to NetEase Cloud Music")
	// ErrUnavailable is returned for a song whose audio NetEase doesn't
	// give: no rights in the region, or for members only.
	ErrUnavailable = errors.New("NetEase Cloud Music doesn't play this song for this account (no rights, or VIP only)")
)

// Client asks NetEase Cloud Music, as the user's session when it has one.
type Client struct {
	HTTP *http.Client
	// BaseURL is where the weapi is asked; tests change it.
	BaseURL string

	mu     sync.Mutex
	musicU string
}

// New returns a client without a session.
func New() *Client { return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}, BaseURL: origin} }

// SetSession signs the client in with the value of a MUSIC_U cookie, or out
// with "".
func (c *Client) SetSession(musicU string) {
	c.mu.Lock()
	c.musicU = musicU
	c.mu.Unlock()
}

// Session returns the value of the session's MUSIC_U cookie, or "", as
// for no client at all.
func (c *Client) Session() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.musicU
}

// SignedIn reports whether the client has a session.
func (c *Client) SignedIn() bool { return c.Session() != "" }

// call posts params to a weapi path and decodes the answer into out.
func (c *Client) call(ctx context.Context, path string, params map[string]any, out any) error {
	if params == nil {
		params = map[string]any{}
	}
	params["csrf_token"] = ""
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	form, err := weapi(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/weapi"+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("User-Agent", userAgent)
	cookie := "os=pc; appver=2.10.13"
	if u := c.Session(); u != "" {
		cookie += "; " + CookieName + "=" + u
	}
	req.Header.Set("Cookie", cookie)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	var status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return fmt.Errorf("netease: %s answered HTTP %d, not JSON", path, resp.StatusCode)
	}
	switch status.Code {
	case 200:
	case 301:
		return ErrSignedOut
	default:
		msg := status.Message + status.Msg
		if msg == "" {
			msg = "code " + strconv.Itoa(status.Code)
		}
		return fmt.Errorf("NetEase Cloud Music refused %s: %s", path, msg)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// Profile is the signed-in user.
type Profile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl,omitempty"`
	// VIP is set for a member, who may play the songs for members.
	VIP bool `json:"vip,omitempty"`
}

// Me returns the signed-in user.
func (c *Client) Me(ctx context.Context) (Profile, error) {
	if !c.SignedIn() {
		return Profile{}, ErrSignedOut
	}
	var r struct {
		Profile *struct {
			UserID    int64  `json:"userId"`
			Nickname  string `json:"nickname"`
			AvatarURL string `json:"avatarUrl"`
			VIPType   int    `json:"vipType"`
		} `json:"profile"`
	}
	if err := c.call(ctx, "/w/nuser/account/get", nil, &r); err != nil {
		return Profile{}, err
	}
	if r.Profile == nil {
		return Profile{}, ErrSignedOut // NetEase answers 200 with no profile once a session ended
	}
	p := r.Profile
	return Profile{ID: strconv.FormatInt(p.UserID, 10), Name: p.Nickname, ImageURL: p.AvatarURL, VIP: p.VIPType > 0}, nil
}
