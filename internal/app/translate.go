package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/lyrics"
)

const (
	// translationsKept bounds the translations kept on disk: past it, the
	// least used go.
	translationsKept = 1000
	// firstHold is how long Unison isn't asked for translations after it
	// first says it is rate limited, and longestHold the longest: each
	// rate limit in a row holds it off twice as long as the one before.
	firstHold, longestHold = 30 * time.Second, 10 * time.Minute
)

// errHeldOff is a translation not asked for while Unison's translation is
// rate limited.
var errHeldOff = fmt.Errorf("%w: not asking again yet", lyrics.ErrRateLimited)

// translations keeps the translations of lyrics on disk, a file each, by
// track, language and lyrics, and holds off asking Unison for more while
// its translation is rate limited.
type translations struct {
	dir string
	// keep bounds the translations kept, translationsKept but in tests.
	keep int
	now  func() time.Time

	mu sync.Mutex
	// until is when Unison may be asked again, after a rate limit, and
	// hold how long that one held it off.
	until time.Time
	hold  time.Duration
}

func newTranslations(dir string) *translations {
	return &translations{dir: dir, keep: translationsKept, now: time.Now}
}

// translationKey names the translation of the lines of a track's lyrics
// into the language to: a translation of other lyrics of the track, as
// another source's, is another.
func translationKey(trackID, to string, lines []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%d", trackID, to, len(lines))
	for _, l := range lines {
		fmt.Fprintf(h, "\x00%s", l)
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// get returns the translation kept under key, of n lines.
func (t *translations) get(key string, n int) (lyrics.Translation, bool) {
	path := filepath.Join(t.dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return lyrics.Translation{}, false
	}
	var tr lyrics.Translation
	if json.Unmarshal(data, &tr) != nil || len(tr.Lines) != n {
		return lyrics.Translation{}, false
	}
	// Used now, it is among the last to go.
	now := t.now()
	os.Chtimes(path, now, now)
	return tr, true
}

// put keeps tr under key, and lets the least used translations go past
// those kept.
func (t *translations) put(key string, tr lyrics.Translation) error {
	data, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(t.dir, "*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err = cmp.Or(err, tmp.Close()); err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(t.dir, key+".json"))
	}
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	t.prune()
	return nil
}

// prune lets the least used translations go past those kept.
func (t *translations) prune() {
	entries, err := os.ReadDir(t.dir)
	if err != nil || len(entries) <= t.keep {
		return
	}
	type file struct {
		name string
		used time.Time
	}
	var files []file
	for _, e := range entries {
		if info, err := e.Info(); err == nil && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, file{e.Name(), info.ModTime()})
		}
	}
	slices.SortFunc(files, func(a, b file) int { return a.used.Compare(b.used) })
	for _, f := range files[:max(len(files)-t.keep, 0)] {
		os.Remove(filepath.Join(t.dir, f.name))
	}
}

// ready tells whether Unison may be asked for a translation: not while a
// rate limit holds it off.
func (t *translations) ready() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.now().Before(t.until)
}

// answered holds off asking Unison again after it said it was rate
// limited, twice as long each time in a row, and asks it again at once
// after it answered.
func (t *translations) answered(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case errors.Is(err, lyrics.ErrRateLimited):
		t.hold = min(max(t.hold*2, firstHold), longestHold)
		t.until = t.now().Add(t.hold)
	case err == nil:
		t.hold, t.until = 0, time.Time{}
	}
}

// resume asks Unison again at the next translation, as the user asked.
func (t *translations) resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.until = time.Time{}
}

// TranslateLyrics returns the translation of a track's lyrics into the
// language to, a code of ISO 639-1, with their romanization: the one kept
// on disk when there is one, else Unison's, which it then keeps. Unison
// gets the lines of the lyrics, their language when known, and the
// track's video ID when MixMusic knows it.
//
// After Unison said its translation was rate limited, it isn't asked
// again for a while, from 30 seconds to 10 minutes, unless TranslateAgain
// asks it to.
func (c *Catalog) TranslateLyrics(ctx context.Context, track media.Track, l lyrics.Lyrics, to string) (lyrics.Translation, error) {
	lines := make([]string, len(l.Lines))
	for i, line := range l.Lines {
		lines[i] = line.Text
	}
	key := translationKey(track.ID, to, lines)
	if tr, ok := c.a.translations.get(key, len(lines)); ok {
		return tr, nil
	}
	if !c.a.translations.ready() {
		return lyrics.Translation{}, translationError{errHeldOff}
	}
	tr, err := c.a.unison.Translate(ctx, lines, to, l.Language, c.videoID(track))
	c.a.translations.answered(err)
	if err != nil {
		log.Printf("mixmusic: the translation of the lyrics of %s: %v", track.ID, err)
		return lyrics.Translation{}, translationError{err}
	}
	if err := c.a.translations.put(key, tr); err != nil {
		log.Printf("mixmusic: keeping the translation of the lyrics of %s: %v", track.ID, err)
	}
	return tr, nil
}

// TranslateAgain has the next translation ask Unison, though it said its
// translation was rate limited not long ago: the user asked to try again.
func (c *Catalog) TranslateAgain() { c.a.translations.resume() }

// translationError is why a translation doesn't show, in the words the
// lyrics show it with.
type translationError struct{ err error }

func (e translationError) Error() string { return "Translation isn't available right now." }
func (e translationError) Unwrap() error { return e.err }

// Language returns the language the user reads, which lyrics translate to
// unless they chose another: the system's first preferred language, as a
// code to translate into, as "en" or "zh-Hant"; English before MixMusic
// runs.
func (s *System) Language() string {
	if !mygo.App.IsReady() {
		return "en"
	}
	langs := mygo.App.PreferredLanguages()
	if len(langs) == 0 {
		return "en"
	}
	return translationLanguage(langs[0])
}

// translationLanguage returns the language of a BCP 47 tag as a code to
// translate into: the language alone, as "pt" for "pt-BR", but Chinese in
// traditional characters, "zh-Hant", and the codes of today for the old.
func translationLanguage(tag string) string {
	parts := strings.Split(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"), "-")
	lang := strings.ToLower(parts[0])
	if len(lang) < 2 || len(lang) > 3 || strings.Trim(lang, "abcdefghijklmnopqrstuvwxyz") != "" {
		return "en"
	}
	switch lang {
	case "zh":
		// The script, when the tag names one, else the region.
		for _, p := range parts[1:] {
			switch strings.ToLower(p) {
			case "hans":
				return "zh"
			case "hant", "tw", "hk", "mo":
				return "zh-Hant"
			}
		}
	case "nb", "nn":
		return "no"
	case "iw":
		return "he"
	case "in":
		return "id"
	case "fil":
		return "tl"
	}
	return lang
}
