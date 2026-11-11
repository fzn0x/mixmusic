package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// queueIDs are the IDs of the tracks of a player's queue.
func queueIDs(st player.State) []string {
	var out []string
	for _, it := range st.Queue {
		out = append(out, it.Track.ID)
	}
	return out
}

// TestRemoteQueueBetweenApps plays, moves, takes out and clears the tracks
// after the one that plays on another computer, by the entries its states
// name.
func TestRemoteQueueBetweenApps(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	p := desk.GoPlayer()
	var tracks []media.Track
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		tracks = append(tracks, media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: "Song " + id, DurationMs: 200000})
	}
	p.PlayTracks(tracks, 0)
	waitPlayer(t, p, "a playing", func(s player.State) bool { return s.Playing })
	pairApps(t, lap, desk)
	deskID := idOf(desk)
	v := untilView(t, lap, deskID, "the entries", func(v RemoteView) bool { return len(v.State.Next) == 4 })
	if !v.CanQueue || !v.CanHandOver {
		t.Errorf("the desktop can't take the queue's commands, or a hand-over: %+v", v)
	}
	entry := func(title string) string {
		t.Helper()
		v := lap.Remote().View(deskID)
		for _, tr := range v.State.Next {
			if tr.Title == title {
				return tr.Entry
			}
		}
		t.Fatalf("no entry of %s in %+v", title, v.State.Next)
		return ""
	}
	do := func(c remote.Command) {
		t.Helper()
		if err := lap.Remote().Do(deskID, c); err != nil {
			t.Fatal(err)
		}
	}
	order := func(want ...string) func(player.State) bool {
		return func(s player.State) bool { return slices.Equal(queueIDs(s), want) }
	}
	do(remote.Command{Op: remote.OpMove, Entry: entry("Song e"), To: entry("Song b")})
	waitPlayer(t, p, "e moved", order("yt:a", "yt:e", "yt:b", "yt:c", "yt:d"))
	untilView(t, lap, deskID, "e moved there", func(v RemoteView) bool { return len(v.State.Next) == 4 && v.State.Next[0].Title == "Song e" })
	do(remote.Command{Op: remote.OpRemove, Entry: entry("Song c")})
	waitPlayer(t, p, "c out", order("yt:a", "yt:e", "yt:b", "yt:d"))
	untilView(t, lap, deskID, "c out there", func(v RemoteView) bool { return len(v.State.Next) == 3 })
	do(remote.Command{Op: remote.OpPlayEntry, Entry: entry("Song b")})
	waitPlayer(t, p, "b playing", func(s player.State) bool { return s.Index == 2 && s.Playing })
	untilView(t, lap, deskID, "b there", func(v RemoteView) bool { return v.State.Track != nil && v.State.Track.Title == "Song b" })
	do(remote.Command{Op: remote.OpClear})
	waitPlayer(t, p, "the queue cleared", order("yt:a", "yt:e", "yt:b"))
}

// musicOf writes a folder of music for the app a, whose files hold the
// FLAC of the flac package's testdata, and scans it: its tracks.
func musicOf(t *testing.T, a *App, names ...string) []media.Track {
	t.Helper()
	music := filepath.Join(t.TempDir(), "music")
	if err := os.MkdirAll(music, 0o755); err != nil {
		t.Fatal(err)
	}
	flac, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(music, name), flac, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := a.settings.Get()
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{music}}
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	scanned(t, a)
	return (&LocalFiles{a}).Tracks()
}

