package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	lyricsURL     = "https://spclient.wg.spotify.com"
	lyricsTimeout = 3 * time.Second
)

// ErrLyricsNotFound reports that Spotify has no lyrics for the requested track.
var ErrLyricsNotFound = errors.New("spotify: lyrics not found")

// ErrLyricsRateLimited reports that Spotify's color-lyrics endpoint answered 429.
var ErrLyricsRateLimited = errors.New("spotify: lyrics rate limited")

// LyricLine is one line returned by Spotify's color-lyrics endpoint.
type LyricLine struct {
	StartMs int
	Words   string
}

// TrackLyrics holds the lines and metadata returned by Spotify's color-lyrics endpoint.
type TrackLyrics struct {
	Synced   bool
	Language string
	Lines    []LyricLine
}

// TrackLyrics fetches Spotify's own lyrics for trackID using the active web
// session token (sp_dc). Callers outside a web session receive ErrNotSignedIn.
func (c *Client) TrackLyrics(ctx context.Context, trackID string) (TrackLyrics, error) {
	if c == nil || !c.IsWebSession() {
		return TrackLyrics{}, ErrNotSignedIn
	}
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return TrackLyrics{}, ErrLyricsNotFound
	}
	c.lyricsWaitMu.Lock()
	until := c.lyricsWaitUntil
	c.lyricsWaitMu.Unlock()
	if time.Now().Before(until) {
		return TrackLyrics{}, ErrLyricsRateLimited
	}
	out, retry, err := c.fetchTrackLyrics(ctx, trackID, false)
	if retry {
		out, _, err = c.fetchTrackLyrics(ctx, trackID, true)
	}
	return out, err
}

func (c *Client) fetchTrackLyrics(ctx context.Context, trackID string, forceRefresh bool) (TrackLyrics, bool, error) {
	tok, err := c.accessToken(ctx, forceRefresh)
	if err != nil {
		return TrackLyrics{}, false, err
	}
	base := c.LyricsURL
	if base == "" {
		base = lyricsURL
	}
	reqCtx, cancel := context.WithTimeout(ctx, lyricsTimeout)
	defer cancel()
	u := strings.TrimRight(base, "/") + "/color-lyrics/v2/track/" + url.PathEscape(trackID) + "?format=json&vocalRemoval=false&market=from_token"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return TrackLyrics{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("app-platform", "WebPlayer")
	req.Header.Set("User-Agent", webPlayerUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://open.spotify.com")
	req.Header.Set("Referer", "https://open.spotify.com/")

	res, err := c.HTTP.Do(req)
	if err != nil {
		return TrackLyrics{}, false, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusUnauthorized:
		return TrackLyrics{}, !forceRefresh, &APIError{Status: res.StatusCode, Message: "unauthorized"}
	case http.StatusNotFound, http.StatusForbidden:
		return TrackLyrics{}, false, ErrLyricsNotFound
	case http.StatusTooManyRequests:
		wait := parseLyricsRetryAfter(res.Header.Get("Retry-After"))
		if wait <= 0 {
			wait = time.Minute
		}
		if wait > time.Hour {
			wait = time.Hour
		}
		c.lyricsWaitMu.Lock()
		c.lyricsWaitUntil = time.Now().Add(wait)
		c.lyricsWaitMu.Unlock()
		return TrackLyrics{}, false, ErrLyricsRateLimited
	case http.StatusOK:
	default:
		return TrackLyrics{}, false, fmt.Errorf("spotify: color-lyrics HTTP %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return TrackLyrics{}, false, err
	}
	var payload struct {
		Lyrics struct {
			SyncType string `json:"syncType"`
			Language string `json:"language"`
			Lines    []struct {
				StartTimeMs string `json:"startTimeMs"`
				Words       string `json:"words"`
			} `json:"lines"`
		} `json:"lyrics"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TrackLyrics{}, false, fmt.Errorf("spotify: decoding color-lyrics: %w", err)
	}

	synced := payload.Lyrics.SyncType == "LINE_SYNCED" || payload.Lyrics.SyncType == "SYLLABLE_SYNCED"
	lines := make([]LyricLine, 0, len(payload.Lyrics.Lines))
	for _, raw := range payload.Lyrics.Lines {
		words := strings.TrimSpace(raw.Words)
		if words == "" || words == "♪" {
			continue
		}
		var ms int
		if synced && raw.StartTimeMs != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(raw.StartTimeMs)); err == nil && parsed >= 0 {
				ms = parsed
			}
		}
		lines = append(lines, LyricLine{StartMs: ms, Words: words})
	}
	if len(lines) == 0 {
		return TrackLyrics{}, false, ErrLyricsNotFound
	}
	return TrackLyrics{
		Synced:   synced,
		Language: strings.ToLower(strings.TrimSpace(payload.Lyrics.Language)),
		Lines:    lines,
	}, false, nil
}

func parseLyricsRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
