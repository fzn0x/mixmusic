package main

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// detailsEpisode is an episode of 34 minutes, whose artists are its
// channel, apart from its show.
var detailsEpisode = media.Track{ID: "yt:ep9", Source: media.SourceYouTube, Title: "Why we dream",
	Artists: []media.Ref{{Name: "The Lab"}}, DurationMs: 2040000, Video: true,
	Episode: &media.Episode{Show: media.Ref{ID: "yt:MPSPPLshow", Name: "Night Science"}, Released: "2025-10-26"}}

// testDetails are the episode's details, made up: a link, three chapters,
// the last past the episode's end, and a link that isn't to a web page.
func testDetails() media.EpisodeDetails {
	d := media.EpisodeDetails{Show: media.Ref{ID: "yt:MPSPPLshow", Name: "Night Science"}, Released: "2025-10-26", Plays: 12000,
		DurationMs: 2040000, Description: media.ParseDescription("Why we dream, and what the owls know.\n" +
			"Notes at https://example.com/dreams\n\n0:00 Dusk\n12:34 The barn\n2:05:00 Long after the end")}
	d.Description = append(d.Description, []media.TextRun{{Text: "Run this", Link: "javascript:alert(1)"}})
	d.Chapters = media.ChaptersOf(d.Description)
	return d
}

// detailsShell is a test shell playing detailsEpisode, then a song with
// lyrics, whose details are d.
func detailsShell(t *testing.T, d *loaded[media.EpisodeDetails]) *shell {
	t.Helper()
	s := newTestShell(t)
	s.loads["episode-details:"+detailsEpisode.ID] = d
	s.loads["related-"+detailsEpisode.ID] = &loaded[[]media.Shelf]{done: true}
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Paper boats on the river"}}}}
	s.p.PlayTracks([]media.Track{detailsEpisode, testTracks[0]}, 0)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == detailsEpisode.ID && st.Playing
	})
	return s
}

// countText is how many of texts are text.
func countText(texts []string, text string) int {
	n := 0
	for _, s := range texts {
		if s == text {
			n++
		}
	}
	return n
}

// TestEpisodeDetailsTab shows an episode's details where a song's lyrics
// show, on the now playing page and in the side panel, the button that
// shows them saying Details; a song after it shows its lyrics again.
func TestEpisodeDetailsTab(t *testing.T) {
	s := detailsShell(t, &loaded[media.EpisodeDetails]{done: true, value: testDetails()})
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/now-playing")
	s.open("/lyrics")
	tt.Frame()
	tt.Advance(time.Second)
	texts := tt.Texts()
	if n := countText(texts, "Why we dream, and what the owls know."); n != 2 {
		t.Errorf("the description shows %d times, not on the page and in the panel: %q", n, texts)
	}
	for _, want := range []string{"details", "Night Science", "12K views", "Oct 26, 2025", "34 min", "12:34", "The barn"} {
		if !showsText(texts, want) {
			t.Errorf("the details don't show %q: %q", want, texts)
		}
	}
	if _, ok := tt.Find("Details"); !ok {
		t.Error("the player bar's button doesn't say Details")
	}
	if _, asked := s.loads["lyrics:"+detailsEpisode.ID]; asked || showsText(texts, "lyrics") {
		t.Errorf("an episode looked for lyrics: %q", texts)
	}

	s.p.Next()
	s.settle(t, func(st player.State) bool { cur, ok := st.Current(); return ok && cur.Track.ID == testTracks[0].ID })
	tt.Frame()
	tt.Advance(time.Second)
	texts = tt.Texts()
	if countText(texts, "Paper boats on the river") != 2 || !slices.Contains(texts, "lyrics") || showsText(texts, "owls know") {
		t.Errorf("the song after shows %q", texts)
	}
	if _, ok := tt.Find("Lyrics"); !ok {
		t.Error("the player bar's button doesn't say Lyrics for a song")
	}
}

