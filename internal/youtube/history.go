package youtube

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ReportPlay adds a track to the account's history on YouTube Music, as its
// own player does when a track starts: the history is what its home and
// its radios are made from. MixMusic plays the audio itself, so YouTube
// learns of the play only from this. Signed out, it does nothing.
func (c *Client) ReportPlay(ctx context.Context, videoID string) error {
	if c.session() != nil {
		return nil
	}
	_, err := c.reportPlay(ctx, videoID, playbackNonce())
	return err
}

// reportPlay asks for the player response of videoID, reports the play to
// its videostatsPlaybackUrl with the playback nonce cpn, and returns the
// response.
func (c *Client) reportPlay(ctx context.Context, videoID, cpn string) (node, error) {
	n, err := c.call(ctx, "player", map[string]any{
		"videoId":         videoID,
		"playbackContext": map[string]any{"contentPlaybackContext": map[string]any{}},
	}, nil)
	if err != nil {
		return node{}, err
	}
	base := n.get("playbackTracking", "videostatsPlaybackUrl", "baseUrl").str()
	if base == "" {
		return node{}, errors.New("youtube music: no playback report for this track")
	}
	u, err := url.Parse(base)
	if err != nil {
		return node{}, err
	}
	if !reportsTo(u) {
		return node{}, fmt.Errorf("youtube music: refusing to report the play to %s", u.Host)
	}
	q := u.Query()
	q.Set("ver", "2")
	q.Set("c", clientName)
	q.Set("cpn", cpn)
	u.RawQuery = q.Encode()
	return n, c.ping(ctx, u)
}

// ping sends one of the reports of YouTube's player, at u, with the
// session, as its website sends them.
func (c *Client) ping(ctx context.Context, u *url.URL) error {
	if !reportsTo(u) {
		return fmt.Errorf("youtube music: refusing to report to %s", u.Host)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", defaultBaseURL)
	req.Header.Set("Referer", defaultBaseURL+"/")
	c.signRequest(req, time.Now())
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("youtube music: %w", err)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	if res.StatusCode/100 != 2 {
		return &APIError{Status: res.StatusCode}
	}
	return nil
}

// reportsTo reports whether a play may be reported to u, with the session:
// the session goes to YouTube alone. It is a variable for tests.
var reportsTo = func(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (host == "youtube.com" || strings.HasSuffix(host, ".youtube.com"))
}

// playbackNonce is the 16 characters a player sends to tell one playback
// from another.
func playbackNonce() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	b := make([]byte, 16)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[b[i]&63]
	}
	return string(b)
}
