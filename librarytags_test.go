package main

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// focusOnRole tabs to the control of role labelled label, until it has
// the focus.
func focusOnRole(t *testing.T, tt *ui.Tester, role ui.Role, label string) {
	t.Helper()
	for range 300 {
		if info, _ := tt.AccessByRole(role, label); info.Focused {
			return
		}
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	t.Fatalf("Tab doesn't reach %q", label)
}

// press presses key, with mods, and lets the window take it in.
func press(s *shell, tt *ui.Tester, mods ui.Modifiers, key ui.Key) {
	tt.Key(mods, key)
	s.drain()
	tt.Frame()
}

// mixMark is the mark of the mix of title, of ids.
func mixMark(ids map[string]string, title string) library.Mark {
	return library.Mark{ID: ids[title], Kind: library.MarkPlaylist, Title: title}
}

// makeTag makes the tag name, of the colour named color, on what marks
// mark, and returns its ID.
func makeTag(t *testing.T, s *shell, name, color string, marks ...library.Mark) string {
	t.Helper()
	tag, err := s.svc.Collection.NewTag(name, color)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.svc.Collection.ChangeTagged(tag.ID, marks, nil); err != nil {
		t.Fatal(err)
	}
	return tag.ID
}

// tagNames returns the tags, in their order, each with the titles of
// what it's on, as library.json keeps them.
func tagNames(s *shell) []string {
	out := []string{}
	for _, t := range s.svc.Collection.Tags() {
		var titles []string
		for _, m := range t.Items {
			titles = append(titles, m.Title)
		}
		out = append(out, t.Name+": "+strings.Join(titles, ", "))
	}
	return out
}

// labOf is c in CIELAB, under D65.
func labOf(c color.NRGBA) [3]float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	r, g, b := lin(c.R), lin(c.G), lin(c.B)
	f := func(t float64) float64 {
		if t > 216.0/24389 {
			return math.Cbrt(t)
		}
		return (24389.0/27*t + 16) / 116
	}
	x := f((0.4124*r + 0.3576*g + 0.1805*b) / 0.95047)
	y := f(0.2126*r + 0.7152*g + 0.0722*b)
	z := f((0.0193*r + 0.1192*g + 0.9505*b) / 1.08883)
	return [3]float64{116*y - 16, 500 * (x - y), 200 * (y - z)}
}

// TestTagColoursRead checks the tags' palette: 25 colours, each named
// once, which new tags take in turn, that read at 3:1, as marks do, on
// the ground, the chips and the dialogs of every built-in theme, light
// and dark, in every layout and with the accents of many artworks, and
// stay apart from each other. On a ground no theme has, a colour is made
// to read there.
func TestTagColoursRead(t *testing.T) {
	if len(tagPalette) < 25 {
		t.Errorf("the palette has %d colours", len(tagPalette))
	}
	names := map[string]bool{}
	for _, h := range tagPalette {
		if names[h.name] {
			t.Errorf("%s twice in the palette", h.name)
		}
		names[h.name] = true
	}
	if len(tagNext) != len(tagPalette) || slices.ContainsFunc(tagNext, func(n string) bool { return !names[n] }) {
		t.Errorf("new tags take %q", tagNext)
	}
	s := newTestShell(t)
	for _, th := range s.inv.Themes {
		for _, layout := range settings.Layouts {
			for _, art := range contrastArtworks {
				k := contrastLook(s, th.ID, layout, art)
				bg := nrgba(k.bg)
				grounds := []struct {
					name   string
					ground color.NRGBA
				}{{"the ground", bg}, {"a chip", over(nrgba(k.surface2), bg)}, {"a dialog", over(nrgba(k.surface), bg)}}
				for _, h := range tagPalette {
					shade := tagShade(h, k.dark)
					for _, g := range grounds {
						if r := contrast(shade, g.ground); r < 3 {
							t.Errorf("%s, %s, artwork %q: %s on %s at %.2f:1", th.ID, layout, art, h.name, g.name, r)
						}
					}
				}
			}
		}
	}
	for _, dark := range []bool{false, true} {
		for i, a := range tagPalette {
			for _, b := range tagPalette[i+1:] {
				la, lb := labOf(tagShade(a, dark)), labOf(tagShade(b, dark))
				if d := math.Hypot(math.Hypot(la[0]-lb[0], la[1]-lb[1]), la[2]-lb[2]); d < 9 {
					t.Errorf("dark %v: %s and %s are %.1f apart", dark, a.name, b.name, d)
				}
			}
		}
	}
	grey := ui.Color{R: 120, G: 120, B: 120, A: 255}
	for _, h := range tagPalette {
		if r := contrast(nrgba(s.tagInk(h.name, grey)), nrgba(grey)); r < 3 {
			t.Errorf("%s on grey at %.2f:1", h.name, r)
		}
	}
	if got, want := nextTagColor(nil), tagNext[0]; got != want {
		t.Errorf("the first tag takes %s, want %s", got, want)
	}
}

