package main

// Tags of the library (internal/library/tags.go): Tags…, in the menu of a
// mix, of a playlist or an album of a service's library, and of an album
// on this computer, on its tile and in its page's More menu, puts the
// user's tags on it and takes them off, and makes a new one. Manage tags
// makes, renames, recolours, orders and deletes them, and its Choose
// items… tags many items at once. Once a tag exists, a row of chips above
// Library › Mixes, and in Library › Tags, which shows the mixes, the
// services' playlists and albums and the albums on this computer
// together, chooses the tags they show: those with any of them, or with
// all. settings.json keeps the choice, by the tags' IDs, and a tag
// deleted leaves it. An album's tags hold for every version its tile
// groups. Each tag has a colour of a palette made to read on every
// theme, light and dark.

import (
	"image/color"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/hct"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// tagsView is how the user tags their library, beyond what library.json
// and settings.json keep.
type tagsView struct {
	// item is what the Tags… dialog tags, nil while it's closed.
	item *tagTarget
	// managing is set while Manage tags shows: edit is the tag it shows,
	// choosing the tag whose items it lists, and deleting the tag it asks
	// to delete, "" for none.
	managing                 bool
	edit, choosing, deleting string
	// name is the name of a new tag, as typed in Manage tags; rename the
	// name of the tag edit, as typed; query the search of Choose items.
	name, rename, query string
	// inks are the tags' colours as they read on the grounds asked.
	inks map[tagInkKey]ui.Color
}

// tagTarget is what the Tags… dialog tags: its mark, the IDs it goes by,
// its own first, then those of the other versions of an album its tile
// groups, and the name of a new tag, as typed.
type tagTarget struct {
	mark library.Mark
	ids  []string
	name string
}

// tagHue is a colour of the tags' palette: its name, which library.json
// keeps, and its hue and chroma in HCT, Material's colour space, whose
// tone sets how light it shows (tagShade).
type tagHue struct {
	name        string
	hue, chroma float64
}

// tagPalette are the tags' colours, in the order Manage tags shows them:
// round the wheel, then the quiet ones.
var tagPalette = []tagHue{
	{"red", 25, 70}, {"coral", 40, 60}, {"orange", 55, 64}, {"amber", 70, 60}, {"gold", 86, 52},
	{"yellow", 102, 60}, {"lime", 122, 58}, {"olive", 108, 30}, {"green", 140, 56}, {"emerald", 155, 50},
	{"mint", 170, 40}, {"teal", 185, 46}, {"cyan", 205, 44}, {"sky", 230, 44}, {"blue", 258, 56},
	{"indigo", 275, 60}, {"violet", 290, 64}, {"purple", 305, 60}, {"plum", 320, 34}, {"magenta", 335, 64},
	{"pink", 350, 56}, {"rose", 8, 46}, {"brown", 55, 26}, {"sand", 80, 16}, {"slate", 250, 8},
}

// tagNext is the order new tags take the palette's colours in: the
// first no tag has, far apart from those before.
var tagNext = []string{"blue", "red", "green", "orange", "purple", "teal", "pink", "yellow", "indigo", "lime",
	"magenta", "cyan", "coral", "emerald", "violet", "gold", "sky", "rose", "mint", "amber", "plum", "olive",
	"brown", "sand", "slate"}

// The tones of the tags' colours: light on dark grounds, and dark on
// light ones, which read at 3:1 on the grounds of the built-in themes.
const (
	tagToneDark  = 72
	tagToneLight = 48
)

// tagInkKey is a colour of the palette, on a ground, in a mode.
type tagInkKey struct {
	name   string
	ground ui.Color
	dark   bool
}

// tagHueOf is the colour of the palette named name: the first for a
// name unknown, as a later version's.
func tagHueOf(name string) tagHue {
	for _, h := range tagPalette {
		if h.name == name {
			return h
		}
	}
	return tagPalette[0]
}

// tagShade is the shade of h for dark grounds, or light ones.
func tagShade(h tagHue, dark bool) color.NRGBA {
	tone := float64(tagToneLight)
	if dark {
		tone = tagToneDark
	}
	r, g, b := hct.From(h.hue, h.chroma, tone).ARGB().RGB()
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

// tagInk is the colour named name as it reads on ground: its shade of the
// look's mode, made to read there at 3:1 where a theme's ground or a
// colour file takes it too close.
func (s *shell) tagInk(name string, ground ui.Color) ui.Color {
	v := &s.order.tags
	key := tagInkKey{name, ground, s.lk.dark}
	if c, ok := v.inks[key]; ok {
		return c
	}
	if v.inks == nil || len(v.inks) >= 512 {
		v.inks = map[tagInkKey]ui.Color{}
	}
	c := col(readableOn(tagShade(tagHueOf(name), s.lk.dark), nrgba(ground)))
	v.inks[key] = c
	return c
}

// The grounds the tags' colours show on: the chips' fill, and the
// dialogs'.
func (s *shell) chipGround() ui.Color   { return col(over(nrgba(s.lk.surface2), nrgba(s.lk.bg))) }
func (s *shell) dialogGround() ui.Color { return col(over(nrgba(s.lk.surface), nrgba(s.lk.bg))) }

// tagDot is the dot of a tag's colour, of size, as it reads on ground,
// with a ring of ring around it, unless ring is clear.
func (s *shell) tagDot(c *ui.Context, name string, ground ui.Color, size float32, ring ui.Color) {
	d := ui.Box(c).Size(size, size).Radius(size / 2).Shrink(0).Background(s.tagInk(name, ground))
	if ring.A > 0 {
		d.Border(1.5, ring)
	}
}

// nextTagColor is the colour of a new tag, of tags: the first of tagNext
// none of them has, or one in turn once they have them all.
func nextTagColor(tags []library.Tag) string {
	for _, name := range tagNext {
		if !slices.ContainsFunc(tags, func(t library.Tag) bool { return t.Color == name }) {
			return name
		}
	}
	return tagNext[len(tags)%len(tagNext)]
}

// tagAt is where the tag of id is among tags, or -1.
func tagAt(tags []library.Tag, id string) int {
	return slices.IndexFunc(tags, func(t library.Tag) bool { return t.ID == id })
}

// tagsOn returns the IDs of the tags on the item of ids: on any of them,
// as on any version of an album.
func (s *shell) tagsOn(ids []string) []string {
	col := s.svc.Collection
	if len(ids) == 1 {
		return col.TagsOf(ids[0])
	}
	var out []string
	for _, id := range ids {
		for _, t := range col.TagsOf(id) {
			if !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	return out
}

// tagsMatch reports whether an item of the tags on shows for the tags
// chosen: with any of them, or, if all, with every one. None chosen shows
// every item.
func tagsMatch(on, chosen []string, all bool) bool {
	if len(chosen) == 0 {
		return true
	}
	for _, id := range chosen {
		has := slices.Contains(on, id)
		switch {
		case all && !has:
			return false
		case !all && has:
			return true
		}
	}
	return all
}

// chosenTags returns the IDs of the tags the chips chose, of tags: a tag
// gone leaves the choice.
func (s *shell) chosenTags(tags []library.Tag) []string {
	var out []string
	for _, id := range s.set.TagFilter {
		if tagAt(tags, id) >= 0 {
			out = append(out, id)
		}
	}
	return out
}

// setTagFilter chooses the tags of ids, which settings.json keeps.
func (s *shell) setTagFilter(ids []string) {
	s.set.TagFilter = append([]string{}, ids...)
	s.save()
}

// swapTagFilter has the choice follow the tag of from, which became the
// tag of to, or, when to is "", went.
func (s *shell) swapTagFilter(from, to string) {
	if !slices.Contains(s.set.TagFilter, from) {
		return
	}
	var ids []string
	for _, id := range s.set.TagFilter {
		switch {
		case id != from:
			if id != to {
				ids = append(ids, id)
			}
		case to != "":
			ids = append(ids, to)
		}
	}
	s.setTagFilter(ids)
}

// albumIDs returns the IDs the album a goes by: its own, then those of
// the other versions its tile groups.
func (s *shell) albumIDs(a media.Album) []string {
	ids := []string{a.ID}
	for _, v := range s.versions.of[a.ID] {
		if v.ID != a.ID {
			ids = append(ids, v.ID)
		}
	}
	return ids
}

// tagsItem is the item of a menu that opens Tags… for mark's item, which
// goes by the IDs of ids, its own first.
func (s *shell) tagsItem(m *ui.Menu, mark library.Mark, ids []string) {
	if m.Item(locales.LibraryTagsItem()).Chosen() {
		s.order.tags.item = &tagTarget{mark: mark, ids: ids}
	}
}

// playlistTagsItem and albumTagsItem are tagsItem of a playlist, and of
// an album, with all its versions.
func (s *shell) playlistTagsItem(m *ui.Menu, p media.Playlist) {
	s.tagsItem(m, playlistMark(p), []string{p.ID})
}

func (s *shell) albumTagsItem(m *ui.Menu, a media.Album) { s.tagsItem(m, albumMark(a), s.albumIDs(a)) }

// withTags is the menu of a page, base, with Tags… of its item, mark's,
// which goes by the IDs of ids.
func (s *shell) withTags(base func(m *ui.Menu), mark library.Mark, ids []string) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if base != nil {
			base(m)
		}
		s.tagsItem(m, mark, ids)
	}
}

// albumLibraryItems end the menu of the tile of an album of a service's
// library: Pin to top or Unpin, Tags… and Hide from library.
func (s *shell) albumLibraryItems(a media.Album) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		m.Separator()
		s.pinItem(m, albumMark(a))
		s.albumTagsItem(m, a)
		s.hideItem(m, albumMark(a))
	}
}

