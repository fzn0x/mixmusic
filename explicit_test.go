package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// The songs, albums and artists here are made up. Of each pair, the one
// whose title says "Salt" or "Tidewater" is explicit, as its service marks
// it.

var (
	gullArtists = []media.Ref{{ID: "yt:ada", Name: "Ada Field"}}
	// paperBoats is a song, and saltWind an explicit one.
	paperBoats = media.Track{ID: "yt:paper", Source: media.SourceYouTube, Title: "Paper Boats", Artists: gullArtists, DurationMs: 200000}
	saltWind   = media.Track{ID: "yt:salt", Source: media.SourceYouTube, Title: "Salt on the Wind", Artists: gullArtists, DurationMs: 200000, Explicit: true}
	// lowWater is another song, and saltSpray another explicit one.
	lowWater  = media.Track{ID: "yt:low", Source: media.SourceYouTube, Title: "Low Water", Artists: gullArtists, DurationMs: 200000}
	saltSpray = media.Track{ID: "yt:spray", Source: media.SourceYouTube, Title: "Salt Spray", Artists: gullArtists, DurationMs: 200000, Explicit: true}
	// saltVideo is an explicit music video, and pierVideo a video with no
	// such mark, as an upload that isn't music.
	saltVideo = media.Track{ID: "yt:saltvideo", Source: media.SourceYouTube, Title: "Salt on the Wind (Live at the Pier)", Artists: gullArtists, Video: true, Explicit: true}
	pierVideo = media.Track{ID: "yt:pier", Source: media.SourceYouTube, Title: "A Walk on the Pier", Artists: []media.Ref{{Name: "Harbour Cam"}}, Video: true}
	// kitesExplicit and kitesClean are an album's two versions, and
	// tidewater an album that comes explicit alone.
	kitesExplicit = media.Album{ID: "yt:kites-e", Source: media.SourceYouTube, Title: "Kites", Year: "2019", Kind: media.AlbumKindAlbum, Artists: gullArtists, Explicit: true}
	kitesClean    = media.Album{ID: "yt:kites-c", Source: media.SourceYouTube, Title: "Kites", Year: "2019", Kind: media.AlbumKindAlbum, Artists: gullArtists}
	tidewater     = media.Album{ID: "yt:tide", Source: media.SourceYouTube, Title: "Tidewater", Year: "2016", Kind: media.AlbumKindAlbum, Artists: gullArtists, Explicit: true}
)

// hidingExplicit has the window hide explicit content, as the switch does,
// and the player, as the app tells it.
func hidingExplicit(t *testing.T, s *shell) {
	t.Helper()
	s.set.HideExplicit = true
	s.save()
	if !s.svc.Preferences.Get().HideExplicit {
		t.Fatal("hiding explicit content wasn't saved")
	}
	s.p.SettingsChanged(s.svc.Preferences.Get())
	// The window shows the player's notices, as it does once open.
	t.Cleanup(s.p.Notices(func(n player.Notice) { s.update(func() { s.notes = append(s.notes, n) }) }))
}

// playingTrack reports whether the player plays t.
func playingTrack(t media.Track) func(player.State) bool {
	return func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == t.ID && st.Playing
	}
}