// TestTagsFromMenus makes a tag from Tags… in the menu of a mix's tile,
// puts it on and takes it off, tags another mix from its page's More
// menu, then a playlist and an album of YouTube Music from their tiles:
// the album's tags hold for both its versions.
func TestTagsFromMenus(t *testing.T) {
	s := libraryShell(t)
	ids := makeMixes(t, s, "Sleep", "Run")
	col := s.svc.Collection
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/mixes")
	chooseIn(t, s, tt, "Run", "Tags…")
	if !tt.HasText("Tags for Run") || !tt.HasText("No tags yet. Name one below to make it.") {
		t.Fatalf("Tags… shows %q", tt.Texts())
	}
	clickRole(t, s, tt, ui.RoleTextField, "New tag")
	tt.Type("  Work   out ")
	press(s, tt, 0, ui.KeyEnter)
	tags := col.Tags()
	if len(tags) != 1 || tags[0].Name != "Work out" || tags[0].Color != tagNext[0] {
		t.Fatalf("library.json keeps %+v", tags)
	}
	if got, want := tagNames(s), []string{"Work out: Run"}; !slices.Equal(got, want) {
		t.Errorf("the tags are %q, want %q", got, want)
	}
	if info, _ := tt.AccessByRole(ui.RoleCheckBox, "Work out"); !info.Checked {
		t.Error("the new tag's box isn't checked")
	}
	clickRole(t, s, tt, ui.RoleCheckBox, "Work out")
	if len(col.TagsOf(ids["Run"])) != 0 {
		t.Error("the box cleared left the tag on")
	}
	clickRole(t, s, tt, ui.RoleCheckBox, "Work out")
	click(t, s, tt, "Done")
	if s.order.tags.item != nil {
		t.Error("Done left Tags… open")
	}

	show(s, tt, playlistPath(ids["Sleep"]))
	until(t, s, tt, "the mix's page", func() bool { _, ok := tt.Find("More"); return ok })
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Tags…"); err != nil {
		t.Fatalf("%v: the page's menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	clickRole(t, s, tt, ui.RoleCheckBox, "Work out")
	press(s, tt, 0, ui.KeyEscape)
	if s.order.tags.item != nil {
		t.Error("Escape left Tags… open")
	}
	if got, want := tagNames(s), []string{"Work out: Run, Sleep"}; !slices.Equal(got, want) {
		t.Errorf("tagged from the page, the tags are %q, want %q", got, want)
	}

	youtubeLibraryOf(s, ownPlaylist, savedPlaylist)
	ada := []media.Ref{{ID: "yt:UCada", Name: "Ada Field"}}
	s.loads["youtube:albums"] = &loaded[[]media.Album]{done: true, value: []media.Album{
		{ID: "yt:MPkites", Source: media.SourceYouTube, Title: "Kites", Artists: ada, Year: "2023"},
		{ID: "yt:MPdeluxe", Source: media.SourceYouTube, Title: "Kites (Deluxe)", Artists: ada, Year: "2024"},
	}}
	show(s, tt, "/library/youtube")
	chooseIn(t, s, tt, "Chill", "Tags…")
	clickRole(t, s, tt, ui.RoleCheckBox, "Work out")
	press(s, tt, 0, ui.KeyEscape)
	mood := makeTag(t, s, "Mood", "teal", library.Mark{ID: "yt:MPdeluxe", Kind: library.MarkAlbum, Title: "Kites (Deluxe)"})
	chooseIn(t, s, tt, "Kites", "Tags…")
	if info, _ := tt.AccessByRole(ui.RoleCheckBox, "Mood"); !info.Checked {
		t.Error("the album's tile doesn't show the tag of its other version")
	}
	clickRole(t, s, tt, ui.RoleCheckBox, "Work out")
	clickRole(t, s, tt, ui.RoleCheckBox, "Mood")
	press(s, tt, 0, ui.KeyEscape)
	if got, want := tagNames(s), []string{"Work out: Run, Sleep, Chill, Kites", "Mood: "}; !slices.Equal(got, want) {
		t.Errorf("tagged in YouTube Music's library, the tags are %q, want %q", got, want)
	}
	if got := col.TagsOf("yt:MPdeluxe"); len(got) != 0 || slices.Contains(col.TagsOf("yt:MPkites"), mood) {
		t.Errorf("the box cleared left the album's versions with %q", got)
	}
	if m := col.Tags()[0].Items[2]; m.ID != savedPlaylist.ID || m.Kind != library.MarkPlaylist {
		t.Errorf("library.json keeps the playlist as %+v", m)
	}
}

// TestManageTags makes a tag in Manage tags, which the chip opens, moves
// the tags by drag and with Alt and the arrows, recolours one, renames
// it, then as the other, which makes them one that the chips' choice
// follows, tags a mix with Choose items… and deletes the tag, which
// leaves the choice.
func TestManageTags(t *testing.T) {
	s := libraryShell(t)
	ids := makeMixes(t, s, "Walk", "Run")
	col := s.svc.Collection
	work := makeTag(t, s, "Work out", "red", mixMark(ids, "Run"))
	s.setTagFilter([]string{work})
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/mixes")
	click(t, s, tt, "Manage tags")
	if !tt.HasText("Drag a tag onto another, or move it with Alt and the arrows, to change the order of the chips. Choose one to rename it, change its colour, tag many items at once or delete it.") {
		t.Fatalf("Manage tags shows %q", tt.Texts())
	}
	clickRole(t, s, tt, ui.RoleTextField, "New tag")
	tt.Type("Calm")
	press(s, tt, 0, ui.KeyEnter)
	if got, want := tagNames(s), []string{"Work out: Run", "Calm: "}; !slices.Equal(got, want) {
		t.Fatalf("a new tag made %q, want %q", got, want)
	}
	if got := col.Tags()[1].Color; got != tagNext[0] {
		t.Errorf("the new tag is %s, want the first colour no tag has, %s", got, tagNext[0])
	}
	dragSide(t, s, tt, "Calm, on 0 items", "Work out, on 1 item")
	if got, want := tagNames(s), []string{"Calm: ", "Work out: Run"}; !slices.Equal(got, want) {
		t.Errorf("dragged, the tags are %q, want %q", got, want)
	}
	focusOn(t, tt, "Calm, on 0 items")
	press(s, tt, ui.Alt, ui.KeyDown)
	tt.Frame()
	if got, want := tagNames(s), []string{"Work out: Run", "Calm: "}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Down, the tags are %q, want %q", got, want)
	}
	if info, _ := tt.Access("Calm, on 0 items"); !info.Focused {
		t.Error("the focus left the tag moved")
	}

	click(t, s, tt, "Work out, on 1 item")
	if !tt.HasText("Edit tag") || !tt.HasText("On 1 item.") {
		t.Fatalf("the tag shows %q", tt.Texts())
	}
	click(t, s, tt, "Teal")
	if got := col.Tags()[0].Color; got != "teal" {
		t.Errorf("recoloured, the tag is %s", got)
	}
	if info, _ := tt.AccessByRole(ui.RoleRadio, "Teal"); !info.Checked {
		t.Error("Teal isn't chosen")
	}
	clickRole(t, s, tt, ui.RoleTextField, "Name")
	press(s, tt, ui.Cmd, ui.KeyA)
	tt.Type("Running")
	press(s, tt, 0, ui.KeyEnter)
	if got := col.Tags()[0].Name; got != "Running" {
		t.Errorf("renamed, the tag is %q", got)
	}
	clickRole(t, s, tt, ui.RoleTextField, "Name")
	press(s, tt, ui.Cmd, ui.KeyA)
	tt.Type(" calm ")
	// Escape goes back a step, as Back does, and names the tag as typed.
	press(s, tt, 0, ui.KeyEscape)
	tags := col.Tags()
	if got, want := tagNames(s), []string{"calm: Run"}; !slices.Equal(got, want) {
		t.Fatalf("renamed as the other, the tags are %q, want %q", got, want)
	}
	if got := s.svc.Preferences.Get().TagFilter; !slices.Equal(got, []string{tags[0].ID}) {
		t.Errorf("the chips chose %q, want the tag it became one with", got)
	}

	click(t, s, tt, "calm, on 1 item")
	click(t, s, tt, "Choose items…")
	if !tt.HasText("Choose items for calm") {
		t.Fatalf("Choose items shows %q", tt.Texts())
	}
	clickRole(t, s, tt, ui.RoleCheckBox, "Walk")
	if got, want := tagNames(s), []string{"calm: Run, Walk"}; !slices.Equal(got, want) {
		t.Errorf("chosen, the tags are %q, want %q", got, want)
	}
	clickRole(t, s, tt, ui.RoleTextField, "Search your mixes, playlists and albums")
	tt.Type("zzz")
	s.drain()
	tt.Frame()
	if !tt.HasText("Nothing matches “zzz”.") {
		t.Errorf("a search without matches says nothing: %q", tt.Texts())
	}
	press(s, tt, 0, ui.KeyEscape)
	if s.order.tags.query != "" || s.order.tags.choosing == "" {
		t.Errorf("Escape in the search left %q, or left Choose items", s.order.tags.query)
	}
	press(s, tt, 0, ui.KeyEscape)
	click(t, s, tt, "Delete tag")
	if !tt.HasText("Delete calm?") || !tt.HasText("It comes off the 2 items it's on. The items themselves stay.") {
		t.Fatalf("Delete tag asks %q", tt.Texts())
	}
	click(t, s, tt, "Delete")
	if len(col.Tags()) != 0 || len(s.svc.Preferences.Get().TagFilter) != 0 {
		t.Errorf("deleted, the tags are %q, the chips choose %q", tagNames(s), s.svc.Preferences.Get().TagFilter)
	}
	click(t, s, tt, "Done")
	if s.order.tags.managing || tt.HasText("Manage tags") {
		t.Errorf("Done left Manage tags, or the chips, there: %q", tt.Texts())
	}
	if got := inReadingOrder(tt, "Walk", "Run"); len(got) != 2 {
		t.Errorf("with no tag, the mixes show %q", got)
	}
}

