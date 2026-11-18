package app

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
)

// explicitBadge is YouTube Music's explicit badge of a row, as its answers
// give it.
const explicitBadge = `"badges": [{"musicInlineBadgeRenderer": {"icon": {"iconType": "MUSIC_EXPLICIT_BADGE"}}}], `

// TestRadioWithoutExplicit has the radio leave out the songs YouTube Music
// badges explicit while the settings hide explicit content, and keep them
// while they don't. The radio is next.json, recorded, whose third row the
// test badges, as no recorded radio holds an explicit song.
func TestRadioWithoutExplicit(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("..", "youtube", "testdata", "next.json"))
	if err != nil {
		t.Fatal(err)
	}
	const row = `"playlistPanelVideoRenderer": {`
	parts := bytes.SplitN(recorded, []byte(row), 4)
	if len(parts) < 4 {
		t.Fatalf("next.json has %d rows", len(parts)-1)
	}
	answer := bytes.Join([][]byte{parts[0], parts[1], parts[2], append([]byte(explicitBadge), parts[3]...)}, []byte(row))
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) { w.Write(answer) },
		func(w http.ResponseWriter, r *http.Request) {})
	c := &Catalog{a}
	ctx := context.Background()
	seed := media.Track{ID: "yt:seed", Source: media.SourceYouTube, Title: "Seed"}
	all, err := c.Radio(ctx, seed)
	if err != nil {
		t.Fatal(err)
	}
	explicit := slices.IndexFunc(all, func(tr media.Track) bool { return tr.Explicit })
	if explicit < 0 || len(all) < 3 {
		t.Fatalf("the radio %v holds no explicit song", trackIDs(all))
	}
	st := a.settings.Get()
	st.HideExplicit = true
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	got, err := c.Radio(ctx, seed)
	want := slices.Delete(slices.Clone(trackIDs(all)), explicit, explicit+1)
	if err != nil || !slices.Equal(trackIDs(got), want) {
		t.Errorf("hiding explicit content, the radio is %v, not %v, %v", trackIDs(got), want, err)
	}
}

// TestKnownRecording tells the player the recording on YouTube Music a
// Spotify track plays from, as found before, without a search: its
// download's, else its match, which keeps its explicit mark (#583).
func TestKnownRecording(t *testing.T) {
	known, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	known.Set(map[string]media.Track{
		"sp:matched": {ID: "yt:explicit001", Source: media.SourceYouTube, Explicit: true},
		"sp:kept":    {ID: "yt:explicit002", Source: media.SourceYouTube, Explicit: true},
	})
	var searched atomic.Int32
	a := &App{matcher: match.New(searcher(func() { searched.Add(1) }), known)}
	s := playerServices{a}
	if rec, ok := s.KnownRecording(media.Track{ID: "sp:matched", Source: media.SourceSpotify}); !ok || !rec.ExplicitSong() {
		t.Errorf("the match: %+v, %v", rec, ok)
	}
	if rec, ok := s.KnownRecording(media.Track{ID: "sp:other", Source: media.SourceSpotify}); ok || searched.Load() != 0 {
		t.Errorf("a track not matched: %+v, after %d searches", rec, searched.Load())
	}
	clean := ytTrack("ccccccccccc")
	withDownloads(t, a, matches{"sp:kept": clean}, media.Track{ID: "sp:kept", Source: media.SourceSpotify, Title: "Kept"})
	if rec, ok := s.KnownRecording(media.Track{ID: "sp:kept", Source: media.SourceSpotify}); !ok || rec.ID != clean.ID {
		t.Errorf("a track downloaded: %+v, %v", rec, ok)
	}
}

// TestHidesExplicit has the downloads leave out explicit songs while the
// settings hide explicit content.
func TestHidesExplicit(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "settings.json"), settings.Default())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st}
	explicit := ytTrack("eeeeeeeeeee")
	explicit.Explicit = true
	episode := explicit
	episode.Episode = &media.Episode{}
	if a.hidesExplicit(explicit) {
		t.Error("hidden while the settings show explicit content")
	}
	s := st.Get()
	s.HideExplicit = true
	st.Set(s)
	if !a.hidesExplicit(explicit) || a.hidesExplicit(ytTrack("ccccccccccc")) || a.hidesExplicit(episode) {
		t.Error("hiding explicit content, the downloads hide another track")
	}
}
