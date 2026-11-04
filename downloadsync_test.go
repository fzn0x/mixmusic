package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
)

// roadTrip is a playlist downloaded, as seedRoadTrip keeps it: two tracks
// downloaded, and one that failed.
var (
	roadTripDone = []media.Track{
		testTrack("yt:aaaaaaaaaaa", "Neon Orchard", "Made-up Band", "Night Drives", media.SourceYouTube),
		testTrack("yt:bbbbbbbbbbb", "Glass Harbor", "Made-up Band", "Night Drives", media.SourceYouTube),
	}
	roadTripFailed = testTrack("yt:fffffffffff", "Static Bloom", "Made-up Band", "Night Drives", media.SourceYouTube)
)

// seedRoadTrip makes a folder of downloads holding the playlist Road Trip,
// pl-road, as an earlier run kept it: roadTripDone downloaded, and
// roadTripFailed, which failed.
func seedRoadTrip(t *testing.T) string {
	t.Helper()
	dir := seedDownloads(t, roadTripDone...)
	path := filepath.Join(dir, "index.json")
	var idx map[string]any
	if data, err := os.ReadFile(path); err != nil || json.Unmarshal(data, &idx) != nil {
		t.Fatalf("the index: %v", err)
	}
	const id = "playlist:pl-road"
	tracks := idx["tracks"].(map[string]any)
	for _, e := range tracks {
		e.(map[string]any)["pins"] = []string{id}
	}
	tracks[roadTripFailed.ID] = map[string]any{"track": roadTripFailed, "quality": "best", "pins": []string{id}, "seq": len(tracks) + 1,
		"failed": "YouTube Music has no audio of it"}
	idx["collections"] = map[string]downloads.Collection{id: {ID: id, Kind: "playlist", Title: "Road Trip", Source: media.SourceYouTube,
		Tracks: []string{roadTripDone[0].ID, roadTripDone[1].ID, roadTripFailed.ID}, Added: time.Now()}}
	if data, err := json.Marshal(idx); err != nil || os.WriteFile(path, data, 0o644) != nil {
		t.Fatal("the index wasn't written")
	}
	return dir
}

// newRoadTripShell is a test shell whose downloads hold Road Trip, whose
// page holds its tracks and more.
func newRoadTripShell(t *testing.T, more ...media.Track) (*shell, *loaded[media.Playlist]) {
	t.Helper()
	dir := seedRoadTrip(t)
	s := newTestShellOf(t, func(o *app.Options) { o.DownloadsDir = dir })
	tracks := append(slices.Clone(roadTripDone), roadTripFailed)
	l := &loaded[media.Playlist]{done: true, answers: 1, value: media.Playlist{ID: "pl-road", Title: "Road Trip", Source: media.SourceYouTube,
		Tracks: append(tracks, more...)}}
	s.loads["playlist:pl-road"] = l
	return s, l
}

// answerAgain has the page of l take a new answer, as a refresh brings,
// holding tracks.
func answerAgain(l *loaded[media.Playlist], tracks []media.Track) {
	l.value.Tracks = tracks
	l.answers++
}

// TestDownloadAddedOnLoad downloads the tracks added to a playlist
// downloaded as its page loads, and again once a refresh brings more,
// telling so; the failed track and the download removed stay as they are,
// and the head offers them.
func TestDownloadAddedOnLoad(t *testing.T) {
	added := testTrack("yt:ccccccccccc", "Paper Lanterns", "Made-up Band", "Night Drives", media.SourceYouTube)
	s, l := newRoadTripShell(t, added)
	tt := tester(t, s)
	show(s, tt, "/playlist/pl-road")
	tt.Frame()
	if k := s.kept.State(added.ID).Kind; k != downloads.Waiting {
		t.Fatalf("the track added is %v", k)
	}
	if c, _ := s.kept.Collection("playlist:pl-road"); !slices.Contains(c.Tracks, added.ID) {
		t.Errorf("the playlist's download holds %v", c.Tracks)
	}
	if !tt.HasText("Downloading 1 track added to Road Trip") || !tt.HasText("downloading 2 of 4") {
		t.Errorf("the page shows %q", tt.Texts())
	}

	// The frames after ask for nothing: the downloads don't change, and the
	// failed track isn't tried again.
	v := s.kept.Version()
	for range 5 {
		tt.Frame()
	}
	if s.kept.Version() != v || s.kept.State(roadTripFailed.ID).Kind != downloads.Failed {
		t.Errorf("the frames changed the downloads; the failed track is %v", s.kept.State(roadTripFailed.ID).Kind)
	}

	// The user removes the download of the track added: the head offers it
	// again, with the failed track, once nothing waits.
	s.svc.Downloads.Remove(added.ID)
	tt.Frame()
	if !tt.HasText("download 2 more") {
		t.Errorf("the page shows %q", tt.Texts())
	}

	// A refresh brings another track.
	another := testTrack("yt:ddddddddddd", "Low Tide Choir", "Made-up Band", "Night Drives", media.SourceYouTube)
	answerAgain(l, append(slices.Clone(l.value.Tracks), another))
	tt.Frame()
	tt.Frame()
	if k := s.kept.State(another.ID).Kind; k != downloads.Waiting {
		t.Errorf("after a refresh, the track added is %v", k)
	}
	if s.kept.State(added.ID).Kind != downloads.None || s.kept.State(roadTripFailed.ID).Kind != downloads.Failed {
		t.Errorf("after a refresh, the removed track is %v, the failed one %v", s.kept.State(added.ID).Kind, s.kept.State(roadTripFailed.ID).Kind)
	}
}