// TestTagFilterMixes chooses tags with the chips above Library › Mixes:
// one, two of any and of all, which settings.json keeps, All, a tag gone,
// which leaves the choice, and Show tag filters, which hides the chips
// and shows all the mixes.
func TestTagFilterMixes(t *testing.T) {
	s := libraryShell(t)
	ids := makeMixes(t, s, "Calm", "Run", "Walk")
	col := s.svc.Collection
	work := makeTag(t, s, "Work out", "red", mixMark(ids, "Run"), mixMark(ids, "Walk"))
	out := makeTag(t, s, "Outside", "green", mixMark(ids, "Walk"))
	tt := tester(t, s)
	show(s, tt, "/library/mixes")
	mixes := []string{"Calm", "Run", "Walk"}
	if got, want := inReadingOrder(tt, "All", "Work out", "Outside", "Manage tags"), []string{"All", "Work out", "Outside", "Manage tags"}; !slices.Equal(got, want) {
		t.Fatalf("the chips are %q, want %q", got, want)
	}
	if info, _ := tt.AccessByRole(ui.RoleToggleButton, "All"); !info.Checked {
		t.Error("All isn't chosen")
	}
	clickRole(t, s, tt, ui.RoleToggleButton, "Work out")
	if got, want := inReadingOrder(tt, mixes...), []string{"Walk", "Run"}; !slices.Equal(got, want) {
		t.Errorf("by Work out, the mixes show %q, want %q", got, want)
	}
	if _, ok := tt.Find("New mix"); ok {
		t.Error("New mix shows while tags are chosen")
	}
	if info, _ := tt.AccessByRole(ui.RoleToggleButton, "Work out"); !info.Checked {
		t.Error("the chip chosen isn't pressed")
	}
	clickRole(t, s, tt, ui.RoleToggleButton, "Outside")
	if got, want := inReadingOrder(tt, mixes...), []string{"Walk", "Run"}; !slices.Equal(got, want) {
		t.Errorf("by any of two, the mixes show %q, want %q", got, want)
	}
	click(t, s, tt, s.lk.words("Match all"))
	if got, want := inReadingOrder(tt, mixes...), []string{"Walk"}; !slices.Equal(got, want) {
		t.Errorf("by all of two, the mixes show %q, want %q", got, want)
	}
	if set := s.svc.Preferences.Get(); !slices.Equal(set.TagFilter, []string{work, out}) || !set.TagsMatchAll {
		t.Errorf("settings.json keeps %q, all %v", set.TagFilter, set.TagsMatchAll)
	}

	// A tag gone leaves the choice at once.
	if err := col.DeleteTag(out); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got, want := inReadingOrder(tt, mixes...), []string{"Walk", "Run"}; !slices.Equal(got, want) {
		t.Errorf("a tag chosen gone, the mixes show %q, want %q", got, want)
	}
	if err := col.ChangeTagged(work, nil, []string{ids["Run"], ids["Walk"]}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("No mix has the tags chosen.") {
		t.Errorf("no mix of the tag says nothing: %q", tt.Texts())
	}
	clickRole(t, s, tt, ui.RoleToggleButton, "All")
	if got := inReadingOrder(tt, mixes...); len(got) != 3 || len(s.svc.Preferences.Get().TagFilter) != 0 {
		t.Errorf("All shows %q, and keeps %q", got, s.svc.Preferences.Get().TagFilter)
	}

	clickRole(t, s, tt, ui.RoleToggleButton, "Work out")
	show(s, tt, "/settings/appearance")
	focusOnRole(t, tt, ui.RoleSwitch, "Show tag filters")
	press(s, tt, 0, ui.KeySpace)
	if s.svc.Preferences.Get().TagFilters {
		t.Fatal("the switch left the tag filters on")
	}
	show(s, tt, "/library/mixes")
	if _, ok := tt.Find("Manage tags"); ok || len(inReadingOrder(tt, mixes...)) != 3 {
		t.Errorf("without the tag filters, the mixes show %q", tt.Texts())
	}
	if got := s.svc.Preferences.Get().TagFilter; !slices.Equal(got, []string{work}) {
		t.Errorf("hiding the chips forgot the choice: %q", got)
	}
}

// TestTagsTab shows Library › Tags once a tag exists: a mix, a playlist
// and an album of YouTube Music and an album on this computer, tagged
// from its tile, together, which the chips filter.
func TestTagsTab(t *testing.T) {
	s, _ := computerShell(t, settings.LayoutTiles, "mix")
	s.set.SidebarCollapsed = true
	s.save()
	ids := makeMixes(t, s, "Run", "Calm")
	youtubeLibraryOf(s, ownPlaylist, savedPlaylist)
	s.loads["youtube:albums"] = &loaded[[]media.Album]{done: true, value: []media.Album{
		{ID: "yt:MPblue", Source: media.SourceYouTube, Title: "Blue Hour", Artists: []media.Ref{{Name: "Ada Field"}}, Year: "2020"},
	}}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library")
	if _, ok := tt.Find(s.lk.words("Tags")); ok {
		t.Error("the Tags tab shows without a tag")
	}
	mood := makeTag(t, s, "Mood", "violet", mixMark(ids, "Calm"), playlistMark(savedPlaylist),
		library.Mark{ID: "yt:MPblue", Kind: library.MarkAlbum, Title: "Blue Hour"})
	fast := makeTag(t, s, "Fast", "orange", mixMark(ids, "Run"))
	show(s, tt, "/library/computer")
	s.computer.view = 1
	tt.Frame()
	chooseIn(t, s, tt, "Kites", "Tags…")
	clickRole(t, s, tt, ui.RoleCheckBox, "Mood")
	press(s, tt, 0, ui.KeyEscape)
	kites := slices.IndexFunc(s.svc.LocalFiles.Albums(), func(a media.Album) bool { return a.Title == "Kites" })
	if got := s.svc.Collection.TagsOf(s.svc.LocalFiles.Albums()[kites].ID); !slices.Equal(got, []string{mood}) {
		t.Fatalf("the album on this computer has the tags %q", got)
	}

	show(s, tt, "/library/tags")
	all := []string{"Run", "Calm", "Chill", "Blue Hour", "Kites", "Late night", "Shore"}
	if got, want := inReadingOrder(tt, all...), []string{"Calm", "Run", "Chill", "Blue Hour", "Kites"}; !slices.Equal(got, want) {
		t.Errorf("Library › Tags shows %q, want %q", got, want)
	}
	clickRole(t, s, tt, ui.RoleToggleButton, "Mood")
	if got, want := inReadingOrder(tt, all...), []string{"Calm", "Chill", "Blue Hour", "Kites"}; !slices.Equal(got, want) {
		t.Errorf("by Mood, Library › Tags shows %q, want %q", got, want)
	}
	clickRole(t, s, tt, ui.RoleToggleButton, "Fast")
	click(t, s, tt, s.lk.words("Match all"))
	if !tt.HasText("Nothing here has the tags chosen.") {
		t.Errorf("by all of two, Library › Tags shows %q", tt.Texts())
	}
	if !slices.Equal(s.svc.Preferences.Get().TagFilter, []string{mood, fast}) {
		t.Errorf("settings.json keeps %q", s.svc.Preferences.Get().TagFilter)
	}
}

// TestTagChipsKeys chooses a tag's chip with Space and Enter, tags a mix
// from Tags…, which the menu key opens, with Space on a box and a new
// tag with Enter, all without a pointer.
func TestTagChipsKeys(t *testing.T) {
	s := libraryShell(t)
	ids := makeMixes(t, s, "Calm", "Run")
	work := makeTag(t, s, "Work out", "red", mixMark(ids, "Run"))
	tt := tester(t, s)
	show(s, tt, "/library/mixes")
	focusOnRole(t, tt, ui.RoleToggleButton, "Work out")
	press(s, tt, 0, ui.KeySpace)
	if got := s.svc.Preferences.Get().TagFilter; !slices.Equal(got, []string{work}) {
		t.Errorf("Space chose %q", got)
	}
	if got, want := inReadingOrder(tt, "Calm", "Run"), []string{"Run"}; !slices.Equal(got, want) {
		t.Errorf("by Work out, the mixes show %q, want %q", got, want)
	}
	press(s, tt, 0, ui.KeyEnter)
	if got := s.svc.Preferences.Get().TagFilter; len(got) != 0 {
		t.Errorf("Enter left %q chosen", got)
	}

	focusOn(t, tt, "Calm")
	tt.Key(0, ui.KeyContextMenu)
	if err := tt.ChooseMenuItem("Tags…"); err != nil {
		t.Fatalf("%v: the menu key opened %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	focusOnRole(t, tt, ui.RoleCheckBox, "Work out")
	press(s, tt, 0, ui.KeySpace)
	focusOnRole(t, tt, ui.RoleTextField, "New tag")
	tt.Type("Evening")
	press(s, tt, 0, ui.KeyEnter)
	if got, want := tagNames(s), []string{"Work out: Run, Calm", "Evening: Calm"}; !slices.Equal(got, want) {
		t.Errorf("by keys, the tags are %q, want %q", got, want)
	}
}

// TestTagChipsRightToLeft lays the chips, and the rows of Manage tags,
// from the right.
func TestTagChipsRightToLeft(t *testing.T) {
	s := libraryShell(t)
	ids := makeMixes(t, s, "Run")
	makeTag(t, s, "Work out", "red", mixMark(ids, "Run"))
	makeTag(t, s, "Calm", "teal")
	tt := tester(t, s)
	tt.SetDirection(ui.RightToLeft)
	show(s, tt, "/library/mixes")
	all, work, calm, manage := placeOf(t, tt, "All"), placeOf(t, tt, "Work out"), placeOf(t, tt, "Calm"), placeOf(t, tt, "Manage tags")
	if !(all.X > work.X && work.X > calm.X && calm.X > manage.X) {
		t.Errorf("the chips go All %v, Work out %v, Calm %v, Manage tags %v", all.X, work.X, calm.X, manage.X)
	}
	if any, every := placeOf(t, tt, s.lk.words("Match any")), placeOf(t, tt, s.lk.words("Match all")); any.X < every.X {
		t.Errorf("Match any at %v is left of Match all at %v", any.X, every.X)
	}
	click(t, s, tt, "Manage tags")
	field, ok := tt.AccessByRole(ui.RoleTextField, "New tag")
	if add := placeOf(t, tt, "Add tag"); !ok || field.Bounds.X < add.X {
		t.Errorf("the field of a new tag, at %v, is left of Add tag, at %v", field.Bounds, add)
	}
}

// TestTagsBound offers no new tag at 200, and says why.
func TestTagsBound(t *testing.T) {
	s := libraryShell(t)
	for i := range library.MaxTags {
		if _, err := s.svc.Collection.NewTag(fmt.Sprint("Tag ", i), "red"); err != nil {
			t.Fatal(err)
		}
	}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/mixes")
	click(t, s, tt, "Manage tags")
	if !tt.HasText("You have 200 tags, the most there can be. Delete one to make another.") {
		t.Errorf("at the most tags, Manage tags shows %q", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "New tag"); ok {
		t.Error("a new tag can be named at the most tags")
	}
}