// TestHideExplicitSearch leaves the explicit songs, videos and albums out
// of what a search found, and counts what's left; a video with no mark,
// and the clean version of an album, stay.
func TestHideExplicitSearch(t *testing.T) {
	s := newTestShell(t)
	s.loads["search:salt"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{NetEase: media.SearchResults{
		Tracks: []media.Track{saltWind, paperBoats}, Videos: []media.Track{saltVideo, pierVideo},
		Albums: []media.Album{kitesExplicit, kitesClean, tidewater}}}}
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, searchPath("salt"))
	for _, title := range []string{"Salt on the Wind", "Paper Boats", "Salt on the Wind (Live at the Pier)", "A Walk on the Pier", "Tidewater"} {
		if !showsExactly(tt, title) {
			t.Fatalf("not hiding, the search lacks %q: %q", title, tt.Texts())
		}
	}
	if !tt.HasText("2019 · 2 versions · Ada Field") {
		t.Errorf("not hiding, Kites shows no versions: %q", tt.Texts())
	}

	hidingExplicit(t, s)
	show(s, tt, searchPath("salt"))
	for _, title := range []string{"Salt on the Wind", "Salt on the Wind (Live at the Pier)", "Tidewater"} {
		if showsExactly(tt, title) {
			t.Errorf("hiding, the search shows %q", title)
		}
	}
	for _, title := range []string{"Paper Boats", "A Walk on the Pier"} {
		if !showsExactly(tt, title) {
			t.Errorf("hiding, the search lacks %q: %q", title, tt.Texts())
		}
	}
	if n := tilesOf(tt, "Kites"); n != 1 || tt.HasText("versions") || !tt.HasText("2019 · Ada Field") {
		t.Errorf("hiding, %d tiles of Kites: %q", n, tt.Texts())
	}
	r := s.shownResults(mergeResults(s.loads["search:salt"].(*loaded[app.SearchResponse]).value, 0))
	if len(r.Tracks) != 1 || len(r.Videos) != 1 || len(r.Albums) != 1 {
		t.Errorf("the pivots count %d songs, %d videos and %d albums", len(r.Tracks), len(r.Videos), len(r.Albums))
	}
}

// TestHideExplicitSuggestions leaves the explicit songs and albums out of
// what the search box suggests.
func TestHideExplicitSuggestions(t *testing.T) {
	s := newTestShell(t)
	s.loads["suggest:salt"] = &loaded[media.Suggestions]{done: true, value: media.Suggestions{
		Searches: []string{"salt on the wind"},
		Items: []media.Item{
			{Kind: media.ItemTrack, Track: &saltWind},
			{Kind: media.ItemAlbum, Album: &tidewater},
			{Kind: media.ItemTrack, Track: &paperBoats},
			{Kind: media.ItemArtist, Artist: &media.Artist{ID: "yt:ada", Source: media.SourceYouTube, Name: "Ada Field"}},
		},
	}}
	hidingExplicit(t, s)
	tt := tester(t, s)
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	tt.Type("salt")
	tt.Advance(time.Second)
	texts := tt.Texts()
	for _, want := range []string{"salt on the wind", "Paper Boats", "Ada Field"} {
		if !slices.Contains(texts, want) {
			t.Errorf("the suggestions lack %q: %q", want, texts)
		}
	}
	for _, hidden := range []string{"Salt on the Wind", "Tidewater"} {
		if slices.Contains(texts, hidden) {
			t.Errorf("the suggestions show %q", hidden)
		}
	}
}

// TestHideExplicitHome leaves the explicit songs and albums out of Home's
// shelves, its own and the services', a shelf of nothing else whole, and
// out of its tile of what played last.
func TestHideExplicitHome(t *testing.T) {
	s := newTestShell(t)
	for _, tr := range []media.Track{paperBoats, saltWind} {
		if err := s.svc.Collection.RecordPlay(tr); err != nil {
			t.Fatal(err)
		}
	}
	tt := testerOf(t, s, 1280, 2400)
	s.loads["home:netease"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
		{Title: "Quick picks", Source: media.SourceNetEase, Items: []media.Item{
			{Kind: media.ItemTrack, Track: &saltSpray}, {Kind: media.ItemTrack, Track: &lowWater}, {Kind: media.ItemAlbum, Album: &kitesExplicit},
			{Kind: media.ItemAlbum, Album: &kitesClean}}},
		{Title: "Late night", Source: media.SourceNetEase, Items: []media.Item{{Kind: media.ItemAlbum, Album: &tidewater}}},
	}}
	show(s, tt, "/")
	until(t, s, tt, "Home's shelves", func() bool { return tt.HasText("Late night") && tt.HasText("Quick picks") })
	if !tt.HasText("Play Salt on the Wind again") || !showsExactly(tt, "Salt Spray") {
		t.Fatalf("not hiding, Home shows %q", tt.Texts())
	}

	hidingExplicit(t, s)
	show(s, tt, "/")
	for _, hidden := range []string{"Salt on the Wind", "Salt Spray", "Tidewater", "Late night"} {
		if tt.HasText(hidden) {
			t.Errorf("hiding, Home shows %q", hidden)
		}
	}
	for _, want := range []string{"Recently played", "Paper Boats", "Quick picks", "Low Water", "Search for music"} {
		if !tt.HasText(want) {
			t.Errorf("hiding, Home lacks %q: %q", want, tt.Texts())
		}
	}
	if n := tilesOf(tt, "Kites"); n != 1 {
		t.Errorf("hiding, Home shows %d tiles of Kites", n)
	}
}

