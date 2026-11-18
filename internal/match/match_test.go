package match

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

func track(title, artist string, ms int, video bool) media.Track {
	return media.Track{ID: "yt:" + title, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: artist}}, DurationMs: ms, Video: video}
}

var oneMoreTime = media.Track{
	ID: "sp:0DiWol3AO6WpXZgp0goxAV", Source: media.SourceSpotify,
	Title: "One More Time - Remastered", Artists: []media.Ref{{Name: "Daft Punk"}}, DurationMs: 320357,
}

func TestScorePrefersTheRecording(t *testing.T) {
	right := Score(oneMoreTime, track("One More Time", "Daft Punk", 320000, false))
	video := Score(oneMoreTime, track("Daft Punk - One More Time (Official Video)", "Daft Punk", 324000, true))
	cover := Score(oneMoreTime, track("One More Time", "Some Cover Band", 250000, false))
	other := Score(oneMoreTime, track("Harder, Better, Faster, Stronger", "Daft Punk", 224000, false))
	if !(right > video && video > cover && right > other) {
		t.Errorf("scores: right %.2f, video %.2f, cover %.2f, other %.2f", right, video, cover, other)
	}
	if right < threshold || video < threshold {
		t.Errorf("good matches under the threshold: %.2f, %.2f", right, video)
	}
	if cover >= threshold || other >= threshold {
		t.Errorf("bad matches over the threshold: %.2f, %.2f", cover, other)
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Beyoncé":                       "beyonce",
		"  AC/DC — Back in Black! ":     "ac dc back in black",
		"Song (Remastered 2011)":        "song remastered 2011",
		"Mötley Crüe":                   "motley crue",
		"Hello, World (feat. Somebody)": "hello world feat somebody",
	} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
	if got := CleanTitle("Song (Remastered 2011)"); got != "Song" {
		t.Errorf("cleanTitle = %q", got)
	}
	if got := CleanTitle("Song - 2011 Remaster"); got != "Song" {
		t.Errorf("cleanTitle = %q", got)
	}
}

type fakeSearch struct {
	songs, videos []media.Item
	calls         atomic.Int32
}

func (f *fakeSearch) Search(_ context.Context, _ string, filter youtube.Filter) ([]media.Item, error) {
	f.calls.Add(1)
	if filter == youtube.FilterSongs {
		return f.songs, nil
	}
	return f.videos, nil
}

func items(tracks ...media.Track) []media.Item {
	out := make([]media.Item, len(tracks))
	for i := range tracks {
		out[i] = media.Item{Kind: media.ItemTrack, Track: &tracks[i]}
	}
	return out
}