// tagChip is a chip of the row of tags, under key, labelled label, with
// the dot of the colour named dot unless it's "". A toggle tells
// assistive technology whether it's on. It returns whether it was
// clicked.
func (s *shell) tagChip(c *ui.Context, key, label, dot string, on, toggle bool, glyph string) bool {
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Height(32).PaddingX(14).Gap(8).Radius(k.round(32)).Label(label).
		Cursor(ui.CursorPointer).AlignItems(ui.Center)
	if toggle {
		b.Role(ui.RoleToggleButton).Checked(on)
	}
	fill, ink := k.surface2, k.text
	switch {
	case on:
		fill, ink = k.selected, k.selectedText
	case b.Pressed():
		fill = k.surface3
	case b.Hovered():
		fill = k.surface2.Mix(k.text, 0.08)
	}
	b.Background(fill)
	s.pressMotion(b, 0.95)
	b.Children(func() {
		switch {
		case dot != "" && on:
			// The chosen chip's fill takes the accent, which the dot's
			// colour may not stand out from: its ring does.
			s.tagDot(c, dot, s.chipGround(), 12, k.selectedText)
		case dot != "":
			s.tagDot(c, dot, s.chipGround(), 12, ui.Color{})
		case glyph != "":
			ui.Icon(c, icon(glyph)).FontSize(16).TextColor(ink)
		}
		t := ui.Text(c, label).FontSize(k.smallSize()).TextColor(ink).SingleLine()
		k.labelWeight(t)
	})
	return b.Clicked()
}