// TestHideExplicitArtist leaves the explicit songs, albums and singles out
// of an artist's page, and an album's page leaves its explicit version
// out of its switch.
func TestHideExplicitArtist(t *testing.T) {
	s := newTestShell(t)
	gulls := media.Album{ID: "yt:gulls", Source: media.SourceYouTube, Title: "Gulls (Explicit)", Year: "2021", Kind: media.AlbumKindSingle, Artists: gullArtists}
	s.loads["artist:yt:ada"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "yt:ada", Source: media.SourceYouTube, Name: "Ada Field",
		TopTracks: []media.Track{saltWind, paperBoats}, Albums: []media.Album{kitesExplicit, kitesClean, tidewater}, Singles: []media.Album{gulls}}}
	for _, a := range []media.Album{kitesExplicit, kitesClean} {
		page := a
		page.Tracks = []media.Track{paperBoats}
		if a.Explicit {
			page.Tracks = []media.Track{saltWind}
		}
		s.loads["album:"+a.ID] = &loaded[media.Album]{done: true, value: page}
	}
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, artistPath("yt:ada"))
	if !showsExactly(tt, "Salt on the Wind") || !tt.HasText("Tidewater") || !tt.HasText("Gulls (Explicit)") || !tt.HasText("2 versions") {
		t.Fatalf("not hiding, the artist's page shows %q", tt.Texts())
	}

	hidingExplicit(t, s)
	show(s, tt, artistPath("yt:ada"))
	for _, hidden := range []string{"Salt on the Wind", "Tidewater", "Gulls (Explicit)", "singles", "versions"} {
		if tt.HasText(hidden) {
			t.Errorf("hiding, the artist's page shows %q: %q", hidden, tt.Texts())
		}
	}
	if !showsExactly(tt, "Paper Boats") || tilesOf(tt, "Kites") != 1 {
		t.Fatalf("hiding, the artist's page lacks its others: %q", tt.Texts())
	}

	// An album's page, opened from a list that held both versions, offers
	// no switch to the explicit one.
	show(s, tt, "/library")
	s.versions = albumVersions{}
	s.albumGroups("library", []media.Album{kitesExplicit, kitesClean})
	s.openAlbum(kitesClean.ID)
	show(s, tt, albumPath(kitesClean.ID))
	until(t, s, tt, "the clean version", func() bool { return showsExactly(tt, "Paper Boats") })
	if tt.HasText("explicit") || tt.HasText("clean") {
		t.Errorf("hiding, the album's switch shows %q", tt.Texts())
	}
}

// TestHideExplicitOwnLists lists the explicit songs of the user's own mix
// and liked songs dimmed, with a note, and leaves them out of a playlist
// of another's; playing one tells why it doesn't.
func TestHideExplicitOwnLists(t *testing.T) {
	s := newTestShell(t)
	mix, err := s.svc.Collection.CreatePlaylist("Harbour", "", []media.Track{saltWind, paperBoats})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.svc.Collection.SetLiked(context.Background(), saltSpray, true); err != nil {
		t.Fatal(err)
	}
	other := media.Playlist{ID: "yt:PLgulls", Source: media.SourceYouTube, Title: "Gull Mix", Owner: "YouTube Music",
		Tracks: []media.Track{saltWind, paperBoats, saltSpray, lowWater}}
	s.loads["playlist:"+other.ID] = &loaded[media.Playlist]{done: true, value: other}
	hidingExplicit(t, s)
	tt := testerOf(t, s, 1280, 1200)

	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix", func() bool { return showsExactly(tt, "Paper Boats") })
	if !showsExactly(tt, "Salt on the Wind") || textCount(tt, hiddenNote) != 1 {
		t.Errorf("the mix doesn't list Salt on the Wind dimmed: %q", tt.Texts())
	}
	click(t, s, tt, "Play Salt on the Wind")
	until(t, s, tt, "the notice", func() bool {
		return tt.HasText("Salt on the Wind is explicit, and your settings hide explicit content")
	})
	if _, ok := s.p.State().Current(); ok {
		t.Errorf("Salt on the Wind played: %+v", s.p.State())
	}

	show(s, tt, "/library/liked")
	until(t, s, tt, "the liked songs", func() bool { return showsExactly(tt, "Salt Spray") })
	if textCount(tt, hiddenNote) != 1 {
		t.Errorf("the liked songs don't list Salt Spray dimmed: %q", tt.Texts())
	}

	show(s, tt, playlistPath(other.ID))
	until(t, s, tt, "the playlist", func() bool { return showsExactly(tt, "Low Water") })
	for _, hidden := range []string{"Salt on the Wind", "Salt Spray", hiddenNote} {
		if showsExactly(tt, hidden) {
			t.Errorf("another's playlist shows %q: %q", hidden, tt.Texts())
		}
	}
}

