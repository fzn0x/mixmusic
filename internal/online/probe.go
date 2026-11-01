package online

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// The addresses Probe asks, and how; tests change them. YouTube's answer
// 204 with no body, where yt-dlp and YouTube Music ask; a captive portal's
// page doesn't. Spotify's API answers anything at all.
var (
	youtubeProbes = []string{"https://www.youtube.com/generate_204", "https://music.youtube.com/generate_204"}
	spotifyProbe  = "https://api.spotify.com/v1/"
	probeClient   = newProbeClient()
)

// newProbeClient returns a client that asks once, on a connection of its
// own, and takes no redirect: a portal redirects.
func newProbeClient() *http.Client {
	c := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t = t.Clone()
		t.DisableKeepAlives = true
		c.Transport = t
	}
	return c
}

// Probe asks YouTube and Spotify whether they answer, at once, and returns
// nil as soon as one does, or why none did.
func Probe(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	n := len(youtubeProbes) + 1
	answers := make(chan error, n)
	for _, u := range youtubeProbes {
		go func() { answers <- ask(ctx, u, http.StatusNoContent) }()
	}
	go func() { answers <- ask(ctx, spotifyProbe, 0) }()
	var errs []error
	for range n {
		err := <-answers
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ask asks u, and returns nil when it answers status, or anything when
// status is 0.
func ask(ctx context.Context, u string, status int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	res, err := probeClient.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if status != 0 && res.StatusCode != status {
		return fmt.Errorf("%s answered HTTP %d", u, res.StatusCode)
	}
	return nil
}
