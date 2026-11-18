package settings

import (
	"fmt"
	"testing"
)

// TestLibraryClean puts the default sorts in place of those unknown, with
// their ways, and keeps each tab and place once, within their bounds,
// those of a later version too.
func TestLibraryClean(t *testing.T) {
	d := defaultLibrary()
	l := Library{MixesSort: "colour", MixesDescending: true, PlaylistsSort: "plays", PlaylistsDescending: true}
	for i := range maxLibraryTabs + 5 {
		l.LibraryTabs = append(l.LibraryTabs, Arranged{Key: fmt.Sprintf("tab%d", i)})
	}
	l.LibraryTabs = append([]Arranged{{Key: "tab3", Hidden: true}}, l.LibraryTabs...)
	l.SidebarPlaces = []SidebarPlace{{Key: PlaceLiked, After: "mix:mx:a"}, {Key: PlaceLiked}, {Key: ""}, {Key: "later", Hidden: true}}
	got := l.clean(d)
	if got.MixesSort != MixesByOrder || got.MixesDescending || got.PlaylistsSort != PlaylistsByService || got.PlaylistsDescending {
		t.Errorf("unknown sorts clean to %q %v and %q %v", got.MixesSort, got.MixesDescending, got.PlaylistsSort, got.PlaylistsDescending)
	}
	if len(got.LibraryTabs) != maxLibraryTabs || got.LibraryTabs[0].Key != "tab3" || !got.LibraryTabs[0].Hidden {
		t.Errorf("kept %d tabs, the first %+v", len(got.LibraryTabs), got.LibraryTabs[0])
	}
	if p := got.SidebarPlaces; len(p) != 2 || p[0].After != "mix:mx:a" || p[1].Key != "later" {
		t.Errorf("places clean to %+v", p)
	}
	known := Library{MixesSort: MixesByTitle, MixesDescending: true, PlaylistsSort: PlaylistsByTracks}
	if got := known.clean(d); got.MixesSort != MixesByTitle || !got.MixesDescending || got.PlaylistsSort != PlaylistsByTracks {
		t.Errorf("known sorts clean to %+v", got)
	}
}
