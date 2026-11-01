package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// coverOf draws a made-up cover, of hue n: two colours across it and a
// disc, as abstract sleeves are.
func coverOf(n int) []byte {
	const size = 300
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	hue := float64(n*67%360) / 360
	a := hsv(hue, 0.75, 0.85)
	b := hsv(math.Mod(hue+0.12, 1), 0.85, 0.35)
	disc := hsv(math.Mod(hue+0.5, 1), 0.5, 0.95)
	for y := range size {
		for x := range size {
			t := float64(x+y) / (2 * size)
			c := color.NRGBA{R: lerp8(a.R, b.R, t), G: lerp8(a.G, b.G, t), B: lerp8(a.B, b.B, t), A: 255}
			if dx, dy := float64(x)-0.62*size, float64(y)-0.4*size; dx*dx+dy*dy < 0.07*size*size {
				c = disc
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func lerp8(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }

func hsv(h, s, v float64) color.NRGBA {
	i := math.Floor(h * 6)
	f := h*6 - i
	p, q, t := v*(1-s), v*(1-f*s), v*(1-(1-f)*s)
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return color.NRGBA{R: uint8(r * 255), G: uint8(g * 255), B: uint8(b * 255), A: 255}
}

// coverServer serves made-up covers at /cover/<n>.png, to the artwork of
// s.
func coverServer(t *testing.T, s *shell) string {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/cover/"), ".png"))
		w.Header().Set("Content-Type", "image/png")
		w.Write(coverOf(n))
	}))
	t.Cleanup(srv.Close)
	s.svc.System.Artwork().HTTP = srv.Client()
	return srv.URL
}

// TestRenderDesignLanguages draws the main pages in the Material and Crate
// themes, light and dark, and in Mix, to MIXMUSIC_RENDER, with made-up
// covers, for a look: MIXMUSIC_THEMES lists others, and MIXMUSIC_FOLLOW=off
// keeps the themes' own accents rather than the covers'.
func TestRenderDesignLanguages(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	themes := []string{"material", "material-dark", "crate", "crate-light", "mix"}
	if only := os.Getenv("MIXMUSIC_THEMES"); only != "" {
		themes = strings.Split(only, ",")
	}
	for _, theme := range themes {
		s := newTestShell(t)
		base := coverServer(t, s)
		art := func(n int) []media.Image {
			return []media.Image{{URL: base + "/cover/" + strconv.Itoa(n) + ".png", Width: 300, Height: 300}}
		}
		tracks := make([]media.Track, len(testTracks))
		for i, tr := range testTracks {
			tr.Artwork = art(i)
			tracks[i] = tr
		}
		st := s.svc.Preferences.Get()
		st.FollowSystem, st.Theme = false, theme
		// The theme's own accent, rather than the cover's.
		st.DynamicAccent = os.Getenv("MIXMUSIC_FOLLOW") != "off"
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		album := media.Album{ID: "al-1", Source: media.SourceYouTube, Title: "Hurry Up, We're Dreaming", Artists: []media.Ref{{ID: "ar-m83", Name: "M83"}},
			Year: "2011", Tracks: tracks, Artwork: art(0)}
		var albums []media.Item
		for i := range 8 {
			a := album
			a.ID, a.Artwork = "al-"+count(i+2), art(i+7)
			a.Title = []string{"Mezzanine", "Untrue", "Takk...", "Dummy", "Selected Ambient Works", "Moon Safari", "Homogenic", "Kid A"}[i]
			albums = append(albums, media.Item{Kind: media.ItemAlbum, Album: &a})
		}
		var trackItems []media.Item
		for i := range tracks {
			trackItems = append(trackItems, media.Item{Kind: media.ItemTrack, Track: &tracks[i]})
		}
		s.loads["home"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
			{Title: "Listen again", Source: media.SourceYouTube, Items: trackItems},
			{Title: "Albums for you", Source: media.SourceYouTube, Items: albums},
		}}
		s.loads["album:al-1"] = &loaded[media.Album]{done: true, value: album}
		s.loads["artist:ar-m83"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "ar-m83", Source: media.SourceYouTube, Name: "M83",
			Artwork: art(3), Subtitle: "2.1M monthly audience", TopTracks: tracks[:4], Related: []media.Ref{{ID: "x1", Name: "Tycho"}, {ID: "x2", Name: "Washed Out"}},
			Albums: func() (out []media.Album) {
				for _, it := range albums[:5] {
					out = append(out, *it.Album)
				}
				return out
			}()}}
		s.loads["lyrics:spotify:2"] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB", Lines: []lyrics.Line{
			{TimeMs: 0, Text: "Love, love is a verb"}, {TimeMs: 4000, Text: "Love is a doing word"},
			{TimeMs: 9000, Text: "Fearless on my breath"}, {TimeMs: 14000, Text: "Gentle impulsion"}}}}
		for i, tr := range tracks {
			for range len(tracks) - i {
				if err := s.svc.Collection.RecordPlay(tr); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
				t.Fatal(err)
			}
		}
		s.loads["search:midnight"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
			YouTube: media.SearchResults{Tracks: tracks[:3], Albums: []media.Album{album, *albums[0].Album, *albums[1].Album},
				Artists: []media.Artist{{ID: "ar-m83", Name: "M83", Source: media.SourceYouTube, Artwork: art(3)}}},
			Spotify: media.SearchResults{Tracks: tracks[3:]}, SpotifySignedIn: true}}
		s.loads["moods"] = &loaded[[]youtube.MoodGroup]{done: true, value: []youtube.MoodGroup{
			{Title: "Moods & moments", Moods: []youtube.Mood{{Title: "Chill", Params: "a"}, {Title: "Commute", Params: "b"}, {Title: "Energy boosters", Params: "c"}, {Title: "Feel good", Params: "d"}, {Title: "Focus", Params: "e"}, {Title: "Party", Params: "f"}}},
			{Title: "Genres", Moods: []youtube.Mood{{Title: "Ambient", Params: "g"}, {Title: "Electronic", Params: "h"}, {Title: "Hip-hop", Params: "i"}}},
		}}

		tt := ui.NewTester(s.view, 1280, 820)
		renderTo(t, s, tt, dir, theme+"-home-idle")
		s.p.PlayTracks(tracks, 1)
		s.settle(t, func(st player.State) bool { return st.Playing })
		for _, page := range []struct{ name, path string }{
			{"home", "/"}, {"album", "/album/al-1"}, {"artist", "/artist/ar-m83"}, {"now-playing", "/now-playing"},
			{"library-liked", "/library/liked"}, {"search", searchPath("midnight")}, {"explore-moods", "/explore/moods"},
			{"stats", "/stats"}, {"settings-appearance", "/settings/appearance"}, {"settings-playback", "/settings/playback"},
		} {
			s.navigate(page.path)
			// The covers come from the server, and their colour.
			renderTo(t, s, tt, dir, theme+"-"+page.name)
			renderTo(t, s, tt, dir, theme+"-"+page.name)
		}
		s.navigate("/")
		renderTo(t, s, ui.NewTester(s.view, 1280, 1500), dir, theme+"-home-tall")
		s.paletteOpen = true
		renderTo(t, s, tt, dir, theme+"-palette")
		s.paletteOpen = false
		s.ask("enable Hello", enableMessage, "enable", func() {})
		renderTo(t, s, tt, dir, theme+"-dialog")
		s.asking = nil
		renderTo(t, s, ui.NewTester(s.miniView, 420, 132), dir, theme+"-mini")
		renderTo(t, s, ui.NewTester(s.view, 960, 640), dir, theme+"-narrow")
	}
}
