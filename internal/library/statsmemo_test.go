package library

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestStatsKeptUntilTheLogChanges keeps the stats, Your year and the play
// counts while the log stays as it was, and counts them again once a play
// counts.
func TestStatsKeptUntilTheLogChanges(t *testing.T) {
	l := open(t)
	l.now = func() time.Time { return time.Date(2026, 12, 3, 12, 0, 0, 0, time.UTC) }
	a := tr("yt:a", media.SourceYouTube, "A")
	l.RecordPlay(a)
	if s, r, n := l.Stats(0), l.Recap(2026), l.PlayCounts(); s.Plays != 1 || r.Plays != 1 || n[a.ID] != 1 {
		t.Fatalf("stats %d, recap %d, counts %v", s.Plays, r.Plays, n)
	}
	// A play the log takes in unseen, which the stats then don't count.
	l.plays.mu.Lock()
	l.plays.plays = append(l.plays.plays, l.plays.plays[0])
	l.plays.mu.Unlock()
	if s, r, n := l.Stats(0), l.Recap(2026), l.PlayCounts(); s.Plays != 1 || r.Plays != 1 || n[a.ID] != 1 {
		t.Errorf("counted again with no change: stats %d, recap %d, counts %v", s.Plays, r.Plays, n)
	}
	l.RecordPlay(a)
	if s, r, n := l.Stats(0), l.Recap(2026), l.PlayCounts(); s.Plays != 3 || r.Plays != 3 || n[a.ID] != 3 {
		t.Errorf("after a play: stats %d, recap %d, counts %v", s.Plays, r.Plays, n)
	}
}

// TestStatsKeptAreCopies gives each caller results of its own to change.
func TestStatsKeptAreCopies(t *testing.T) {
	l := open(t)
	a := tr("yt:a", media.SourceYouTube, "A")
	l.RecordPlay(a)
	s := l.Stats(0)
	s.TopTracks[0].Plays, s.Daily[len(s.Daily)-1].Plays = 99, 99
	l.PlayCounts()[a.ID] = 99
	l.Recap(time.Now().Year()).TopTracks[0].Plays = 99
	l.RecapYears()[0] = 1999
	if s := l.Stats(0); s.TopTracks[0].Plays != 1 || s.Daily[len(s.Daily)-1].Plays != 1 {
		t.Errorf("the stats kept changed: %+v", s)
	}
	if n := l.PlayCounts(); n[a.ID] != 1 {
		t.Errorf("the counts kept changed: %v", n)
	}
	if r := l.Recap(time.Now().Year()); r.TopTracks[0].Plays != 1 {
		t.Errorf("the recap kept changed: %+v", r)
	}
	if y := l.RecapYears(); y[0] != time.Now().Year() {
		t.Errorf("the years kept changed: %v", y)
	}
}

// TestStatsKeptMoveOn counts the last days again as time passes, with no
// play since.
func TestStatsKeptMoveOn(t *testing.T) {
	l := open(t)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
	if s := l.Stats(7); s.Plays != 1 {
		t.Fatalf("this week: %+v", s)
	}
	now = now.AddDate(0, 0, 8)
	if s := l.Stats(7); s.Plays != 0 || s.Daily[6].Date != "2026-10-26" {
		t.Errorf("a week later, the week counts %d plays, ending %s", s.Plays, s.Daily[6].Date)
	}
}

// TestStatsKeptFollowTheLibrary gives a top track its cover once the
// library holds it, with no play since.
func TestStatsKeptFollowTheLibrary(t *testing.T) {
	l := open(t)
	a := tr("yt:a", media.SourceYouTube, "A")
	withLog(t, l, []Play{{Track: a, PlayedAt: time.Now().Add(-time.Hour)}})
	if s := l.Stats(0); len(s.TopTracks) != 1 || len(s.TopTracks[0].Track.Artwork) != 0 {
		t.Fatalf("top tracks of the log alone: %+v", s.TopTracks)
	}
	if err := l.SetLiked(a, true); err != nil {
		t.Fatal(err)
	}
	if s := l.Stats(0); len(s.TopTracks[0].Track.Artwork) == 0 {
		t.Errorf("a liked top track has no cover: %+v", s.TopTracks[0])
	}
}

// TestStatsWhilePlaysCount reads the stats while plays count meanwhile,
// as the windows and the player do: go test -race checks it.
func TestStatsWhilePlaysCount(t *testing.T) {
	l := open(t)
	songs := []media.Track{tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B")}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 40 {
			if err := l.RecordPlay(songs[i%2]); err != nil {
				t.Error(err)
			}
		}
	}()
	for reading := true; reading; {
		select {
		case <-done:
			reading = false
		default:
		}
		l.Stats(0)
		l.Recap(time.Now().Year())
		l.RecapYears()
		l.PlayCounts()
	}
	if s := l.Stats(0); s.Plays != 40 {
		t.Errorf("the stats count %d plays of 40", s.Plays)
	}
}

// TestStatsDontWaitForTheFile counts the stats while the log's file is
// being written, as a play's append and its sync take a while, so that a
// page asking in each frame doesn't stall.
func TestStatsDontWaitForTheFile(t *testing.T) {
	l := open(t)
	l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
	l.plays.io.Lock() // a write going on
	defer l.plays.io.Unlock()
	done := make(chan Stats)
	go func() { done <- l.Stats(0) }()
	select {
	case s := <-done:
		if s.Plays != 1 {
			t.Errorf("the stats count %d plays", s.Plays)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stats waited for the file")
	}
}