// tagChips are the row of the tags, once there are some, under key: All,
// which chooses none, the tags in the user's order, which each turn on
// and off, and Manage tags, then Match any and Match all.
func (s *shell) tagChips(c *ui.Context, key string) {
	tags := s.svc.Collection.Tags()
	if len(tags) == 0 {
		return
	}
	k := &s.lk
	chosen := s.chosenTags(tags)
	ui.Row(c.Key(key+"-tags")).Gap(16).Wrap().AlignItems(ui.Center).Margin(0, 0, 24).Children(func() {
		chips := ui.Row(c.Key(key + "-tag-chips")).Role(ui.RoleGroup).Label(locales.LibraryTagsFilter())
		chips.Gap(8).Wrap().AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
			if s.tagChip(c, key+"-tag-all", k.sentence(locales.LibraryTagsAll()), "", len(chosen) == 0, true, "") {
				s.setTagFilter(nil)
			}
			for _, t := range tags {
				on := slices.Contains(chosen, t.ID)
				if s.tagChip(c, key+"-tag-"+t.ID, t.Name, t.Color, on, true, "") {
					if on {
						s.setTagFilter(slices.DeleteFunc(slices.Clone(chosen), func(id string) bool { return id == t.ID }))
					} else {
						s.setTagFilter(append(chosen, t.ID))
					}
				}
			}
			if s.tagChip(c, key+"-tag-manage", k.sentence(locales.LibraryTagsManage()), "", false, false, "tag") {
				s.openManageTags()
			}
		})
		way := 0
		if s.set.TagsMatchAll {
			way = 1
		}
		if s.segmented(c, key+"-tag-match", locales.LibraryTagsMatchLabel(), &way, locales.LibraryTagsMatchAny(), locales.LibraryTagsMatchAll()) {
			s.set.TagsMatchAll = way == 1
			s.save()
		}
	})
}

// taggedItem is an item of the library that takes tags: a mix, a
// playlist or an album, by where it comes from, "mix", "spotify",
// "youtube" or "computer", and the IDs it goes by, its own first.
type taggedItem struct {
	from     string
	playlist *media.Playlist
	album    *media.Album
	ids      []string
}

func (it taggedItem) id() string { return it.ids[0] }

func (it taggedItem) title() string {
	if it.album != nil {
		return it.album.Title
	}
	return it.playlist.Title
}

