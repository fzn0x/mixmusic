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
// time zone, from the play log, which keeps every play.
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
	// It is only known (NewKnown) when the play log reaches back before the
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

// RecapYears lists the years the play log has plays of music in, the
// latest first. It doesn't know which of them are over: the app decides.
func (l *Library) RecapYears() []int {
	loc, v := l.now().Location(), l.plays.view()
	return slices.Clone(memoized(&l.memo, l.stampOf(v), &l.memo.years, loc, func() []int { return recapYears(v, loc) }))
}

// recapYears lists the years of loc that v has plays of music in, the
// latest first.
func recapYears(v playsView, loc *time.Location) []int {
	seen := map[int]bool{}
	var years []int
	// The plays come in order: one in the year of the one before needs no
	// date.
	from, until := int64(1), int64(0)
	for _, p := range v.plays {
		if v.tracks[p.track].IsEpisode() || p.at >= from && p.at < until {
			continue
		}
		y := time.Unix(p.at, 0).In(loc).Year()
		from, until = time.Date(y, 1, 1, 0, 0, 0, 0, loc).Unix(), time.Date(y+1, 1, 1, 0, 0, 0, 0, loc).Unix()
		if !seen[y] {
			seen[y] = true
			years = append(years, y)
		}
	}
	slices.SortFunc(years, func(a, b int) int { return b - a })
	return years
}

// Recap sums up the plays of year.
func (l *Library) Recap(year int) Recap {
	loc, v := l.now().Location(), l.plays.view()
	return memoized(&l.memo, l.stampOf(v), &l.memo.recaps, recapKey{year, loc}, func() Recap {
		return countRecap(v, loc, year)
	}).clone()
}

// countRecap sums up the plays of v in year, of loc.
func countRecap(v playsView, loc *time.Location, year int) Recap {
	r := Recap{Year: year, TopTracks: []TrackPlays{}, TopArtists: []ArtistPlays{}, TopAlbums: []AlbumPlays{}}
	from, until := time.Date(year, 1, 1, 0, 0, 0, 0, loc).Unix(), time.Date(year+1, 1, 1, 0, 0, 0, 0, loc).Unix()
	counts := make([]int, len(v.tracks))
	earlier := make([]bool, len(v.tracks))
	var order []int32
	type dayTrack struct{ date, id string }
	perDay := map[dayTrack]int{}
	var months [13]int // milliseconds of each month, from 1
	ms := 0
	first := -1 // the first play of the year, of v
	// The latest plays first, as the history had them: the first play of
	// the year is the last one met, and a day's repeats are told apart by
	// counting.
	for i, p := range slices.Backward(v.plays) {
		t := &v.tracks[p.track]
		if t.IsEpisode() {
			continue
		}
		if p.at < from {
			r.NewKnown = true
			earlier[p.track] = true
			continue
		}
		if p.at >= until {
			continue
		}
		at := time.Unix(p.at, 0).In(loc)
		r.Plays++
		ms += t.DurationMs
		months[at.Month()] += t.DurationMs
		if first < 0 || p.at < v.plays[first].at {
			first = i
		}
		if counts[p.track] == 0 {
			order = append(order, p.track)
		}
		counts[p.track]++
		date := at.Format(time.DateOnly)
		n := perDay[dayTrack{date, t.ID}] + 1
		perDay[dayTrack{date, t.ID}] = n
		if n >= 2 && (r.Repeated == nil || n > r.Repeated.Plays ||
			n == r.Repeated.Plays && date < r.Repeated.Date) {
			r.Repeated = &Repeated{Track: t.Track, Date: date, Plays: n}
		}
	}
	if first >= 0 {
		p := v.plays[first]
		r.First = &Play{Track: v.tracks[p.track].Track, PlayedAt: time.Unix(p.at, 0).In(loc)}
	}
	r.Minutes = ms / 60000
	for m := time.January; m <= time.December; m++ {
		if months[m] > months[r.BusiestMonth] {
			r.BusiestMonth = m
		}
	}
	r.BusiestMinutes = months[r.BusiestMonth] / 60000
	tracks, artists := tops(v, counts, order)
	albums := map[string]*AlbumPlays{}
	for _, i := range order {
		t, n := v.tracks[i].Track, counts[i]
		if al := t.Album; al != nil && al.Name != "" {
			key := cmp.Or(al.ID, al.Name)
			if ap := albums[key]; ap != nil {
				ap.Plays += n
			} else {
				ap := &AlbumPlays{Album: *al, Plays: n}
				if len(t.Artists) > 0 {
					ap.Artist = t.Artists[0].Name
				}
				albums[key] = ap
			}
		}
	}
	if r.NewKnown {
		before := map[string]bool{}
		for i, played := range earlier {
			if played {
				for _, a := range v.tracks[i].Artists {
					before[a.Name] = true
				}
			}
		}
		for name := range artists {
			if !before[name] {
				r.NewArtists++
			}
		}
	}
	r.TopTracks = topOf(tracks, recapTop, func(a, b *TrackPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Track.Title, b.Track.Title), cmp.Compare(a.Track.ID, b.Track.ID))
	})
	r.TopArtists = topOf(artists, recapTop, func(a, b *ArtistPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Artist.Name, b.Artist.Name))
	})
	r.TopAlbums = topOf(albums, recapTop, func(a, b *AlbumPlays) int {
		return cmp.Or(b.Plays-a.Plays, cmp.Compare(a.Album.Name, b.Album.Name), cmp.Compare(a.Album.ID, b.Album.ID))
	})
	return r
}
