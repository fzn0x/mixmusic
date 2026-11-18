package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleLibraryOrder shows Library › Mixes sorted, searched and
// with a pin, and Library › YouTube Music with its playlists' search and
// a playlist hidden, in en-XA: only the names of the mixes and the
// playlists are English.
func TestPseudoLocaleLibraryOrder(t *testing.T) {
	usePseudoLocale(t)
	s := libraryShell(t)
	ids := makeMixes(t, s, "Night drive", "Rainy day")
	if err := s.svc.Collection.SetPinned(library.Mark{ID: ids["Rainy day"], Kind: library.MarkPlaylist, Title: "Rainy day"}, true); err != nil {
		t.Fatal(err)
	}
	s.set.MixesSort, s.set.MixesDescending = settings.MixesByTitle, true
	s.save()
	tt := testerOf(t, s, 1280, 1400)
	tt.SetLocale("en-XA")
	show(s, tt, "/library/mixes")
	checkPseudo(t, "Library › Mixes", tt.Texts(), "Night drive", "Rainy day")
	s.order.mixes = "snow day"
	tt.Frame()
	checkPseudo(t, "Library › Mixes searched", tt.Texts(), "snow day")

	youtubeLibraryOf(s, ownPlaylist, savedPlaylist)
	if err := s.svc.Collection.SetHidden(playlistMark(savedPlaylist), true); err != nil {
		t.Fatal(err)
	}
	show(s, tt, "/library/youtube")
	checkPseudo(t, "Library › YouTube Music", tt.Texts(), "Late night", "Chill")
	if xa := locales.In("en-XA"); !tt.HasText(xa.LibraryHiddenHint("YouTube Music")) || !tt.HasText(xa.LibraryShowAgain()) {
		t.Errorf("Hidden doesn't show in en-XA: %q", tt.Texts())
	}
}

// TestPseudoLocaleSidebarMixes shows the sidebar's Your mixes with a pin
// of YouTube Music and a mix, then searched without a match, in en-XA.
func TestPseudoLocaleSidebarMixes(t *testing.T) {
	usePseudoLocale(t)
	s, tt, _ := sideShell(t, "Night drive", "Night drive", "Rainy day")
	tt.SetLocale("en-XA")
	tt.Frame()
	checkPseudo(t, "Your mixes", tt.Texts(), "Night drive", "Rainy day", "Road trip")
	s.order.side = "snow day"
	tt.Frame()
	if !tt.HasText(locales.In("en-XA").RailNothingMatches("snow day")) {
		t.Errorf("the search without a match says nothing: %q", tt.Texts())
	}
	checkPseudo(t, "Your mixes searched", tt.Texts(), "snow day")
}
