package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestHomeShelfKeys names shelves by their service's name for them, where
// it gives one, else by their titles, and tells apart two of one name by
// their places.
func TestHomeShelfKeys(t *testing.T) {
	got := shelfKeys([]media.Shelf{
		{Title: "Listen again", Source: media.SourceYouTube, Key: "FEmusic_listen_again"},
		{Title: "Quick picks", Source: media.SourceYouTube},
		{Title: "Your top tracks", Source: media.SourceSpotify, Key: "top"},
		{Title: " Quick picks ", Source: media.SourceYouTube},
		{Title: "Recently played", Source: media.SourceLocal, Key: "recent"},
	})
	want := []string{"youtube:FEmusic_listen_again", "youtube:title:Quick picks", "spotify:top", "youtube:title:Quick picks#2", homeRecent}
	if !slices.Equal(got, want) {
		t.Errorf("keys %q, want %q", got, want)
	}
}

// homeTestGroups are the groups of Home of the user's own shelves, of
// keys yours, and YouTube Music's, of keys youtube, which done tells came.
func homeTestGroups(done bool, yours, youtube []string) []*homeGroup {
	own := &homeGroup{key: "home-yours", done: true}
	for _, key := range yours {
		own.shelves = append(own.shelves, homeShelf{key: key, title: key, source: media.SourceLocal})
	}
	yt := &homeGroup{key: "home:youtube", source: media.SourceYouTube, done: done}
	for _, key := range youtube {
		yt.shelves = append(yt.shelves, homeShelf{key: key, title: key, source: media.SourceYouTube})
	}
	return []*homeGroup{own, {key: "home:spotify", source: media.SourceSpotify, done: true}, yt}
}

// placeKeys are the keys of the shelves of places, with "…" where a group
// still loads and "!" where one failed.
func placeKeys(places []homePlace) []string {
	var out []string
	for _, p := range places {
		switch {
		case p.shelf != nil:
			out = append(out, p.shelf.key)
		case !p.group.done:
			out = append(out, "…")
		default:
			out = append(out, "!"+p.group.key)
		}
	}
	return out
}

// layoutOf is the arrangement of the keys given, those starting with "-"
// hidden.
func layoutOf(keys ...string) []settings.HomeShelf {
	var out []settings.HomeShelf
	for _, k := range keys {
		hidden := k[0] == '-'
		if hidden {
			k = k[1:]
		}
		out = append(out, settings.HomeShelf{Key: k, Title: k, Hidden: hidden})
	}
	return out
}

// TestHomePlaces shows the shelves the user placed first, where they
// placed them, without those they hid, then the others of each group in
// their own order: a service's new shelves come after the placed ones,
// and a shelf placed that the service no longer has leaves no gap.
func TestHomePlaces(t *testing.T) {
	yt := func(keys ...string) []string {
		for i := range keys {
			keys[i] = "youtube:" + keys[i]
		}
		return keys
	}
	groups := homeTestGroups(true, []string{homeRecent, homeMixes}, yt("a", "b", "c"))
	if got, want := placeKeys(homePlaces(nil, groups, "")), append([]string{homeRecent, homeMixes}, yt("a", "b", "c")...); !slices.Equal(got, want) {
		t.Errorf("as it comes: %q, want %q", got, want)
	}

	layout := layoutOf("youtube:c", homeRecent, "-youtube:a", homeMixes, "youtube:b")
	if got, want := placeKeys(homePlaces(layout, groups, "")), []string{"youtube:c", homeRecent, homeMixes, "youtube:b"}; !slices.Equal(got, want) {
		t.Errorf("arranged: %q, want %q", got, want)
	}
	// Another day, the service has new shelves, before and after, and no
	// longer one placed.
	groups = homeTestGroups(true, []string{homeRecent, homeMixes}, yt("new", "a", "c", "d"))
	if got, want := placeKeys(homePlaces(layout, groups, "")), []string{"youtube:c", homeRecent, homeMixes, "youtube:new", "youtube:d"}; !slices.Equal(got, want) {
		t.Errorf("with new shelves: %q, want %q", got, want)
	}
	// The pivots keep their own, the user's under Yours.
	if got, want := placeKeys(homePlaces(layout, groups, media.SourceLocal)), []string{homeRecent, homeMixes}; !slices.Equal(got, want) {
		t.Errorf("yours: %q, want %q", got, want)
	}
	if got, want := placeKeys(homePlaces(layout, groups, media.SourceYouTube)), []string{"youtube:c", "youtube:new", "youtube:d"}; !slices.Equal(got, want) {
		t.Errorf("YouTube Music's: %q, want %q", got, want)
	}
}

