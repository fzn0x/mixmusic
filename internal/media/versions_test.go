package media

import (
	"slices"
	"testing"
)

func TestVersionOf(t *testing.T) {
	for _, c := range []struct {
		title, base, edition, rating string
		clean, explicit              bool
	}{
		{"Abbey Road (Super Deluxe Edition)", "abbey road", "Super Deluxe Edition", "", false, false},
		{"Let It Be (Remastered 2009)", "let it be", "Remastered 2009", "", false, false},
		{"Nevermind - 2011 Remaster", "nevermind", "2011 Remaster", "", false, false},
		{"good kid, m.A.A.d city (Deluxe)", "good kid, m.a.a.d city", "Deluxe", "", false, false},
		{"Recovery (Clean)", "recovery", "", "Clean", true, false},
		{"Recovery [Explicit]", "recovery", "", "Explicit", false, true},
		{"Recovery (Explicit Version)", "recovery", "", "Explicit", false, true},
		{"Recovery (Edited Version)", "recovery", "", "Edited", true, false},
		{"Recovery (Deluxe Edition) [Clean]", "recovery", "Deluxe Edition", "Clean", true, false},
		{"The Dark Side of the Moon (50th Anniversary) [2023 Remaster]", "the dark side of the moon", "50th Anniversary · 2023 Remaster", "", false, false},
		{"Hybrid Theory (Bonus Track Version)", "hybrid theory", "Bonus Track Version", "", false, false},
		{"Rumours (Standard Edition)", "rumours", "", "", false, false},
		{"Pet Sounds (Mono)", "pet sounds", "Mono", "", false, false},
		{"Kites (Collector’s Edition)", "kites", "Collector’s Edition", "", false, false},
		// Not versions' marks: albums of their own.
		{"Fearless (Taylor's Version)", "fearless (taylor's version)", "", "", false, false},
		{"MTV Unplugged (Live)", "mtv unplugged (live)", "", "", false, false},
		{"Amigo in blood (short version)", "amigo in blood (short version)", "", "", false, false},
		{"Volume 2 (Remixes)", "volume 2 (remixes)", "", "", false, false},
		{"Hits - Volume 2", "hits - volume 2", "", "", false, false},
		{"Revolver (2022 Mix)", "revolver (2022 mix)", "", "", false, false},
		{"恭喜发财（daft punk）", "恭喜发财（daft punk）", "", "", false, false},
		{"(Deluxe)", "(deluxe)", "", "", false, false},
		{"Kites  (Deluxe)", "kites", "Deluxe", "", false, false},
		// After a colon, and with words of their own beside a word only
		// an edition's name has (#459).
		{"Harbour Lights: Deluxe Edition", "harbour lights", "Deluxe Edition", "", false, false},
		{"Harbour Lights (Night Tide): Expanded Lantern Edition", "harbour lights (night tide)", "Expanded Lantern Edition", "", false, false},
		{"Harbour Lights (Lantern Keeper's Edition) [Clean]", "harbour lights", "Lantern Keeper's Edition", "Clean", true, false},
		{"Night Drive - Side B: 10th Anniversary", "night drive - side b", "10th Anniversary", "", false, false},
		{"Harbour Lights：デラックス", "harbour lights：デラックス", "", "", false, false},
		{"Episode Four: A New Dawn", "episode four: a new dawn", "", "", false, false},
		{"Paper Planets: Songs from the Ferry", "paper planets: songs from the ferry", "", "", false, false},
		{"Paper Planets (Lantern's Version)", "paper planets (lantern's version)", "", "", false, false},
		{"Paper Planets: Live", "paper planets: live", "", "", false, false},
	} {
		v := VersionOf(c.title)
		if v.Base != c.base || v.Edition != c.edition || v.Rating != c.rating || v.Clean != c.clean || v.Explicit != c.explicit {
			t.Errorf("%q: %+v", c.title, v)
		}
	}
	// Editions match by their words, not their spelling.
	if VersionOf("Kites (Deluxe)").edition != VersionOf("Kites [Deluxe Edition]").edition {
		t.Error("Deluxe and Deluxe Edition are other editions")
	}
	if VersionOf("Kites (Remastered 2011)").edition != VersionOf("Kites - 2011 Remaster").edition {
		t.Error("Remastered 2011 and 2011 Remaster are other editions")
	}
	if VersionOf("Kites (Super Deluxe)").edition == VersionOf("Kites (Deluxe)").edition {
		t.Error("Super Deluxe and Deluxe are one edition")
	}
}

// album is an album of the test's artist, Ada Field, on source.
func album(source Source, id, title, year string, tracks int, explicit bool) Album {
	return Album{ID: id, Source: source, Title: title, Year: year, TrackCount: tracks, Explicit: explicit,
		Artists: []Ref{{ID: "ar-ada", Name: "Ada Field"}}, Kind: AlbumKindAlbum}
}