func (it taggedItem) mark() library.Mark {
	if it.album != nil {
		return albumMark(*it.album)
	}
	return playlistMark(*it.playlist)
}

// kindOf tells what it is and where, under its name in Choose items.
func (it taggedItem) kindOf() string {
	service := locales.LibraryOnThisComputer()
	switch it.from {
	case "mix":
		return locales.LibraryTagsKindOf("mix", "")
	case "spotify", "youtube":
		service = sourceName(media.Source(it.from))
	}
	kind := "playlist"
	if it.album != nil {
		kind = "album"
	}
	return locales.LibraryTagsKindOf(kind, service)
}

// taggables returns what of the library takes tags: the mixes, as the
// user sorted them, the playlists of the services the user signed in
// to, as they sorted those, their saved albums, one for the versions of
// each, and the albums on this computer, without what the user hid.
// loading is set while a list of a service still comes.
func (s *shell) taggables() (items []taggedItem, loading bool) {
	col := s.svc.Collection
	for _, p := range col.Playlists() {
		items = append(items, taggedItem{from: "mix", playlist: &p, ids: []string{p.ID}})
	}
	accounts := s.accounts()
	type lists struct {
		key       string
		playlists *loaded[[]media.Playlist]
		albums    *loaded[[]media.Album]
	}
	var services []lists
	if accounts.Spotify.SignedIn {
		services = append(services, lists{"spotify", fetch(s, "spotify:playlists", col.SpotifyPlaylists), fetch(s, "spotify:albums", col.SpotifyAlbums)})
	}
	if accounts.YouTube.SignedIn {
		services = append(services, lists{"youtube", s.youtubePlaylists(), s.youtubeAlbums()})
	}
	for _, l := range services {
		loading = loading || !l.playlists.done || !l.albums.done
		for _, p := range s.playlistsView("tags-"+l.key, l.playlists.shown(), "") {
			items = append(items, taggedItem{from: l.key, playlist: &p, ids: []string{p.ID}})
		}
	}
	for _, l := range services {
		albums := pinnedFirst(s, l.albums.shown(), func(a media.Album) string { return a.ID }, true)
		for _, g := range s.albumGroups("tags-"+l.key+"-album", albums) {
			a := s.preferredVersion(g)
			ids := []string{a.ID}
			for _, v := range g {
				if v.ID != a.ID {
					ids = append(ids, v.ID)
				}
			}
			items = append(items, taggedItem{from: l.key, album: &a, ids: ids})
		}
	}
	for _, a := range s.svc.LocalFiles.Albums() {
		items = append(items, taggedItem{from: "computer", album: &a, ids: []string{a.ID}})
	}
	return items, loading
}

// tagsLibrary is Library › Tags: the row of tags, then all that has the
// tags chosen, or, with none chosen, all that has a tag, together in one
// grid, the pinned first.
func (s *shell) tagsLibrary(c *ui.Context) {
	col := s.svc.Collection
	s.tagChips(c, "tags")
	chosen := s.chosenTags(col.Tags())
	items, loading := s.taggables()
	var shown []taggedItem
	for _, it := range items {
		if on := s.tagsOn(it.ids); len(on) > 0 && tagsMatch(on, chosen, s.set.TagsMatchAll) {
			shown = append(shown, it)
		}
	}
	slices.SortStableFunc(shown, func(a, b taggedItem) int { return comparePins(col.PinAt(a.id()), col.PinAt(b.id())) })
	if len(shown) > 0 {
		s.tileWrap(c, func() {
			for _, it := range shown {
				s.taggedTile(c, it)
			}
		})
	}
	switch {
	case loading:
		s.moreComing(c, "tags")
	case len(shown) == 0 && len(chosen) == 0:
		s.emptyLine(c, locales.LibraryTagsNothingTagged())
	case len(shown) == 0:
		s.emptyLine(c, locales.LibraryTagsNoItemMatch())
	}
}

// taggedTile is the tile of it in Library › Tags, whose menu is the one
// of its own list's.
func (s *shell) taggedTile(c *ui.Context, it taggedItem) {
	key := "tagged-" + it.id()
	switch {
	case it.from == "mix":
		s.playlistTileOf(c, key, *it.playlist, nil, nil)
	case it.playlist != nil:
		s.playlistTileWith(c, key, *it.playlist, s.libraryItems(it.from))
	case it.from == "computer":
		s.albumTile(c, key, *it.album)
	default:
		s.albumTileWith(c, key, *it.album, s.albumLibraryItems(*it.album))
	}
}

