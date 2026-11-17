package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// slowPlaylists answers the playlists of YouTube Music's library as a
// library read slowly does: pages[0] at once, and each page after it when
// the test sends on the returned func, handing over what was read so far
// as it goes. The last send ends the list.
func slowPlaylists(t *testing.T, pages ...[]media.Playlist) (next func()) {
	t.Helper()
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	was := listOnYouTube
	listOnYouTube = func(_ *shell, ctx context.Context) ([]media.Playlist, error) {
		take := media.Partial[[]media.Playlist](ctx)
		var read []media.Playlist
		for i, page := range pages {
			if i > 0 {
				<-gate
			}
			read = slices.Concat(read, page)
			if take != nil && i < len(pages)-1 {
				take(slices.Clone(read))
			}
		}
		return read, nil
	}
	t.Cleanup(func() { listOnYouTube = was })
	return func() { gate <- struct{}{} }
}

func playlistNamed(title string) media.Playlist {
	p := lateNight
	p.ID, p.Title = "yt:PL"+strings.ToLower(title), title
	return p
}

// TestYouTubeLibraryShowsPlaylistsAsTheyCome shows in Library › YouTube
// Music the playlists of its first page before the last page comes, adds
// the others as they come, and says Loading more… under them until the
// list ends, which then says nothing.
func TestYouTubeLibraryShowsPlaylistsAsTheyCome(t *testing.T) {
	next := slowPlaylists(t,
		[]media.Playlist{playlistNamed("Amber")},
		[]media.Playlist{playlistNamed("Birch")},
		[]media.Playlist{playlistNamed("Cedar")})
	s := youtubeLibraryShell(t, nil, &loaded[[]media.Track]{done: true, value: []media.Track{testTracks[2]}})
	delete(s.loads, "youtube:playlists")
	tt := testerOf(t, s, 1280, 1400)
	s.navigate("/library/youtube")
	until(t, s, tt, "the first page", func() bool { return tt.HasText("Amber") })
	if !tt.HasText("Loading more…") || tt.HasText("Birch") {
		t.Fatalf("after the first page the library shows %q", tt.Texts())
	}
	if l := s.youtubePlaylists(); l.done || !l.growing() || len(l.shown()) != 1 {
		t.Errorf("the load holds %d playlists, done %v", len(l.shown()), l.done)
	}

	next()
	until(t, s, tt, "the second page", func() bool { return tt.HasText("Birch") })
	if !tt.HasText("Amber") || !tt.HasText("Loading more…") || tt.HasText("Cedar") {
		t.Fatalf("after the second page the library shows %q", tt.Texts())
	}

	next()
	until(t, s, tt, "the last page", func() bool { return tt.HasText("Cedar") && !tt.HasText("Loading more…") })
	if !tt.HasText("Amber") || !tt.HasText("Birch") || !tt.HasText("new playlist") || tt.HasText("Showing the first") {
		t.Errorf("the whole list shows %q", tt.Texts())
	}
	if l := s.youtubePlaylists(); !l.done || l.growing() || len(l.value) != 3 {
		t.Errorf("the load holds %d playlists, done %v", len(l.value), l.done)
	}
}

// TestYouTubeLibraryFootChangesToInPart changes the foot of a list that
// was coming to #716's line, with Try again, once it ends in part, and
// Loading more… goes.
func TestYouTubeLibraryFootChangesToInPart(t *testing.T) {
	playlists := &loaded[[]media.Playlist]{part: []media.Playlist{lateNight}, coming: true}
	songs := &loaded[[]media.Track]{part: []media.Track{testTracks[2]}, coming: true}
	s := youtubeLibraryShell(t, playlists, songs)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/youtube")
	if !tt.HasText(lateNight.Title) || !tt.HasText(testTracks[2].Title) || tt.HasText("Showing the first") || tt.HasText("try again") {
		t.Fatalf("while they come, the library shows %q", tt.Texts())
	}
	if n := countTexts(tt, "Loading more…"); n != 2 {
		t.Errorf("%d feet say Loading more…, in %q", n, tt.Texts())
	}

	songs.value, songs.err, songs.done, songs.part, songs.coming = []media.Track{testTracks[2]}, firstOf(1), true, nil, false
	tt.Frame()
	if !tt.HasText("Showing the first 1 songs") || !tt.HasText("try again") || countTexts(tt, "Loading more…") != 1 {
		t.Errorf("a song list in part shows %q", tt.Texts())
	}
	playlists.value, playlists.done, playlists.part, playlists.coming = []media.Playlist{lateNight}, true, nil, false
	tt.Frame()
	if tt.HasText("Loading more…") || !tt.HasText(lateNight.Title) {
		t.Errorf("whole lists show %q", tt.Texts())
	}
}

