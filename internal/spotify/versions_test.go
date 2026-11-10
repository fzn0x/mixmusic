package spotify

import (
	"context"
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// versionsAlbum is a Spotify album of the Web API's reference shape, of
// the test's artist; tracks are the explicit marks of its tracks, for an
// album's page, or none, for a list.
func versionsAlbum(id, name, date string, total int, tracks ...bool) string {
	s := `{"id": "` + id + `", "name": "` + name + `", "album_type": "album", "release_date": "` + date + `", "total_tracks": ` +
		strconv.Itoa(total) + `, "images": [], "artists": [{"id": "a1", "name": "Ada Field"}]`
	if tracks != nil {
		s += `, "tracks": {"items": [`
		for i, e := range tracks {
			if i > 0 {
				s += ", "
			}
			explicit := "false"
			if e {
				explicit = "true"
			}
			s += `{"id": "` + id + `-t` + strconv.Itoa(i) + `", "name": "Track", "duration_ms": 1000, "explicit": ` + explicit +
				`, "artists": [{"id": "a1", "name": "Ada Field"}]}`
		}
		s += `]}`
	}
	return s + "}"
}

// TestAlbumVersions groups an artist's albums that Spotify lists twice,
// explicit and clean, alike but for their IDs, with the deluxe edition:
// their list doesn't tell which is explicit, and their pages do, by their
// tracks. The answers are hand-written from the Web API's reference, not
// recorded.
func TestAlbumVersions(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /artists/a1": `{"id": "a1", "name": "Ada Field", "images": []}`,
		"GET /artists/a1/albums": `{"total": 4, "items": [` +
			versionsAlbum("clean", "Kites", "2019-05-03", 3) + `, ` +
			versionsAlbum("explicit", "Kites", "2019-05-03", 3) + `, ` +
			versionsAlbum("deluxe", "Kites (Deluxe)", "2020-01-10", 5) + `, ` +
			versionsAlbum("tide", "Tidewater", "2016-02-01", 10) + `]}`,
		"GET /albums/clean":    versionsAlbum("clean", "Kites", "2019-05-03", 3, false, false, false),
		"GET /albums/explicit": versionsAlbum("explicit", "Kites", "2019-05-03", 3, false, true, false),
		"GET /search":          `{"tracks": {"items": []}}`,
	})
	a, err := c.Artist(context.Background(), "a1")
	if err != nil {
		t.Fatal(err)
	}
	groups := media.GroupVersions(a.Albums)
	if len(groups) != 2 || len(groups[0]) != 3 || groups[0][0].ID != "sp:clean" || groups[0][2].ID != "sp:deluxe" {
		t.Fatalf("groups %+v", groups)
	}
	kites := groups[0]
	if media.VersionsTold(kites) {
		t.Error("Spotify's list tells which version is explicit")
	}
	for i, id := range []string{"clean", "explicit"} {
		if kites[i], err = c.Album(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if kites[0].Explicit || !kites[1].Explicit {
		t.Errorf("explicit: clean %v, explicit %v", kites[0].Explicit, kites[1].Explicit)
	}
	if i, told := media.PreferredVersion(kites, false); i != 1 || !told {
		t.Errorf("opens on %d (told %v), not the explicit one", i, told)
	}
	if labels := media.VersionLabels(kites); labels[0] != "Clean" || labels[1] != "Explicit" || labels[2] != "Deluxe" {
		t.Errorf("labels %q", labels)
	}
}