// libraryTagDialogs are the dialogs of the tags: Tags… and Manage tags.
func (s *shell) libraryTagDialogs(c *ui.Context) {
	s.itemTagsDialog(c)
	s.manageTagsDialog(c)
}

// tagCheck is the check box of the tag t, under key, on when on: its
// colour's dot as it reads on the dialog, and its name. It returns
// whether the user turned it on or off.
func (s *shell) tagCheck(c *ui.Context, key string, t library.Tag, on bool) bool {
	return s.checkRow(c, key, t.Name, "", on, func() { s.tagDot(c, t.Color, s.dialogGround(), 12, ui.Color{}) })
}

// checkRow is a row of a dialog that checks what it names, title, with
// sub under it unless it's "", and what lead shows before the title: on
// when on. It returns whether the user turned it on or off.
func (s *shell) checkRow(c *ui.Context, key, title, sub string, on bool, lead func()) bool {
	k := &s.lk
	was := on
	box := ui.CheckboxBase(c.Key(key), &on).Label(title).Row().Gap(12).AlignItems(ui.Center).PaddingY(6).PaddingX(8).
		Radius(k.radiusSm).Cursor(ui.CursorPointer)
	if box.Hovered() {
		box.Background(k.hover)
	}
	box.Children(func() {
		face := ui.Box(c).Size(18, 18).Radius(4).Center().Shrink(0)
		if on {
			face.Background(k.accent).Children(func() {
				ui.Icon(c, icon("check")).FontSize(14).TextColor(k.accentText)
			})
		} else {
			face.Border(2, k.outline)
		}
		if lead != nil {
			lead()
		}
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(k.bodySize()).SingleLine().MinWidth(0)
			if sub != "" {
				ui.Text(c, sub).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().MinWidth(0)
			}
		})
	})
	return on != was
}

// newTagField is the field that names a new tag, under key, with Add tag
// beside it, among tags: a tag made, or the tag of the name already,
// goes to made. At the most tags, it says so instead.
func (s *shell) newTagField(c *ui.Context, key string, name *string, tags []library.Tag, made func(t library.Tag)) {
	k := &s.lk
	if len(tags) >= library.MaxTags {
		ui.Text(c, locales.LibraryTagsFull(library.MaxTags)).FontSize(k.smallSize()).TextColor(k.muted)
		return
	}
	ui.Row(c.Key(key)).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		label := locales.LibraryTagsNewLabel()
		in := ui.TextInput(c.Key(key+"-name"), name).Placeholder(k.sentence(label)).Label(label).
			MaxLength(library.MaxTagName).Width(260).Shrink(1).MinWidth(160)
		clean := library.TagName(*name)
		add := s.dialogButton(c, key+"-add", locales.LibraryTagsAdd(), false).Disabled(clean == "")
		if (add.Clicked() || in.Submitted()) && clean != "" {
			t, err := s.svc.Collection.NewTag(clean, nextTagColor(tags))
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			*name = ""
			made(t)
		}
	})
}

// itemTagsDialog is the dialog Tags… opens: the tags as check boxes,
// which put each on the item or take it off at once, and the field that
// makes a new one, on it.
func (s *shell) itemTagsDialog(c *ui.Context) {
	v := &s.order.tags
	it := v.item
	if it == nil {
		return
	}
	k := &s.lk
	tags := s.svc.Collection.Tags()
	on := s.tagsOn(it.ids)
	open := true
	s.dialog(c, &open, dialogOptions{key: "item-tags", title: locales.LibraryTagsItemTitle(it.mark.Title), width: 480}, func() {
		if len(tags) == 0 {
			ui.Text(c, locales.LibraryTagsNone()).FontSize(k.bodySize()).TextColor(k.muted)
		} else {
			ui.Column(c.Key("item-tags-list")).Gap(2).Children(func() {
				for _, t := range tags {
					had := slices.Contains(on, t.ID)
					if s.tagCheck(c, "item-tag-"+t.ID, t, had) {
						s.setTagged(t.ID, it, !had)
					}
				}
			})
		}
		s.newTagField(c, "item-tags-new", &it.name, tags, func(t library.Tag) { s.setTagged(t.ID, it, true) })
	}, func() {
		if s.dialogButton(c, "item-tags-manage", locales.LibraryTagsManageItem(), false).Clicked() {
			v.item = nil
			s.openManageTags()
		}
		if s.dialogButton(c, "item-tags-done", locales.LibraryTagsDone(), true).Clicked() {
			v.item = nil
		}
	})
	if !open {
		v.item = nil
	}
}

