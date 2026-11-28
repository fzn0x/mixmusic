package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The lyrics the listener picks for a track (#598), in place of those the
// lookup chooses (askLyrics): another source that is on, a caption track
// of its video by its language, a source asked for another title and
// artist, or lyrics they edited. A pick is kept for the track in the data
// folder, a file each in lyrics-picks/, lyricsPicksKept of them, the
// least used going past that. The answers of the sources asked for a
// track, by the lookup or for a pick, are kept in memory, so that the
// lyrics panel tells each source's, and a source that found lyrics shows
// them at once. A source the lookup didn't ask is asked only once the
// listener picks it.

// lyricsPicksKept is how many tracks' picks the data folder keeps.
const lyricsPicksKept = 10_000

// LyricsPick is what the listener picked for a track's lyrics. Its zero
// value is Automatic: the lookup's choice.
type LyricsPick struct {
	// Source is the key of the source picked, one of
	// settings.LyricsSourceKeys; "" for an edit.
	Source string `json:"source,omitempty"`
	// Caption is the language of the caption track picked, as YouTube
	// tells it ("en", "pt-BR"), of YouTube captions; "" for the one the
	// lookup takes.
	Caption string `json:"caption,omitempty"`
	// Title and Artist are what Source is asked for, as a search picked
	// them (SearchLyrics); "" asks for the track as the lookup does.
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
	// CandidateID identifies a specific version within a multi-result
	// source (e.g. lrc.red ISRC, LRCLIB record ID, QQ Music songmid, YouTube video ID).
	CandidateID string `json:"candidateId,omitempty"`
	// Candidate names the chosen candidate (e.g. its title) for display.
	Candidate string `json:"candidate,omitempty"`
	// Edited are the lyrics the listener edited, with the credit of the
	// lyrics they started from: they show before every source, and before
	// the lyrics of a file.
	Edited *lyrics.Lyrics `json:"edited,omitempty"`
}

// IsZero reports whether p is Automatic.
func (p LyricsPick) IsZero() bool { return p.Source == "" && p.Edited == nil }

// Searched reports whether p is a result of a search by title and artist
// or a specific candidate picked from a provider's search results.
func (p LyricsPick) Searched() bool {
	return p.Edited == nil && (p.Title != "" || p.Artist != "" || p.CandidateID != "")
}

// answerKey is the key the answer of p's source is kept under: its
// source's, of a caption track its language's, of a specific candidate
// its ID's, or of a search its title's and artist's.
func (p LyricsPick) answerKey() string {
	switch {
	case p.CandidateID != "":
		return p.Source + "\x00id:" + p.CandidateID
	case p.Searched():
		return p.Source + "\x00" + p.Title + "\x00" + p.Artist
	case p.Caption != "":
		return p.Source + ":" + p.Caption
	}
	return p.Source
}

// LyricsState is what a source of lyrics answered for a track.
type LyricsState int

const (
	// LyricsNotAsked: the source wasn't asked for the track yet.
	LyricsNotAsked LyricsState = iota
	// LyricsFound: it has lyrics of the track.
	LyricsFound
	// LyricsNotFound: it has none, or can't look the track up.
	LyricsNotFound
	// LyricsUnreachable: it couldn't be reached, or answered with an error.
	LyricsUnreachable
)

// LyricsChoice is a choice of the lyrics of a track: a source, a caption
// track, or a result of a search.
type LyricsChoice struct {
	// Pick is what choosing it picks.
	Pick LyricsPick
	// CaptionName is YouTube's name of the caption track, as "English",
	// for a language the window doesn't name itself.
	CaptionName string
	// State is what its source answered, and Lyrics its lyrics, when it
	// found some.
	State  LyricsState
	Lyrics lyrics.Lyrics
	// TrackTitle, TrackArtist, Album, and Duration describe a specific
	// candidate returned by a multi-result provider.
	TrackTitle  string
	TrackArtist string
	Album       string
	Duration    time.Duration
	// Loaded reports whether Lyrics is already populated (false for
	// lazy-loaded candidates until PreviewLyricsCandidate is called).
	Loaded bool
	// SyncHint hints the expected timing ("words", "lines", "plain")
	// before Loaded becomes true.
	SyncHint string
	// On is set for the choice picked, and Showing for the one whose
	// lyrics show.
	On, Showing bool
}

// LyricsChoices are the choices of the lyrics of a track, as the lyrics
// panel offers them.
type LyricsChoices struct {
	// Picked is the pick that shows, zero for Automatic: a pick of a
	// source switched off since shows the lookup's choice.
	Picked LyricsPick
	// Search is the result of a search picked, as it shows, when Picked is
	// one.
	Search *LyricsChoice
	// Sources are the sources that are on, in the user's order, each
	// caption track of the video in place of YouTube captions once they
	// are known.
	Sources []LyricsChoice
}

// ErrLyricsEmpty is lyrics edited to hold no words.
var ErrLyricsEmpty = errors.New("lyrics: the edit holds no words")

// lyricsByTitle are the sources that look lyrics up by a title and an
// artist, which a search asks for those given; the others look up the
// track's recording on YouTube.
var lyricsByTitle = map[string]bool{
	settings.LyricsBetterLyrics: true,
	settings.LyricsUnison:       true,
	settings.LyricsLrcRed:       true,
	settings.LyricsLRCLIB:       true,
	settings.LyricsQQMusic:      true,
	settings.LyricsYouTubePage:  true,
}

// lyricsAnswer is what a source answered for a track's lyrics, and the
// version of the timing of music videos' lyrics it saw (lyricsStay).
type lyricsAnswer struct {
	l    lyrics.Lyrics
	err  error
	seen int64
}

// lyricsAsked is what the sources answered for a track, as the settings
// of its lookup ask them: their answers by answerKey, the key of the one
// the lookup chose, and whether the file's own lyrics showed in its
// place. Its methods are safe for concurrent use; a nil one knows none.
type lyricsAsked struct {
	mu      sync.Mutex
	answers map[string]lyricsAnswer
	online  string
	file    bool
}

func (a *lyricsAsked) put(key string, ans lyricsAnswer) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.answers == nil {
		a.answers = map[string]lyricsAnswer{}
	}
	a.answers[key] = ans
}