func TestMatchRemembers(t *testing.T) {
	known, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	yt := &fakeSearch{songs: items(track("One More Time", "Daft Punk", 320000, false))}
	m := New(yt, known)
	got, err := m.Match(context.Background(), oneMoreTime)
	if err != nil || got.Title != "One More Time" {
		t.Fatalf("match = %+v, %v", got, err)
	}
	if _, err := m.Match(context.Background(), oneMoreTime); err != nil {
		t.Fatal(err)
	}
	if yt.calls.Load() != 1 {
		t.Errorf("searched %d times for one track", yt.calls.Load())
	}
	if got, ok := m.Matched(oneMoreTime); !ok || got.Title != "One More Time" {
		t.Errorf("matched = %+v, %v", got, ok)
	}
	if err := m.Forget(oneMoreTime.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Matched(oneMoreTime); ok {
		t.Error("a forgotten match is still matched")
	}
	m.Match(context.Background(), oneMoreTime)
	if yt.calls.Load() != 2 {
		t.Error("a forgotten match was not searched again")
	}
}

// TestMatched tells the match of a track without searching for one.
func TestMatched(t *testing.T) {
	yt := &fakeSearch{songs: items(track("One More Time", "Daft Punk", 320000, false))}
	m := New(yt, nil)
	if _, ok := m.Matched(oneMoreTime); ok || yt.calls.Load() != 0 {
		t.Errorf("a track never matched is matched, after %d searches", yt.calls.Load())
	}
	own := track("Song", "Artist", 1000, false)
	if got, ok := m.Matched(own); !ok || got.ID != own.ID {
		t.Errorf("a YouTube Music track matched = %+v, %v", got, ok)
	}
}

// TestMatchedFoundBefore tells the match found before without a search.
func TestMatchedFoundBefore(t *testing.T) {
	known, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	yt := &fakeSearch{songs: items(track("One More Time", "Daft Punk", 320000, false))}
	m := New(yt, known)
	if _, ok := m.Matched(oneMoreTime); ok {
		t.Error("matched before a search")
	}
	if _, err := m.Match(context.Background(), oneMoreTime); err != nil {
		t.Fatal(err)
	}
	if got, ok := m.Matched(oneMoreTime); !ok || got.Title != "One More Time" {
		t.Errorf("Matched = %+v, %v", got, ok)
	}
	if yt.calls.Load() != 1 {
		t.Errorf("%d searches", yt.calls.Load())
	}
}

func TestMatchFallsBackToVideos(t *testing.T) {
	yt := &fakeSearch{
		songs:  items(track("Something Else", "Nobody", 100000, false)),
		videos: items(track("Daft Punk - One More Time (Official Video)", "Daft Punk", 324000, true)),
	}
	got, err := New(yt, nil).Match(context.Background(), oneMoreTime)
	if err != nil || !got.Video {
		t.Fatalf("match = %+v, %v", got, err)
	}
}

func TestMatchFindsNothing(t *testing.T) {
	yt := &fakeSearch{songs: items(track("Something Else", "Nobody", 100000, false))}
	if _, err := New(yt, nil).Match(context.Background(), oneMoreTime); !errors.Is(err, ErrNoMatch) {
		t.Errorf("err = %v", err)
	}
}

// marked is track, marked explicit when explicit is set.
func marked(tr media.Track, explicit bool) media.Track {
	tr.Explicit = explicit
	tr.ID += map[bool]string{true: " (explicit)", false: " (clean)"}[explicit]
	return tr
}

// TestBestPrefersTheSameMark takes, of two recordings alike, the one marked
// explicit, or clean, as the track is, whichever YouTube Music lists first
// (#583).
func TestBestPrefersTheSameMark(t *testing.T) {
	song := track("One More Time", "Daft Punk", 320000, false)
	explicit, clean := marked(song, true), marked(song, false)
	for _, tc := range []struct {
		explicit bool
		listed   []media.Track
		want     string
	}{
		{false, []media.Track{explicit, clean}, clean.ID},
		{false, []media.Track{clean, explicit}, clean.ID},
		{true, []media.Track{clean, explicit}, explicit.ID},
		{true, []media.Track{explicit, clean}, explicit.ID},
	} {
		sp := oneMoreTime
		sp.Explicit = tc.explicit
		if got, ok := Best(sp, items(tc.listed...)); !ok || got.ID != tc.want {
			t.Errorf("explicit %v, of %s then %s: %s, %v", tc.explicit, tc.listed[0].ID, tc.listed[1].ID, got.ID, ok)
		}
	}
	// The other mark alone still matches.
	if got, ok := Best(oneMoreTime, items(explicit)); !ok || got.ID != explicit.ID {
		t.Errorf("the explicit recording alone: %s, %v", got.ID, ok)
	}
	if s := Score(oneMoreTime, explicit); s >= Score(oneMoreTime, clean) || s < threshold {
		t.Errorf("the explicit recording of a clean track scores %.2f", s)
	}
}

// TestMatchCleanVideo matches a clean track whose song YouTube Music lists
// explicit alone to a clean video of it, and to the explicit song when it
// has none; an explicit track's search stops at its songs.
func TestMatchCleanVideo(t *testing.T) {
	song := marked(track("One More Time", "Daft Punk", 320000, false), true)
	video := marked(track("Daft Punk - One More Time (Official Video)", "Daft Punk", 324000, true), false)
	yt := &fakeSearch{songs: items(song), videos: items(video)}
	got, err := New(yt, nil).Match(context.Background(), oneMoreTime)
	if err != nil || got.ID != video.ID {
		t.Errorf("with a clean video: %s, %v", got.ID, err)
	}

	yt = &fakeSearch{songs: items(song), videos: items(marked(video, true))}
	got, err = New(yt, nil).Match(context.Background(), oneMoreTime)
	if err != nil || got.ID != song.ID || yt.calls.Load() != 2 {
		t.Errorf("without one: %s, %v, after %d searches", got.ID, err, yt.calls.Load())
	}

	sp := oneMoreTime
	sp.Explicit = true
	yt = &fakeSearch{songs: items(marked(song, false)), videos: items(marked(video, true))}
	got, err = New(yt, nil).Match(context.Background(), sp)
	if err != nil || got.Video || yt.calls.Load() != 1 {
		t.Errorf("an explicit track: %s, %v, after %d searches", got.ID, err, yt.calls.Load())
	}
}
