package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Candidate is one version of a track's lyrics offered by a provider
// that returns multiple search results.
type Candidate struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	Lyrics   Lyrics
	// Loaded reports whether Lyrics is already populated; when false, the
	// caller can fetch it on demand with GetByID.
	Loaded bool
	// SyncHint describes the expected timing before Lyrics is loaded:
	// "words", "lines", "plain", or "".
	SyncHint string
}

// syncHintOf returns the SyncHint describing l's timing.
func syncHintOf(l Lyrics) string {
	switch {
	case l.WordTimed():
		return "words"
	case l.Synced:
		return "lines"
	default:
		return "plain"
	}
}

// matchScore ranks how closely a search result matches the desired
// title, artist and duration. Higher is better.
func matchScore(wantTitle, wantArtist string, wantDur time.Duration, gotTitle, gotArtist string, gotDur time.Duration) float64 {
	var score float64
	if normMatch(wantTitle) != "" {
		score += titleAffinity(wantTitle, gotTitle)
	}
	if normMatch(wantArtist) != "" {
		score += artistAffinity(wantArtist, gotArtist)
	}

	if wantDur > 0 && gotDur > 0 {
		diff := math.Abs((wantDur - gotDur).Seconds())
		switch {
		case diff <= 2:
			score += 20
		case diff <= 5:
			score += 12
		case diff <= 10:
			score += 5
		case diff > 15:
			score -= 20 + (diff - 15)
		}
	}
	return score
}

func titleAffinity(wantTitle, gotTitle string) float64 {
	wt, gt := normMatch(wantTitle), normMatch(gotTitle)
	if wt == "" {
		return 1
	}
	if gt == "" {
		return 0
	}
	switch {
	case wt == gt:
		return 50
	case strings.Contains(gt, wt) || strings.Contains(wt, gt):
		return 28
	default:
		return tokenOverlap(wt, gt) * 20
	}
}

func artistAffinity(wantArtist, gotArtist string) float64 {
	wa, ga := normMatch(wantArtist), normMatch(gotArtist)
	if wa == "" {
		return 1
	}
	if ga == "" {
		return 0
	}
	switch {
	case wa == ga:
		return 40
	case strings.Contains(ga, wa) || strings.Contains(wa, ga):
		return 25
	default:
		return tokenOverlap(wa, ga) * 18
	}
}

// TitleAffinity scores how closely gotTitle matches wantTitle (0 for none).
func TitleAffinity(wantTitle, gotTitle string) float64 { return titleAffinity(wantTitle, gotTitle) }

// ArtistAffinity scores how closely gotArtist matches wantArtist (0 for none).
func ArtistAffinity(wantArtist, gotArtist string) float64 {
	return artistAffinity(wantArtist, gotArtist)
}

// NormMatch normalizes s into lowercase alphanumeric/space tokens for matching.
func NormMatch(s string) string { return normMatch(s) }

func normMatch(s string) string {
	s = norm.NFKC.String(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			space = true
		}
	}
	return b.String()
}

func tokenOverlap(a, b string) float64 {
	ta := strings.Fields(a)
	tb := strings.Fields(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	set := make(map[string]bool, len(tb))
	for _, w := range tb {
		set[w] = true
	}
	hits := 0
	for _, w := range ta {
		if set[w] {
			hits++
		}
	}
	return float64(hits) / float64(max(len(ta), len(tb)))
}

// providerDown wraps a transport, rate-limit, or server error from a
// lyrics provider so errors.Is(err, ErrUnavailable) reports true.
type providerDown struct {
	source string
	err    error
}

func (e providerDown) Error() string {
	return "lyrics: " + e.source + " isn't answering: " + e.err.Error()
}
func (e providerDown) Unwrap() error        { return e.err }
func (e providerDown) Is(target error) bool { return target == ErrUnavailable }

// fetchBytes performs a bounded HTTP GET with holdoff handling for 429s.
func fetchBytes(ctx context.Context, hc *http.Client, wait *holdoff, source, ua, rawURL string, headers map[string]string, limit int64) ([]byte, int, error) {
	if wait != nil && wait.active() {
		return nil, http.StatusTooManyRequests, providerDown{source, errors.New("HTTP 429, asking too fast")}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, 0, providerDown{source, err}
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests && wait != nil {
		wait.set(res.Header.Get("Retry-After"))
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit))
	if err != nil {
		return nil, res.StatusCode, providerDown{source, err}
	}
	return body, res.StatusCode, nil
}

// fetchJSON performs fetchBytes and decodes JSON into out for 200 OK.
func fetchJSON(ctx context.Context, hc *http.Client, wait *holdoff, source, ua, rawURL string, headers map[string]string, out any) error {
	body, code, err := fetchBytes(ctx, hc, wait, source, ua, rawURL, headers, 4<<20)
	if err != nil {
		return err
	}
	switch code {
	case http.StatusOK:
		if err := json.Unmarshal(body, out); err != nil {
			return providerDown{source, err}
		}
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return providerDown{source, fmt.Errorf("HTTP %d", code)}
	}
}