// setTagged puts the tag of id on it, or takes it off it, with all the
// IDs it goes by.
func (s *shell) setTagged(id string, it *tagTarget, on bool) {
	var err error
	if on {
		err = s.svc.Collection.ChangeTagged(id, []library.Mark{it.mark}, nil)
	} else {
		err = s.svc.Collection.ChangeTagged(id, nil, it.ids)
	}
	if err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// openManageTags opens Manage tags, on the list of the tags.
func (s *shell) openManageTags() {
	v := &s.order.tags
	v.managing, v.edit, v.choosing, v.deleting, v.name, v.query = true, "", "", "", "", ""
}

// manageTagsDialog is Manage tags, while it shows: the list of the
// tags, a tag, its Choose items… or the question before it's deleted.
// Escape goes back a step.
func (s *shell) manageTagsDialog(c *ui.Context) {
	v := &s.order.tags
	if !v.managing {
		return
	}
	tags := s.svc.Collection.Tags()
	i := tagAt(tags, v.edit)
	if v.edit != "" && i < 0 {
		// The tag went meanwhile: the list shows again.
		v.edit, v.choosing, v.deleting = "", "", ""
	}
	open := true
	switch {
	case i >= 0 && v.deleting == v.edit:
		s.deleteTagView(c, &open, tags[i])
	case i >= 0 && v.choosing == v.edit:
		s.chooseItemsView(c, &open, tags[i])
	case i >= 0:
		s.editTagView(c, &open, tags[i])
	default:
		s.tagListView(c, &open, tags)
	}
	if !open {
		s.manageBack(tags)
	}
}

// manageBack goes a step back in Manage tags, of the tags of tags: from
// a question or Choose items to its tag, from a tag, named as typed, to
// the list, and from the list out.
func (s *shell) manageBack(tags []library.Tag) {
	v := &s.order.tags
	switch {
	case v.deleting != "":
		v.deleting = ""
	case v.choosing != "":
		v.choosing, v.query = "", ""
	case v.edit != "":
		if i := tagAt(tags, v.edit); i >= 0 {
			s.renameTag(tags[i])
		}
		v.edit = ""
	default:
		v.managing = false
	}
}

// manageDone closes Manage tags, the tag shown named as typed.
func (s *shell) manageDone(tags []library.Tag) {
	v := &s.order.tags
	if i := tagAt(tags, v.edit); i >= 0 && v.deleting == "" {
		s.renameTag(tags[i])
	}
	v.managing, v.edit, v.choosing, v.deleting = false, "", "", ""
}

// tagListView is the list of Manage tags: a row of each tag, which opens
// it and moves among the others, by drag or with Alt and the arrows, and
// the field of a new tag.
func (s *shell) tagListView(c *ui.Context, open *bool, tags []library.Tag) {
	k := &s.lk
	v := &s.order.tags
	keys := make([]string, len(tags))
	for i, t := range tags {
		keys[i] = t.ID
	}
	s.dialog(c, open, dialogOptions{key: "manage-tags", title: locales.LibraryTagsManage(), width: 520}, func() {
		if len(tags) == 0 {
			ui.Text(c, locales.LibraryTagsNone()).FontSize(k.bodySize()).TextColor(k.muted)
		} else {
			ui.Text(c, locales.LibraryTagsManageHint()).FontSize(k.smallSize()).TextColor(k.muted)
			ui.Column(c.Key("manage-tags-list")).Gap(2).Children(func() {
				for _, t := range tags {
					s.tagRow(c, t, keys)
				}
			})
		}
		s.newTagField(c, "manage-tags-new", &v.name, tags, func(library.Tag) {})
	}, func() {
		if s.dialogButton(c, "manage-tags-done", locales.LibraryTagsDone(), true).Clicked() {
			s.manageDone(tags)
		}
	})
}

// tagRow is the row of the tag t in Manage tags, among the tags of keys:
// its colour, its name and how many items it's on. A click, or Enter,
// opens it.
func (s *shell) tagRow(c *ui.Context, t library.Tag, keys []string) {
	k := &s.lk
	row := s.buttonBase(c.Key("tag-row-" + t.ID)).Row().Gap(12).AlignItems(ui.Center).PaddingY(10).PaddingX(12).
		Radius(k.radiusSm).Label(locales.LibraryTagsRow(t.Name, len(t.Items))).Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(k.hover)
	}
	if row.Clicked() {
		v := &s.order.tags
		v.edit, v.rename = t.ID, t.Name
	}
	s.orderable(row, "tags", t.ID, keys, false, func(string, string) bool { return true }, func(from, onto string) {
		if err := s.svc.Collection.MoveTag(from, onto, nil); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	})
	row.Children(func() {
		s.tagDot(c, t.Color, s.dialogGround(), 14, ui.Color{})
		ui.Text(c, t.Name).FontSize(k.bodySize()).SingleLine().Grow(1).MinWidth(0)
		ui.Text(c, count(len(t.Items))).FontSize(k.smallSize()).TextColor(k.muted).FontFeatures("tnum")
		ui.Icon(c, lineIcon(c, "chevronRight")).FontSize(14).TextColor(k.muted).Shrink(0)
	})
}

