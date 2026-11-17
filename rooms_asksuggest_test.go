package main

import (
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/locales"
)

// askSuggestion is what the room's page suggests as friends outside this
// network may join.
const askSuggestion = "Turn on Ask me before someone joins, and whoever comes with the link or the code waits until you let them in."

// TestRoomOutsideSuggestsAskingFirst turns on the room page's switch for
// friends outside this network: the page suggests Ask me before someone
// joins, which turns it on, and keeps it, as the room's rules read it.
func TestRoomOutsideSuggestsAskingFirst(t *testing.T) {
	s, _, _ := newFarRoomShell(t, roomtest.New(), "Desk", "203.0.113.7")
	tt := tester(t, s)
	if err := s.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(s, tt, roomsPath)
	until(t, s, tt, "the switch", func() bool { return tt.HasText("Let friends outside this network join") })
	if tt.HasText(askSuggestion) {
		t.Fatal("suggested while friends outside may not join")
	}
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	until(t, s, tt, "the suggestion", func() bool { return tt.HasText(askSuggestion) && tt.HasText("turn it on") })
	click(t, s, tt, "Turn on Ask me before someone joins")
	if tt.HasText(askSuggestion) {
		t.Fatal("still suggested once taken")
	}
	if !s.set.RoomAskJoin || !s.svc.Preferences.Get().RoomAskJoin {
		t.Fatalf("Ask me before someone joins: shown %v, kept %v", s.set.RoomAskJoin, s.svc.Preferences.Get().RoomAskJoin)
	}
	// Off and on again, with the rule on, it suggests nothing.
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	until(t, s, tt, "the switch on again", func() bool { return tt.HasText("copy web link") })
	if tt.HasText(askSuggestion) {
		t.Fatal("suggested while the rule is on")
	}
}

// TestRoomOutsideSuggestionNotNow turns the suggestion down: it goes, the
// rule stays off, and turning the switch on again suggests it again.
func TestRoomOutsideSuggestionNotNow(t *testing.T) {
	s, _, _ := newFarRoomShell(t, roomtest.New(), "Desk", "203.0.113.7")
	tt := tester(t, s)
	if err := s.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(s, tt, roomsPath)
	until(t, s, tt, "the switch", func() bool { return tt.HasText("Let friends outside this network join") })
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	until(t, s, tt, "the suggestion", func() bool { return tt.HasText(askSuggestion) })
	click(t, s, tt, "not now")
	if tt.HasText(askSuggestion) {
		t.Fatal("still suggested once turned down")
	}
	if s.set.RoomAskJoin || s.svc.Preferences.Get().RoomAskJoin {
		t.Fatal("Ask me before someone joins turned on, though turned down")
	}
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	until(t, s, tt, "the switch off", func() bool { return !tt.HasText("copy web link") })
	toggle(t, s, tt, "Let friends outside this network join", anywhereAbout)
	until(t, s, tt, "the suggestion again", func() bool { return tt.HasText(askSuggestion) })
}

// TestRoomOutsideSuggestionTranslated shows the suggestion in the pseudo
// locale, en-XA, which brackets every message of the catalogs: its words
// come from them, not the code.
func TestRoomOutsideSuggestionTranslated(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	xa := locales.In("en-XA")
	s, _, _ := newFarRoomShell(t, roomtest.New(), "Desk", "203.0.113.7")
	tt := tester(t, s)
	tt.SetLocale("en-XA")
	if err := s.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(s, tt, roomsPath)
	until(t, s, tt, "the switch", func() bool { return tt.HasText(xa.RoomReachTitle()) })
	toggle(t, s, tt, xa.RoomReachTitle(), xa.RoomReachDesc())
	until(t, s, tt, "the suggestion", func() bool { return tt.HasText(xa.RoomReachSuggestAsk()) })
	if !tt.HasText(xa.RoomReachSuggestAskYes()) || !tt.HasText(xa.RoomReachSuggestAskNo()) || tt.HasText(askSuggestion) {
		t.Errorf("the suggestion shows %q", tt.Texts())
	}
	click(t, s, tt, xa.RoomReachSuggestAskLabel())
	if !s.set.RoomAskJoin {
		t.Error("its button, by the label of the catalog, didn't turn on Ask me before someone joins")
	}
}