func ids(groups [][]Album) [][]string {
	out := make([][]string, len(groups))
	for i, g := range groups {
		for _, a := range g {
			out[i] = append(out[i], a.ID)
		}
	}
	return out
}

func TestGroupVersions(t *testing.T) {
	sp, yt := SourceSpotify, SourceYouTube
	other := album(sp, "other", "Kites", "2019", 12, false)
	other.Artists = []Ref{{Name: "Harbour Lights"}}
	single := album(sp, "single", "Kites", "2019", 1, false)
	single.Kind = AlbumKindSingle
	file := album(SourceFile, "file", "Kites", "2019", 12, false)
	for _, c := range []struct {
		name   string
		albums []Album
		want   [][]string
	}{
		{"an album explicit and clean, and its deluxe edition, on Spotify", []Album{
			album(sp, "deluxe", "Kites (Deluxe)", "2020", 16, false),
			album(sp, "e", "Kites", "2019", 12, false),
			album(sp, "tide", "Tidewater", "2016", 10, false),
			album(sp, "c", "Kites", "2019", 12, false),
		}, [][]string{{"e", "c", "deluxe"}, {"tide"}}},
		{"the same, badged on YouTube Music, and marked", []Album{
			album(yt, "e", "Kites", "2019", 0, true),
			album(yt, "c", "Kites (Clean)", "2019", 0, false),
			album(yt, "remaster", "Kites (Remastered 2024)", "2024", 0, false),
		}, [][]string{{"e", "c", "remaster"}}},
		{"a clean version of fewer tracks is another album", []Album{
			album(sp, "e", "Kites", "2019", 12, false),
			album(sp, "c", "Kites", "2019", 11, false),
		}, [][]string{{"e"}, {"c"}}},
		{"another artist's, a single's, a file's and a service's other album of the title", []Album{
			album(sp, "e", "Kites", "2019", 12, false), other, single, file, album(yt, "yt", "Kites", "2019", 0, false),
		}, [][]string{{"e"}, {"other"}, {"single"}, {"file"}, {"yt"}}},
		{"a self-titled album and a live one of its title from another year", []Album{
			album(sp, "first", "Ada Field", "2010", 12, false),
			album(sp, "live", "Ada Field", "2014", 15, false),
			album(sp, "first-deluxe", "Ada Field (Deluxe Edition)", "2010", 18, false),
			album(sp, "live-explicit", "Ada Field", "2014", 15, false),
			album(sp, "anniversary", "Ada Field (10th Anniversary Edition)", "2020", 20, false),
		}, [][]string{{"first", "first-deluxe"}, {"live", "live-explicit"}, {"anniversary"}}},
		{"the same on YouTube Music, which tells no numbers of tracks", []Album{
			album(yt, "first", "Ada Field", "2010", 0, false),
			album(yt, "live", "Ada Field", "2014", 0, false),
		}, [][]string{{"first"}, {"live"}}},
		{"two editions without the album as first put out", []Album{
			album(sp, "remaster", "Kites (Remastered)", "2009", 12, false),
			album(sp, "super", "Kites (Super Deluxe Edition)", "2019", 40, false),
		}, [][]string{{"remaster", "super"}}},
		{"an edition named after a colon, with words of its own", []Album{
			album(yt, "first", "Kites (Night Tide)", "2024", 0, false),
			album(yt, "expanded", "Kites (Night Tide): Expanded Lantern Edition", "2025", 0, false),
			album(yt, "dawn", "Kites (Night Tide): A New Dawn", "2025", 0, false),
		}, [][]string{{"first", "expanded"}, {"dawn"}}},
	} {
		if got := ids(GroupVersions(c.albums)); !slices.EqualFunc(got, c.want, slices.Equal) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if got := GroupVersions(nil); len(got) != 0 {
		t.Errorf("no albums made %v", got)
	}
}

// tracksOf are tracks, explicit or not.
func tracksOf(explicit ...bool) []Track {
	var out []Track
	for _, e := range explicit {
		out = append(out, Track{Explicit: e})
	}
	return out
}

func TestPreferredVersion(t *testing.T) {
	sp, yt := SourceSpotify, SourceYouTube
	loadedE := album(sp, "e", "Kites", "2019", 12, false)
	loadedE.Tracks = tracksOf(false, true, false)
	loadedC := album(sp, "c", "Kites", "2019", 12, false)
	loadedC.Tracks = tracksOf(false, false, false)
	for _, c := range []struct {
		name      string
		versions  []Album
		explicit  int
		clean     int
		toldWants bool
	}{
		{"one", []Album{album(sp, "a", "Kites", "2019", 12, false)}, 0, 0, true},
		{"badged", []Album{album(yt, "c", "Kites", "2019", 0, false), album(yt, "e", "Kites", "2019", 0, true)}, 1, 0, true},
		{"marked clean", []Album{album(sp, "c", "Kites (Clean)", "2019", 12, false), album(sp, "e", "Kites", "2019", 12, false)}, 1, 0, true},
		{"marked explicit", []Album{album(sp, "e", "Kites [Explicit]", "2019", 12, false), album(sp, "c", "Kites", "2019", 12, false)}, 0, 1, true},
		{"by their tracks", []Album{loadedC, loadedE}, 1, 0, true},
		{"an edition each", []Album{album(sp, "a", "Kites", "2019", 12, false), album(sp, "d", "Kites (Deluxe)", "2020", 16, false)}, 0, 0, true},
		{"of the first one's edition", []Album{
			album(yt, "c", "Kites", "2019", 0, false), album(yt, "d", "Kites (Deluxe)", "2020", 0, true), album(yt, "e", "Kites", "2019", 0, true),
		}, 2, 0, true},
		{"untold", []Album{album(sp, "x", "Kites", "2019", 12, false), album(sp, "y", "Kites", "2019", 12, false)}, 0, 0, false},
	} {
		if i, told := PreferredVersion(c.versions, false); i != c.explicit || told != c.toldWants {
			t.Errorf("%s: opens on %d (told %v), want %d (%v)", c.name, i, told, c.explicit, c.toldWants)
		}
		if i, told := PreferredVersion(c.versions, true); i != c.clean || told != c.toldWants {
			t.Errorf("%s, preferring clean versions: opens on %d (told %v), want %d (%v)", c.name, i, told, c.clean, c.toldWants)
		}
		if told := VersionsTold(c.versions); told != c.toldWants {
			t.Errorf("%s: told %v", c.name, told)
		}
	}
}

func TestVersionLabels(t *testing.T) {
	sp, yt := SourceSpotify, SourceYouTube
	loadedE := album(sp, "e", "Kites", "2019", 12, false)
	loadedE.Tracks = tracksOf(true)
	loadedC := album(sp, "c", "Kites", "2019", 12, false)
	loadedC.Tracks = tracksOf(false)
	for _, c := range []struct {
		name     string
		versions []Album
		want     []string
	}{
		{"badged, with an edition", []Album{
			album(yt, "e", "Kites", "2019", 0, true), album(yt, "c", "Kites", "2019", 0, false), album(yt, "d", "Kites (Deluxe Edition)", "2020", 0, true),
		}, []string{"Explicit", "Clean", "Deluxe Edition"}},
		{"by their tracks", []Album{loadedE, loadedC}, []string{"Explicit", "Clean"}},
		{"as their titles mark them", []Album{album(sp, "c", "Kites (Edited)", "2019", 12, false), album(sp, "e", "Kites", "2019", 12, false)}, []string{"Edited", "Explicit"}},
		{"editions", []Album{album(sp, "a", "Kites", "1969", 17, false), album(sp, "s", "Kites (Super Deluxe Edition)", "2019", 40, false)}, []string{"Standard", "Super Deluxe Edition"}},
		{"editions only", []Album{album(sp, "r", "Kites (Remastered)", "2009", 17, false), album(sp, "s", "Kites (Super Deluxe Edition)", "2019", 40, false)}, []string{"Remastered", "Super Deluxe Edition"}},
		{"an edition explicit and clean", []Album{
			album(yt, "de", "Kites (Deluxe)", "2020", 0, true), album(yt, "dc", "Kites (Deluxe)", "2020", 0, false), album(yt, "a", "Kites", "2019", 0, true),
		}, []string{"Deluxe · Explicit", "Deluxe · Clean", "Standard"}},
		{"alike but for their years", []Album{album(sp, "x", "Kites", "2019", 0, false), album(sp, "y", "Kites", "", 0, false), album(sp, "z", "Kites", "2020", 0, false)},
			[]string{"Standard · 1", "Standard · 2", "Standard · 3"}},
		{"alike but for their numbers of tracks", []Album{album(yt, "x", "Kites", "2019", 12, false), album(yt, "y", "Kites", "2019", 14, false)},
			[]string{"Standard · 12 tracks", "Standard · 14 tracks"}},
		{"alike", []Album{album(sp, "x", "Kites", "2019", 12, false), album(sp, "y", "Kites", "2019", 12, false)}, []string{"Standard · 1", "Standard · 2"}},
	} {
		if got := VersionLabels(c.versions); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	years := VersionLabels([]Album{album(sp, "x", "Kites", "2019", 0, false), album(sp, "y", "Kites", "2020", 0, false)})
	if !slices.Equal(years, []string{"Standard · 2019", "Standard · 2020"}) {
		t.Errorf("alike but for their years: %q", years)
	}
}