// TestRemoteHandOverBetweenApps hands what plays on one computer over to
// another, which plays it from where it was, with its tracks of the
// services, its own file of the song of a file of the first, and without
// the file it doesn't have; the first pauses. When the track that played
// is one left out, the next plays from its start.
func TestRemoteHandOverBetweenApps(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	deskNotes := notesOf(t, desk)
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	mine := musicOf(t, desk, "01 Slow Tide.flac")
	if len(mine) != 1 {
		t.Fatalf("the desktop's music %+v", mine)
	}
	elsewhere := t.TempDir()
	// The laptop's own file of the same song, and one the desktop hasn't.
	same := mine[0]
	same.ID = localfiles.TrackID(filepath.Join(elsewhere, "Slow Tide.flac"))
	same.Album, same.Artwork = &media.Ref{ID: "file:album:eA", Name: "Shore"}, []media.Image{{URL: "local-art:x"}}
	other := media.Track{ID: localfiles.TrackID(filepath.Join(elsewhere, "Kelp.flac")), Source: media.SourceFile, Title: "Kelp",
		Artists: []media.Ref{{Name: "Harbour Lights"}}, DurationMs: 180000}
	yt := media.Track{ID: "yt:aaaaaaaaaaa", Source: media.SourceYouTube, Title: "Paper Boats", DurationMs: 240000,
		Artists: []media.Ref{{ID: "yt:UCaaaa", Name: "Harbour Lights"}}, Artwork: []media.Image{{URL: "https://example.com/p.jpg", Width: 544}}}
	sp := media.Track{ID: "sp:4uLU6hMCjMI75M1A2tKUQC", Source: media.SourceSpotify, Title: "Glass Harbour", DurationMs: 200000,
		Album: &media.Ref{ID: "sp:1aaaa", Name: "Tides"}}

	lp, dp := lap.GoPlayer(), desk.GoPlayer()
	lp.PlayTracks([]media.Track{sp, yt, same, other}, 1)
	waitPlayer(t, lp, "the laptop playing", func(s player.State) bool { return s.Playing && s.Index == 1 })
	lp.Seek(30 * time.Second)
	waitPlayer(t, lp, "the seek", func(s player.State) bool { return s.Position >= 30*time.Second })
	lp.CycleRepeat()
	waitPlayer(t, lp, "repeating", func(s player.State) bool { return s.Repeat == player.RepeatAll })

	pairApps(t, lap, desk)
	deskID := idOf(desk)
	got, err := lap.Remote().HandOver(deskID)
	if err != nil {
		t.Fatal(err)
	}
	if got != (HandedOver{Kept: 3, Left: 1}) {
		t.Errorf("handed over %+v", got)
	}
	waitPlayer(t, dp, "the desktop playing it", func(s player.State) bool { return s.Playing && s.Index == 1 })
	st := dp.State()
	if want := []string{sp.ID, yt.ID, mine[0].ID}; !slices.Equal(queueIDs(st), want) {
		t.Errorf("the desktop's queue %v, want %v", queueIDs(st), want)
	}
	if st.Position < 30*time.Second || st.Repeat != player.RepeatAll || st.Shuffle {
		t.Errorf("the desktop at %v, repeat %q, shuffle %v", st.Position, st.Repeat, st.Shuffle)
	}
	if a := st.Queue[1].Track; len(a.Artists) != 1 || a.Artists[0].ID != "yt:UCaaaa" || len(a.Artwork) != 1 || a.Album != nil {
		t.Errorf("the track of YouTube Music %+v", a)
	}
	if a := st.Queue[0].Track; a.Album == nil || a.Album.ID != "sp:1aaaa" || a.Album.Name != "Tides" {
		t.Errorf("the track of Spotify %+v", a)
	}
	waitPlayer(t, lp, "the laptop paused", func(s player.State) bool { return !s.Playing })
	until(t, "the desktop's note", func() bool { return deskNotes.has("Laptop moved what it played to this computer") })

	// The track that played is a file the desktop hasn't: the next plays.
	lp.PlayTracks([]media.Track{other, yt}, 0)
	waitPlayer(t, lp, "the laptop playing the file", func(s player.State) bool { return s.Playing && len(s.Queue) == 2 && s.Index == 0 })
	lp.Seek(20 * time.Second)
	waitPlayer(t, lp, "the seek", func(s player.State) bool { return s.Position >= 20*time.Second })
	if got, err = lap.Remote().HandOver(deskID); err != nil || got != (HandedOver{Kept: 1, Left: 1, CurrentLeft: true}) {
		t.Errorf("handed over %+v, %v", got, err)
	}
	waitPlayer(t, dp, "the next playing", func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == yt.ID && s.Playing && len(s.Queue) == 1 && s.Position < 20*time.Second
	})
	waitPlayer(t, lp, "the laptop paused again", func(s player.State) bool { return !s.Playing })

	// Only files the desktop hasn't: nothing changes, and the laptop plays
	// on.
	lp.PlayTracks([]media.Track{other}, 0)
	waitPlayer(t, lp, "the laptop playing", func(s player.State) bool { return s.Playing && len(s.Queue) == 1 })
	if got, err = lap.Remote().HandOver(deskID); err != nil || got != (HandedOver{Left: 1, CurrentLeft: true}) {
		t.Errorf("handed over %+v, %v", got, err)
	}
	if !lp.State().Playing {
		t.Error("the laptop paused, though the desktop took nothing")
	}
	if cur, _ := dp.State().Current(); cur.Track.ID != yt.ID {
		t.Errorf("the desktop plays %s", cur.Track.ID)
	}
}

