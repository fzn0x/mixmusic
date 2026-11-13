package main

import (
	"os"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestRenderFollows draws an artist's page, Library › Artists and Home
// with the new releases of the artists followed, in each layout design,
// in Mix and Daylight, to MIXMUSIC_RENDER, for a look.
func TestRenderFollows(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	for _, layout := range settings.Layouts {
		for _, theme := range []string{"mix", "daylight"} {
			renderFollows(t, dir, layout, theme)
		}
	}
}

func renderFollows(t *testing.T, dir string, layout settings.Layout, theme string) {
	s := newTestShell(t)
	base := coverServer(t, s)
	art := func(n int) []media.Image {
		return []media.Image{{URL: base + "/cover/" + strconv.Itoa(n) + ".png", Width: 300, Height: 300}}
	}
	st := s.svc.Preferences.Get()
	st.FollowSystem, st.Theme, st.Layout, st.DynamicAccent = false, theme, layout, false
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	artists := []media.Artist{
		{ID: "yt:UCharbour", Source: media.SourceYouTube, Name: "Harbour Lights", Artwork: art(3)},
		{ID: "yt:UCdunes", Source: media.SourceYouTube, Name: "Dunes", Artwork: art(5)},
		{ID: "yt:UCtide", Source: media.SourceYouTube, Name: "Low Tide Club", Artwork: art(8)},
	}
	for i := len(artists) - 1; i >= 0; i-- {
		if err := s.svc.Collection.SetArtistFollowed(artists[i], true); err != nil {
			t.Fatal(err)
		}
	}
	page := artists[0]
	page.Subtitle, page.TopTracks = "1.2M subscribers", testTracks[:4]
	s.loads["artist:"+page.ID] = &loaded[media.Artist]{done: true, value: page}
	var releases []media.Release
	for i, r := range []struct {
		title, released string
		kind            media.AlbumKind
		of              media.Artist
	}{
		{"Pier at Night", "2026", media.AlbumKindSingle, artists[0]},
		{"Second Light", "2026", media.AlbumKindAlbum, artists[0]},
		{"Salt", "2026", media.AlbumKindSingle, artists[1]},
		{"Glass Floats", "2026", media.AlbumKindEP, artists[2]},
		{"Tidewater", "2026", media.AlbumKindSingle, artists[1]},
	} {
		ref := media.Ref{ID: r.of.ID, Name: r.of.Name}
		releases = append(releases, media.Release{Released: r.released, Of: ref, Album: media.Album{ID: "yt:MPRE" + strconv.Itoa(i),
			Source: media.SourceYouTube, Title: r.title, Kind: r.kind, Artists: []media.Ref{ref}, Artwork: art(10 + i)}})
	}
	s.loads["releases"] = &loaded[[]media.Release]{done: true, value: releases}
	for _, key := range []string{"home:spotify", "home:netease"} {
		s.loads[key] = &loaded[[]media.Shelf]{done: true}
	}
	name := string(layout) + "-" + theme
	// Tall enough for Home's shelves under its tiles.
	tt := ui.NewTester(s.view, 1280, 1400)
	for _, p := range []struct{ name, path string }{
		{"artist", artistPath(page.ID)}, {"library-artists", "/library/artists"}, {"home", "/"},
	} {
		s.navigate(p.path)
		renderTo(t, s, tt, dir, name+"-follow-"+p.name)
		renderTo(t, s, tt, dir, name+"-follow-"+p.name)
	}
}
