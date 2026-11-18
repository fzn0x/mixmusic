package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// chooseIn right-clicks what is labelled label and chooses the item of
// its menu at path.
func chooseIn(t *testing.T, s *shell, tt *ui.Tester, label string, path ...string) {
	t.Helper()
	if err := tt.RightClick(label); err != nil {
		t.Fatalf("%v; the window shows %q", err, tt.Texts())
	}
	if err := tt.ChooseMenuItem(path...); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
}

// nameIt gives the name asked for, as typed, with Enter.
func nameIt(t *testing.T, s *shell, tt *ui.Tester, name string) {
	t.Helper()
	if s.naming == nil {
		t.Fatal("no name was asked for")
	}
	s.naming.value = name
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
}

// fileMix puts the mix of title, of ids, in the folder named folder,
// which it makes if need be, and returns the folder's ID.
func fileMix(t *testing.T, s *shell, ids map[string]string, title, folder string) string {
	t.Helper()
	f, err := s.svc.Collection.NewFolder(folder, library.Mark{ID: ids[title], Kind: library.MarkPlaylist, Title: title})
	if err != nil {
		t.Fatal(err)
	}
	return f.ID
}

// folderList returns the names of the folders, as the sidebar lists
// them, with the titles of what each holds, as library.json keeps them.
func folderList(s *shell) []string {
	out := []string{}
	byID := map[string]string{}
	for _, p := range s.svc.Collection.Playlists() {
		byID[p.ID] = p.Title
	}
	for _, f := range s.svc.Collection.Folders() {
		var titles []string
		for _, m := range f.Items {
			title := byID[m.ID]
			if title == "" {
				title = m.Title
			}
			titles = append(titles, title)
		}
		out = append(out, f.Name+": "+strings.Join(titles, ", "))
	}
	return out
}

