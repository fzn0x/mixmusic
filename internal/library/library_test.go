package library

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

func open(t *testing.T) *Library {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func tr(id string, source media.Source, artist string) media.Track {
	return media.Track{ID: id, Source: source, Title: "Title " + id, Artists: []media.Ref{{Name: artist}}, DurationMs: 180000,
		Artwork: []media.Image{{URL: "https://img/" + id}}}
}

func TestLikes(t *testing.T) {
	l := open(t)
	a, b := tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B")
	l.SetLiked(a, true)
	l.SetLiked(b, true)
	l.SetLiked(a, true) // liking again moves it first
	if liked := l.Liked(); len(liked) != 2 || liked[0].ID != "yt:a" {
		t.Errorf("liked = %+v", liked)
	}
	l.SetLiked(a, false)
	if l.IsLiked("yt:a") || !l.IsLiked("sp:b") {
		t.Error("unlike failed")
	}
}

func TestMixedPlaylists(t *testing.T) {
	l := open(t)
	p, err := l.CreatePlaylist("  Road trip ", "", []media.Track{tr("yt:a", media.SourceYouTube, "A")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Road trip" || p.Source != media.SourceLocal || len(p.Artwork) == 0 {
		t.Errorf("created = %+v", p)
	}
	_, id, _ := media.ParseID(p.ID)
	l.AddToPlaylist(id, []media.Track{tr("sp:b", media.SourceSpotify, "B"), tr("yt:a", media.SourceYouTube, "A"), tr("yt:c", media.SourceYouTube, "C")})
	got, _ := l.Playlist(id)
	if len(got.Tracks) != 3 || got.Tracks[1].Source != media.SourceSpotify {
		t.Fatalf("tracks = %+v", got.Tracks)
	}
	l.MoveInPlaylist(id, 2, 0)
	l.RemoveFromPlaylist(id, 2)
	got, _ = l.Playlist(id)
	if len(got.Tracks) != 2 || got.Tracks[0].ID != "yt:c" || got.Tracks[1].ID != "yt:a" {
		t.Errorf("after move and remove = %+v", got.Tracks)
	}
	if list := l.Playlists(); len(list) != 1 || list[0].TrackCount != 2 || len(list[0].Tracks) != 0 {
		t.Errorf("list = %+v", list)
	}
	if err := l.DeletePlaylist(id); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Playlist(id); !errors.Is(err, ErrNoPlaylist) {
		t.Errorf("deleted playlist: %v", err)
	}
}

func TestHistory(t *testing.T) {
	l := open(t)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	a, b := tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B")
	l.RecordPlay(a)
	now = now.Add(time.Hour)
	l.RecordPlay(b)
	l.RecordPlay(a)
	if recent := l.RecentTracks(10); len(recent) != 2 || recent[0].ID != "yt:a" {
		t.Errorf("recent = %+v", recent)
	}
	l.ClearHistory()
	if len(l.History(0)) != 0 {
		t.Error("history not cleared")
	}
}

// TestVersion counts what changes the library, a play as a new mix.
func TestVersion(t *testing.T) {
	l := open(t)
	before := l.Version()
	l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
	if _, err := l.CreatePlaylist("Night drive", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := l.Version(); got != before+2 {
		t.Errorf("a play and a new mix moved the version from %d to %d", before, got)
	}
}

func TestSearches(t *testing.T) {
	l := open(t)
	for _, q := range []string{"one", "two", "ONE", "  "} {
		l.AddSearch(q)
	}
	if got := l.Searches(); len(got) != 2 || got[0] != "ONE" {
		t.Errorf("searches = %q", got)
	}
}
