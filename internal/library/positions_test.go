package library

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// episode is a podcast's episode an hour long.
func episode(id string) media.Track {
	return media.Track{ID: id, Source: media.SourceYouTube, Title: id, DurationMs: 3600000,
		Episode: &media.Episode{Show: media.Ref{ID: "yt:MPSPx", Name: "Show"}}}
}

func openPositions(t *testing.T) (*Positions, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "positions.json")
	p, err := OpenPositions(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Flush() })
	return p, path
}

// TestPositionRules keeps where an episode was left from 15 seconds in,
// resumes it 5 seconds before, and counts it played near its end.
func TestPositionRules(t *testing.T) {
	p, _ := openPositions(t)
	ep := episode("yt:e1")
	p.Keep(ep, 10*time.Second, time.Hour)
	if _, ok := p.Get(ep.ID); ok || p.ResumeAt(ep) != 0 {
		t.Error("10 s in was kept")
	}
	p.Keep(ep, 64*time.Second, time.Hour)
	if got := p.ResumeAt(ep); got != 59*time.Second {
		t.Errorf("ResumeAt = %v, want 59s", got)
	}
	if pos, _ := p.Get(ep.ID); !pos.Started() || pos.Left() != time.Hour-64*time.Second {
		t.Errorf("position = %+v", pos)
	}
	// Seeking back to its start forgets the place.
	p.Keep(ep, time.Second, time.Hour)
	if _, ok := p.Get(ep.ID); ok {
		t.Error("its start kept the place")
	}
	// Two minutes from the end, it's played, and plays from its start.
	p.Keep(ep, time.Hour-90*time.Second, time.Hour)
	if pos, _ := p.Get(ep.ID); !pos.Played || pos.Started() || p.ResumeAt(ep) != 0 {
		t.Errorf("near the end: %+v, resumes at %v", pos, p.ResumeAt(ep))
	}
	// Played again from its start, it stays played until it's 15 s in.
	p.Keep(ep, 0, time.Hour)
	if pos, ok := p.Get(ep.ID); !ok || !pos.Played {
		t.Error("its start forgot it was played")
	}
	p.Keep(ep, 20*time.Second, time.Hour)
	if pos, _ := p.Get(ep.ID); pos.Played || pos.PositionMs != 20000 {
		t.Errorf("heard again: %+v", pos)
	}
	// A short episode is played within 3% of its end.
	short := episode("yt:e2")
	p.Keep(short, 9*time.Minute+40*time.Second, 10*time.Minute)
	if pos, _ := p.Get(short.ID); pos.Played {
		t.Error("20 s before the end of 10 min is played")
	}
	p.Keep(short, 9*time.Minute+50*time.Second, 10*time.Minute)
	if pos, _ := p.Get(short.ID); !pos.Played {
		t.Error("10 s before the end of 10 min isn't played")
	}
	// A song keeps nothing.
	song := media.Track{ID: "yt:s", DurationMs: 200000}
	p.Keep(song, time.Minute, 0)
	if _, ok := p.Get(song.ID); ok || p.ResumeAt(song) != 0 {
		t.Error("a song's place was kept")
	}
}

// TestResumeFromService starts an episode where its service says the user
// left it, while MixMusic has no place of its own.
func TestResumeFromService(t *testing.T) {
	p, _ := openPositions(t)
	ep := episode("sp:e1")
	ep.Episode.ResumeMs = 600000
	if got := p.ResumeAt(ep); got != 595*time.Second {
		t.Errorf("ResumeAt = %v", got)
	}
	p.SetPlayed(ep, true)
	if got := p.ResumeAt(ep); got != 0 {
		t.Errorf("played here: ResumeAt = %v", got)
	}
	ep.Episode.Played, ep.Episode.ResumeMs = true, 0
	p.SetPlayed(ep, false)
	if got := p.ResumeAt(ep); got != 0 {
		t.Errorf("played there: ResumeAt = %v", got)
	}
}

// TestInProgress lists the episodes left partway, the latest first.
func TestInProgress(t *testing.T) {
	p, _ := openPositions(t)
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	p.now = func() time.Time { now = now.Add(time.Minute); return now }
	for _, id := range []string{"yt:a", "yt:b", "yt:c"} {
		p.Keep(episode(id), time.Minute, time.Hour)
	}
	p.SetPlayed(episode("yt:d"), true)
	got := p.InProgress(2)
	if len(got) != 2 || got[0].Track.ID != "yt:c" || got[1].Track.ID != "yt:b" {
		t.Errorf("InProgress(2) = %+v", got)
	}
	v := p.Version()
	p.Keep(episode("yt:a"), 2*time.Minute, time.Hour)
	if p.Version() == v || p.InProgress(0)[0].Track.ID != "yt:a" {
		t.Errorf("after a's change: %+v", p.InProgress(0))
	}
}

// TestPositionsTrim keeps the latest 500 episodes.
func TestPositionsTrim(t *testing.T) {
	p, _ := openPositions(t)
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	p.now = func() time.Time { now = now.Add(time.Second); return now }
	for i := range maxPositions + 2 {
		p.Keep(episode("yt:"+string(rune('a'+i%26))+string(rune('a'+i/26))), time.Minute, time.Hour)
	}
	if _, ok := p.Get("yt:aa"); ok {
		t.Error("the oldest is kept")
	}
	if _, ok := p.Get("yt:ca"); !ok {
		t.Error("the third oldest is gone")
	}
}

// TestPositionsWrite writes the places once they rested, and at once on
// Flush, for the next start to read.
func TestPositionsWrite(t *testing.T) {
	was := writeAfter
	writeAfter = 20 * time.Millisecond
	t.Cleanup(func() { writeAfter = was })
	p, path := openPositions(t)
	p.Keep(episode("yt:e1"), time.Minute, time.Hour)
	deadline := time.Now().Add(5 * time.Second)
	for {
		again, err := OpenPositions(path)
		if err != nil {
			t.Fatal(err)
		}
		if pos, ok := again.Get("yt:e1"); ok && pos.PositionMs == 60000 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the place wasn't written")
		}
		time.Sleep(10 * time.Millisecond)
	}
	writeAfter = time.Hour
	p.Keep(episode("yt:e2"), 2*time.Minute, time.Hour)
	if err := p.Flush(); err != nil {
		t.Fatal(err)
	}
	again, _ := OpenPositions(path)
	if pos, ok := again.Get("yt:e2"); !ok || pos.PositionMs != 120000 || pos.Track.Episode == nil {
		t.Errorf("after Flush: %+v, %v", pos, ok)
	}
}