// TestHomePlacesWait holds back what comes after a shelf placed of a
// service that still loads, which would move once it came, and tells of
// a service that failed once, where its first shelf goes.
func TestHomePlacesWait(t *testing.T) {
	layout := layoutOf(homeRecent, "youtube:a", homeMixes)
	groups := homeTestGroups(false, []string{homeRecent, homeMixes}, nil)
	if got, want := placeKeys(homePlaces(layout, groups, "")), []string{homeRecent, "…"}; !slices.Equal(got, want) {
		t.Errorf("loading: %q, want %q", got, want)
	}
	if got, want := placeKeys(homePlaces(layout, groups, media.SourceLocal)), []string{homeRecent, homeMixes}; !slices.Equal(got, want) {
		t.Errorf("yours, loading: %q, want %q", got, want)
	}
	groups[2].done, groups[2].err = true, errors.New("offline")
	layout = layoutOf(homeRecent, "youtube:a", homeMixes, "youtube:b")
	if got, want := placeKeys(homePlaces(layout, groups, "")), []string{homeRecent, "!home:youtube", homeMixes}; !slices.Equal(got, want) {
		t.Errorf("failed: %q, want %q", got, want)
	}
}

// TestHomeArranged starts an arrangement from every shelf Home shows, in
// its place, with the titles shown now: a shelf placed that its service's
// answer no longer has goes, unless hidden, and one of a service that
// hasn't answered stays.
func TestHomeArranged(t *testing.T) {
	layout := []settings.HomeShelf{
		{Key: "youtube:b", Title: "B of before"},
		{Key: "youtube:gone", Title: "Gone"},
		{Key: "youtube:hid", Title: "Hidden", Hidden: true},
		{Key: "spotify:top", Title: "Top"},
	}
	rows := []homeRow{{homeRecent, "Recently played"}, {"youtube:a", "A"}, {"youtube:b", "B"}}
	gone := func(key string) bool { return keySource(key) == media.SourceYouTube }
	got := arranged(layout, rows, gone)
	want := []settings.HomeShelf{
		{Key: "youtube:b", Title: "B"},
		{Key: "youtube:hid", Title: "Hidden", Hidden: true},
		{Key: "spotify:top", Title: "Top"},
		{Key: homeRecent, Title: "Recently played"},
		{Key: "youtube:a", Title: "A"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("arranged as %+v,\nwant %+v", got, want)
	}
}

// TestHomeMoves moves a shelf onto another, down and up, hides it and
// shows it where it was, and adds one first.
func TestHomeMoves(t *testing.T) {
	keys := func(l []settings.HomeShelf) []string {
		var out []string
		for _, e := range l {
			k := e.Key
			if e.Hidden {
				k = "-" + k
			}
			out = append(out, k)
		}
		return out
	}
	l := layoutOf("a", "b", "c", "d")
	for _, c := range []struct {
		got  []settings.HomeShelf
		want []string
	}{
		{movedOnto(l, "a", "c"), []string{"b", "c", "a", "d"}},
		{movedOnto(l, "d", "b"), []string{"a", "d", "b", "c"}},
		{movedOnto(l, "b", "a"), []string{"b", "a", "c", "d"}},
		{movedOnto(l, "b", "nowhere"), []string{"a", "b", "c", "d"}},
		{withHidden(l, "b", true), []string{"a", "-b", "c", "d"}},
		{withHidden(withHidden(l, "b", true), "b", false), []string{"a", "b", "c", "d"}},
		{withoutShelf(l, "c"), []string{"a", "b", "d"}},
		{withAdded(l, "playlist:mx:1", "Night drive"), []string{"playlist:mx:1", "a", "b", "c", "d"}},
		{withAdded(withHidden(l, "c", true), "c", "C"), []string{"c", "a", "b", "d"}},
	} {
		if got := keys(c.got); !slices.Equal(got, c.want) {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
	if got := keys(l); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("the moves changed the arrangement they started from: %q", got)
	}
}
