package lyrics

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// LrcRedURL is the base URL of lrc.red's lyrics lookup and search service.
	LrcRedURL = "https://lrc.red"
	// LrcRedSite is the attribution link shown with lyrics from lrc.red.
	LrcRedSite = "https://lrc.red"

	lrcRedTimeout = 3 * time.Second
	lrcRedPreload = 3
)

// LrcRed finds word- and line-synced TTML lyrics on lrc.red by ISRC first,
// falling back to its title/artist search when no ISRC is known.
type LrcRed struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string

	wait holdoff
}

// NewLrcRed returns a client for lrc.red.
func NewLrcRed(userAgent string) *LrcRed {
	return &LrcRed{
		HTTP:      &http.Client{Timeout: lrcRedTimeout},
		BaseURL:   LrcRedURL,
		UserAgent: userAgent,
	}
}

type lrcRedSearchResp struct {
	Hits []lrcRedHit `json:"hits"`
}

type lrcRedHit struct {
	ISRC     string  `json:"isrc"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Duration float64 `json:"duration"`
}

type lrcRedScored struct {
	hit   lrcRedHit
	isrc  string
	dur   time.Duration
	score float64
}

func cleanISRC(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(raw)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '-' {
			continue
		} else {
			return ""
		}
	}
	if b.Len() != 12 {
		return ""
	}
	return b.String()
}

// Asks reports whether t carries a valid ISRC or a title to look up on lrc.red.
func (c *LrcRed) Asks(t Query) bool {
	return c != nil && c.BaseURL != "" && (cleanISRC(t.ISRC) != "" || strings.TrimSpace(t.Title) != "")
}

func (c *LrcRed) searchHits(ctx context.Context, t Query) ([]lrcRedScored, error) {
	kw := strings.TrimSpace(t.Title)
	if kw == "" {
		return nil, ErrNotFound
	}
	if a := strings.TrimSpace(t.Artist); a != "" {
		kw = kw + " " + a
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/search.json?q=" + url.QueryEscape(kw)
	var resp lrcRedSearchResp
	if err := fetchJSON(ctx, c.HTTP, &c.wait, "lrc.red", c.UserAgent, u, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Hits) == 0 {
		return nil, ErrNotFound
	}
	scored := make([]lrcRedScored, 0, len(resp.Hits))
	seen := map[string]bool{}
	for _, h := range resp.Hits {
		isrc := cleanISRC(h.ISRC)
		if isrc == "" || seen[isrc] {
			continue
		}
		seen[isrc] = true
		dur := time.Duration(h.Duration * float64(time.Second))
		ta := titleAffinity(t.Title, h.Title)
		aa := artistAffinity(t.Artist, h.Artist)
		if ta == 0 && (normMatch(t.Artist) == "" || aa == 0) {
			continue
		}
		sc := matchScore(t.Title, t.Artist, t.Duration, h.Title, h.Artist, dur)
		scored = append(scored, lrcRedScored{hit: h, isrc: isrc, dur: dur, score: sc})
	}
	if len(scored) == 0 {
		return nil, ErrNotFound
	}
	slices.SortStableFunc(scored, func(a, b lrcRedScored) int {
		if a.score != b.score {
			if a.score > b.score {
				return -1
			}
			return 1
		}
		return 0
	})
	if len(scored) > 10 {
		scored = scored[:10]
	}
	return scored, nil
}

// Get fetches TTML lyrics from lrc.red by ISRC first, falling back to a strict
// title/artist search within a single 3-second deadline.
func (c *LrcRed) Get(ctx context.Context, t Query) (Lyrics, error) {
	if !c.Asks(t) {
		return Lyrics{}, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, lrcRedTimeout)
	defer cancel()

	if isrc := cleanISRC(t.ISRC); isrc != "" {
		l, err := c.GetByID(ctx, isrc)
		if err == nil {
			return l, nil
		}
		if !errors.Is(err, ErrNotFound) || strings.TrimSpace(t.Title) == "" {
			return Lyrics{}, err
		}
	}

	items, err := c.searchHits(ctx, t)
	if err != nil {
		return Lyrics{}, err
	}
	tried := 0
	for _, it := range items {
		if tried >= 3 {
			break
		}
		if titleAffinity(t.Title, it.hit.Title) == 0 {
			continue
		}
		if normMatch(t.Artist) != "" && artistAffinity(t.Artist, it.hit.Artist) == 0 {
			continue
		}
		if t.Duration > 0 && it.dur > 0 && (it.dur-t.Duration).Abs() > 15*time.Second {
			exact := normMatch(it.hit.Title) == normMatch(t.Title) &&
				(normMatch(t.Artist) == "" || normMatch(it.hit.Artist) == normMatch(t.Artist))
			if !exact {
				continue
			}
		}
		tried++
		l, err := c.GetByID(ctx, it.isrc)
		if err == nil {
			return l, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Lyrics{}, err
		}
	}
	return Lyrics{}, ErrNotFound
}

// Search returns matching lrc.red candidates, preloading the top matches
// concurrently and leaving the rest for lazy previewing via GetByID.
func (c *LrcRed) Search(ctx context.Context, t Query) ([]Candidate, error) {
	if c == nil || c.BaseURL == "" || strings.TrimSpace(t.Title) == "" {
		return nil, ErrNotFound
	}
	items, err := c.searchHits(ctx, t)
	if err != nil {
		return nil, err
	}
	preload := min(lrcRedPreload, len(items))
	preLyrics := make([]Lyrics, preload)
	preErrs := make([]error, preload)
	var wg sync.WaitGroup
	for i := range preload {
		wg.Go(func() {
			preLyrics[i], preErrs[i] = c.GetByID(ctx, items[i].isrc)
		})
	}
	wg.Wait()

	out := make([]Candidate, 0, len(items))
	for i, it := range items {
		cand := Candidate{
			ID:       it.isrc,
			Title:    it.hit.Title,
			Artist:   it.hit.Artist,
			Album:    it.hit.Album,
			Duration: it.dur,
		}
		if i < preload {
			if preErrs[i] != nil {
				continue
			}
			cand.Lyrics = preLyrics[i]
			cand.Loaded = true
			cand.SyncHint = syncHintOf(preLyrics[i])
		}
		out = append(out, cand)
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, nil
}

// GetByID fetches TTML lyrics for a specific ISRC from lrc.red.
func (c *LrcRed) GetByID(ctx context.Context, rawISRC string) (Lyrics, error) {
	isrc := cleanISRC(rawISRC)
	if c == nil || c.BaseURL == "" || isrc == "" {
		return Lyrics{}, ErrNotFound
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/s/" + isrc + ".ttml"
	body, code, err := fetchBytes(ctx, c.HTTP, &c.wait, "lrc.red", c.UserAgent, u, nil, 2<<20)
	if err != nil {
		return Lyrics{}, err
	}
	switch code {
	case http.StatusOK:
	case http.StatusNotFound:
		return Lyrics{}, ErrNotFound
	default:
		return Lyrics{}, providerDown{"lrc.red", fmt.Errorf("HTTP %d", code)}
	}
	raw := string(body)
	l, err := ParseTTML(raw)
	if err != nil {
		return Lyrics{}, providerDown{"lrc.red", err}
	}
	if len(l.Lines) == 0 {
		return Lyrics{}, ErrNotFound
	}
	l.Source = "lrc.red"
	l.SourceURL = LrcRedSite + "/s/" + isrc
	if l.Language == "" {
		l.Language = ttmlRootLang(raw)
	}
	return l, nil
}

func ttmlRootLang(raw string) string {
	d := xml.NewDecoder(strings.NewReader(raw))
	d.Strict = false
	for {
		tok, err := d.Token()
		if err != nil {
			return ""
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "tt" {
			return ""
		}
		for _, a := range se.Attr {
			if a.Name.Local == "lang" {
				lang := strings.TrimSpace(a.Value)
				if i := strings.IndexAny(lang, "-_"); i > 0 {
					lang = lang[:i]
				}
				return strings.ToLower(lang)
			}
		}
		return ""
	}
}
