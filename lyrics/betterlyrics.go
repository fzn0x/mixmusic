package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BetterLyricsURL is the address of Better Lyrics' API. Its lyrics are
// timed word by word, and its terms ask apps that show them to credit
// Better Lyrics, whose address is BetterLyricsSite.
const BetterLyricsURL = "https://api.betterlyrics.org"

// BetterLyricsSite is Better Lyrics' own address, which the lyrics from
// its API are credited with.
const BetterLyricsSite = "https://betterlyrics.org"

// betterLyricsTimeout bounds a lookup on Better Lyrics' API, which answers
// from its cache in under a second: lyrics never wait long on it, and
// Unison is asked next.
const betterLyricsTimeout = 3 * time.Second

// BetterLyrics finds word-synced lyrics on Better Lyrics' API. Reading
// from it needs no key, but without one it answers only for songs it has
// already kept, and finds them only by their title, artist, album and
// length, all four.
type BetterLyrics struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string
}

// NewBetterLyrics returns a client of Better Lyrics' API introducing
// itself as userAgent.
func NewBetterLyrics(userAgent string) *BetterLyrics {
	return &BetterLyrics{HTTP: &http.Client{Timeout: betterLyricsTimeout}, BaseURL: BetterLyricsURL, UserAgent: userAgent}
}

// Get returns Better Lyrics' lyrics of a track, when it has them timed word
// by word. Their Source is "Better Lyrics", their SourceURL Better Lyrics'
// address, to credit them with.
//
// A track it has no lyrics of, or only lines timed, and a query without a
// title, artist, album and duration, which it can't find a song by without
// a key, is ErrNotFound; Better Lyrics down, busy or out of reach is
// ErrUnavailable. A nil client finds nothing.
func (b *BetterLyrics) Get(ctx context.Context, t Query) (Lyrics, error) {
	if b == nil || t.Title == "" || t.Artist == "" || t.Album == "" || t.Duration <= 0 {
		return Lyrics{}, ErrNotFound
	}
	q := url.Values{}
	q.Set("s", t.Title)
	q.Set("a", t.Artist)
	q.Set("al", t.Album)
	q.Set("d", strconv.Itoa(int(math.Round(t.Duration.Seconds()))))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.BaseURL+"/getLyrics?"+q.Encode(), nil)
	if err != nil {
		return Lyrics{}, err
	}
	req.Header.Set("User-Agent", b.UserAgent)
	res, err := b.HTTP.Do(req)
	if err != nil {
		return Lyrics{}, betterLyricsDown{err}
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusUnauthorized, http.StatusUnprocessableEntity:
		// 401 is a song it hasn't kept and would need a key to fetch.
		return Lyrics{}, ErrNotFound
	default:
		// 429 is MixMusic asking too fast, 5xx the service failing.
		return Lyrics{}, betterLyricsDown{fmt.Errorf("HTTP %d", res.StatusCode)}
	}
	var ans struct {
		TTML string `json:"ttml"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ans); err != nil {
		return Lyrics{}, betterLyricsDown{err}
	}
	if strings.TrimSpace(ans.TTML) == "" {
		return Lyrics{}, ErrNotFound
	}
	l, err := ParseTTML(ans.TTML)
	if err != nil {
		// Lyrics MixMusic can't read are as good as none.
		return Lyrics{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if !wordTimed(l) {
		return Lyrics{}, ErrNotFound
	}
	l.Source, l.SourceURL = "Better Lyrics", BetterLyricsSite
	return l, nil
}

// wordTimed reports whether some line of l is timed word by word, which
// is what Better Lyrics is asked for.
func wordTimed(l Lyrics) bool {
	if !l.Synced {
		return false
	}
	for _, line := range l.Lines {
		if len(line.Words) > 0 {
			return true
		}
	}
	return false
}

// betterLyricsDown is Better Lyrics failing to answer, which is
// ErrUnavailable.
type betterLyricsDown struct{ err error }

func (e betterLyricsDown) Error() string {
	return "lyrics: Better Lyrics isn't answering: " + e.err.Error()
}
func (e betterLyricsDown) Unwrap() error        { return e.err }
func (e betterLyricsDown) Is(target error) bool { return target == ErrUnavailable }