// TestRemoteHandOverRefused hands nothing over without a session, nor
// from a computer where nothing plays.
func TestRemoteHandOverRefused(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	if _, err := lap.Remote().HandOver(idOf(desk)); err == nil || !strings.Contains(err.Error(), "Not connected") {
		t.Errorf("without a session: %v", err)
	}
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	pairApps(t, lap, desk)
	if _, err := lap.Remote().HandOver(idOf(desk)); err != errNothingPlays {
		t.Errorf("nothing playing: %v", err)
	}
}

// TestHandOverOf tells the queue around the track that plays, a file of
// this computer without its path, where the track is but for a live one,
// and how the queue goes on.
func TestHandOverOf(t *testing.T) {
	file := media.Track{ID: localfiles.TrackID(filepath.Join(t.TempDir(), "a.flac")), Source: media.SourceFile, Title: "Low Tide",
		Artists: []media.Ref{{ID: "file:artist:eA", Name: "Kelp"}}, Album: &media.Ref{ID: "file:album:eA", Name: "Shore"},
		Artwork: []media.Image{{URL: "local-art:a"}}, DurationMs: 1000}
	ep := media.Track{ID: "yt:bbbbbbbbbbb", Source: media.SourceYouTube, Title: "Episode", Episode: &media.Episode{Show: media.Ref{ID: "yt:MPSPa", Name: "Show"}},
		Artwork: []media.Image{{URL: "http://example.com/a.jpg"}, {URL: "https://example.com/a.jpg"}}}
	queue := make([]player.Item, 1200)
	for i := range queue {
		queue[i] = player.Item{Track: media.Track{ID: "yt:x", Source: media.SourceYouTube, Title: "Filler"}}
	}
	queue[300], queue[301] = player.Item{Track: file}, player.Item{Track: ep}
	h, ok := handOverOf(player.State{Queue: queue, Index: 300, Position: 5 * time.Second, Loading: true, Repeat: player.RepeatOne, Shuffle: true})
	if !ok || len(h.Queue) != remote.MaxHandOver || h.Index != 100 || h.PositionMs != 5000 || !h.Playing || h.Repeat != "one" || !h.Shuffle {
		t.Fatalf("handed over %d tracks, at %d: %+v", len(h.Queue), h.Index, h.Queue[h.Index])
	}
	f := h.Queue[h.Index]
	if f.Source != "file" || f.ID != "" || f.Artists[0].ID != "" || f.Artists[0].Name != "Kelp" || f.Album.ID != "" || len(f.Artwork) != 0 {
		t.Errorf("the file %+v", f)
	}
	e := h.Queue[h.Index+1]
	if e.ID != ep.ID || e.Show == nil || e.Show.ID != "yt:MPSPa" || len(e.Artwork) != 1 {
		t.Errorf("the episode %+v", e)
	}
	if h, _ := handOverOf(player.State{Queue: queue[:2], Index: 1, Position: time.Minute, Live: true, Playing: true}); h.PositionMs != 0 || h.Index != 1 {
		t.Errorf("a live stream from %d", h.PositionMs)
	}
	if _, ok := handOverOf(player.State{Queue: queue[:2], Index: -1}); ok {
		t.Error("handed over with nothing current")
	}
}

