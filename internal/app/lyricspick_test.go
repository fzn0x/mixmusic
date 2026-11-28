package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/lyrics"
)

// withPicks has the catalog of ls keep picks in a folder of the test's,
// and what the sources answered, as the app does. It returns the picks
// it was told of, as "<track> ready" or "<track> waits".
func withPicks(t *testing.T, ls *lyricsSources) func() []string {
	t.Helper()
	a := ls.c.a
	a.lyricsPicks = newLyricsPicks(filepath.Join(t.TempDir(), "lyrics-picks"))
	a.lyricsAsked = newMemo[*lyricsAsked](8)
	a.captionLists = newMemo[[]stream.CaptionTrack](8)
	var mu sync.Mutex
	var told []string
	cancel := a.OnLyricsPicked(func(id string, ready bool) {
		mu.Lock()
		defer mu.Unlock()
		if ready {
			told = append(told, id+" ready")
		} else {
			told = append(told, id+" waits")
		}
	})
	t.Cleanup(cancel)
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := told
		told = nil
		return out
	}
}

// choiceOf returns the choice of the source key among ch's, and of its
// caption track lang for YouTube captions.
func choiceOf(t *testing.T, ch LyricsChoices, key, lang string) LyricsChoice {
	t.Helper()
	for _, c := range ch.Sources {
		if c.Pick.Source == key && c.Pick.Caption == lang {
			return c
		}
	}
	t.Fatalf("no choice of %s %s in %+v", key, lang, ch.Sources)
	return LyricsChoice{}
}

// lyricsOf returns the lyrics of a track, failing the test on an error.
func lyricsOf(t *testing.T, ls *lyricsSources, track media.Track) lyrics.Lyrics {
	t.Helper()
	l, err := ls.c.Lyrics(context.Background(), track)
	if err != nil {
		t.Fatalf("lyrics of %s: %v", track.ID, err)
	}
	return l
}

// TestLyricsPickSource tells what each source answered, picks a source
// the lookup didn't ask, which is asked then, and one that found lyrics,
// which show at once, then goes back to the lookup's choice.
func TestLyricsPickSource(t *testing.T) {
	ls := newLyricsSources(t)
	told := withPicks(t, ls)
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	if l := lyricsOf(t, ls, aSong); l.Source != "Unison" {
		t.Fatalf("lyrics from %s", l.Source)
	}
	ch := ls.c.LyricsChoices(aSong)
	if u := choiceOf(t, ch, settings.LyricsUnison, ""); u.State != LyricsFound || !u.Showing || u.On || len(u.Lyrics.Lines) != 2 {
		t.Errorf("Unison: %+v", u)
	}
	if l := choiceOf(t, ch, settings.LyricsLRCLIB, ""); l.State != LyricsNotAsked || l.Showing {
		t.Errorf("LRCLIB, not asked: %+v", l)
	}
	if b := choiceOf(t, ch, settings.LyricsBetterLyrics, ""); b.State != LyricsNotFound {
		t.Errorf("Better Lyrics, which isn't there: %+v", b)
	}
	if !ch.Picked.IsZero() || ch.Search != nil {
		t.Errorf("picked %+v before a pick", ch.Picked)
	}

	if err := ls.c.PickLyrics(aSong, LyricsPick{Source: settings.LyricsLRCLIB}, nil); err != nil {
		t.Fatal(err)
	}
	if got := told(); len(got) != 1 || got[0] != aSong.ID+" waits" {
		t.Errorf("told %q of a source not asked", got)
	}
	if l := lyricsOf(t, ls, aSong); l.Source != "LRCLIB" || l.Lines[0].Text != "From LRCLIB" || ls.lrcAsks.Load() != 1 {
		t.Fatalf("picked LRCLIB: %+v, asked %d times", l, ls.lrcAsks.Load())
	}
	ch = ls.c.LyricsChoices(aSong)
	if l := choiceOf(t, ch, settings.LyricsLRCLIB, ""); !l.On || !l.Showing || l.State != LyricsFound {
		t.Errorf("LRCLIB, picked: %+v", l)
	}
	if u := choiceOf(t, ch, settings.LyricsUnison, ""); u.On || u.Showing || u.State != LyricsFound {
		t.Errorf("Unison, not picked: %+v", u)
	}

	if err := ls.c.PickLyrics(aSong, LyricsPick{Source: settings.LyricsUnison}, nil); err != nil {
		t.Fatal(err)
	}
	if got := told(); len(got) != 1 || got[0] != aSong.ID+" ready" {
		t.Errorf("told %q of a source that found lyrics", got)
	}
	if l := lyricsOf(t, ls, aSong); l.Source != "Unison" || ls.unisonAsks.Load() != 1 {
		t.Errorf("picked Unison again: %+v, asked %d times", l, ls.unisonAsks.Load())
	}
	if err := ls.c.PickLyrics(aSong, LyricsPick{}, nil); err != nil {
		t.Fatal(err)
	}
	if l := lyricsOf(t, ls, aSong); l.Source != "Unison" || ls.unisonAsks.Load() != 1 || ls.lrcAsks.Load() != 1 {
		t.Errorf("automatic: %+v; Unison asked %d times, LRCLIB %d", l, ls.unisonAsks.Load(), ls.lrcAsks.Load())
	}
	if got := told(); len(got) != 1 || got[0] != aSong.ID+" ready" {
		t.Errorf("told %q of Automatic", got)
	}
}

