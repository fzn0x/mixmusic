package youtube

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLiveAlbumVersions reads the albums of an artist known for explicit
// and clean versions of them, on their page and in a search of albums,
// and logs them as MixMusic groups them, which artist-versions.json and
// search-album-versions.json, hand-written, stand for. MIXMUSIC_YT_ARTIST
// names the artist to search for, Eminem at first; MIXMUSIC_YT_RECORD
// names a folder to keep the answers in, for testdata. It runs with
// MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_RECORD=dir go test ./internal/youtube -run TestLiveAlbumVersions -v
func TestLiveAlbumVersions(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to read an artist's albums on YouTube Music")
	}
	name := os.Getenv("MIXMUSIC_YT_ARTIST")
	if name == "" {
		name = "Eminem"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	record := func(file string, n node) {
		dir := os.Getenv("MIXMUSIC_YT_RECORD")
		if dir == "" {
			return
		}
		data, err := json.MarshalIndent(n.v, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	logGroups := func(what string, albums []media.Album) (explicit int) {
		for _, g := range media.GroupVersions(albums) {
			labels := media.VersionLabels(g)
			for i, a := range g {
				if a.Explicit {
					explicit++
				}
				t.Logf("%s: %s (%s, %s), explicit %v: %q", what, a.Title, a.Year, a.ID, a.Explicit, labels[i])
			}
			if len(g) > 1 {
				t.Logf("%s: %d versions of %s", what, len(g), g[0].Title)
			}
		}
		return explicit
	}

	found, err := c.Search(ctx, name, FilterArtists)
	if err != nil || len(found) == 0 || found[0].Artist == nil {
		t.Fatalf("no artist found for %s: %v", name, err)
	}
	_, channel, _ := media.ParseID(found[0].Artist.ID)
	page, err := c.browse(ctx, channel)
	if err != nil {
		t.Fatal(err)
	}
	record("artist-versions", page)
	a, err := parseArtist(page, channel)
	if err != nil {
		t.Fatal(err)
	}
	explicit := logGroups("albums", a.Albums) + logGroups("singles", a.Singles)

	n, err := c.call(ctx, "search", map[string]any{"query": name, "params": filterParams[FilterAlbums]}, nil)
	if err != nil {
		t.Fatal(err)
	}
	record("search-album-versions", n)
	var albums []media.Album
	for _, it := range parseSearch(n) {
		if it.Album != nil {
			albums = append(albums, *it.Album)
		}
	}
	explicit += logGroups("search", albums)
	if explicit == 0 {
		t.Errorf("no album of %s is marked explicit: has the badge moved?", name)
	}
}