// TestTrackOfHanded plays the tracks of the services by their IDs, when
// they are some, with the IDs of their artists and albums of the same
// service, and nothing else.
func TestTrackOfHanded(t *testing.T) {
	none := func(remote.QueueTrack) (media.Track, bool) { return media.Track{}, false }
	got, ok := trackOfHanded(remote.QueueTrack{Source: "spotify", ID: "sp:4uLU6hMCjMI75M1A2tKUQC", Title: "Glass Harbour", DurationMs: 1 << 50,
		Artists: []remote.Ref{{ID: "yt:UCaaaa", Name: "Kelp"}, {ID: "sp:2bbb", Name: "Harbour Lights"}}, Show: &remote.Ref{ID: "sp:show1", Name: "Show"},
		Artwork: []remote.Image{{URL: "https://example.com/c.jpg", Width: 640}}}, none)
	if !ok || got.Source != media.SourceSpotify || got.Artists[0].ID != "" || got.Artists[1].ID != "sp:2bbb" || got.DurationMs != 24*60*60*1000 ||
		got.Episode == nil || got.Episode.Show.ID != "sp:show1" || len(got.Artwork) != 1 || got.Artwork[0].Width != 640 {
		t.Errorf("the Spotify track %+v", got)
	}
	for _, q := range []remote.QueueTrack{
		{Source: "youtube", ID: "yt:short"},
		{Source: "youtube", ID: "sp:4uLU6hMCjMI75M1A2tKUQC"},
		{Source: "spotify", ID: "sp:4uLU6h/../..MCjMI75"},
		{Source: "local", ID: "mx:a"},
		{Source: "file", Title: "Kelp"},
		{Source: "", ID: "yt:aaaaaaaaaaa"},
	} {
		if got, ok := trackOfHanded(q, none); ok {
			t.Errorf("%+v plays as %+v", q, got)
		}
	}
}

// TestLocalSongsFind finds the file of the song of another computer's
// file: of the same title and an artist of the same name, as tags tell,
// and nearly the same length; of the same album first.
func TestLocalSongsFind(t *testing.T) {
	tr := func(id, title, artist, album string, ms int) media.Track {
		t := media.Track{ID: id, Source: media.SourceFile, Title: title, Artists: []media.Ref{{Name: artist}}, DurationMs: ms}
		if album != "" {
			t.Album = &media.Ref{Name: album}
		}
		return t
	}
	m := &localSongs{byTitle: map[string][]media.Track{}}
	for _, t := range []media.Track{tr("1", "Low  Tide", "Kelp", "Shore", 200000), tr("2", "Low Tide", "Kelp", "Live at Sea", 201000),
		tr("3", "Paper Boats", "Harbour Lights", "", 0)} {
		m.byTitle[foldTag(t.Title)] = append(m.byTitle[foldTag(t.Title)], t)
	}
	for _, c := range []struct {
		q    remote.QueueTrack
		want string
	}{
		{remote.QueueTrack{Title: "low tide", Artists: []remote.Ref{{Name: "KELP"}}, DurationMs: 202000}, "1"},
		{remote.QueueTrack{Title: "Low Tide", Artists: []remote.Ref{{Name: "Kelp"}}, Album: &remote.Ref{Name: "live at sea"}}, "2"},
		{remote.QueueTrack{Title: "Low Tide", Artists: []remote.Ref{{Name: "Kelp"}}, DurationMs: 260000}, ""},
		{remote.QueueTrack{Title: "Low Tide", Artists: []remote.Ref{{Name: "Harbour Lights"}}}, ""},
		{remote.QueueTrack{Title: "Paper Boats", Artists: []remote.Ref{{Name: "Kelp"}, {Name: "Harbour Lights"}}, DurationMs: 5}, "3"},
		{remote.QueueTrack{Title: "Salt", Artists: []remote.Ref{{Name: "Kelp"}}}, ""},
	} {
		got, ok := m.find(c.q)
		if c.want == "" && ok || c.want != "" && got.ID != c.want {
			t.Errorf("%+v found %q, %v; want %q", c.q, got.ID, ok, c.want)
		}
	}
}
