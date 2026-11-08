package settings

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestHomeShelvesRoundTrip writes the order of Home's shelves to
// settings.json and reads it back as it was: their keys in order, their
// titles and which are hidden. A file without the key, as before there was
// one, leaves Home as it comes.
func TestHomeShelvesRoundTrip(t *testing.T) {
	s := Default()
	s.HomeShelves = []HomeShelf{
		{Key: "playlist:mx:night-drive", Title: "Night drive"},
		{Key: "youtube:FEmusic_listen_again", Title: "Listen again"},
		{Key: "local:recent", Title: "Recently played", Hidden: true},
		{Key: "youtube:title:Quick picks", Title: "Quick picks"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	read := Default()
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if read = read.Clean(); !slices.Equal(read.HomeShelves, s.HomeShelves) {
		t.Errorf("read back as %+v, not %+v", read.HomeShelves, s.HomeShelves)
	}

	before := Default()
	if err := json.Unmarshal([]byte(`{"skippedYtdlp": "2026.10.22"}`), &before); err != nil {
		t.Fatal(err)
	}
	if got := before.Clean().HomeShelves; got == nil || len(got) != 0 {
		t.Errorf("a file of before reads as %#v", got)
	}
	if data, _ := json.Marshal(Default()); !strings.Contains(string(data), `"homeShelves":[]`) {
		t.Errorf("a new install writes %s", data)
	}
}

// TestHomeShelvesClean keeps each shelf once, by a key of its own, and
// bounds how many there are and how long their titles run.
func TestHomeShelvesClean(t *testing.T) {
	long := strings.Repeat("é", maxHomeTitle) // two bytes each
	var h Home
	h.HomeShelves = []HomeShelf{
		{Key: "local:recent", Title: "Recently played"},
		{Key: "", Title: "No key"},
		{Key: "local:recent", Title: "Twice", Hidden: true},
		{Key: strings.Repeat("k", maxHomeKey+1)},
		{Key: "local:mixes", Title: long},
	}
	got := h.clean().HomeShelves
	if len(got) != 2 || got[0].Key != "local:recent" || got[0].Hidden || got[1].Key != "local:mixes" {
		t.Fatalf("cleaned to %+v", got)
	}
	if title := got[1].Title; len(title) > maxHomeTitle || !utf8.ValidString(title) || !strings.HasPrefix(long, title) {
		t.Errorf("a long title cut to %d bytes, valid %v", len(title), utf8.ValidString(title))
	}
	if again := (Home{HomeShelves: got}).clean().HomeShelves; !slices.Equal(again, got) {
		t.Errorf("cleaned again to %+v", again)
	}

	h.HomeShelves = nil
	for i := range maxHomeShelves + 10 {
		h.HomeShelves = append(h.HomeShelves, HomeShelf{Key: "youtube:title:Shelf " + strings.Repeat("x", i)})
	}
	if got := h.clean().HomeShelves; len(got) != maxHomeShelves {
		t.Errorf("kept %d shelves of %d", len(got), maxHomeShelves+10)
	}
}