// countTexts counts the texts of the window that are text.
func countTexts(tt interface{ Texts() []string }, text string) int {
	n := 0
	for _, x := range tt.Texts() {
		if x == text {
			n++
		}
	}
	return n
}

// TestYouTubeArtistsAndPodcastsShowAsTheyCome shows Library › Artists and
// Library › Podcasts the artists and shows read so far, and Loading more…
// under them.
func TestYouTubeArtistsAndPodcastsShowAsTheyCome(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	s.loads["youtube:artists"] = &loaded[[]media.Artist]{coming: true,
		part: []media.Artist{{ID: "yt:UCharbour", Source: media.SourceYouTube, Name: "Harbour"}}}
	s.loads["youtube:podcasts"] = &loaded[[]media.Podcast]{coming: true,
		part: []media.Podcast{{ID: "yt:MPSPshow", Source: media.SourceYouTube, Title: "The Daily Thing", Authors: []media.Ref{}, Episodes: []media.Track{}}}}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/library/artists")
	if !tt.HasText("Harbour") || !tt.HasText("Loading more…") || tt.HasText("Nothing") {
		t.Errorf("Library › Artists shows %q", tt.Texts())
	}
	show(s, tt, "/library/podcasts")
	if !tt.HasText("The Daily Thing") || !tt.HasText("Loading more…") {
		t.Errorf("Library › Podcasts shows %q", tt.Texts())
	}
}

// TestYouTubeAlbumsShowAsTheyCome shows an album read so far in Library ›
// YouTube Music, with Loading more… under it, while a list whose first
// page hasn't come shows Loading.
func TestYouTubeAlbumsShowAsTheyCome(t *testing.T) {
	playlists := &loaded[[]media.Playlist]{done: true, value: []media.Playlist{lateNight}}
	s := youtubeLibraryShell(t, playlists, &loaded[[]media.Track]{done: true, value: []media.Track{testTracks[2]}})
	s.loads["youtube:albums"] = &loaded[[]media.Album]{coming: true,
		part: []media.Album{{ID: "yt:MPREblue", Source: media.SourceYouTube, Title: "Blue Hour", Tracks: []media.Track{}}}}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/youtube")
	if !tt.HasText("Blue Hour") || !tt.HasText("Loading more…") {
		t.Errorf("the albums show %q", tt.Texts())
	}
	s.loads["youtube:albums"] = &loaded[[]media.Album]{}
	tt.Frame()
	if tt.HasText("Blue Hour") || !tt.HasText("Loading") || tt.HasText("Loading more…") {
		t.Errorf("before the first page the library shows %q", tt.Texts())
	}
}

// TestAddToYouTubePlaylistShowsThoseReadSoFar lists in a track's Add to
// YouTube Music playlist the playlists read so far, with Loading more… as
// its last item, which can't be chosen; once the list ends, the item goes.
func TestAddToYouTubePlaylistShowsThoseReadSoFar(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	l := &loaded[[]media.Playlist]{part: []media.Playlist{lateNight}, coming: true}
	s.loads["youtube:playlists"] = l
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	if err := trackMenuChoice(t, tt, "Add to YouTube Music playlist", "Loading more…"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Loading more… isn't an item to refuse: %v", err)
	}
	if err := tt.RightClick(testTracks[0].Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Add to YouTube Music playlist", "Late night"); err != nil {
		t.Errorf("a playlist read so far isn't offered: %v", err)
	}

	l.value, l.done, l.part, l.coming = []media.Playlist{lateNight}, true, nil, false
	if err := trackMenuChoice(t, tt, "Add to YouTube Music playlist", "Loading more…"); err == nil || !strings.Contains(err.Error(), "no item") {
		t.Errorf("a list that came whole still says Loading more…: %v", err)
	}
}
