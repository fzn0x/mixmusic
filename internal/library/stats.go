package library

import (
	"cmp"
	"math"
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

// Stats sums up the plays of the last days, or of all the play log holds
// when days is 0.
func (l *Library) Stats(days int) Stats {
	s := countStats(l.plays.view(), l.now(), days)
	l.completeTracks(s.TopTracks)
	return s
}

// countStats sums up the plays of v of the last days up to now, or all of
// them when days is 0.
func countStats(v playsView, now time.Time, days int) Stats {
	s := Stats{TopTracks: []TrackPlays{}, TopArtists: []ArtistPlays{}, Days: days}
	since := int64(math.MinInt64)
	if days > 0 {
		since = now.AddDate(0, 0, -days).Unix()
	}
	daily, dayIndex := dailyDays(now, cmp.Or(days, dailyAllTime))
	y, m, d := now.Date()
	dailyFrom := time.Date(y, m, d-(len(daily)-1), 0, 0, 0, 0, now.Location()).Unix()
	dayMs := make([]int, len(daily))
	counts := make([]int, len(v.tracks))
	var order []int32
	ms := 0
	for _, p := range slices.Backward(v.plays) {
		t := &v.tracks[p.track]
		if p.at < since || t.IsEpisode() {
			continue // the stats are of music; the log keeps episodes for the Plays sort
		}
		s.Plays++
		ms += t.DurationMs
		if p.at >= dailyFrom {
			if i, ok := dayIndex[time.Unix(p.at, 0).In(now.Location()).Format(time.DateOnly)]; ok {
				daily[i].Plays++
				dayMs[i] += t.DurationMs
			}
		}
		switch t.Source {
		case media.SourceYouTube:
			s.YouTube++
		case media.SourceSpotify:
			s.Spotify++
		}
		if counts[p.track] == 0 {
			order = append(order, p.track)
		}
		counts[p.track]++
	}
	s.Minutes = ms / 60000
	for i := range daily {
		daily[i].Minutes = dayMs[i] / 60000
	}
	s.Daily = daily
	tracks, artists := tops(v, counts, order)
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

// tops sums up the plays counted of each track of v, by its ID, and of
// each artist, by name: counts holds the plays of each track of v, and
// order the tracks counted, by their latest play first, whose track and
// artists the sums show, as the history newest first had them.
func tops(v playsView, counts []int, order []int32) (map[string]*TrackPlays, map[string]*ArtistPlays) {
	tracks := map[string]*TrackPlays{}
	artists := map[string]*ArtistPlays{}
	for _, i := range order {
		t, n := v.tracks[i].Track, counts[i]
		if tp := tracks[t.ID]; tp != nil {
			tp.Plays += n
		} else {
			tracks[t.ID] = &TrackPlays{Track: t, Plays: n}
		}
		for _, a := range t.Artists {
			// Services spell the same artist alike, not with the same ID.
			if ap := artists[a.Name]; ap != nil {
				ap.Plays += n
			} else {
				artists[a.Name] = &ArtistPlays{Artist: a, Plays: n}
			}
		}
	}
	return tracks, artists
}

// completeTracks gives the tracks of tops what the play log leaves out of
// them, as their covers: each as the history, the liked or disliked songs
// or a mix holds it. One the library no longer holds stays as the log has
// it.
func (l *Library) completeTracks(tops []TrackPlays) {
	want := make(map[string]int, len(tops))
	for i, tp := range tops {
		want[tp.Track.ID] = i
	}
	take := func(t media.Track) bool {
		if i, ok := want[t.ID]; ok {
			tops[i].Track = t
			delete(want, t.ID)
		}
		return len(want) == 0
	}
	if len(want) == 0 {
		return
	}
	d := l.file.Get()
	for _, p := range d.History {
		if take(p.Track) {
			return
		}
	}
	lists := [][]media.Track{d.Liked, d.Disliked}
	for _, p := range d.Playlists {
		lists = append(lists, p.Tracks)
	}
	for _, list := range lists {
		if slices.ContainsFunc(list, take) {
			return
		}
	}
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