// renameTag names the tag t as typed, if that changed it. Named as
// another, it becomes one with it, which the choice of the chips and
// Manage tags follow.
func (s *shell) renameTag(t library.Tag) {
	v := &s.order.tags
	name := library.TagName(v.rename)
	if name == "" || name == t.Name {
		v.rename = t.Name
		return
	}
	into, err := s.svc.Collection.RenameTag(t.ID, name)
	if err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	if into != t.ID {
		s.swapTagFilter(t.ID, into)
		for _, id := range []*string{&v.edit, &v.choosing, &v.deleting} {
			if *id == t.ID {
				*id = into
			}
		}
	}
	v.rename = name
}

// editTagView is a tag of Manage tags: its name, which takes what is
// typed on Enter, Back or Done; its colour, of the palette; Choose
// items…, and Delete tag.
func (s *shell) editTagView(c *ui.Context, open *bool, t library.Tag) {
	k := &s.lk
	v := &s.order.tags
	tags := s.svc.Collection.Tags()
	s.dialog(c, open, dialogOptions{key: "manage-tags-edit", title: locales.LibraryTagsEditTitle(), width: 520}, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, locales.LibraryTagsName()).FontSize(k.smallSize()).TextColor(k.muted)
			in := ui.TextInput(c.Key("tag-name"), &v.rename).Label(locales.LibraryTagsName()).MaxLength(library.MaxTagName).
				Width(320).Shrink(1).MinWidth(160)
			if in.Submitted() {
				s.renameTag(t)
			}
			ui.Text(c, locales.LibraryTagsOnCount(len(t.Items))).FontSize(k.smallSize()).TextColor(k.muted)
		})
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, locales.LibraryTagsColour()).FontSize(k.smallSize()).TextColor(k.muted)
			s.tagSwatches(c, t)
		})
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if s.dialogButton(c, "tag-choose", locales.LibraryTagsChooseItems(), false).Clicked() {
				s.renameTag(t)
				v.choosing, v.query = v.edit, ""
			}
			if s.dialogButton(c, "tag-delete", locales.LibraryTagsDelete(), false).Clicked() {
				v.deleting = t.ID
			}
		})
	}, func() {
		if s.dialogButton(c, "tag-back", locales.LibraryTagsBack(), false).Clicked() {
			s.manageBack(tags)
		}
		if s.dialogButton(c, "tag-done", locales.LibraryTagsDone(), true).Clicked() {
			s.manageDone(tags)
		}
	})
}

// tagSwatches are the colours of the palette, as radio buttons, the one
// of the tag t chosen: a ring around it, and a check on it.
func (s *shell) tagSwatches(c *ui.Context, t library.Tag) {
	k := &s.lk
	ground := s.dialogGround()
	chosen := tagHueOf(t.Color).name
	ui.Row(c.Key("tag-colours")).Role(ui.RoleRadioGroup).Label(locales.LibraryTagsColour()).Wrap().Gap(6).Children(func() {
		for _, h := range tagPalette {
			on := h.name == chosen
			name := locales.LibraryTagsColourName(h.name)
			b := s.buttonBase(c.Key("tag-colour-"+h.name)).Size(34, 34).Radius(17).Center().Role(ui.RoleRadio).Checked(on).
				Label(name).Tooltip(name).Cursor(ui.CursorPointer)
			switch {
			case on:
				b.Border(2, k.text)
			case b.Hovered():
				b.Border(2, k.muted)
			}
			if b.Clicked() && !on {
				if err := s.svc.Collection.RecolorTag(t.ID, h.name); err != nil {
					s.notes = append(s.notes, noticeOf(err))
				}
			}
			ink := s.tagInk(h.name, ground)
			b.Children(func() {
				ui.Box(c).Size(24, 24).Radius(12).Center().Background(ink).Children(func() {
					if on {
						ui.Icon(c, icon("check")).FontSize(16).TextColor(col(inkOn(nrgba(ink))))
					}
				})
			})
		}
	})
}

