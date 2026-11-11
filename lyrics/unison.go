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

// UnisonURL is the address of Unison, an open, crowdsourced database of
// lyrics, most of them synchronized word by word. Its lyrics are under the
// ODbL, and an app that shows them credits Unison and the person who
// submitted them, both linked: "Lyrics from Unison · submitted by <name>",
// Unison to the song's page there and the name to theirs. Unison asks apps
// for that from 7 February 2027.
const UnisonURL = "https://unison.betterlyrics.org"

// unisonTimeout bounds a lookup on Unison, which answers in a fraction of
// a second: lyrics never wait long on it, and LRCLIB is asked next.
const unisonTimeout = 3 * time.Second

// Unison finds lyrics on Unison. Reading from it needs no key.
type Unison struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string
}

// NewUnison returns a client of Unison introducing itself as userAgent.
func NewUnison(userAgent string) *Unison {
	return &Unison{HTTP: &http.Client{Timeout: unisonTimeout}, BaseURL: UnisonURL, UserAgent: userAgent}
}

// unisonAnswer is what Unison answers for a lookup.
type unisonAnswer struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Data    struct {
		Lyrics string `json:"lyrics"`
		// Format is "ttml", "lrc" or "plain".
		Format string `json:"format"`
		// Language is the lyrics' language, as "ja".
		Language string `json:"language"`
		// VideoID is the YouTube video the lyrics are of: the credit links
		// to its page on Unison.
		VideoID string `json:"videoId"`
		// Submitter is who gave the lyrics to Unison, its name and the ID
		// of its page there.
		Submitter struct {
			DisplayName string `json:"displayName"`
			KeyID       string `json:"keyId"`
		} `json:"submitter"`
	} `json:"data"`
}

// Get returns Unison's lyrics of a track: by its video ID when the query
// has one, else by its title, artist, album and duration, which Unison
// matches within two seconds. To credit them with, their Source is
// "Unison", their SourceURL the song's page on Unison, and By and ByURL
// the name and the page of who submitted them. Their Language is the one
// Unison tells.
//
// A track Unison has no lyrics of is ErrNotFound; Unison down, out of
// reach or slower than a few seconds is ErrUnavailable.
func (u *Unison) Get(ctx context.Context, t Query) (Lyrics, error) {
	q := url.Values{}
	switch {
	case t.VideoID != "":
		q.Set("v", t.VideoID)
	case t.Title != "" && t.Artist != "":
		q.Set("song", t.Title)
		q.Set("artist", t.Artist)
		if t.Album != "" {
			q.Set("album", t.Album)
		}
		if t.Duration > 0 {
			q.Set("duration", strconv.Itoa(int(math.Round(t.Duration.Seconds()))))
		}
	default:
		// Unison looks songs up by title and artist together.
		return Lyrics{}, ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.BaseURL+"/lyrics?"+q.Encode(), nil)
	if err != nil {
		return Lyrics{}, err
	}
	req.Header.Set("User-Agent", u.UserAgent)
	res, err := u.HTTP.Do(req)
	if err != nil {
		return Lyrics{}, unisonDown{err}
	}
	defer res.Body.Close()
	var ans unisonAnswer
	decodeErr := json.NewDecoder(res.Body).Decode(&ans)
	switch {
	case res.StatusCode == http.StatusNotFound && ans.Code == "NOT_FOUND":
		return Lyrics{}, ErrNotFound
	case res.StatusCode != http.StatusOK:
		return Lyrics{}, unisonDown{fmt.Errorf("HTTP %d", res.StatusCode)}
	case decodeErr != nil:
		return Lyrics{}, unisonDown{decodeErr}
	case !ans.Success || strings.TrimSpace(ans.Data.Lyrics) == "":
		return Lyrics{}, ErrNotFound
	}
	l, err := readUnison(ans.Data.Lyrics, ans.Data.Format)
	if err != nil {
		// Lyrics MixMusic can't read are as good as none.
		return Lyrics{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if len(l.Lines) == 0 {
		return Lyrics{}, ErrNotFound
	}
	l.Source, l.SourceURL = "Unison", unisonPage("song", ans.Data.VideoID)
	if name := strings.TrimSpace(ans.Data.Submitter.DisplayName); name != "" {
		l.By, l.ByURL = name, unisonPage("curator", ans.Data.Submitter.KeyID)
	}
	l.Language = languageCode(ans.Data.Language)
	return l, nil
}

// unisonPage returns the address of a page of Unison's: a song's
// (/song/<video ID>) or a submitter's (/curator/<key ID>). With no ID, it
// is Unison's own address for a song, and "" for a submitter.
func unisonPage(kind, id string) string {
	switch {
	case id != "":
		return UnisonURL + "/" + kind + "/" + url.PathEscape(id)
	case kind == "song":
		return UnisonURL
	}
	return ""
}

// readUnison reads lyrics in the format Unison names, or in the one they
// look like when it names another.
func readUnison(text, format string) (Lyrics, error) {
	if format != "ttml" && format != "lrc" && format != "plain" {
		switch trimmed := strings.TrimSpace(text); {
		case strings.HasPrefix(trimmed, "<"):
			format = "ttml"
		case timeTagRe.MatchString(trimmed) && strings.HasPrefix(trimmed, "["):
			format = "lrc"
		default:
			format = "plain"
		}
	}
	switch format {
	case "ttml":
		return ParseTTML(text)
	case "lrc":
		if lines := ParseLRC(text); len(lines) > 0 {
			return timedLyrics(lines, ""), nil
		}
	}
	return Plain(text, ""), nil
}

// unisonDown is Unison failing to answer, which is ErrUnavailable.
type unisonDown struct{ err error }

func (e unisonDown) Error() string        { return "lyrics: Unison isn't answering: " + e.err.Error() }
func (e unisonDown) Unwrap() error        { return e.err }
func (e unisonDown) Is(target error) bool { return target == ErrUnavailable }
