// Package lyrics finds the lyrics of tracks on LRCLIB, an open database of
// lyrics synchronized to the music, and reads the LRC format they come in.
package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

const defaultBaseURL = "https://lrclib.net/api"

var (
	// ErrNotFound is returned for tracks without lyrics.
	ErrNotFound = errors.New("lyrics: not found")
	// ErrUnavailable is returned when LRCLIB can't be reached, or answers
	// with an error of its own, as when it is down: the track may have
	// lyrics all the same.
	ErrUnavailable = errors.New("lyrics: LRCLIB isn't answering")
)

// Line is a line of lyrics, with the time it is sung at when the lyrics
// are synchronized.
type Line struct {
	TimeMs int    `json:"timeMs"`
	Text   string `json:"text"`
}

// Lyrics are the words of a track.
type Lyrics struct {
	// Synced is set when the lines have times.
	Synced       bool   `json:"synced"`
	Instrumental bool   `json:"instrumental,omitempty"`
	Lines        []Line `json:"lines"`
	// Source names where the lyrics come from.
	Source string `json:"source"`
}

// Plain makes lyrics of text without times.
func Plain(text, source string) Lyrics {
	l := Lyrics{Lines: []Line{}, Source: source}
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		l.Lines = append(l.Lines, Line{Text: strings.TrimSpace(line)})
	}
	return l
}

// Client finds lyrics on LRCLIB.
type Client struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string
}

// New returns a client of LRCLIB introducing itself as userAgent, as
// LRCLIB asks apps to.
func New(userAgent string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, BaseURL: defaultBaseURL, UserAgent: userAgent}
}

// record is a track of LRCLIB.
type record struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// Get returns the lyrics of a track: the exact track when LRCLIB has it,
// else the search result closest in duration.
func (c *Client) Get(ctx context.Context, t media.Track) (Lyrics, error) {
	artist := ""
	if len(t.Artists) > 0 {
		artist = t.Artists[0].Name
	}
	q := url.Values{"track_name": {t.Title}, "artist_name": {artist}}
	if t.Album != nil && t.Album.Name != "" {
		q.Set("album_name", t.Album.Name)
	}
	if t.DurationMs > 0 {
		q.Set("duration", strconv.Itoa(int(math.Round(float64(t.DurationMs)/1000))))
	}
	var rec record
	err := c.get(ctx, "/get", q, &rec)
	if errors.Is(err, ErrNotFound) {
		var found []record
		if err := c.get(ctx, "/search", url.Values{"track_name": {t.Title}, "artist_name": {artist}}, &found); err != nil {
			return Lyrics{}, err
		}
		best, ok := closest(found, t.DurationMs)
		if !ok {
			return Lyrics{}, ErrNotFound
		}
		rec = best
	} else if err != nil {
		return Lyrics{}, err
	}
	return rec.lyrics()
}

func (r record) lyrics() (Lyrics, error) {
	switch {
	case r.Instrumental:
		return Lyrics{Instrumental: true, Lines: []Line{}, Source: "LRCLIB"}, nil
	case r.SyncedLyrics != "":
		return Lyrics{Synced: true, Lines: ParseLRC(r.SyncedLyrics), Source: "LRCLIB"}, nil
	case r.PlainLyrics != "":
		return Plain(r.PlainLyrics, "LRCLIB"), nil
	}
	return Lyrics{}, ErrNotFound
}

// closest returns the record with lyrics closest in duration to ms, if
// one is within ten seconds of it (any, when ms is unknown). Synced lyrics
// win over plain ones up to two seconds further off.
func closest(records []record, ms int) (record, bool) {
	var best record
	bestDiff := math.Inf(1)
	for _, r := range records {
		if r.SyncedLyrics == "" && r.PlainLyrics == "" && !r.Instrumental {
			continue
		}
		diff := 0.0
		if ms > 0 {
			diff = math.Abs(r.Duration - float64(ms)/1000)
		}
		if r.SyncedLyrics == "" {
			diff += 2
		}
		if diff < bestDiff {
			best, bestDiff = r, diff
		}
	}
	return best, bestDiff <= 12
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	}
	return fmt.Errorf("%w: HTTP %d", ErrUnavailable, res.StatusCode)
}

var (
	timeTagRe = regexp.MustCompile(`\[(\d+):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
	offsetRe  = regexp.MustCompile(`^\[offset:\s*([+-]?\d+)\]`)
)

// ParseLRC reads lyrics in the LRC format: lines of "[mm:ss.xx]text", a
// line possibly having several times. Lines without times are left out.
func ParseLRC(lrc string) []Line {
	lines := []Line{}
	offset := 0
	for raw := range strings.SplitSeq(strings.ReplaceAll(lrc, "\r\n", "\n"), "\n") {
		raw = strings.TrimSpace(raw)
		if m := offsetRe.FindStringSubmatch(raw); m != nil {
			offset, _ = strconv.Atoi(m[1])
			continue
		}
		tags := timeTagRe.FindAllStringSubmatchIndex(raw, -1)
		if len(tags) == 0 || tags[0][0] != 0 {
			continue // metadata such as [ar:…], or text without a time
		}
		end := 0
		var times []int
		for _, tag := range tags {
			if tag[0] != end {
				break // a tag inside the text
			}
			end = tag[1]
			times = append(times, tagTime(raw, tag))
		}
		text := strings.TrimSpace(raw[end:])
		for _, ms := range times {
			// A positive offset makes the lyrics come earlier.
			lines = append(lines, Line{TimeMs: max(ms-offset, 0), Text: text})
		}
	}
	slices.SortStableFunc(lines, func(a, b Line) int { return a.TimeMs - b.TimeMs })
	return lines
}

// tagTime returns the time of a [mm:ss.xx] tag found at loc in s.
func tagTime(s string, loc []int) int {
	min, _ := strconv.Atoi(s[loc[2]:loc[3]])
	sec, _ := strconv.Atoi(s[loc[4]:loc[5]])
	ms := 0
	if loc[6] >= 0 {
		frac := s[loc[6]:loc[7]]
		n, _ := strconv.Atoi(frac)
		switch len(frac) {
		case 1:
			ms = n * 100
		case 2:
			ms = n * 10
		default:
			ms = n
		}
	}
	return (min*60+sec)*1000 + ms
}
