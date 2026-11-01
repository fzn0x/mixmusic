package lyrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"
)

// ErrRateLimited is returned when Unison's translation is rate limited by
// the translation service it asks, as it sometimes is: the lyrics can be
// translated later. It is ErrUnavailable too.
var ErrRateLimited = errors.New("lyrics: translation is rate limited")

// Translation is lyrics translated into another language, and in Latin
// letters, line by line.
type Translation struct {
	// From is the language the lyrics are in, as Unison told or detected
	// it, and To the one they were translated to: codes of ISO 639-1, as
	// "ja" and "en". From is "" when Unison had nothing to translate.
	From string `json:"from"`
	To   string `json:"to"`
	// Lines are the lines of the lyrics translated, one for each of their
	// lines, in order.
	Lines []TranslatedLine `json:"lines"`
}

// TranslatedLine is a line of lyrics translated, and in Latin letters.
type TranslatedLine struct {
	// Text is the line's translation, "" for a line that needs none, as a
	// blank line, or a line already in the language.
	Text string `json:"text,omitempty"`
	// Romanized is the line in Latin letters, as romaji for Japanese, ""
	// for a line in them already.
	Romanized string `json:"romanized,omitempty"`
}

const (
	// translateTimeout bounds a translation, which Unison may ask the
	// translation service for while the lyrics wait in their language.
	translateTimeout = 20 * time.Second
	// maxLines and maxLineLength are what Unison translates at once: 200
	// lines of 500 characters, as JavaScript counts them.
	maxLines, maxLineLength = 200, 500
)

// translateAnswer is what Unison answers for a translation.
type translateAnswer struct {
	Lines []struct {
		Translation      *string `json:"translation"`
		Romanization     *string `json:"romanization"`
		NeedsTranslation bool    `json:"needsTranslation"`
	} `json:"lines"`
	DetectedLang string `json:"detectedLang"`
}

// Translate translates lines of lyrics into the language to, and
// romanizes them, on Unison, which asks Google's translation of lyrics
// for them. from is their language, "" to have Unison detect it, and
// videoID the ID of the track's video, if known, which Unison keeps with
// its translation.
//
// Lyrics of more than 200 lines are asked for 200 at a time, and lines
// longer than 500 characters stay untranslated. Lyrics in the language
// to already, or with nothing to translate, translate to lines with
// nothing in them. Unison down, out of reach or rate limited is
// ErrUnavailable; rate limited, ErrRateLimited too.
func (u *Unison) Translate(ctx context.Context, lines []string, to, from, videoID string) (Translation, error) {
	t := Translation{To: to, Lines: make([]TranslatedLine, 0, len(lines))}
	for start := 0; start < len(lines); start += maxLines {
		part := lines[start:min(start+maxLines, len(lines))]
		got, lang, err := u.translate(ctx, part, to, from, videoID)
		if err != nil {
			return Translation{}, err
		}
		if t.From == "" {
			t.From = lang
		}
		t.Lines = append(t.Lines, got...)
	}
	return t, nil
}

// translate asks Unison for the translation of at most 200 lines.
func (u *Unison) translate(ctx context.Context, lines []string, to, from, videoID string) ([]TranslatedLine, string, error) {
	body := struct {
		Lines   []string `json:"lines"`
		To      string   `json:"to"`
		From    string   `json:"from,omitempty"`
		VideoID string   `json:"videoId,omitempty"`
	}{make([]string, len(lines)), to, from, videoID}
	for i, l := range lines {
		if len(utf16.Encode([]rune(l))) <= maxLineLength {
			body.Lines[i] = l
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.BaseURL+"/translate", bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", u.UserAgent)
	// A translation may take longer than a lookup: the client's own
	// timeout, bounding lookups, gives way to translateTimeout.
	client := *u.HTTP
	client.Timeout = translateTimeout
	res, err := client.Do(req)
	if err != nil {
		return nil, "", unisonDown{err}
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusBadRequest:
		// No line to translate, or no language Unison could tell.
		return make([]TranslatedLine, len(lines)), "", nil
	case res.StatusCode == http.StatusServiceUnavailable || res.StatusCode == http.StatusTooManyRequests:
		return nil, "", unisonDown{fmt.Errorf("%w: HTTP %d", ErrRateLimited, res.StatusCode)}
	case res.StatusCode != http.StatusOK:
		return nil, "", unisonDown{fmt.Errorf("HTTP %d", res.StatusCode)}
	}
	var ans translateAnswer
	if err := json.NewDecoder(res.Body).Decode(&ans); err != nil {
		return nil, "", unisonDown{err}
	}
	if len(ans.Lines) != len(lines) {
		return nil, "", unisonDown{fmt.Errorf("%d lines translated of %d", len(ans.Lines), len(lines))}
	}
	out := make([]TranslatedLine, len(lines))
	for i, a := range ans.Lines {
		// What only repeats the line adds nothing to it.
		if a.NeedsTranslation && a.Translation != nil && !same(*a.Translation, lines[i]) {
			out[i].Text = strings.TrimSpace(*a.Translation)
		}
		if a.Romanization != nil && !same(*a.Romanization, lines[i]) {
			out[i].Romanized = strings.TrimSpace(*a.Romanization)
		}
	}
	return out, strings.TrimSpace(ans.DetectedLang), nil
}

// same tells whether a translation or a romanization is the line itself.
func same(s, line string) bool {
	return strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(line))
}