// TestDownloadAddedOffer downloads, from the head's download 2 more, what
// the downloads of the tracks added leave: the failed track, tried again,
// and the one whose download was removed.
func TestDownloadAddedOffer(t *testing.T) {
	added := testTrack("yt:ccccccccccc", "Paper Lanterns", "Made-up Band", "Night Drives", media.SourceYouTube)
	s, _ := newRoadTripShell(t, added)
	tt := tester(t, s)
	show(s, tt, "/playlist/pl-road")
	s.svc.Downloads.Remove(added.ID)
	tt.Frame()
	click(t, s, tt, "download 2 more")
	tt.Frame()
	if s.kept.State(added.ID).Kind != downloads.Waiting || s.kept.State(roadTripFailed.ID).Kind != downloads.Waiting {
		t.Errorf("download 2 more left the removed track %v, the failed one %v", s.kept.State(added.ID).Kind, s.kept.State(roadTripFailed.ID).Kind)
	}
}

// TestDownloadAddedOff leaves the tracks added to a playlist downloaded
// alone with the setting off: the head offers them.
func TestDownloadAddedOff(t *testing.T) {
	added := testTrack("yt:ccccccccccc", "Paper Lanterns", "Made-up Band", "Night Drives", media.SourceYouTube)
	s, _ := newRoadTripShell(t, added)
	s.set.DownloadAdded = false
	s.save()
	tt := tester(t, s)
	show(s, tt, "/playlist/pl-road")
	tt.Frame()
	if k := s.kept.State(added.ID).Kind; k != downloads.None || tt.HasText("Downloading 1 track added to Road Trip") {
		t.Errorf("with the setting off, the track added is %v", k)
	}
	if !tt.HasText("download 2 more") {
		t.Errorf("the page shows %q", tt.Texts())
	}
}

// TestDownloadAddedNotDownloaded leaves a playlist that isn't downloaded
// alone.
func TestDownloadAddedNotDownloaded(t *testing.T) {
	s := newTestShell(t)
	s.loads["playlist:pl-other"] = &loaded[media.Playlist]{done: true, answers: 1, value: media.Playlist{ID: "pl-other", Title: "Sunday Errands",
		Source: media.SourceYouTube, Tracks: roadTripDone}}
	tt := tester(t, s)
	show(s, tt, "/playlist/pl-other")
	tt.Frame()
	if s.kept.Any() {
		t.Errorf("the downloads hold %+v", s.kept.Status())
	}
	if _, ok := tt.Find("download"); !ok {
		t.Errorf("the page shows %q", tt.Texts())
	}
}

// TestDownloadAddedMany waits for the head's button before downloading as
// many tracks added as Download asks before, which a notice tells.
func TestDownloadAddedMany(t *testing.T) {
	var many []media.Track
	for i := range 150 {
		many = append(many, testTrack("yt:"+strings.Repeat(string(rune('g'+i%15)), 9)+count(10+i/15), "Song "+count(i), "Made-up Band", "Night Drives", media.SourceYouTube))
	}
	s, _ := newRoadTripShell(t, many...)
	tt := tester(t, s)
	show(s, tt, "/playlist/pl-road")
	tt.Frame()
	if st := s.kept.Status(); st.Waiting != 0 {
		t.Errorf("%d tracks wait to download", st.Waiting)
	}
	told := func() int {
		n := 0
		for _, text := range tt.Texts() {
			if strings.HasPrefix(text, "150 tracks were added to Road Trip since you downloaded it. They take about ") &&
				strings.HasSuffix(text, ", so they download only once you ask.") {
				n++
			}
		}
		return n
	}
	if n := told(); n != 1 {
		t.Errorf("told %d times: the window shows %q", n, tt.Texts())
	}
	// Once, not in each frame, which would keep the toast up.
	for range 3 {
		tt.Advance(10 * time.Second)
	}
	if n := told(); n != 0 {
		t.Errorf("still told %d times, a while after", n)
	}
	click(t, s, tt, "download 151 more")
	if s.asking == nil || s.asking.title != "download 153 tracks" {
		t.Errorf("download 151 more asks %+v", s.asking)
	}
}

// TestDownloadAddedLiked downloads a song liked once the liked songs are
// downloaded, as their page shows it.
func TestDownloadAddedLiked(t *testing.T) {
	s := newTestShell(t)
	ctx := context.Background()
	if err := s.svc.Collection.SetLiked(ctx, roadTripDone[0], true); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, "/library/liked")
	click(t, s, tt, "download")
	if err := s.svc.Collection.SetLiked(ctx, roadTripDone[1], true); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if k := s.kept.State(roadTripDone[1].ID).Kind; k != downloads.Waiting {
		t.Errorf("the song liked is %v", k)
	}
}