func (a *lyricsAsked) answer(key string) (lyricsAnswer, bool) {
	if a == nil {
		return lyricsAnswer{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	ans, ok := a.answers[key]
	return ans, ok
}

// chose keeps which answer the lookup chose, "" for none, and whether the
// file's lyrics showed in its place.
func (a *lyricsAsked) chose(online string, file bool) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.online, a.file = online, file
}

func (a *lyricsAsked) shown() (online string, file bool) {
	if a == nil {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.online, a.file
}

// getOr returns what m keeps under key, or keeps and returns what fresh
// makes when it keeps nothing.
func (m *memo[T]) getOr(key string, fresh func() T) T {
	m.mu.Lock()
	v, ok := m.items[key]
	m.mu.Unlock()
	if ok {
		return v
	}
	v = fresh()
	m.mu.Lock()
	defer m.mu.Unlock()
	if kept, ok := m.items[key]; ok {
		return kept
	}
	m.order = append(m.order, key)
	if len(m.order) > m.max {
		delete(m.items, m.order[0])
		m.order = m.order[1:]
	}
	m.items[key] = v
	return v
}

// askedKey is the key of what the sources answered for a track, as set
// asks them: the lookup's.
func askedKey(track media.Track, set settings.Lyrics) string {
	return track.ID + " " + lyricsSetup(set)
}

// askedOf returns what the sources answered for a track, as set asks
// them, kept from now on.
func (c *Catalog) askedOf(track media.Track, set settings.Lyrics) *lyricsAsked {
	if c.a.lyricsAsked == nil {
		return &lyricsAsked{}
	}
	return c.a.lyricsAsked.getOr(askedKey(track, set), func() *lyricsAsked { return &lyricsAsked{} })
}

// knownAsked returns what the sources answered for a track, as set asks
// them, without keeping anything: nil when nothing was asked.
func (c *Catalog) knownAsked(track media.Track, set settings.Lyrics) *lyricsAsked {
	if c.a.lyricsAsked == nil {
		return nil
	}
	a, _ := c.a.lyricsAsked.get(askedKey(track, set))
	return a
}

// answerState is the state of an answer.
func answerState(ans lyricsAnswer) LyricsState {
	switch {
	case ans.err == nil:
		return LyricsFound
	case lyricsNone(ans.err), errors.Is(ans.err, match.ErrLocal), errors.Is(ans.err, lyrics.ErrNotFoundYet):
		return LyricsNotFound
	}
	return LyricsUnreachable
}

// sourceOn reports whether the source key is on in set.
func sourceOn(set settings.Lyrics, key string) bool {
	return slices.ContainsFunc(set.SourceOrder(), func(s settings.LyricsSource) bool { return s.Key == key && !s.Off })
}

// pickedLyrics returns the lyrics of a track the listener picked, ok
// false while they picked none, or a source switched off since.
func (c *Catalog) pickedLyrics(ctx context.Context, track media.Track) (l lyrics.Lyrics, ok bool, err error) {
	pick, ok := c.a.lyricsPicks.get(track.ID)
	if !ok {
		return lyrics.Lyrics{}, false, nil
	}
	if pick.Edited != nil {
		l = *pick.Edited
		ensureLyricsLanguage(&l)
		return l, true, nil
	}
	set := c.a.settings.Get().Lyrics
	if !sourceOn(set, pick.Source) || lyricsLookups[pick.Source] == nil {
		return lyrics.Lyrics{}, false, nil
	}
	asked := c.askedOf(track, set)
	key := pick.answerKey()
	ans, known := c.answerOf(asked, track, set, pick)
	// An answer kept shows at once, but one that failed is asked again, as
	// is LRCLIB's of a music video timed since (lyricsStay).
	if !known || answerState(ans) == LyricsUnreachable ||
		track.Video && pick.Source == settings.LyricsLRCLIB && !c.a.lyricsStay(track.ID, ans.seen) {
		ans = lyricsAnswer{seen: c.a.lyricsTiming.version.Load()}
		ans.l, ans.err = c.askPick(ctx, track, pick, set)
		asked.put(key, ans)
	}
	switch answerState(ans) {
	case LyricsNotFound:
		return lyrics.Lyrics{}, true, lyricsError{locales.LyricsNotFound(), lyrics.ErrNotFound}
	case LyricsUnreachable:
		log.Printf("mixmusic: the lyrics of %s, on %s, picked: %v", track.ID, pick.Source, ans.err)
		return lyrics.Lyrics{}, true, lyricsError{locales.LyricsCantLoad(), ans.err}
	}
	l = ans.l
	ensureLyricsLanguage(&l)
	return l, true, nil
}

// answerOf returns the answer of pick's source kept in asked: its own, or,
// for the caption track the lookup takes, the lookup's.
func (c *Catalog) answerOf(asked *lyricsAsked, track media.Track, set settings.Lyrics, pick LyricsPick) (lyricsAnswer, bool) {
	if ans, ok := asked.answer(pick.answerKey()); ok {
		return ans, true
	}
	if pick.Source == settings.LyricsCaptions && pick.Caption != "" && !pick.Searched() {
		if t, ok := captionTrack(c.captionList(track), set.LyricsCaptionLanguage, false); ok && t.Language == pick.Caption {
			return asked.answer(settings.LyricsCaptions)
		}
	}
	return lyricsAnswer{}, false
}

// askPick asks pick's source for the lyrics of a track: for a specific
// candidate ID when set, for its title and artist when a search picked
// them, else as the lookup asks it.
func (c *Catalog) askPick(ctx context.Context, track media.Track, pick LyricsPick, set settings.Lyrics) (lyrics.Lyrics, error) {
	q := lyricsQuery(track)
	q.VideoID = c.videoID(track)
	if pick.Searched() {
		q = searchQuery(track, pick.Title, pick.Artist)
	}
	if pick.CandidateID != "" {
		if byID := lyricsByIDLookups[pick.Source]; byID != nil {
			return byID(c, ctx, track, q, pick.CandidateID)
		}
	}
	if pick.Source == settings.LyricsCaptions && pick.Caption != "" {
		return c.captionLyricsIn(ctx, track, pick.Caption)
	}
	return lyricsLookups[pick.Source](c, ctx, track, q)
}

// searchQuery asks for a track by the title and artist given: its album
// and length stay, which Better Lyrics needs, and its video ID, Spotify ID
// and ISRC go, so manual searches look up what the user typed.
func searchQuery(track media.Track, title, artist string) lyrics.Query {
	q := lyricsQuery(track)
	q.Title, q.Artist, q.VideoID, q.SpotifyID, q.ISRC = title, artist, "", "", ""
	return q
}

// LyricsSearchQuery returns the title and artist the lyrics of a track are
// looked up by: those of the search picked, else those of the lookup.
func (c *Catalog) LyricsSearchQuery(track media.Track) (title, artist string) {
	if pick, ok := c.a.lyricsPicks.get(track.ID); ok && pick.Searched() {
		return pick.Title, pick.Artist
	}
	q := lyricsQuery(track)
	return q.Title, q.Artist
}

// LyricsChoices returns the choices of the lyrics of a track, from what
// the sources answered so far: it asks none.
func (c *Catalog) LyricsChoices(track media.Track) LyricsChoices {
	set := c.a.settings.Get().Lyrics
	var ch LyricsChoices
	pick, picked := c.a.lyricsPicks.get(track.ID)
	if picked && pick.Edited == nil && !sourceOn(set, pick.Source) {
		picked = false
	}
	asked := c.knownAsked(track, set)
	// shown is the source whose lyrics show, and shownKey the key of its
	// answer: none for lyrics edited, or a file's.
	shown, shownKey := "", ""
	switch {
	case picked && pick.Edited != nil:
	case picked:
		if ans, ok := c.answerOf(asked, track, set, pick); ok && ans.err == nil {
			shown, shownKey = pick.Source, pick.answerKey()
		}
	default:
		if online, file := asked.shown(); !file {
			shown, shownKey = online, online
		}
	}
	if picked {
		ch.Picked = pick
	}
	if picked && pick.Searched() {
		s := LyricsChoice{Pick: pick, On: true, Showing: shownKey == pick.answerKey()}
		if ans, ok := asked.answer(pick.answerKey()); ok {
			s.State, s.Lyrics = answerState(ans), ans.l
		}
		ch.Search = &s
	}
	// A source picked as the lookup asks it is on.
	sourcePicked := picked && !pick.Searched() && pick.Edited == nil
	for _, src := range set.SourceOrder() {
		if src.Off || lyricsLookups[src.Key] == nil {
			continue
		}
		if src.Key == settings.LyricsCaptions {
			if tracks := c.captionList(track); len(tracks) > 0 {
				auto, _ := captionTrack(tracks, set.LyricsCaptionLanguage, false)
				for _, t := range tracks {
					p := LyricsPick{Source: src.Key, Caption: t.Language}
					row := LyricsChoice{Pick: p, CaptionName: t.Name}
					if ans, ok := c.answerOf(asked, track, set, p); ok {
						row.State, row.Lyrics = answerState(ans), ans.l
					}
					lookup := t.Language == auto.Language
					row.On = sourcePicked && pick.Source == src.Key && (pick.Caption == t.Language || pick.Caption == "" && lookup)
					row.Showing = shown == src.Key && (shownKey == p.answerKey() || shownKey == src.Key && lookup)
					ch.Sources = append(ch.Sources, row)
				}
				continue
			}
		}
		row := LyricsChoice{Pick: LyricsPick{Source: src.Key}}
		if ans, ok := asked.answer(src.Key); ok {
			row.State, row.Lyrics = answerState(ans), ans.l
		}
		row.On = sourcePicked && pick.Source == src.Key
		row.Showing = shownKey == src.Key
		ch.Sources = append(ch.Sources, row)
	}
	return ch
}

// SearchLyrics asks every source that is on for the lyrics of a track by
// the title and artist given, at once, and returns their candidates in the
// user's order of the sources, those that failed too. Multi-result
// providers return every matching candidate so the listener can browse
// and pick an exact version.
func (c *Catalog) SearchLyrics(ctx context.Context, track media.Track, title, artist string) []LyricsChoice {
	set := c.a.settings.Get().Lyrics
	title, artist = strings.TrimSpace(title), strings.TrimSpace(artist)
	var keys []string
	for _, src := range set.SourceOrder() {
		if !src.Off && lyricsLookups[src.Key] != nil {
			keys = append(keys, src.Key)
		}
	}
	perSource := make([][]LyricsChoice, len(keys))
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Go(func() {
			q := lyricsQuery(track)
			q.VideoID = c.videoID(track)
			if lyricsByTitle[key] {
				q = searchQuery(track, title, artist)
			}
			if searchFn := lyricsSearchLookups[key]; searchFn != nil {
				cands, err := searchFn(c, ctx, track, q)
				if err != nil || len(cands) == 0 {
					if err == nil {
						err = lyrics.ErrNotFound
					}
					ch := LyricsChoice{Pick: LyricsPick{Source: key}, State: answerState(lyricsAnswer{err: err})}
					if lyricsByTitle[key] {
						ch.Pick.Title, ch.Pick.Artist = title, artist
					}
					if ch.State == LyricsUnreachable {
						log.Printf("mixmusic: searching the lyrics of %s on %s: %v", track.ID, key, err)
					}
					perSource[i] = []LyricsChoice{ch}
					return
				}
				choices := make([]LyricsChoice, 0, len(cands))
				for _, cand := range cands {
					l := cand.Lyrics
					if cand.Loaded {
						ensureLyricsLanguage(&l)
					}
					pick := LyricsPick{
						Source:      key,
						Title:       title,
						Artist:      artist,
						CandidateID: cand.ID,
						Candidate:   cand.Title,
					}
					choices = append(choices, LyricsChoice{
						Pick:        pick,
						State:       LyricsFound,
						Lyrics:      l,
						TrackTitle:  cand.Title,
						TrackArtist: cand.Artist,
						Album:       cand.Album,
						Duration:    cand.Duration,
						Loaded:      cand.Loaded,
						SyncHint:    cand.SyncHint,
					})
				}
				perSource[i] = choices
				return
			}
			ch := LyricsChoice{Pick: LyricsPick{Source: key}}
			if lyricsByTitle[key] {
				ch.Pick.Title, ch.Pick.Artist = title, artist
			}
			l, err := lyricsLookups[key](c, ctx, track, q)
			if err == nil {
				ensureLyricsLanguage(&l)
			}
			ch.State, ch.Lyrics = answerState(lyricsAnswer{l: l, err: err}), l
			ch.Loaded = err == nil
			if ch.Loaded {
				switch {
				case l.WordTimed():
					ch.SyncHint = "words"
				case l.Synced:
					ch.SyncHint = "lines"
				default:
					ch.SyncHint = "plain"
				}
			}
			if ch.State == LyricsUnreachable {
				log.Printf("mixmusic: searching the lyrics of %s on %s: %v", track.ID, key, err)
			}
			perSource[i] = []LyricsChoice{ch}
		})
	}
	wg.Wait()
	var found []LyricsChoice
	for _, list := range perSource {
		found = append(found, list...)
	}
	return found
}

// PreviewLyricsCandidate fetches the lyrics of an unloaded candidate on
// demand and caches the answer so picking it afterward shows at once.
func (c *Catalog) PreviewLyricsCandidate(ctx context.Context, track media.Track, pick LyricsPick) (lyrics.Lyrics, error) {
	set := c.a.settings.Get().Lyrics
	asked := c.askedOf(track, set)
	if ans, ok := c.answerOf(asked, track, set, pick); ok && answerState(ans) != LyricsUnreachable {
		if ans.err != nil {
			return lyrics.Lyrics{}, ans.err
		}
		return ans.l, nil
	}
	seen := c.a.lyricsTiming.version.Load()
	l, err := c.askPick(ctx, track, pick, set)
	if err == nil {
		ensureLyricsLanguage(&l)
	}
	asked.put(pick.answerKey(), lyricsAnswer{l: l, err: err, seen: seen})
	return l, err
}

// PickLyrics keeps pick for the lyrics of a track, the zero pick going
// back to the lookup's choice, and tells the windows. found are the
// lyrics the pick's source found, as a search's, which then show at once.
func (c *Catalog) PickLyrics(track media.Track, pick LyricsPick, found *lyrics.Lyrics) error {
	set := c.a.settings.Get().Lyrics
	if found != nil && pick.Edited == nil && !pick.IsZero() {
		c.askedOf(track, set).put(pick.answerKey(), lyricsAnswer{l: *found, seen: c.a.lyricsTiming.version.Load()})
	}
	err := c.a.lyricsPicks.set(track.ID, pick)
	ready := pick.Edited != nil
	switch {
	case pick.IsZero():
		// The lookup's lyrics are kept, or the file's are read at once.
		_, ready = c.a.lyricsMemo.get(askedKey(track, set))
		_, file := c.knownAsked(track, set).shown()
		ready = ready || file
	case !ready:
		ans, ok := c.answerOf(c.knownAsked(track, set), track, set, pick)
		ready = ok && answerState(ans) != LyricsUnreachable
	}
	c.a.lyricsTold.tell(track.ID, ready)
	return err
}

func ensureLyricsLanguage(l *lyrics.Lyrics) {
	if l.Language == "" && l.Translation != nil && l.Translation.From != "" {
		l.Language = l.Translation.From
	}
	if l.Language == "" {
		l.Language = lyrics.DetectLanguage(l.Lines)
	}
}

// EditLyrics keeps the lyrics of a track the listener edited, text as
// LRC or plain text (lyrics.Lyrics.LRC), with the credit of from, those
// they started from, and returns them; ErrLyricsEmpty for text of no
// words.
func (c *Catalog) EditLyrics(track media.Track, text string, from lyrics.Lyrics) (lyrics.Lyrics, error) {
	l, ok := lyrics.FromText(text, from.Source)
	if !ok {
		return lyrics.Lyrics{}, ErrLyricsEmpty
	}
	l.SourceURL, l.By, l.ByURL = from.SourceURL, from.By, from.ByURL
	ensureLyricsLanguage(&l)
	if l.Language == "" {
		l.Language = from.Language
	}
	return l, c.PickLyrics(track, LyricsPick{Edited: &l}, nil)
}

// OnLyricsPicked has fn told, from any goroutine, the ID of a track whose
// lyrics the listener picked, and whether they show at once, until
// cancel: the windows look them up again then.
func (a *App) OnLyricsPicked(fn func(trackID string, ready bool)) (cancel func()) {
	return a.lyricsTold.add(fn)
}

// lyricsTold are those OnLyricsPicked tells.
type lyricsTold struct {
	mu   sync.Mutex
	told []*func(string, bool)
}

func (t *lyricsTold) add(fn func(string, bool)) (cancel func()) {
	f := &fn
	t.mu.Lock()
	t.told = append(slices.Clone(t.told), f)
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		t.told = slices.DeleteFunc(slices.Clone(t.told), func(g *func(string, bool)) bool { return g == f })
		t.mu.Unlock()
	}
}

