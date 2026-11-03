package library

import (
	"cmp"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

const (
	// topCount is how many tracks and artists the stats rank.
	topCount = 10
	// dailyAllTime is how many days the daily plays cover for the whole
	// history.
	dailyAllTime = 90
)

// TrackPlays is a track and how often it was played.
type TrackPlays struct {
	Track media.Track `json:"track"`
	Plays int         `json:"plays"`
}

// ArtistPlays is an artist and how often their tracks were played.
type ArtistPlays struct {
	Artist media.Ref `json:"artist"`
	Plays  int       `json:"plays"`
}

// DayPlays is what the user played on one day of their own time zone.
type DayPlays struct {
	// Date is the day, as 2026-10-19.
	Date    string `json:"date"`
	Plays   int    `json:"plays"`
	Minutes int    `json:"minutes"`
}

// Stats sums up what the user listened to.
type Stats struct {
	Plays   int `json:"plays"`
	Minutes int `json:"minutes"`
	// YouTube and Spotify count the plays of each service's tracks.
	YouTube    int           `json:"youtube"`
	Spotify    int           `json:"spotify"`
	TopTracks  []TrackPlays  `json:"topTracks"`
	TopArtists []ArtistPlays `json:"topArtists"`
	// Days is the number of days counted; 0 counts the whole history.
	Days int `json:"days"`
	// Daily holds every day of the period, the oldest first, those
	// without plays too: the last Days days, or the last 90 for the whole
	// history.
	Daily []DayPlays `json:"daily"`
}

// Stats sums up the plays of the last days, or of the whole history when
// days is 0.
func (l *Library) Stats(days int) Stats {
	s := Stats{TopTracks: []TrackPlays{}, TopArtists: []ArtistPlays{}, Days: days}
	since := time.Time{}
	if days > 0 {
		since = l.now().AddDate(0, 0, -days)
	}
	tracks := map[string]*TrackPlays{}
	artists := map[string]*ArtistPlays{}
	daily, dayIndex := dailyDays(l.now(), cmp.Or(days, dailyAllTime))
	dayMs := make([]int, len(daily))
	ms := 0
	for _, p := range l.file.Get().History {
		if p.PlayedAt.Before(since) {
			break // the history is newest first
		}
		if p.Track.IsEpisode() {
			continue // the stats are of music; the history keeps episodes
		}
		s.Plays++
		ms += p.Track.DurationMs
		if i, ok := dayIndex[p.PlayedAt.In(l.now().Location()).Format(time.DateOnly)]; ok {
			daily[i].Plays++
			dayMs[i] += p.Track.DurationMs
		}
		switch p.Track.Source {
		case media.SourceYouTube:
			s.YouTube++
		case media.SourceSpotify:
			s.Spotify++
		}
		if tp := tracks[p.Track.ID]; tp != nil {
			tp.Plays++
		} else {
			tracks[p.Track.ID] = &TrackPlays{Track: p.Track, Plays: 1}
		}
		for _, a := range p.Track.Artists {
			// Services spell the same artist alike, not with the same ID.
			if ap := artists[a.Name]; ap != nil {
				ap.Plays++
			} else {
				artists[a.Name] = &ArtistPlays{Artist: a, Plays: 1}
			}
		}
	}
	s.Minutes = ms / 60000
	for i := range daily {
		daily[i].Minutes = dayMs[i] / 60000
	}
	s.Daily = daily
	for _, tp := range tracks {
		s.TopTracks = append(s.TopTracks, *tp)
	}
	for _, ap := range artists {
		s.TopArtists = append(s.TopArtists, *ap)
	}
	slices.SortFunc(s.TopTracks, func(a, b TrackPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Track.Title, b.Track.Title))
	})
	slices.SortFunc(s.TopArtists, func(a, b ArtistPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Artist.Name, b.Artist.Name))
	})
	s.TopTracks = s.TopTracks[:min(len(s.TopTracks), topCount)]
	s.TopArtists = s.TopArtists[:min(len(s.TopArtists), topCount)]
	return s
}

// dailyDays returns the last n days up to now's, in now's time zone, the
// oldest first, and the index of each by its date.
func dailyDays(now time.Time, n int) ([]DayPlays, map[string]int) {
	days := make([]DayPlays, n)
	index := make(map[string]int, n)
	y, m, d := now.Date()
	for i := range n {
		// Adding days to the calendar date stays right across clock changes.
		date := time.Date(y, m, d-(n-1-i), 12, 0, 0, 0, now.Location()).Format(time.DateOnly)
		days[i] = DayPlays{Date: date}
		index[date] = i
	}
	return days, index
}
