package library

import (
	"cmp"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// recapTop is how many songs, artists and albums a recap ranks.
const recapTop = 5

// AlbumPlays is an album and how often its tracks were played.
type AlbumPlays struct {
	Album media.Ref `json:"album"`
	// Artist is the first artist of the first track played from it.
	Artist string `json:"artist,omitempty"`
	Plays  int    `json:"plays"`
}

// Recap sums up what the user played in one calendar year of their own
// time zone, from the history, which keeps the last plays only.
type Recap struct {
	Year    int `json:"year"`
	Plays   int `json:"plays"`
	Minutes int `json:"minutes"`
	// The five most played songs, artists and albums.
	TopTracks  []TrackPlays  `json:"topTracks"`
	TopArtists []ArtistPlays `json:"topArtists"`
	TopAlbums  []AlbumPlays  `json:"topAlbums"`
	// BusiestMonth is the month the user listened most minutes in, and
	// BusiestMinutes those minutes; the month is 0 without plays.
	BusiestMonth   time.Month `json:"busiestMonth"`
	BusiestMinutes int        `json:"busiestMinutes"`
	// First is the first track played in the year.
	First *Play `json:"first,omitempty"`
	// Repeated is the track played most often on one day, with that day
	// and its plays; it is unset unless some track played twice in a day.
	Repeated *Repeated `json:"repeated,omitempty"`
	// NewArtists counts the artists played in the year and not before it.
	// It is only known (NewKnown) when the history reaches back before the
	// year: otherwise every artist would look new.
	NewArtists int  `json:"newArtists"`
	NewKnown   bool `json:"newKnown"`
}

// Repeated is a track played again and again on a day.
type Repeated struct {
	Track media.Track `json:"track"`
	// Date is the day, as 2026-10-19.
	Date  string `json:"date"`
	Plays int    `json:"plays"`
}

// RecapYears lists the years the history has plays of music in, the latest
// first. It doesn't know which of them are over: the app decides.
func (l *Library) RecapYears() []int {
	loc := l.now().Location()
	seen := map[int]bool{}
	var years []int
	for _, p := range l.file.Get().History {
		if p.Track.IsEpisode() {
			continue
		}
		if y := p.PlayedAt.In(loc).Year(); !seen[y] {
			seen[y] = true
			years = append(years, y)
		}
	}
	slices.SortFunc(years, func(a, b int) int { return b - a })
	return years
}

// Recap sums up the plays of year.
func (l *Library) Recap(year int) Recap {
	r := Recap{Year: year, TopTracks: []TrackPlays{}, TopArtists: []ArtistPlays{}, TopAlbums: []AlbumPlays{}}
	loc := l.now().Location()
	tracks := map[string]*TrackPlays{}
	artists := map[string]*ArtistPlays{}
	albums := map[string]*AlbumPlays{}
	earlier := map[string]bool{}
	type dayTrack struct{ date, id string }
	perDay := map[dayTrack]int{}
	var months [13]int // milliseconds of each month, from 1
	ms := 0
	// The history is newest first; the first play of the year is the
	// last one met, and a day's repeats are told apart by counting.
	for _, p := range l.file.Get().History {
		if p.Track.IsEpisode() {
			continue
		}
		at := p.PlayedAt.In(loc)
		if at.Year() < year {
			r.NewKnown = true
			for _, a := range p.Track.Artists {
				earlier[a.Name] = true
			}
			continue
		}
		if at.Year() > year {
			continue
		}
		r.Plays++
		ms += p.Track.DurationMs
		months[at.Month()] += p.Track.DurationMs
		if r.First == nil || at.Before(r.First.PlayedAt.In(loc)) {
			p := p
			r.First = &p
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
		if al := p.Track.Album; al != nil && al.Name != "" {
			key := cmp.Or(al.ID, al.Name)
			if ap := albums[key]; ap != nil {
				ap.Plays++
			} else {
				ap := &AlbumPlays{Album: *al, Plays: 1}
				if len(p.Track.Artists) > 0 {
					ap.Artist = p.Track.Artists[0].Name
				}
				albums[key] = ap
			}
		}
		date := at.Format(time.DateOnly)
		n := perDay[dayTrack{date, p.Track.ID}] + 1
		perDay[dayTrack{date, p.Track.ID}] = n
		if n >= 2 && (r.Repeated == nil || n > r.Repeated.Plays ||
			n == r.Repeated.Plays && date < r.Repeated.Date) {
			r.Repeated = &Repeated{Track: p.Track, Date: date, Plays: n}
		}
	}
	r.Minutes = ms / 60000
	for m := time.January; m <= time.December; m++ {
		if months[m] > months[r.BusiestMonth] {
			r.BusiestMonth = m
		}
	}
	r.BusiestMinutes = months[r.BusiestMonth] / 60000
	for _, a := range artists {
		if r.NewKnown && !earlier[a.Artist.Name] {
			r.NewArtists++
		}
	}
	for _, tp := range tracks {
		r.TopTracks = append(r.TopTracks, *tp)
	}
	for _, ap := range artists {
		r.TopArtists = append(r.TopArtists, *ap)
	}
	for _, ap := range albums {
		r.TopAlbums = append(r.TopAlbums, *ap)
	}
	slices.SortFunc(r.TopTracks, func(a, b TrackPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Track.Title, b.Track.Title), cmp.Compare(a.Track.ID, b.Track.ID))
	})
	slices.SortFunc(r.TopArtists, func(a, b ArtistPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Artist.Name, b.Artist.Name))
	})
	slices.SortFunc(r.TopAlbums, func(a, b AlbumPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Album.Name, b.Album.Name), cmp.Compare(a.Album.ID, b.Album.ID))
	})
	r.TopTracks = r.TopTracks[:min(len(r.TopTracks), recapTop)]
	r.TopArtists = r.TopArtists[:min(len(r.TopArtists), recapTop)]
	r.TopAlbums = r.TopAlbums[:min(len(r.TopAlbums), recapTop)]
	return r
}