// TestLyricsPickNotFound says no lyrics were found for a source picked
// that has none, and asks again one that couldn't be reached.
func TestLyricsPickNotFound(t *testing.T) {
	ls := newLyricsSources(t)
	withPicks(t, ls)
	ls.unison = status(http.StatusOK, unisonSynced)
	if err := ls.c.PickLyrics(aSong, LyricsPick{Source: settings.LyricsLRCLIB}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ls.c.Lyrics(context.Background(), aSong); !errors.Is(err, lyrics.ErrNotFound) || err.Error() != "No lyrics found for this track." {
		t.Errorf("LRCLIB has none: %v", err)
	}
	asks := ls.lrcAsks.Load()
	ls.lrclib = status(http.StatusBadGateway, "")
	if _, err := ls.c.Lyrics(context.Background(), aSong); !errors.Is(err, lyrics.ErrNotFound) || ls.lrcAsks.Load() != asks {
		t.Errorf("none found is kept: %v, LRCLIB asked %d more times", err, ls.lrcAsks.Load()-asks)
	}
	if c := choiceOf(t, ls.c.LyricsChoices(aSong), settings.LyricsLRCLIB, ""); c.State != LyricsNotFound || !c.On || c.Showing {
		t.Errorf("LRCLIB: %+v", c)
	}

	ls = newLyricsSources(t)
	withPicks(t, ls)
	ls.lrclib = status(http.StatusBadGateway, "")
	ls.c.PickLyrics(aSong, LyricsPick{Source: settings.LyricsLRCLIB}, nil)
	if _, err := ls.c.Lyrics(context.Background(), aSong); !errors.Is(err, lyrics.ErrUnavailable) || err.Error() != "Lyrics can't be loaded right now. Check your connection, or try again in a moment." {
		t.Errorf("LRCLIB down: %v", err)
	}
	if c := choiceOf(t, ls.c.LyricsChoices(aSong), settings.LyricsLRCLIB, ""); c.State != LyricsUnreachable {
		t.Errorf("LRCLIB down: %+v", c)
	}
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	if l := lyricsOf(t, ls, aSong); l.Source != "LRCLIB" {
		t.Errorf("LRCLIB back: %+v", l)
	}
}

// TestLyricsPickSwitchedOff shows the lookup's choice for a track whose
// source picked was switched off since, and the pick once it's on again.
func TestLyricsPickSwitchedOff(t *testing.T) {
	ls := newLyricsSources(t)
	withPicks(t, ls)
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	ls.c.PickLyrics(aSong, LyricsPick{Source: settings.LyricsLRCLIB}, nil)
	withSources(t, ls, false, settings.LyricsSource{Key: settings.LyricsLRCLIB, Off: true})
	if l := lyricsOf(t, ls, aSong); l.Source != "Unison" {
		t.Errorf("LRCLIB off: lyrics from %s", l.Source)
	}
	if ch := ls.c.LyricsChoices(aSong); !ch.Picked.IsZero() {
		t.Errorf("LRCLIB off: picked %+v", ch.Picked)
	}
	withSources(t, ls, false)
	if l := lyricsOf(t, ls, aSong); l.Source != "LRCLIB" {
		t.Errorf("LRCLIB on again: lyrics from %s", l.Source)
	}
}

// TestLyricsPicksKept keeps a pick for its track across runs, and lets
// the least used go past those kept.
func TestLyricsPicksKept(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "lyrics-picks")
	p := newLyricsPicks(dir)
	p.keep = 2
	pick := LyricsPick{Source: settings.LyricsLRCLIB, Title: "Other", Artist: "Someone"}
	for _, id := range []string{"yt:a", "yt:b"} {
		if err := p.set(id, pick); err != nil {
			t.Fatal(err)
		}
	}
	if got, ok := newLyricsPicks(dir).get("yt:a"); !ok || got != pick {
		t.Fatalf("read again: %+v, %v", got, ok)
	}
	// yt:a, read last, was used last: yt:b, older, goes.
	old := time.Now().Add(-time.Hour)
	os.Chtimes(p.path("yt:b"), old, old)
	if err := p.set("yt:c", pick); err != nil {
		t.Fatal(err)
	}
	q := newLyricsPicks(dir)
	for id, want := range map[string]bool{"yt:a": true, "yt:b": false, "yt:c": true} {
		if _, ok := q.get(id); ok != want {
			t.Errorf("%s kept: %v, want %v", id, ok, want)
		}
	}
	if err := q.set("yt:a", LyricsPick{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := newLyricsPicks(dir).get("yt:a"); ok {
		t.Error("Automatic kept a pick")
	}
	if lyricsPicksKept < 10_000 {
		t.Errorf("only %d tracks' picks are kept", lyricsPicksKept)
	}
}

// TestLyricsSearch asks every source that is on for another title and
// artist at once, lists those that failed too, and keeps the result picked
// with its title and artist, which are asked again the next time.
func TestLyricsSearch(t *testing.T) {
	ls := newLyricsSources(t)
	withPicks(t, ls)
	ls.unison = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("song") == "Other Song" && r.URL.Query().Get("artist") == "Other Artist" && r.URL.Query().Get("v") == "" {
			w.Write([]byte(unisonPlain))
			return
		}
		status(http.StatusNotFound, unisonMiss)(w, r)
	}
	ls.lrclib = status(http.StatusBadGateway, "")
	found := ls.c.SearchLyrics(context.Background(), aSong, " Other Song ", "Other Artist")
	var keys []string
	for _, f := range found {
		keys = append(keys, f.Pick.Source)
	}
	if strings.Join(keys, " ") != "betterlyrics unison lrcred spotify lrclib qqmusic youtube captions youtubepage" {
		t.Fatalf("results of %q", keys)
	}
	u := found[1]
	if u.State != LyricsFound || u.Lyrics.Source != "Unison" || len(u.Lyrics.Lines) != 3 || u.Pick.Title != "Other Song" || u.Pick.Artist != "Other Artist" {
		t.Errorf("Unison's: %+v", u)
	}
	if found[4].State != LyricsUnreachable || found[4].Pick.Title != "Other Song" {
		t.Errorf("LRCLIB's, down: %+v", found[4])
	}
	if found[6].Pick.Title != "" {
		t.Errorf("YouTube Music, which looks up the track, is asked for a title: %+v", found[6])
	}
	asks := ls.unisonAsks.Load()
	if err := ls.c.PickLyrics(aSong, u.Pick, &u.Lyrics); err != nil {
		t.Fatal(err)
	}
	if l := lyricsOf(t, ls, aSong); l.Lines[0].Text != "Plain from Unison" || ls.unisonAsks.Load() != asks {
		t.Errorf("picked: %+v, Unison asked %d more times", l, ls.unisonAsks.Load()-asks)
	}
	ch := ls.c.LyricsChoices(aSong)
	if ch.Search == nil || !ch.Search.Showing || ch.Search.Pick != u.Pick || choiceOf(t, ch, settings.LyricsUnison, "").On {
		t.Errorf("choices after the search: %+v", ch)
	}
	if title, artist := ls.c.LyricsSearchQuery(aSong); title != "Other Song" || artist != "Other Artist" {
		t.Errorf("searches again %q by %q", title, artist)
	}
	// The next time it plays, its title and artist are asked again.
	ls.c.a.lyricsAsked = newMemo[*lyricsAsked](8)
	if l := lyricsOf(t, ls, aSong); l.Lines[0].Text != "Plain from Unison" || ls.unisonAsks.Load() != asks+1 {
		t.Errorf("played again: %+v, Unison asked %d more times", l, ls.unisonAsks.Load()-asks)
	}
}

// TestLyricsEdit keeps the lyrics edited, with the credit of those they
// started from, before every source, until Automatic replaces them.
func TestLyricsEdit(t *testing.T) {
	ls := newLyricsSources(t)
	withPicks(t, ls)
	ls.unison = status(http.StatusOK, unisonSynced)
	from := lyricsOf(t, ls, aSong)
	text := strings.Replace(from.LRC(), "Second line", "Second lime", 1)
	if _, err := ls.c.EditLyrics(aSong, " \n", from); !errors.Is(err, ErrLyricsEmpty) {
		t.Errorf("edited to nothing: %v", err)
	}
	if _, err := ls.c.EditLyrics(aSong, text, from); err != nil {
		t.Fatal(err)
	}
	asks := ls.unisonAsks.Load() + ls.lrcAsks.Load()
	ls.c.a.lyricsMemo = newMemo[lyrics.Lyrics](4)
	l := lyricsOf(t, ls, aSong)
	if !l.Synced || l.Lines[1].Text != "Second lime" || l.Lines[1].TimeMs != 4000 || l.Source != "Unison" || l.SourceURL != from.SourceURL {
		t.Errorf("edited: %+v", l)
	}
	if n := ls.unisonAsks.Load() + ls.lrcAsks.Load(); n != asks {
		t.Errorf("sources asked %d times for lyrics edited", n-asks)
	}
	if ch := ls.c.LyricsChoices(aSong); ch.Picked.Edited == nil || choiceOf(t, ch, settings.LyricsUnison, "").Showing {
		t.Errorf("choices of lyrics edited: %+v", ch)
	}
	ls.c.PickLyrics(aSong, LyricsPick{}, nil)
	if l := lyricsOf(t, ls, aSong); l.Lines[1].Text != "Second line" {
		t.Errorf("automatic: %+v", l)
	}
}

// TestLyricsPickCaptionTrack lists each caption track of the video its
// uploader wrote by its language once captions were asked, the lookup's
// showing, and picks another, which is asked then.
func TestLyricsPickCaptionTrack(t *testing.T) {
	ls := newLyricsSources(t)
	withPicks(t, ls)
	cs := withCaptions(t, ls)
	if l := lyricsOf(t, ls, captionedVideo); l.Source != CaptionsSource || cs.langs.Load() != "en" {
		t.Fatalf("lyrics from %s, captions in %q", l.Source, cs.langs.Load())
	}
	ch := ls.c.LyricsChoices(captionedVideo)
	en, de := choiceOf(t, ch, settings.LyricsCaptions, "en"), choiceOf(t, ch, settings.LyricsCaptions, "de-DE")
	if en.State != LyricsFound || !en.Showing || de.State != LyricsNotAsked || de.Showing || de.CaptionName != "German (Germany)" {
		t.Errorf("caption tracks: %+v, %+v", en, de)
	}
	if n := len(ch.Sources); n != 10 {
		t.Errorf("%d choices, want the 8 sources besides captions and 2 caption tracks", n)
	}
	asks := cs.asks.Load()
	ls.c.PickLyrics(captionedVideo, de.Pick, nil)
	if l := lyricsOf(t, ls, captionedVideo); l.Language != "de" || cs.langs.Load() != "de-DE" || cs.asks.Load() != asks+1 {
		t.Errorf("German: lyrics in %q, captions in %q", l.Language, cs.langs.Load())
	}
	ch = ls.c.LyricsChoices(captionedVideo)
	if de := choiceOf(t, ch, settings.LyricsCaptions, "de-DE"); !de.On || !de.Showing {
		t.Errorf("German, picked: %+v", de)
	}
	// The lookup's caption track, picked, shows the lookup's answer.
	ls.c.PickLyrics(captionedVideo, en.Pick, nil)
	if l := lyricsOf(t, ls, captionedVideo); l.Language != "en" || cs.asks.Load() != asks+1 {
		t.Errorf("English: lyrics in %q, captions asked %d more times", l.Language, cs.asks.Load()-asks)
	}
}
