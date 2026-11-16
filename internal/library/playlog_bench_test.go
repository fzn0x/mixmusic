package library

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The play log over years of plays: go test ./internal/library -run '^$'
// -bench PlayLog -benchmem. "5y" is five years of 10 plays a day among
// 3,000 songs, "10y-heavy" ten years of 50 a day among 20,000.
var playLogSizes = []struct {
	name                 string
	years, perDay, songs int
}{
	{"5y", 5, 10, 3000},
	{"10y-heavy", 10, 50, 20000},
}

// madeUpID returns n letters of a made-up ID.
func madeUpID(r *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[r.IntN(len(letters))]
	}
	return string(b)
}

// madeUpTitle returns a title of words.
func madeUpTitle(r *rand.Rand, words int) string {
	vocabulary := strings.Fields("glass harbor moss garden kite weather old ferry paper lanterns slow orchard north pier quiet tide low night drive river light summer")
	out := make([]string, words)
	for i := range out {
		out[i] = vocabulary[r.IntN(len(vocabulary))]
	}
	return strings.Join(out, " ")
}

// yearsOfPlays makes up the plays of a user who played perDay tracks a day
// for years, up to the end of 2026, among songs songs, a few of them far
// more than the others, the oldest first.
func yearsOfPlays(years, perDay, songs int) []Play {
	r := rand.New(rand.NewPCG(1, 2))
	artists := make([]media.Ref, songs/10+1)
	for i := range artists {
		artists[i] = media.Ref{ID: "yt:UC" + madeUpID(r, 22), Name: madeUpTitle(r, 2)}
	}
	tracks := make([]media.Track, songs)
	for i := range tracks {
		t := media.Track{ID: "yt:" + madeUpID(r, 11), Source: media.SourceYouTube, Title: madeUpTitle(r, 1+r.IntN(4)),
			Artists: []media.Ref{artists[r.IntN(len(artists))]}, DurationMs: 120000 + r.IntN(240000)}
		if i%3 == 0 {
			t.ID, t.Source = "sp:"+madeUpID(r, 22), media.SourceSpotify
		}
		if i%4 != 0 {
			t.Album = &media.Ref{ID: "yt:MPREb_" + madeUpID(r, 11), Name: madeUpTitle(r, 1+r.IntN(3))}
		}
		if i%5 == 0 {
			t.Artists = append(t.Artists, artists[r.IntN(len(artists))])
		}
		tracks[i] = t
	}
	zipf := rand.NewZipf(r, 1.1, 2, uint64(songs-1))
	start := time.Date(2026-years+1, 1, 1, 8, 0, 0, 0, time.UTC)
	var plays []Play
	for day := 0; day < 365*years; day++ {
		for n := range perDay {
			at := start.AddDate(0, 0, day).Add(time.Duration(n*14) * time.Minute)
			plays = append(plays, Play{Track: tracks[zipf.Uint64()], PlayedAt: at})
		}
	}
	return plays
}

// logOfYears writes a log of the plays of years, and returns its folder.
func logOfYears(b *testing.B, years, perDay, songs int) string {
	b.Helper()
	dir := b.TempDir()
	l, err := Open(filepath.Join(dir, "library.json"))
	if err != nil {
		b.Fatal(err)
	}
	g := l.plays
	g.mu.Lock()
	for _, p := range yearsOfPlays(years, perDay, songs) {
		line := lineOf(p.PlayedAt, 0, loggedTrack{Track: p.Track})
		g.plays = append(g.plays, loggedPlay{at: p.PlayedAt.Unix(), track: g.intern(line.trackLine)})
	}
	err = g.writeWhole(g.plays, g.tracks)
	g.mu.Unlock()
	if err != nil {
		b.Fatal(err)
	}
	return dir
}

// BenchmarkPlayLogOpen reads the log as the library opens, and tells its
// size: bytes a play, and megabytes a year.
func BenchmarkPlayLogOpen(b *testing.B) {
	for _, size := range playLogSizes {
		b.Run(size.name, func(b *testing.B) {
			dir := logOfYears(b, size.years, size.perDay, size.songs)
			info, err := os.Stat(filepath.Join(dir, playLogName))
			if err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				if _, _, err := openPlayLog(filepath.Join(dir, playLogName)); err != nil {
					b.Fatal(err)
				}
			}
			plays := size.years * 365 * size.perDay
			b.ReportMetric(float64(info.Size())/float64(plays), "bytes/play")
			b.ReportMetric(float64(info.Size())/float64(size.years)/1e6, "MB/year")
		})
	}
}

// BenchmarkPlayLogStats counts the stats over the whole log, each time
// anew, as after a play: All time, the last 30 days, Your year, its years
// and the Plays sort; and All time again, as kept until the log changes.
func BenchmarkPlayLogStats(b *testing.B) {
	for _, size := range playLogSizes {
		dir := logOfYears(b, size.years, size.perDay, size.songs)
		l, err := Open(filepath.Join(dir, "library.json"))
		if err != nil {
			b.Fatal(err)
		}
		now := time.Date(2026, 12, 31, 22, 0, 0, 0, time.UTC)
		v := l.plays.view()
		for _, bench := range []struct {
			name string
			run  func()
		}{
			{"all-time", func() { countStats(v, now, 0) }},
			{"30-days", func() { countStats(v, now, 30) }},
			{"recap", func() { countRecap(v, time.UTC, 2026) }},
			{"recap-years", func() { recapYears(v, time.UTC) }},
			{"play-counts", func() { playCounts(v) }},
			// As the Stats page asks in each frame, the log unchanged.
			{"all-time-kept", func() { l.Stats(0) }},
		} {
			b.Run(fmt.Sprintf("%s/%s", size.name, bench.name), func(b *testing.B) {
				for b.Loop() {
					bench.run()
				}
			})
		}
	}
}