func (t *lyricsTold) tell(trackID string, ready bool) {
	t.mu.Lock()
	told := t.told
	t.mu.Unlock()
	for _, fn := range told {
		(*fn)(trackID, ready)
	}
}

// lyricsPicks keeps the picks of the tracks' lyrics in a folder, a file
// each, named by a hash of the track's ID, keep of them at most, the
// least used going first; it knows those it read or kept this run. Its
// methods are safe for concurrent use; a nil one keeps none.
type lyricsPicks struct {
	dir  string
	keep int

	mu    sync.Mutex
	known map[string]LyricsPick
}

func newLyricsPicks(dir string) *lyricsPicks {
	return &lyricsPicks{dir: dir, keep: lyricsPicksKept, known: map[string]LyricsPick{}}
}

// keptPick is a pick, as its file keeps it.
type keptPick struct {
	Track string     `json:"track"`
	Pick  LyricsPick `json:"pick"`
}

// path is the file of the pick of the track trackID.
func (p *lyricsPicks) path(trackID string) string {
	h := sha256.Sum256([]byte(trackID))
	return filepath.Join(p.dir, hex.EncodeToString(h[:16])+".json")
}

// get returns the pick of the track trackID, ok false for none.
func (p *lyricsPicks) get(trackID string) (LyricsPick, bool) {
	if p == nil {
		return LyricsPick{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	pick, ok := p.known[trackID]
	if !ok {
		pick = p.read(trackID)
		if len(p.known) >= 4096 {
			clear(p.known)
		}
		p.known[trackID] = pick
	}
	return pick, !pick.IsZero()
}

// read reads the pick of the track trackID from its file: none when there
// is none, or it doesn't read. Its caller holds p.mu.
func (p *lyricsPicks) read(trackID string) LyricsPick {
	path := p.path(trackID)
	data, err := os.ReadFile(path)
	if err != nil {
		return LyricsPick{}
	}
	var k keptPick
	if json.Unmarshal(data, &k) != nil || k.Track != trackID {
		return LyricsPick{}
	}
	// Used now, it is among the last to go.
	now := time.Now()
	os.Chtimes(path, now, now)
	return k.Pick
}

// set keeps pick for the track trackID, the zero pick none, and lets the
// least used go past those kept.
func (p *lyricsPicks) set(trackID string, pick LyricsPick) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.known[trackID] = pick
	path := p.path(trackID)
	if pick.IsZero() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(keptPick{Track: trackID, Pick: pick})
	if err != nil {
		return err
	}
	if err := store.WriteFile(path, data); err != nil {
		return err
	}
	p.prune()
	return nil
}

// prune lets the least used picks go past those kept. Its caller holds
// p.mu.
func (p *lyricsPicks) prune() {
	entries, err := os.ReadDir(p.dir)
	if err != nil || len(entries) <= p.keep {
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
	for _, f := range files[:max(len(files)-p.keep, 0)] {
		os.Remove(filepath.Join(p.dir, f.name))
	}
	// What it knows of those gone, it reads again.
	clear(p.known)
}

// captionList returns the caption tracks of a track's video that lyrics
// come from, those its uploader wrote, as the lookup or a pick listed them:
// none until then. It asks nothing.
func (c *Catalog) captionList(track media.Track) []stream.CaptionTrack {
	if c.a.captionLists == nil {
		return nil
	}
	id := c.videoID(track)
	if id == "" {
		return nil
	}
	tracks, _ := c.a.captionLists.get(id)
	return tracks
}
