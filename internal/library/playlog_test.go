package library

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// openIn opens the library of dir, as the app does on each start.
func openIn(t *testing.T, dir string) *Library {
	t.Helper()
	l, err := Open(filepath.Join(dir, "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// loggedIDs lists the tracks of the plays the log holds, the oldest first.
func loggedIDs(l *Library) []string {
	l.plays.mu.Lock()
	defer l.plays.mu.Unlock()
	var out []string
	for _, p := range l.plays.plays {
		out = append(out, l.plays.tracks[p.track].ID)
	}
	return out
}

// clockFrom makes l's clock tick a minute each time it's read, from start.
func clockFrom(l *Library, start time.Time) {
	at := start
	l.now = func() time.Time { at = at.Add(time.Minute); return at }
}

// withLog makes the log of l the plays given, the oldest first, written
// whole, as the log of a user who played them would be.
func withLog(t *testing.T, l *Library, plays []Play) {
	t.Helper()
	g := l.plays
	g.mu.Lock()
	defer g.mu.Unlock()
	g.plays, g.tracks, g.byKey = nil, nil, map[string]int32{}
	for _, p := range plays {
		line := lineOf(p.PlayedAt, 0, loggedTrack{Track: p.Track})
		g.plays = append(g.plays, loggedPlay{at: p.PlayedAt.Unix(), track: g.intern(line.trackLine)})
	}
	g.changes++
	if err := g.writeWhole(g.plays, g.tracks); err != nil {
		t.Fatal(err)
	}
}

// TestPlayLogKeepsEveryPlay keeps the plays past the history's 1,000, in
// order, with what the stats count of their tracks, after a restart too.
func TestPlayLogKeepsEveryPlay(t *testing.T) {
	dir := t.TempDir()
	l := openIn(t, dir)
	start := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	a, b := tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B")
	a.Album = &media.Ref{ID: "yt:MPREb_a", Name: "Album A"}
	var plays []Play
	for i := range maxHistory {
		plays = append(plays, Play{Track: []media.Track{a, b}[i%2], PlayedAt: start.Add(time.Duration(i) * time.Minute)})
	}
	withHistory(t, l, plays)
	clockFrom(l, start.Add(maxHistory*time.Minute))
	for range 5 {
		if err := l.RecordPlay(a); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(l.History(0)); n != maxHistory {
		t.Errorf("the history holds %d plays, want %d", n, maxHistory)
	}
	again := openIn(t, dir)
	got := loggedIDs(again)
	if len(got) != maxHistory+5 || got[0] != a.ID || got[1] != b.ID || got[len(got)-1] != a.ID {
		t.Fatalf("the log holds %d plays, from %q to %q", len(got), got[:2], got[len(got)-1])
	}
	if n := len(again.plays.tracks); n != 2 {
		t.Errorf("the log keeps %d tracks for 2", n)
	}
	first := again.plays.tracks[again.plays.plays[0].track]
	if first.Title != a.Title || first.Source != a.Source || first.Artists[0].Name != "A" || first.Album.Name != "Album A" ||
		first.DurationMs != a.DurationMs {
		t.Errorf("the first play's track = %+v", first.Track)
	}
	if at := time.Unix(again.plays.plays[0].at, 0); !at.Equal(start) {
		t.Errorf("the first play counted at %v, want %v", at, start)
	}
}

// TestPlayLogLines writes a line of JSON a play, without the covers, an
// episode marked as one.
func TestPlayLogLines(t *testing.T) {
	dir := t.TempDir()
	l := openIn(t, dir)
	l.now = func() time.Time { return time.Date(2026, 3, 5, 21, 4, 5, 0, time.FixedZone("UTC+7", 7*60*60)) }
	song := media.Track{ID: "yt:dQw4w9WgXcQ", Source: media.SourceYouTube, Title: "Glass Harbor",
		Artists:    []media.Ref{{ID: "yt:UCnorth", Name: "North Pier"}},
		Album:      &media.Ref{ID: "yt:MPREb_paper", Name: "Paper Lanterns"},
		DurationMs: 213000, Artwork: []media.Image{{URL: "https://img/glass"}}}
	l.RecordPlay(song)
	l.RecordPlay(episode("yt:e"))
	data, err := os.ReadFile(filepath.Join(dir, playLogName))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"at":"2026-03-05T21:04:05+07:00","id":"yt:dQw4w9WgXcQ","source":"youtube","title":"Glass Harbor",` +
		`"artists":[{"id":"yt:UCnorth","name":"North Pier"}],"album":{"id":"yt:MPREb_paper","name":"Paper Lanterns"},` +
		`"durationMs":213000}` + "\n" +
		`{"at":"2026-03-05T21:04:05+07:00","id":"yt:e","source":"youtube","title":"yt:e","durationMs":3600000,"episode":true}` + "\n"
	if string(data) != want {
		t.Errorf("the log holds\n%s\nwant\n%s", data, want)
	}
	if ep := openIn(t, dir).plays.tracks[1]; !ep.IsEpisode() {
		t.Errorf("the episode reads as %+v", ep.Track)
	}
}

// TestPlayLogTornLine skips a line a crash cut short, and starts the next
// play on a line of its own.
func TestPlayLogTornLine(t *testing.T) {
	dir := t.TempDir()
	l := openIn(t, dir)
	clockFrom(l, time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
	l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
	l.RecordPlay(tr("yt:b", media.SourceYouTube, "B"))
	path := filepath.Join(dir, playLogName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The second line, cut short.
	if err := os.WriteFile(path, data[:len(data)-20], 0o600); err != nil {
		t.Fatal(err)
	}
	l = openIn(t, dir)
	if got := loggedIDs(l); !slices.Equal(got, []string{"yt:a"}) {
		t.Errorf("a torn line read: %q", got)
	}
	clockFrom(l, time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC))
	if err := l.RecordPlay(tr("yt:c", media.SourceYouTube, "C")); err != nil {
		t.Fatal(err)
	}
	if got := loggedIDs(openIn(t, dir)); !slices.Equal(got, []string{"yt:a", "yt:c"}) {
		t.Errorf("after a play that followed a torn line, the log reads %q", got)
	}
}

// TestPlayLogRetriesFailedAppend writes a play whose append failed with
// the next play, in order.
func TestPlayLogRetriesFailedAppend(t *testing.T) {
	dir := t.TempDir()
	l := openIn(t, dir)
	clockFrom(l, time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
	l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
	path := filepath.Join(dir, playLogName)
	// A folder in the file's place fails the append.
	if err := os.Rename(path, path+".aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordPlay(tr("yt:b", media.SourceYouTube, "B")); err == nil {
		t.Error("an append that failed told no error")
	}
	if got := loggedIDs(l); !slices.Equal(got, []string{"yt:a", "yt:b"}) {
		t.Errorf("while the file can't be written, the log counts %q", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".aside", path); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordPlay(tr("yt:c", media.SourceYouTube, "C")); err != nil {
		t.Fatal(err)
	}
	if got := loggedIDs(openIn(t, dir)); !slices.Equal(got, []string{"yt:a", "yt:b", "yt:c"}) {
		t.Errorf("after the file came back, the log reads %q", got)
	}
}

// TestPlayLogAppendCutShort writes again, on a line of its own, a line an
// append wrote only part of, and counts once one it wrote but for its line
// break.
func TestPlayLogAppendCutShort(t *testing.T) {
	for _, tc := range []struct {
		name string
		// cut is how much of the second line reached the file.
		cut func(line []byte) int
	}{
		{"partway", func(line []byte) int { return 10 }},
		{"but its line break", func(line []byte) int { return len(line) - 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			l := openIn(t, dir)
			clockFrom(l, time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
			l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
			path := filepath.Join(dir, playLogName)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			l.RecordPlay(tr("yt:b", media.SourceYouTube, "B"))
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// As if the second append failed after cut bytes.
			second := bytes.TrimPrefix(after, before)
			n := tc.cut(second)
			if err := os.WriteFile(path, append(before, second[:n]...), 0o600); err != nil {
				t.Fatal(err)
			}
			l.plays.io.Lock()
			l.plays.pending = slices.Clone(second)
			l.plays.wrote(n, true)
			if !l.plays.torn || !bytes.Equal(l.plays.pending, second) {
				t.Errorf("after %d bytes of %d: torn %v, pending %q", n, len(second), l.plays.torn, l.plays.pending)
			}
			l.plays.io.Unlock()
			if err := l.RecordPlay(tr("yt:c", media.SourceYouTube, "C")); err != nil {
				t.Fatal(err)
			}
			if got := loggedIDs(openIn(t, dir)); !slices.Equal(got, []string{"yt:a", "yt:b", "yt:c"}) {
				t.Errorf("the log reads %q", got)
			}
		})
	}
}

// TestPlayLogClear forgets every play with the history, also when the file
// can't be written then: the next play writes it anew.
func TestPlayLogClear(t *testing.T) {
	dir := t.TempDir()
	l := openIn(t, dir)
	clockFrom(l, time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC))
	l.RecordPlay(tr("yt:a", media.SourceYouTube, "A"))
	l.RecordPlay(tr("yt:b", media.SourceYouTube, "B"))
	if err := l.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if got := loggedIDs(openIn(t, dir)); len(got) != 0 {
		t.Errorf("after a clear, the log reads %q", got)
	}

	l.RecordPlay(tr("yt:c", media.SourceYouTube, "C"))
	path := filepath.Join(dir, playLogName)
	if err := os.Rename(path, path+".aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := l.ClearHistory(); err == nil {
		t.Error("a clear that couldn't write the log told no error")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".aside", path); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordPlay(tr("yt:d", media.SourceYouTube, "D")); err != nil {
		t.Fatal(err)
	}
	if got := loggedIDs(openIn(t, dir)); !slices.Equal(got, []string{"yt:d"}) {
		t.Errorf("after a clear that failed and a play, the log reads %q", got)
	}
}

// TestPlayLogFromHistory fills a new log with the history's plays, once:
// the plays counted before there was a log count, each once, after a
// restart too.
func TestPlayLogFromHistory(t *testing.T) {
	dir := t.TempDir()
	old := openIn(t, dir)
	start := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)
	a, b := tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B")
	withHistory(t, old, []Play{{Track: a, PlayedAt: start}, {Track: b, PlayedAt: start.Add(time.Hour)}, {Track: a, PlayedAt: start.Add(2 * time.Hour)}})
	// As a library.json from before the log was kept.
	if err := os.Remove(filepath.Join(dir, playLogName)); err != nil {
		t.Fatal(err)
	}

	l := openIn(t, dir)
	if got := loggedIDs(l); !slices.Equal(got, []string{"yt:a", "sp:b", "yt:a"}) {
		t.Fatalf("the log made from the history reads %q", got)
	}
	if at := time.Unix(l.plays.plays[1].at, 0); !at.Equal(start.Add(time.Hour)) {
		t.Errorf("the second play counted at %v", at)
	}
	clockFrom(l, start.Add(3*time.Hour))
	if err := l.RecordPlay(b); err != nil {
		t.Fatal(err)
	}
	again := openIn(t, dir)
	if got := loggedIDs(again); !slices.Equal(got, []string{"yt:a", "sp:b", "yt:a", "sp:b"}) {
		t.Errorf("after a play and a restart, the log reads %q", got)
	}
	// A log there is never filled again, from what the history holds.
	again.file.Update(func(d *Data) error { d.History = append(d.History, Play{Track: a, PlayedAt: start}); return nil })
	if got := loggedIDs(openIn(t, dir)); len(got) != 4 {
		t.Errorf("the log was filled again: %q", got)
	}
}

// TestPlayLogRetarget moves the plays of files that became copies of a
// song to the song, each when it played, those older than the history
// too, which moves the library's version.
func TestPlayLogRetarget(t *testing.T) {
	dir := t.TempDir()
	l := openIn(t, dir)
	mp3, flac := tr("file:bXAz", media.SourceFile, "Ada Field"), tr("file:ZmxhYw", media.SourceFile, "Ada Field")
	song := tr("file:song:ZmxhYw", media.SourceFile, "Ada Field")
	other := tr("yt:other", media.SourceYouTube, "Orin Vale")
	start := time.Date(2024, 3, 1, 20, 0, 0, 0, time.UTC)
	withLog(t, l, []Play{{Track: mp3, PlayedAt: start}, {Track: other, PlayedAt: start.Add(time.Hour)}, {Track: flac, PlayedAt: start.Add(2 * time.Hour)}})
	v := l.Version()
	if err := l.Retarget(map[string]media.Track{mp3.ID: song, flac.ID: song}); err != nil {
		t.Fatal(err)
	}
	if l.Version() == v {
		t.Error("a retarget of the log alone kept the library's version")
	}
	want := []string{song.ID, other.ID, song.ID}
	if got := loggedIDs(l); !slices.Equal(got, want) {
		t.Errorf("the log holds %q, want %q", got, want)
	}
	again := openIn(t, dir)
	if got := loggedIDs(again); !slices.Equal(got, want) {
		t.Errorf("after a restart, the log holds %q, want %q", got, want)
	}
	if at := time.Unix(again.plays.plays[2].at, 0); !at.Equal(start.Add(2 * time.Hour)) {
		t.Errorf("the third play counted at %v", at)
	}
	v = again.Version()
	if err := again.Retarget(map[string]media.Track{mp3.ID: song}); err != nil || again.Version() != v {
		t.Errorf("a retarget of nothing held: %v, version %d, was %d", err, again.Version(), v)
	}
}

// TestStatsCountEveryPlay counts every play the log holds, years of them,
// far past the history's 1,000: All time, Your year of each year, whole,
// and the Plays sort. A top track has its cover from the library, which
// the log leaves out.
func TestStatsCountEveryPlay(t *testing.T) {
	l := open(t)
	l.now = func() time.Time { return time.Date(2027, 1, 1, 0, 30, 0, 0, time.UTC) }
	liked := tr("yt:liked", media.SourceYouTube, "North Pier")
	songs := []media.Track{liked, tr("yt:b", media.SourceYouTube, "Quiet Tide"), tr("sp:c", media.SourceSpotify, "Low Orchard")}
	var plays []Play
	for d := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC); d.Year() < 2027; d = d.AddDate(0, 0, 1) {
		plays = append(plays, Play{Track: songs[0], PlayedAt: d}, Play{Track: songs[len(plays)/2%3], PlayedAt: d.Add(30 * time.Minute)})
	}
	withLog(t, l, plays)
	if err := l.SetLiked(liked, true); err != nil {
		t.Fatal(err)
	}
	days := 366 + 365 + 365
	s := l.Stats(0)
	if s.Plays != 2*days || s.Minutes != 3*2*days || s.YouTube+s.Spotify != 2*days {
		t.Errorf("all time: %d plays, %d minutes, %d+%d", s.Plays, s.Minutes, s.YouTube, s.Spotify)
	}
	if top := s.TopTracks[0]; top.Track.ID != liked.ID || top.Plays < days || len(top.Track.Artwork) == 0 {
		t.Errorf("the top track = %+v", top)
	}
	if week := l.Stats(7); week.Plays != 14 {
		t.Errorf("the last 7 days count %d plays, want 14", week.Plays)
	}
	if got := l.RecapYears(); !slices.Equal(got, []int{2026, 2025, 2024}) {
		t.Errorf("years %v", got)
	}
	if r := l.Recap(2024); r.Plays != 2*366 || r.First == nil || !r.First.PlayedAt.Equal(time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)) || r.NewKnown {
		t.Errorf("2024: %d plays, first %+v, new known %v", r.Plays, r.First, r.NewKnown)
	}
	if r := l.Recap(2025); r.Plays != 2*365 || !r.NewKnown || r.NewArtists != 0 {
		t.Errorf("2025: %d plays, new artists %d, known %v", r.Plays, r.NewArtists, r.NewKnown)
	}
	total := 0
	for _, n := range l.PlayCounts() {
		total += n
	}
	if total != 2*days {
		t.Errorf("the Plays sort counts %d plays, want %d", total, 2*days)
	}
}