// TestEpisodeDetailsSeek plays the episode from a time of its description,
// lights the line of the chapter that plays, and leaves a time past the
// episode's end as text.
func TestEpisodeDetailsSeek(t *testing.T) {
	s := detailsShell(t, &loaded[media.EpisodeDetails]{done: true, value: testDetails()})
	tt := testerOf(t, s, 1280, 1000)
	s.open("/lyrics")
	tt.Frame()
	// The accent's ink of the lines of the first two chapters: a line lit
	// is all of it, one that isn't only its time. Each line is measured
	// against itself, as glyphs draw apart on each system.
	accent := func() (dusk, barn int) {
		img := tt.Image()
		for _, l := range []struct {
			text string
			n    *int
		}{{"0:00 Dusk", &dusk}, {"12:34 The barn", &barn}} {
			r, ok := tt.Find(l.text)
			if !ok {
				t.Fatalf("%q doesn't show: %q", l.text, tt.Texts())
			}
			*l.n = inkIn(img, r, s.lk.accent)
		}
		return dusk, barn
	}
	dusk, barn := accent()
	click(t, s, tt, "Play from 12:34")
	s.settle(t, func(st player.State) bool { return st.Position == 754*time.Second })
	tt.Frame()
	if duskThen, barnThen := accent(); 2*dusk < 3*duskThen || 2*barnThen < 3*barn {
		t.Errorf("the line lit didn't move from the first chapter to the second: the accent's ink went from %d to %d, and from %d to %d",
			dusk, duskThen, barn, barnThen)
	}
	if _, ok := tt.Find("Play from 2:05:00"); ok || !tt.HasText("2:05:00") {
		t.Errorf("a time past the end seeks, or doesn't show: %q", tt.Texts())
	}
	if _, ok := tt.Find("Play from 0:00"); !ok {
		t.Errorf("the start doesn't seek: %q", tt.Texts())
	}
	if at := chapterAt(testDetails().Chapters, 754*time.Second, s.seekLimit(detailsEpisode, testDetails())); at != 1 {
		t.Errorf("the chapter playing at 12:34 is %d", at)
	}
}

// TestEpisodeDetailsLinks opens a link of the description in the browser,
// never one that isn't to a web page, and the show's page from its name.
func TestEpisodeDetailsLinks(t *testing.T) {
	var opened []string
	was := openWebPage
	openWebPage = func(_ *shell, u string) { opened = append(opened, u) }
	t.Cleanup(func() { openWebPage = was })
	s := detailsShell(t, &loaded[media.EpisodeDetails]{done: true, value: testDetails()})
	tt := testerOf(t, s, 1280, 1000)
	s.open("/lyrics")
	tt.Frame()
	click(t, s, tt, "https://example.com/dreams")
	click(t, s, tt, "Run this")
	if !slices.Equal(opened, []string{"https://example.com/dreams"}) {
		t.Errorf("opened %q", opened)
	}
	click(t, s, tt, "Night Science")
	if got := s.router.Path(); got != podcastPath("yt:MPSPPLshow") {
		t.Errorf("the show's name went to %q", got)
	}
}

// TestEpisodeDetailsLoading says the details load, and why they failed, as
// the lyrics do, with what the episode's row told meanwhile; a failure
// that may pass can be tried again, an episode without details can't.
func TestEpisodeDetailsLoading(t *testing.T) {
	key := "episode-details:" + detailsEpisode.ID
	s := detailsShell(t, &loaded[media.EpisodeDetails]{})
	tt := testerOf(t, s, 1280, 1000)
	s.open("/lyrics")
	tt.Frame()
	if !tt.HasText("Looking for the details…") || !showsText(tt.Texts(), "Night Science") || !showsText(tt.Texts(), "34 min") {
		t.Errorf("while the details load, the panel shows %q", tt.Texts())
	}
	s.loads[key] = &loaded[media.EpisodeDetails]{done: true, err: fmt.Errorf("No details found for this episode.: %w", app.ErrNoEpisodeDetails)}
	tt.Frame()
	if !tt.HasText("No details found for this episode.") || tt.HasText("try again") {
		t.Errorf("an episode without details shows %q", tt.Texts())
	}
	failed := &loaded[media.EpisodeDetails]{done: true, err: fmt.Errorf("The episode's details can't be loaded right now.")}
	s.loads[key] = failed
	tt.Frame()
	if !tt.HasText("can't be loaded right now") {
		t.Fatalf("the failure shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if s.loads[key] == failed {
		t.Error("try again didn't ask for the details again")
	}
}

// TestNotchChapters shows in the notch the chapter of the episode that
// plays, with those around it, and none past the episode's end.
func TestNotchChapters(t *testing.T) {
	s := detailsShell(t, &loaded[media.EpisodeDetails]{done: true, value: testDetails()})
	x := newNotchTest(t, s, testHousing)
	x.openPanel()
	if !x.tt.HasText("Dusk") || !x.tt.HasText("The barn") || x.tt.HasText("Paper boats") {
		t.Errorf("at the start, the notch shows %q", x.tt.Texts())
	}
	s.p.Seek(800 * time.Second)
	s.settle(t, func(st player.State) bool { return st.Position >= 800*time.Second })
	x.frame()
	if !x.tt.HasText("Dusk") || !x.tt.HasText("The barn") || x.tt.HasText("Long after the end") {
		t.Errorf("in the last chapter, the notch shows %q", x.tt.Texts())
	}
}
