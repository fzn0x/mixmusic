package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lyricsServers are Better Lyrics, Unison and LRCLIB answering a test:
// Better Lyrics has nothing; Unison has synced lyrics of every track, and
// plain ones of "Other Song" by "Other Artist", which LRCLIB fails to
// answer for; LRCLIB has synced lyrics of the others. With none, neither
// has lyrics of any track. The counts count their lookups.
type lyricsServers struct {
	none                atomic.Bool
	unisonAsks, lrcAsks atomic.Int32
}

const (
	pickUnisonSynced = `{"success":true,"data":{"format":"ttml","syncType":"linesync","lyrics":"<tt><body><div><p begin=\"0:01.000\">From Unison</p><p begin=\"0:04.000\">Second line</p></div></body></tt>"}}`
	pickUnisonOther  = `{"success":true,"data":{"format":"plain","syncType":"plain","lyrics":"Plain from Unison\nAnother verse\nA third\nA fourth"}}`
)

// lyricsPickShell is a test shell whose sources of lyrics are
// lyricsServers' Better Lyrics, Unison and LRCLIB, YouTube Music and YouTube
// captions being off, playing testTracks[0], its lyrics in the panel: the
// lyrics follow the picks, as they do in a window. With none, no source
// has lyrics.
func lyricsPickShell(t *testing.T, none bool) (*shell, *ui.Tester, *lyricsServers) {
	t.Helper()
	ls := &lyricsServers{}
	ls.none.Store(none)
	mux := http.NewServeMux()
	mux.HandleFunc("/betterlyrics/getLyrics", http.NotFound)
	mux.HandleFunc("/unison/lyrics", func(w http.ResponseWriter, r *http.Request) {
		ls.unisonAsks.Add(1)
		switch {
		case ls.none.Load():
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"success":false,"code":"NOT_FOUND"}`))
		case r.URL.Query().Get("song") == "Other Song" && r.URL.Query().Get("artist") == "Other Artist":
			w.Write([]byte(pickUnisonOther))
		default:
			w.Write([]byte(pickUnisonSynced))
		}
	})
	lrclib := func(w http.ResponseWriter, r *http.Request) {
		ls.lrcAsks.Add(1)
		switch {
		case ls.none.Load() && r.URL.Path == "/lrclib/search":
			w.Write([]byte("[]"))
		case ls.none.Load():
			http.NotFound(w, r)
		case r.URL.Query().Get("track_name") == "Other Song":
			w.WriteHeader(http.StatusBadGateway)
		case r.URL.Path == "/lrclib/search":
			w.Write([]byte(`[{"id":1,"trackName":"Midnight City","artistName":"M83","syncedLyrics":"[00:01.00]From LRCLIB\n[00:04.00]Again"}]`))
		default:
			w.Write([]byte(`{"syncedLyrics":"[00:01.00]From LRCLIB\n[00:04.00]Again"}`))
		}
	}
	mux.HandleFunc("/lrclib/get", lrclib)
	mux.HandleFunc("/lrclib/get-cached", lrclib)
	mux.HandleFunc("/lrclib/search", lrclib)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := newTestShellOf(t, func(o *app.Options) {
		o.LyricsURLs = &app.LyricsURLs{LRCLIB: srv.URL + "/lrclib", Unison: srv.URL + "/unison", BetterLyrics: srv.URL + "/betterlyrics"}
		set := `{"layout": "tiles", "lyricsSources": [
			{"key": "betterlyrics"}, {"key": "unison"}, {"key": "lrclib"},
			{"key": "lrcred", "off": true}, {"key": "spotify", "off": true}, {"key": "qqmusic", "off": true},
			{"key": "youtube", "off": true}, {"key": "captions", "off": true},
			{"key": "youtubepage", "off": true}
		]}`
		if err := os.WriteFile(filepath.Join(o.DataDir, "settings.json"), []byte(set), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	t.Cleanup(s.followLyricsPicks())
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	tt.Frame()
	return s, tt, ls
}

// openSourceMenu opens the menu of the lyrics' sources by the button that
// names name, and returns its items.
func openSourceMenu(t *testing.T, s *shell, tt *ui.Tester, name string) []string {
	t.Helper()
	click(t, s, tt, locales.LyricsSourceLabel(name))
	items := tt.Menu()
	if items == nil {
		t.Fatalf("the button of %s opens no menu: %q", name, tt.Texts())
	}
	return items
}

// chooseSource chooses the item label of the menu shown.
func chooseSource(t *testing.T, s *shell, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.ChooseMenuItem(label); err != nil {
		t.Fatalf("choosing %q of %q: %v", label, tt.Menu(), err)
	}
	s.drain()
	tt.Frame()
}

// pressButton clicks the button named name, as the window's other
// elements may show the same words.
func pressButton(t *testing.T, s *shell, tt *ui.Tester, name string) {
	t.Helper()
	a, ok := tt.AccessByRole(ui.RoleButton, name)
	if !ok {
		t.Fatalf("no button %q: %q", name, tt.Texts())
	}
	tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	s.drain()
	tt.Frame()
}

// sourceDetails are the items of the menu of the lyrics' sources of
// testTracks[0], each with what its source answered.
func sourceDetails(s *shell) []string {
	var out []string
	for _, it := range lyricsSourceItems(s.svc.Catalog.LyricsChoices(testTracks[0])) {
		out = append(out, strings.TrimSuffix(it.label+": "+it.detail, ": "))
	}
	return out
}

// TestLyricsSourceMenu offers every source that is on, above the lyrics,
// each with what it answered; picks one the lookup didn't ask, which is
// asked then, and one that found lyrics, which show at once; and goes
// back to the lookup's choice with Automatic.
func TestLyricsSourceMenu(t *testing.T) {
	s, tt, ls := lyricsPickShell(t, false)
	waitText(t, s, tt, "From Unison")
	if !tt.HasText("source") {
		t.Errorf("no source above the lyrics: %q", tt.Texts())
	}
	items := openSourceMenu(t, s, tt, "Automatic (Unison)")
	want := []string{"Automatic", "Better Lyrics", "Unison", "LRCLIB", "-", "Search by title and artist…", "Edit lyrics…"}
	if !slices.Equal(items, want) {
		t.Errorf("menu %q, want %q", items, want)
	}
	details := []string{"Automatic", "Better Lyrics: Not found", "Unison: Showing, timed by line, 2 lines", "LRCLIB: Not asked yet"}
	if got := sourceDetails(s); !slices.Equal(got, details) {
		t.Errorf("menu says %q, want %q", got, details)
	}
	chooseSource(t, s, tt, "LRCLIB")
	waitText(t, s, tt, "From LRCLIB")
	if n := ls.lrcAsks.Load(); n != 1 {
		t.Errorf("LRCLIB asked %d times", n)
	}
	if !tt.HasText("lyrics from LRCLIB") {
		t.Errorf("LRCLIB's lyrics aren't credited: %q", tt.Texts())
	}
	asks := ls.unisonAsks.Load()
	openSourceMenu(t, s, tt, "LRCLIB")
	if got := sourceDetails(s); got[2] != "Unison: Found, timed by line, 2 lines" || got[3] != "LRCLIB: Showing, timed by line, 2 lines" {
		t.Errorf("after LRCLIB was picked, the menu says %q", got)
	}
	chooseSource(t, s, tt, "Unison")
	waitText(t, s, tt, "From Unison")
	openSourceMenu(t, s, tt, "Unison")
	chooseSource(t, s, tt, "Automatic")
	waitText(t, s, tt, "From Unison")
	if _, ok := tt.Find(locales.LyricsSourceLabel("Automatic (Unison)")); !ok {
		t.Errorf("the button doesn't say Automatic (Unison): %q", tt.Texts())
	}
	if n := ls.unisonAsks.Load(); n != asks {
		t.Errorf("Unison asked %d more times for lyrics it had found", n-asks)
	}
}

// TestLyricsSourceUnderNone offers the sources under "No lyrics found for
// this track.", whose menu tells that none had any.
func TestLyricsSourceUnderNone(t *testing.T) {
	s, tt, _ := lyricsPickShell(t, true)
	waitText(t, s, tt, "No lyrics found for this track.")
	openSourceMenu(t, s, tt, "Automatic")
	details := []string{"Automatic", "Better Lyrics: Not found", "Unison: Not found", "LRCLIB: Not found"}
	if got := sourceDetails(s); !slices.Equal(got, details) {
		t.Errorf("menu says %q, want %q", got, details)
	}
	tt.CloseMenu()
}

// TestLyricsSearchDialog asks every source for the track's title and
// artist, then for others, listing each source's answer, one that failed
// too, and shows the one picked, which the menu names.
func TestLyricsSearchDialog(t *testing.T) {
	s, tt, _ := lyricsPickShell(t, false)
	waitText(t, s, tt, "From Unison")
	openSourceMenu(t, s, tt, "Automatic (Unison)")
	chooseSource(t, s, tt, "Search by title and artist…")
	v := &s.lyricsView.pick.search
	if !v.open || v.title != "Midnight City" || v.artist != "M83" {
		t.Fatalf("the search asks for %q by %q", v.title, v.artist)
	}
	until(t, s, tt, "the first answers", func() bool { return !v.searching && len(v.found) == 3 })
	v.title, v.artist = "Other Song", "Other Artist"
	pressButton(t, s, tt, "search")
	until(t, s, tt, "the answers", func() bool { return !v.searching && tt.HasText("Plain from Unison") })
	for _, want := range []string{"Better Lyrics", "lyrics from Unison", "Found, not timed, 4 lines", "Another verse", "A third",
		"LRCLIB", "Can't be reached", "Not found"} {
		if !tt.HasText(want) {
			t.Errorf("the answers don't show %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("A fourth") {
		t.Error("the answer shows more than its first lines")
	}
	pressButton(t, s, tt, "Show the lyrics from Unison")
	waitText(t, s, tt, "A fourth")
	if v.open {
		t.Error("the dialog stays open")
	}
	items := openSourceMenu(t, s, tt, "Unison")
	if !slices.Contains(items, "Unison: Other Song by Other Artist") {
		t.Errorf("the menu doesn't name the search picked: %q", items)
	}
	tt.CloseMenu()
}

// TestLyricsEditDialog opens the lyrics shown as LRC, keeps a word
// corrected, with their credit, and says so of lyrics of no words.
func TestLyricsEditDialog(t *testing.T) {
	s, tt, _ := lyricsPickShell(t, false)
	waitText(t, s, tt, "From Unison")
	openSourceMenu(t, s, tt, "Automatic (Unison)")
	chooseSource(t, s, tt, "Edit lyrics…")
	e := &s.lyricsView.pick.edit
	if !e.open || e.text != "[00:01.00]From Unison\n[00:04.00]Second line\n" {
		t.Fatalf("editing %q", e.text)
	}
	e.text = " "
	pressButton(t, s, tt, "save")
	until(t, s, tt, "why it isn't kept", func() bool { return tt.HasText("These lyrics have no words to keep.") })
	e.text = "[00:01.00]From Unison\n[00:04.00]Second lime\n"
	pressButton(t, s, tt, "save")
	waitText(t, s, tt, "Second lime")
	if e.open || !tt.HasText("lyrics from Unison") {
		t.Errorf("after the edit, the window shows %q", tt.Texts())
	}
	if _, ok := tt.Find(locales.LyricsSourceLabel("Your edit")); !ok {
		t.Errorf("the button doesn't name the edit: %q", tt.Texts())
	}
}

// TestLyricsPickedElsewhere looks the lyrics up again in every window as
// one picks them, and lets their translation and romanization go: the mini
// player's and the notch's follow.
func TestLyricsPickedElsewhere(t *testing.T) {
	s, tt, _ := lyricsPickShell(t, false)
	waitText(t, s, tt, "From Unison")
	id := testTracks[0].ID
	other := newShell(s.a, s.p)
	t.Cleanup(other.followLyricsPicks())
	other.loads["lyrics:"+id] = &loaded[lyrics.Lyrics]{done: true}
	other.loads["lyrics-translation:"+id+":en"] = &loaded[translated]{done: true}
	other.loads[romajiKey(testTracks[0])] = &loaded[translated]{done: true}
	if err := s.svc.Catalog.PickLyrics(testTracks[0], app.LyricsPick{Source: "lrclib"}, nil); err != nil {
		t.Fatal(err)
	}
	other.drain()
	for _, key := range []string{"lyrics:" + id, "lyrics-translation:" + id + ":en", romajiKey(testTracks[0])} {
		if _, ok := other.loads[key]; ok {
			t.Errorf("the other window keeps %s", key)
		}
	}
	waitText(t, s, tt, "From LRCLIB")
}

// TestPseudoLocaleLyricsPick shows the choice of the lyrics' source, its
// menu and its dialogs in en-XA: only the names of the sources and the
// lyrics are English.
func TestPseudoLocaleLyricsPick(t *testing.T) {
	usePseudoLocale(t)
	s, tt, _ := lyricsPickShell(t, false)
	tt.SetLocale("en-XA")
	xa := locales.In("en-XA")
	waitText(t, s, tt, "From Unison")
	names := []string{"From Unison", "Second line", "Midnight City", "midnight city", "M83", "Massive Attack", "Teardrop",
		"Plain from Unison", "Another verse", "A third", "Hurry Up, We're Dreaming", "Unison"}
	checkPseudo(t, "The lyrics panel", tt.Texts(), names...)
	items := openSourceMenu(t, s, tt, xa.LyricsSourceAutomaticFrom("Unison"))
	checkPseudo(t, "The menu of the lyrics' sources", items, names...)
	var details []string
	for _, it := range lyricsSourceItems(s.svc.Catalog.LyricsChoices(testTracks[0])) {
		details = append(details, it.detail)
	}
	checkPseudo(t, "What the sources answered", details, names...)
	chooseSource(t, s, tt, xa.LyricsSourceSearch())
	v := &s.lyricsView.pick.search
	v.title, v.artist = "Other Song", "Other Artist"
	pressButton(t, s, tt, xa.LyricsSearchGo())
	until(t, s, tt, "the answers", func() bool { return !v.searching && tt.HasText("Plain from Unison") })
	checkPseudo(t, "The search dialog", tt.Texts(), append(names, "Other Song", "Other Artist")...)
	v.open = false
	tt.Frame()
	openSourceMenu(t, s, tt, xa.LyricsSourceAutomaticFrom("Unison"))
	chooseSource(t, s, tt, xa.LyricsSourceEdit())
	checkPseudo(t, "The edit dialog", tt.Texts(), names...)
	s.lyricsView.pick.edit.open = false
	show(s, tt, "/now-playing")
	checkPseudo(t, "The now playing page", tt.Texts(), names...)
	if _, ok := tt.Find(xa.LyricsSourceLabel(xa.LyricsSourceAutomaticFrom("Unison"))); !ok {
		t.Errorf("the now playing page has no choice of the lyrics' source: %q", tt.Texts())
	}
}

// TestLyricsSearchMultipleCandidatesAndFilter verifies scrollable multi-candidate
// browsing, per-source filter pills, lazy previewing, and picking a specific CandidateID.
func TestLyricsSearchMultipleCandidatesAndFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/betterlyrics/getLyrics", http.NotFound)
	mux.HandleFunc("/unison/lyrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"success":false,"code":"NOT_FOUND"}`))
	})
	mux.HandleFunc("/lrclib/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"id":201,"trackName":"Midnight City","artistName":"M83","albumName":"Hurry Up","duration":211,"syncedLyrics":"[00:01.00]LRCLIB One\n[00:04.00]End"},
			{"id":202,"trackName":"Midnight City (Live)","artistName":"M83","albumName":"Live","duration":250,"syncedLyrics":"[00:01.00]LRCLIB Two\n[00:04.00]End"},
			{"id":203,"trackName":"Midnight City (Acoustic)","artistName":"M83","albumName":"Acoustic","duration":210,"plainLyrics":"LRCLIB Three\nPlain"}
		]`))
	})
	mux.HandleFunc("/lrclib/get", http.NotFound)
	mux.HandleFunc("/lrclib/get-cached", http.NotFound)
	mux.HandleFunc("/qqmusic/soso/fcgi-bin/client_search_cp", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"song":{"list":[
			{"songmid":"qq4","songname":"Midnight City","albumname":"Hurry Up","interval":211,"singer":[{"name":"M83"}]},
			{"songmid":"qq3","songname":"Midnight City","albumname":"Deluxe","interval":211,"singer":[{"name":"M83"}]},
			{"songmid":"qq2","songname":"Midnight City","albumname":"Remastered","interval":211,"singer":[{"name":"M83"}]},
			{"songmid":"qq1","songname":"Midnight City Acoustic","albumname":"Sessions","interval":210,"singer":[{"name":"M83"}]}
		]}}}`))
	})
	mux.HandleFunc("/qqmusic/lyric/fcgi-bin/fcg_query_lyric_new.fcg", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("songmid") {
		case "qq4":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]QQ4\n[00:04.00]Second line"}`))
		case "qq3":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]QQ3\n[00:04.00]Second line"}`))
		case "qq2":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]QQ2\n[00:04.00]Second line"}`))
		case "qq1":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]QQ Lazy Preview\n[00:04.00]Second line"}`))
		default:
			w.Write([]byte(`{"retcode":-1}`))
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := newTestShellOf(t, func(o *app.Options) {
		o.LyricsURLs = &app.LyricsURLs{
			LRCLIB:       srv.URL + "/lrclib",
			Unison:       srv.URL + "/unison",
			BetterLyrics: srv.URL + "/betterlyrics",
			QQMusic:      srv.URL + "/qqmusic",
		}
		set := `{"layout": "tiles", "lyricsSources": [
			{"key": "lrclib"}, {"key": "qqmusic"},
			{"key": "betterlyrics", "off": true}, {"key": "unison", "off": true},
			{"key": "lrcred", "off": true}, {"key": "spotify", "off": true},
			{"key": "youtube", "off": true}, {"key": "captions", "off": true},
			{"key": "youtubepage", "off": true}
		]}`
		if err := os.WriteFile(filepath.Join(o.DataDir, "settings.json"), []byte(set), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	t.Cleanup(s.followLyricsPicks())
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 1300)
	s.open("/lyrics")
	tt.Frame()
	waitText(t, s, tt, "LRCLIB One")

	openSourceMenu(t, s, tt, "Automatic (LRCLIB)")
	chooseSource(t, s, tt, "Search by title and artist…")
	v := &s.lyricsView.pick.search
	until(t, s, tt, "all candidates", func() bool { return !v.searching && len(v.found) == 7 })

	if !tt.HasText("All (7)") || !tt.HasText("LRCLIB (3)") || !tt.HasText("QQ Music (4)") {
		t.Fatalf("filter pills missing: %q", tt.Texts())
	}

	// Filter to QQ Music only and lazy-preview candidate #4 ("Midnight City Acoustic").
	click(t, s, tt, "QQ Music (4)")
	if tt.HasText("LRCLIB Two") || !tt.HasText("Midnight City Acoustic") {
		t.Errorf("filter to QQ Music failed: %q", tt.Texts())
	}

	pressButton(t, s, tt, "preview")
	until(t, s, tt, "lazy preview text", func() bool { return tt.HasText("QQ Lazy Preview") })

	// Pick the lazy-previewed QQ Music candidate.
	pressButton(t, s, tt, "Show the lyrics from QQ Music — Midnight City Acoustic")
	waitText(t, s, tt, "QQ Lazy Preview")
	if v.open {
		t.Error("search dialog stayed open after picking candidate")
	}
}