// chooseShown is how many items Choose items lists at once: a search
// finds the others.
const chooseShown = 300

// chooseItemsView is Choose items… of the tag t: every playlist and album
// of the library with a check box, which puts the tag on it or takes it
// off at once, and a search.
func (s *shell) chooseItemsView(c *ui.Context, open *bool, t library.Tag) {
	k := &s.lk
	v := &s.order.tags
	tags := s.svc.Collection.Tags()
	items, loading := s.taggables()
	query := strings.TrimSpace(v.query)
	var shown []taggedItem
	for _, it := range items {
		if library.Matches(it.title(), query) {
			shown = append(shown, it)
		}
	}
	s.dialog(c, open, dialogOptions{key: "manage-tags-choose", title: locales.LibraryTagsChooseTitle(t.Name), width: 600}, func() {
		ui.Text(c, locales.LibraryTagsChooseHint(t.Name)).FontSize(k.smallSize()).TextColor(k.muted)
		s.filterBox(c, "tag-items-search", locales.LibraryTagsSearchPlaceholder(), locales.LibraryTagsSearchLabel(), &v.query, false,
			func() { v.query = "" })
		ui.Column(c.Key("tag-items")).Gap(2).Children(func() {
			for _, it := range shown[:min(len(shown), chooseShown)] {
				had := slices.Contains(s.tagsOn(it.ids), t.ID)
				if s.checkRow(c, "tag-item-"+it.id(), it.title(), it.kindOf(), had, nil) {
					var err error
					if had {
						err = s.svc.Collection.ChangeTagged(t.ID, nil, it.ids)
					} else {
						err = s.svc.Collection.ChangeTagged(t.ID, []library.Mark{it.mark()}, nil)
					}
					if err != nil {
						s.notes = append(s.notes, noticeOf(err))
					}
				}
			}
		})
		switch {
		case len(shown) > chooseShown:
			ui.Text(c, locales.LibraryTagsSomeShown(chooseShown, len(shown))).FontSize(k.smallSize()).TextColor(k.muted)
		case query != "" && len(shown) == 0:
			s.emptyLine(c, locales.LibraryTagsNoMatch(query))
		}
		if loading {
			ui.Spinner(c.Key("tag-items-coming")).Label(locales.LibraryLoadingMore())
		}
	}, func() {
		if s.dialogButton(c, "tag-items-back", locales.LibraryTagsBack(), false).Clicked() {
			s.manageBack(tags)
		}
		if s.dialogButton(c, "tag-items-done", locales.LibraryTagsDone(), true).Clicked() {
			s.manageDone(tags)
		}
	})
}

// deleteTagView asks before the tag t is deleted, which takes it off all
// it's on, and out of the chips' choice.
func (s *shell) deleteTagView(c *ui.Context, open *bool, t library.Tag) {
	v := &s.order.tags
	body := locales.LibraryTagsDeleteBody(len(t.Items))
	s.dialog(c, open, dialogOptions{key: "manage-tags-delete", title: locales.LibraryTagsDeleteTitle(t.Name), width: 480, alert: true, message: body}, func() {
		ui.Text(c, body).FontSize(s.lk.bodySize())
	}, func() {
		if s.dialogButton(c, "tag-delete-cancel", locales.ShortcutsCancel(), false).Clicked() {
			v.deleting = ""
		}
		if s.dialogButton(c, "tag-delete-ok", locales.LibraryTagsDeleteButton(), true).AutoFocus().Clicked() {
			if err := s.svc.Collection.DeleteTag(t.ID); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
			s.swapTagFilter(t.ID, "")
			v.edit, v.deleting, v.choosing = "", "", ""
		}
	})
}

// The icon of Manage tags: a tag, with its hole.
func init() {
	icons["tag"] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square" stroke-linejoin="miter"><path d="M3.5 3.5h8.5l8.5 8.5-8.5 8.5-8.5-8.5zM8 8h.01"/></svg>`))
}

func init() { addSettings(colorTypeRows, 65, (*shell).tagFiltersRow) }

// tagFiltersRow shows the row of the tags' chips above Library › Mixes,
// in Settings › Appearance › Colour and type.
func (s *shell) tagFiltersRow(c *ui.Context) {
	s.row(c, locales.LibraryTagsShowFilters(), locales.LibraryTagsShowFiltersDescription(), func() {
		if s.switchOf(c.Key("tag-filters"), &s.set.TagFilters).Label(locales.LibraryTagsShowFilters()).Changed() {
			s.save()
		}
	}, "tags", "chips", "labels")
}
