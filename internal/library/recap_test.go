package library

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// recapTrack is a made-up track of an artist and an album, of minutes.
func recapTrack(id, title, artist, album string, minutes int) media.Track {
	t := media.Track{ID: id, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: artist}}, DurationMs: minutes * 60000}
	if album != "" {
		t.Album = &media.Ref{ID: "al-" + album, Name: album}
	}
	return t
}

// withHistory makes the history of l the plays given, oldest first.
func withHistory(t *testing.T, l *Library, plays []Play) {
	t.Helper()
	err := l.file.Update(func(d *Data) error {
		d.History = nil
		for _, p := range plays {
			d.History = append([]Play{p}, d.History...)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func day(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }

func TestRecap(t *testing.T) {
	l := open(t)
	l.now = func() time.Time { return day(2026, 12, 3, 12) }
	glass := recapTrack("yt:glass", "Glass Harbor", "North Pier", "Paper Lanterns", 4)
	moss := recapTrack("yt:moss", "Moss Garden", "Quiet Tide", "Paper Lanterns", 3)
	kite := recapTrack("yt:kite", "Kite Weather", "Low Orchard", "", 5)
	old := recapTrack("yt:old", "Old Ferry", "North Pier", "", 2)
	ep := media.Track{ID: "ep:1", Source: media.SourceYouTube, Title: "Talk", DurationMs: 3600000, Episode: &media.Episode{}}
	withHistory(t, l, []Play{
		{old, day(2025, 12, 31, 23)},
		{kite, day(2026, 1, 2, 9)},
		{glass, day(2026, 3, 5, 9)},
		{glass, day(2026, 3, 5, 10)},
		{glass, day(2026, 3, 5, 11)},
		{moss, day(2026, 3, 6, 9)},
		{ep, day(2026, 3, 6, 10)},
		{glass, day(2026, 7, 1, 9)},
		{kite, day(2026, 12, 1, 9)},
	})
	if got := l.RecapYears(); len(got) != 2 || got[0] != 2026 || got[1] != 2025 {
		t.Errorf("years = %v", got)
	}
	r := l.Recap(2026)
	if r.Plays != 7 || r.Minutes != 5+4*4+3+5 {
		t.Errorf("plays %d, minutes %d", r.Plays, r.Minutes)
	}
	if len(r.TopTracks) != 3 || r.TopTracks[0].Track.ID != "yt:glass" || r.TopTracks[0].Plays != 4 || r.TopTracks[1].Track.ID != "yt:kite" {
		t.Errorf("top tracks = %+v", r.TopTracks)
	}
	if len(r.TopArtists) != 3 || r.TopArtists[0].Artist.Name != "North Pier" || r.TopArtists[0].Plays != 4 {
		t.Errorf("top artists = %+v", r.TopArtists)
	}
	if len(r.TopAlbums) != 1 || r.TopAlbums[0].Album.Name != "Paper Lanterns" || r.TopAlbums[0].Plays != 5 || r.TopAlbums[0].Artist != "North Pier" {
		t.Errorf("top albums = %+v", r.TopAlbums)
	}
	if r.BusiestMonth != time.March || r.BusiestMinutes != 4*3+3 {
		t.Errorf("busiest month %v, %d minutes", r.BusiestMonth, r.BusiestMinutes)
	}
	if r.First == nil || r.First.Track.ID != "yt:kite" {
		t.Errorf("first = %+v", r.First)
	}
	if r.Repeated == nil || r.Repeated.Track.ID != "yt:glass" || r.Repeated.Plays != 3 || r.Repeated.Date != "2026-03-05" {
		t.Errorf("repeated = %+v", r.Repeated)
	}
	// Kite Weather's artist and Quiet Tide are new; North Pier played in 2025.
	if !r.NewKnown || r.NewArtists != 2 {
		t.Errorf("new artists = %d, known %v", r.NewArtists, r.NewKnown)
	}
	if p := l.Recap(2025); p.Plays != 1 || p.NewKnown {
		t.Errorf("2025 = %+v", p)
	}
}

// TestRecapEmpty has nothing to rank, and no month.
func TestRecapEmpty(t *testing.T) {
	l := open(t)
	r := l.Recap(2026)
	if r.Plays != 0 || r.BusiestMonth != 0 || r.First != nil || r.Repeated != nil || len(r.TopTracks) != 0 {
		t.Errorf("recap of nothing = %+v", r)
	}
	if len(l.RecapYears()) != 0 {
		t.Error("years of an empty history")
	}
}

// TestRecapZone counts a play by the day of the user's own time zone.
func TestRecapZone(t *testing.T) {
	l := open(t)
	zone := time.FixedZone("UTC+7", 7*60*60)
	l.now = func() time.Time { return day(2026, 6, 1, 0).In(zone) }
	a := recapTrack("yt:a", "Glass Harbor", "North Pier", "", 3)
	// 20:00 UTC on New Year's Eve is 03:00 on the 1st in UTC+7.
	withHistory(t, l, []Play{{a, day(2025, 12, 31, 20)}})
	if got := l.RecapYears(); len(got) != 1 || got[0] != 2026 {
		t.Errorf("years = %v", got)
	}
	if r := l.Recap(2026); r.Plays != 1 || r.BusiestMonth != time.January {
		t.Errorf("recap = %+v", r)
	}
}