// TestSidebarFolders makes folders from the menus of the sidebar's
// entries and of a mix's page, takes in the one of the same name but for
// case and spacing, lists them after the pins in none, by name, with the
// pinned first in each, and lets a folder go once Remove from folder
// empties it.
func TestSidebarFolders(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Charlie", "Bravo", "Alpha")
	liked := s.lk.words("Liked songs")
	chooseIn(t, s, tt, "Alpha", "Move to folder", "New folder…")
	nameIt(t, s, tt, "  Road   trips ")
	if !tt.HasText("Moved Alpha to Road trips") {
		t.Errorf("no toast tells the move: %q", tt.Texts())
	}
	chooseIn(t, s, tt, "Road trip, pinned", "Move to folder", "Road trips")
	chooseIn(t, s, tt, "Bravo", "Move to folder", "New folder…")
	nameIt(t, s, tt, "ROAD TRIPS")
	head := "Road trips, folder, 3 playlists"
	labels := []string{liked, head, "Road trip, pinned", "Alpha", "Bravo", "Charlie"}
	if got := sideOrder(tt, labels...); !slices.Equal(got, labels) {
		t.Fatalf("the sidebar lists %q, want %q", got, labels)
	}
	if got, want := folderList(s), []string{"Road trips: Alpha, Road trip, Bravo"}; !slices.Equal(got, want) {
		t.Errorf("library.json keeps %q, want %q", got, want)
	}
	if info, _ := tt.Access(head); !info.Expanded {
		t.Error("the folder's header doesn't say it's open")
	}

	// From the More menu of a mix's page, a folder whose name comes first.
	show(s, tt, playlistPath(ids["Charlie"]))
	until(t, s, tt, "the mix's page", func() bool { _, ok := tt.Find("More"); return ok })
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Move to folder", "New folder…"); err != nil {
		t.Fatalf("%v: the page's menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	nameIt(t, s, tt, "Autumn")
	show(s, tt, "/settings/about")
	labels = []string{liked, "Autumn, folder, 1 playlist", "Charlie", head}
	if got := sideOrder(tt, labels...); !slices.Equal(got, labels) {
		t.Errorf("with another folder, the sidebar lists %q, want %q", got, labels)
	}
	chooseIn(t, s, tt, "Charlie", "Remove from folder")
	if _, ok := tt.Find("Autumn, folder, 1 playlist"); ok || len(s.svc.Collection.Folders()) != 1 {
		t.Errorf("the folder emptied stays: %q", folderList(s))
	}
	labels = []string{liked, head, "Charlie"}
	if got := sideOrder(tt, labels...); !slices.Equal(got, labels) {
		t.Errorf("out of its folder, the sidebar lists %q, want %q", got, labels)
	}
}

// TestSidebarFolderRename renames a folder from its header's menu, then
// as another, which makes them one, and ungroups it, which keeps what it
// held.
func TestSidebarFolderRename(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Bravo", "Alpha")
	fileMix(t, s, ids, "Alpha", "Autumn")
	fileMix(t, s, ids, "Bravo", "Rain")
	tt.Frame()
	chooseIn(t, s, tt, "Autumn, folder, 1 playlist", "Rename folder…")
	if s.naming == nil || s.naming.value != "Autumn" {
		t.Fatalf("Rename asks %+v", s.naming)
	}
	nameIt(t, s, tt, "Winter")
	if got, want := folderList(s), []string{"Rain: Bravo", "Winter: Alpha"}; !slices.Equal(got, want) {
		t.Errorf("renamed, the folders are %q, want %q", got, want)
	}
	chooseIn(t, s, tt, "Winter, folder, 1 playlist", "Rename folder…")
	nameIt(t, s, tt, " rain ")
	if got, want := folderList(s), []string{"rain: Bravo, Alpha"}; !slices.Equal(got, want) {
		t.Errorf("renamed as the other, the folders are %q, want %q", got, want)
	}
	chooseIn(t, s, tt, "rain, folder, 2 playlists", "Ungroup folder")
	labels := []string{s.lk.words("Liked songs"), "Road trip, pinned", "Alpha", "Bravo"}
	if got := sideOrder(tt, labels...); !slices.Equal(got, labels) || len(s.svc.Collection.Folders()) != 0 {
		t.Errorf("ungrouped, the sidebar lists %q, want %q", got, labels)
	}
}

// TestSidebarFolderDrag drags a mix, and the pin of a playlist, onto
// folders' headers, which takes them in, a mix of a folder onto another
// of it, and onto a mix of none, which takes it out, and a header onto
// another.
func TestSidebarFolderDrag(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Delta", "Charlie", "Bravo", "Alpha")
	col := s.svc.Collection
	autumn := fileMix(t, s, ids, "Alpha", "Autumn")
	fileMix(t, s, ids, "Bravo", "Winter")
	tt.Frame()
	dragSide(t, s, tt, "Charlie", "Autumn, folder, 1 playlist")
	dragSide(t, s, tt, "Road trip, pinned", "Winter, folder, 1 playlist")
	if got, want := folderList(s), []string{"Autumn: Alpha, Charlie", "Winter: Bravo, Road trip"}; !slices.Equal(got, want) {
		t.Fatalf("dragged onto the headers, the folders are %q, want %q", got, want)
	}
	// A pinned playlist leads its folder.
	labels := []string{"Winter, folder, 2 playlists", "Road trip, pinned", "Bravo"}
	if got := sideOrder(tt, labels...); !slices.Equal(got, labels) {
		t.Errorf("the sidebar lists %q, want %q", got, labels)
	}
	dragSide(t, s, tt, "Charlie", "Alpha")
	if got, want := folderList(s), []string{"Autumn: Charlie, Alpha", "Winter: Bravo, Road trip"}; !slices.Equal(got, want) {
		t.Errorf("dragged in its folder, the folders are %q, want %q", got, want)
	}
	// A pin of a folder goes no further than the pins: the mix stays.
	dragSide(t, s, tt, "Bravo", "Road trip, pinned")
	if got, want := folderList(s), []string{"Autumn: Charlie, Alpha", "Winter: Bravo, Road trip"}; !slices.Equal(got, want) {
		t.Errorf("a mix dragged onto a pin of its folder made %q", got)
	}
	dragSide(t, s, tt, "Alpha", "Delta")
	if col.FolderOf(ids["Alpha"]) != "" || col.FolderOf(ids["Charlie"]) != autumn {
		t.Errorf("dragged out, the folders are %q", folderList(s))
	}
	if got, want := mixesKept(t, s), []string{"Bravo", "Charlie", "Delta", "Alpha"}; !slices.Equal(got, want) {
		t.Errorf("dragged out onto a mix, the library keeps %q, want %q", got, want)
	}
	dragSide(t, s, tt, "Winter, folder, 2 playlists", "Autumn, folder, 1 playlist")
	if got, want := folderList(s), []string{"Winter: Bravo, Road trip", "Autumn: Charlie"}; !slices.Equal(got, want) {
		t.Errorf("a header dragged onto another, the folders are %q, want %q", got, want)
	}
	// Places don't go in folders.
	liked := s.lk.words("Liked songs")
	dragSide(t, s, tt, liked, "Autumn, folder, 1 playlist")
	if got := s.svc.Preferences.Get().SidebarPlaces; len(got) != 0 {
		t.Errorf("Liked songs dropped on a folder moved: %+v", got)
	}
}

// TestSidebarFolderOpen closes a folder from its header, which
// settings.json keeps, opens it while the search finds what it holds or
// its name, and when its page opens, which the user may close again.
func TestSidebarFolderOpen(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Storm", "Sun", "Rain")
	weather := fileMix(t, s, ids, "Rain", "Weather")
	fileMix(t, s, ids, "Sun", "Weather")
	tt.Frame()
	head := "Weather, folder, 2 playlists"
	shows := func(label string) bool { _, ok := tt.Find(label); return ok }
	click(t, s, tt, head)
	if shows("Sun") || shows("Rain") || !shows("Storm") {
		t.Errorf("closed, the sidebar shows %q", tt.Texts())
	}
	if got := s.svc.Preferences.Get().CollapsedFolders; !slices.Equal(got, []string{weather}) {
		t.Errorf("settings.json keeps %q closed", got)
	}
	if info, _ := tt.Access(head); info.Expanded {
		t.Error("the folder's header says it's open")
	}

	click(t, s, tt, "Search your mixes")
	tt.Type("sun")
	s.drain()
	tt.Frame()
	if !shows("Sun") || shows("Rain") || shows("Storm") {
		t.Errorf("searched, the sidebar shows %q", tt.Texts())
	}
	// Closed while searching, it stays closed in settings.json alone.
	click(t, s, tt, head)
	if shows("Sun") {
		t.Error("closed while searching, the folder shows what it holds")
	}
	s.order.side = "weath"
	tt.Frame()
	if !shows("Sun") || !shows("Rain") || shows("Storm") {
		t.Errorf("searched by its name, the sidebar shows %q", tt.Texts())
	}
	s.order.side = ""
	tt.Frame()
	if shows("Sun") {
		t.Error("with no search, the folder closed opens")
	}

	show(s, tt, playlistPath(ids["Sun"]))
	if !shows("Rain") || len(s.svc.Preferences.Get().CollapsedFolders) != 0 {
		t.Errorf("the page of its mix leaves the folder closed: %q", tt.Texts())
	}
	click(t, s, tt, head)
	tt.Frame()
	if shows("Rain") {
		t.Error("closed again on its mix's page, the folder opens")
	}
}

// TestSidebarFolderKeys opens and closes a folder with Enter, moves it
// with Alt and the arrows, and a mix in its folder, which the keys don't
// take out, and takes the mix out from the menu the menu key opens.
func TestSidebarFolderKeys(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Charlie", "Bravo", "Alpha")
	col := s.svc.Collection
	autumn := fileMix(t, s, ids, "Alpha", "Autumn")
	fileMix(t, s, ids, "Bravo", "Autumn")
	fileMix(t, s, ids, "Charlie", "Winter")
	tt.Frame()
	head := "Autumn, folder, 2 playlists"
	focusOn(t, tt, head)
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if _, ok := tt.Find("Alpha"); ok || !slices.Contains(s.svc.Preferences.Get().CollapsedFolders, autumn) {
		t.Errorf("Enter left the folder open: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if _, ok := tt.Find("Alpha"); !ok {
		t.Errorf("Enter again left the folder closed: %q", tt.Texts())
	}
	tt.Key(ui.Alt, ui.KeyDown)
	s.drain()
	tt.Frame()
	if got, want := folderList(s), []string{"Winter: Charlie", "Autumn: Alpha, Bravo"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Down, the folders are %q, want %q", got, want)
	}
	if info, _ := tt.Access(head); !info.Focused {
		t.Error("the focus left the folder moved")
	}

	focusOn(t, tt, "Bravo")
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	if got, want := folderList(s), []string{"Winter: Charlie", "Autumn: Bravo, Alpha"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Up twice, the folders are %q, want %q", got, want)
	}
	if info, _ := tt.Access("Bravo"); !info.Focused {
		t.Error("the focus left the mix moved")
	}
	tt.Key(0, ui.KeyContextMenu)
	if err := tt.ChooseMenuItem("Remove from folder"); err != nil {
		t.Fatalf("%v: the menu key opened %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if col.FolderOf(ids["Bravo"]) != "" {
		t.Errorf("Remove from folder left %q", folderList(s))
	}
}

// TestServicePlaylistFolder puts a playlist of YouTube Music in a folder
// from its tile in the Library, which the sidebar shows by its name.
func TestServicePlaylistFolder(t *testing.T) {
	s := newTestShell(t)
	youtubeLibraryOf(s, ownPlaylist, savedPlaylist)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/youtube")
	chooseIn(t, s, tt, "Chill", "Move to folder", "New folder…")
	nameIt(t, s, tt, "Lo-fi")
	fs := s.svc.Collection.Folders()
	if len(fs) != 1 || len(fs[0].Items) != 1 || fs[0].Items[0].ID != savedPlaylist.ID || fs[0].Items[0].Title != "Chill" ||
		fs[0].Items[0].Kind != library.MarkPlaylist {
		t.Fatalf("library.json keeps %+v", fs)
	}
	show(s, tt, "/settings/about")
	if got, want := sideOrder(tt, "Lo-fi, folder, 1 playlist", "Chill"), []string{"Lo-fi, folder, 1 playlist", "Chill"}; !slices.Equal(got, want) {
		t.Errorf("the sidebar lists %q, want %q", got, want)
	}
}

// TestPlayFolder plays the mixes of a folder one after another, each
// track once, and leaves out a playlist that doesn't load, which a toast
// names.
func TestPlayFolder(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Bravo", "Alpha")
	col := s.svc.Collection
	for title, tracks := range map[string][]media.Track{"Alpha": testTracks[:2], "Bravo": testTracks[1:3]} {
		if _, err := col.AddToMix(ids[title], tracks); err != nil {
			t.Fatal(err)
		}
	}
	both := fileMix(t, s, ids, "Alpha", "Both")
	fileMix(t, s, ids, "Bravo", "Both")
	if err := col.FileInFolder(both, library.Mark{ID: "sp:gone", Title: "Gone"}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	chooseIn(t, s, tt, "Both, folder, 3 playlists", "Play folder")
	s.settle(t, func(st player.State) bool { return len(st.Queue) > 0 })
	var got []string
	for _, it := range s.p.State().Queue {
		got = append(got, it.Track.ID)
	}
	if want := []string{testTracks[0].ID, testTracks[1].ID, testTracks[2].ID}; !slices.Equal(got, want) {
		t.Errorf("the queue holds %q, want %q", got, want)
	}
	tt.Frame()
	if !slices.ContainsFunc(tt.Texts(), func(text string) bool {
		return strings.HasPrefix(text, "Play folder skipped Gone, which didn't load: ")
	}) {
		t.Errorf("no toast names the playlist left out: %q", tt.Texts())
	}
}

// TestPseudoLocaleSidebarFolders shows a folder of the sidebar, its menu,
// the menu of a mix and New folder in en-XA.
func TestPseudoLocaleSidebarFolders(t *testing.T) {
	usePseudoLocale(t)
	s, tt, ids := sideShell(t, "", "Night drive", "Rainy day")
	fileMix(t, s, ids, "Night drive", "Evenings")
	tt.SetLocale("en-XA")
	tt.Frame()
	names := []string{"Night drive", "Rainy day", "Road trip", "Evenings"}
	checkPseudo(t, "Your mixes with a folder", tt.Texts(), names...)
	xa := locales.In("en-XA")
	head := xa.FoldersHeader("Evenings", 1)
	if err := tt.RightClick(head); err != nil {
		t.Fatalf("%v; the window shows %q", err, tt.Texts())
	}
	checkPseudo(t, "The folder's menu", tt.Menu(), names...)
	tt.CloseMenu()
	chooseIn(t, s, tt, "Rainy day", xa.FoldersMoveTo(), xa.FoldersNewItem())
	checkPseudo(t, "New folder", tt.Texts(), names...)
	nameIt(t, s, tt, "Evenings")
	if !tt.HasText(xa.FoldersMoved("Rainy day", "Evenings")) {
		t.Errorf("the toast isn't in en-XA: %q", tt.Texts())
	}
	if err := tt.RightClick("Rainy day"); err != nil {
		t.Fatal(err)
	}
	checkPseudo(t, "A mix's menu in a folder", tt.Menu(), names...)
}