// TestHideExplicitQueue lists the explicit songs of the queue dimmed, with
// a note, and passes over them; a list of explicit songs alone doesn't
// play, and says why.
func TestHideExplicitQueue(t *testing.T) {
	s := newTestShell(t)
	hidingExplicit(t, s)
	tt := tester(t, s)
	s.p.PlayTracks([]media.Track{paperBoats, saltWind, lowWater}, 0)
	s.settle(t, playingTrack(paperBoats))
	s.showPanel("queue")
	until(t, s, tt, "the queue", func() bool { return tt.HasText("Salt on the Wind") })
	if !tt.HasText("Ada Field · " + hiddenNote) {
		t.Errorf("the queue doesn't note Salt on the Wind hidden: %q", tt.Texts())
	}
	s.p.Next()
	s.settle(t, playingTrack(lowWater))

	s.p.PlayAll([]media.Track{saltWind, saltSpray}, false)
	s.settle(t, nil)
	until(t, s, tt, "the notice", func() bool {
		return tt.HasText("Every song here is explicit, and your settings hide explicit content")
	})
	if cur, _ := s.p.State().Current(); cur.Track.ID != lowWater.ID {
		t.Errorf("a list of explicit songs replaced the queue, at %s", cur.Track.ID)
	}
}

// TestHideExplicitSetting turns on Hide explicit content in Settings ›
// Playback › Explicit content, beside Prefer clean versions, which is on
// meanwhile and can't be turned off; the player passes over explicit
// songs from then on.
func TestHideExplicitSetting(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3600)
	show(s, tt, "/settings/playback")
	const label = "Hide explicit content"
	if a, ok := tt.AccessByRole(ui.RoleSwitch, "Prefer clean versions"); !ok || a.Checked || a.Disabled {
		t.Fatalf("Prefer clean versions at first: %+v, %v", a, ok)
	}
	if r, ok := tt.Find(label); !ok {
		t.Fatalf("no %s: %q", label, tt.Texts())
	} else if p, _ := tt.Find("Prefer clean versions"); r.Y < p.Y {
		t.Errorf("%s comes before Prefer clean versions", label)
	}
	focused := func() bool { return tt.Focused(label) && !tt.Focused(hideExplicitDescription()) }
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	until(t, s, tt, "Hide explicit content saved", func() bool { return s.set.HideExplicit && s.svc.Preferences.Get().HideExplicit })
	tt.Frame()
	if a, _ := tt.AccessByRole(ui.RoleSwitch, "Prefer clean versions"); !a.Checked || !a.Disabled || !tt.HasText("On while explicit content is hidden") {
		t.Errorf("hiding, Prefer clean versions is %+v", a)
	}
	if s.svc.Preferences.Get().CleanVersions {
		t.Error("hiding set Prefer clean versions itself")
	}
	// The app tells its player of the settings saved, and the window
	// shows its notices.
	s.p.SettingsChanged(s.svc.Preferences.Get())
	t.Cleanup(s.p.Notices(func(n player.Notice) { s.update(func() { s.notes = append(s.notes, n) }) }))
	until(t, s, tt, "the player told", func() bool {
		s.p.PlayTracks([]media.Track{saltWind}, 0)
		return tt.HasText("Salt on the Wind is explicit, and your settings hide explicit content")
	})
	if _, ok := s.p.State().Current(); ok {
		t.Errorf("Salt on the Wind played: %+v", s.p.State())
	}
}
