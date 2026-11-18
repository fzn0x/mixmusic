package library

// Tags of the library: the user marks their mixes, the playlists of their
// services' libraries, their saved albums and the albums on this computer
// with tags of their own, each a name and a colour, and filters the
// Library by them. A tag keeps what it's on as marks, by their IDs, in
// the order they were tagged, and an item takes any number of tags. Tags
// list in the user's order, a new one last. Names that differ only in
// case or spacing are one tag. What a service lists leaves its tags as
// its pin does (KeepListed); hiding an item, or deleting a mix, takes its
// tags off. A tag on nothing stays. Tags never reach a service.

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MaxTags is how many tags the library keeps at most, and MaxTagName how
// many characters a tag's name holds at most.
const (
	MaxTags    = 200
	MaxTagName = 64
)

// maxTagColor is how long the name of a tag's colour is at most, in
// bytes.
const maxTagColor = 32

var (
	// ErrNoTag is returned for tags that do not exist.
	ErrNoTag = errors.New("library: no such tag")
	// ErrTagName is returned for a name with nothing to show.
	ErrTagName = errors.New("library: a tag needs a name")
	// ErrTooManyTags is returned for a new tag past MaxTags.
	ErrTooManyTags = errors.New("library: too many tags")
)

// Tag is a tag of the user's: its name, its colour, by the name the
// window's palette gives it, and what it's on, in the order it was
// tagged.
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Items []Mark `json:"items"`
}

// TagName is name as a tag keeps it: in Unicode's NFKC form, without
// spaces around it, each run of spaces in it made one, and cut to
// MaxTagName characters.
func TagName(name string) string {
	name = strings.Join(strings.Fields(norm.NFKC.String(name)), " ")
	if utf8.RuneCountInString(name) > MaxTagName {
		name = strings.TrimSpace(string([]rune(name)[:MaxTagName]))
	}
	return name
}

// tagColor is color as a tag keeps it: cut to maxTagColor bytes.
func tagColor(color string) string {
	color = strings.TrimSpace(color)
	if len(color) > maxTagColor {
		color = color[:maxTagColor]
	}
	return color
}

// tagMemo is the tags as they listed last, of the library at version, and
// the tags of each item, by its ID, in the tags' order: the Library asks
// for them of every tile, in every frame.
type tagMemo struct {
	mu      sync.Mutex
	version uint64
	set     bool
	list    []Tag
	of      map[string][]string
}

// tagged returns the tags, and the IDs of the tags of each item, as the
// file has them now. Both are shared: don't change them.
func (l *Library) tagged() ([]Tag, map[string][]string) {
	m := &l.tags
	m.mu.Lock()
	defer m.mu.Unlock()
	if v := l.file.Version(); !m.set || m.version != v {
		m.list = l.file.Get().Tags
		m.of = map[string][]string{}
		for _, t := range m.list {
			for _, it := range t.Items {
				m.of[it.ID] = append(m.of[it.ID], t.ID)
			}
		}
		m.version, m.set = v, true
	}
	return m.list, m.of
}

// Tags returns the tags, with what each is on, in the user's order. Their
// Items are shared: don't change them.
func (l *Library) Tags() []Tag {
	list, _ := l.tagged()
	return slices.Clone(orEmpty(list))
}

// TagsOf returns the IDs of the tags on the item id, in the tags' order.
// The slice is shared: don't change it.
func (l *Library) TagsOf(id string) []string {
	_, of := l.tagged()
	return of[id]
}

// NewTag makes a tag named name, of the colour color, last of the tags,
// and returns it. A tag of that name already, as TagName keeps it and
// regardless of case, is returned as it is instead.
func (l *Library) NewTag(name, color string) (Tag, error) {
	name = TagName(name)
	if name == "" {
		return Tag{}, ErrTagName
	}
	var made Tag
	err := l.file.Update(func(d *Data) error {
		if i := tagNamed(d.Tags, name, ""); i >= 0 {
			made = d.Tags[i]
			return nil
		}
		if len(d.Tags) >= MaxTags {
			return ErrTooManyTags
		}
		made = Tag{ID: newID(), Name: name, Color: tagColor(color), Items: []Mark{}}
		d.Tags = append(d.Tags, made)
		return nil
	})
	made.Items = slices.Clone(made.Items)
	return made, err
}

// RenameTag names the tag of id name. Another tag of that name already,
// as NewTag tells, becomes one with it: it takes what this one is on
// after its own, and the name. into is the ID of the tag of the name.
func (l *Library) RenameTag(id, name string) (into string, err error) {
	name = TagName(name)
	if name == "" {
		return "", ErrTagName
	}
	err = l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Tags, func(t Tag) bool { return t.ID == id })
		if i < 0 {
			return ErrNoTag
		}
		j := tagNamed(d.Tags, name, id)
		if j < 0 {
			d.Tags[i].Name, into = name, id
			return nil
		}
		for _, m := range d.Tags[i].Items {
			if !slices.ContainsFunc(d.Tags[j].Items, func(x Mark) bool { return x.ID == m.ID }) {
				d.Tags[j].Items = append(d.Tags[j].Items, m)
			}
		}
		d.Tags[j].Name, into = name, d.Tags[j].ID
		d.Tags = slices.Delete(d.Tags, i, i+1)
		return nil
	})
	return into, err
}

// RecolorTag gives the tag of id the colour color.
func (l *Library) RecolorTag(id, color string) error {
	return l.editTag(id, func(t *Tag) { t.Color = tagColor(color) })
}

// editTag changes the tag of id with fn.
func (l *Library) editTag(id string, fn func(t *Tag)) error {
	return l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Tags, func(t Tag) bool { return t.ID == id })
		if i < 0 {
			return ErrNoTag
		}
		fn(&d.Tags[i])
		return nil
	})
}

// DeleteTag deletes the tag of id, which comes off all it was on.
func (l *Library) DeleteTag(id string) error {
	return l.file.Update(func(d *Data) error {
		n := len(d.Tags)
		d.Tags = slices.DeleteFunc(d.Tags, func(t Tag) bool { return t.ID == id })
		if len(d.Tags) == n {
			return ErrNoTag
		}
		return nil
	})
}

// MoveTag moves the tag of id onto the tag of onto, as a drag drops it,
// among the tags of among: every other tag keeps its place (MovedAmong).
func (l *Library) MoveTag(id, onto string, among []string) error {
	return l.file.Update(func(d *Data) error {
		if !slices.ContainsFunc(d.Tags, func(t Tag) bool { return t.ID == id }) {
			return ErrNoTag
		}
		d.Tags = movedAmong(d.Tags, func(t Tag) string { return t.ID }, among, id, onto)
		return nil
	})
}

// ChangeTagged puts the tag of id on the items of on, after those it's
// on, and takes it off the items of the IDs of off. An item it's on
// already keeps its place, and what its mark knew that the new one
// doesn't.
func (l *Library) ChangeTagged(id string, on []Mark, off []string) error {
	return l.editTag(id, func(t *Tag) {
		t.Items = slices.DeleteFunc(t.Items, func(m Mark) bool { return slices.Contains(off, m.ID) })
		for _, m := range on {
			if j := slices.IndexFunc(t.Items, func(x Mark) bool { return x.ID == m.ID }); j >= 0 {
				t.Items[j] = keptMark(t.Items[j], m)
				continue
			}
			t.Items = append(t.Items, m)
		}
	})
}

// tagNamed returns where the tag named name is among tags, as TagName
// keeps names and regardless of case, but the tag of besides: -1 for
// none.
func tagNamed(tags []Tag, name, besides string) int {
	return slices.IndexFunc(tags, func(t Tag) bool {
		return t.ID != besides && strings.EqualFold(TagName(t.Name), name)
	})
}

// untagged returns tags without the item id on any of them. The tags
// stay, on nothing as they may be.
func untagged(tags []Tag, id string) []Tag {
	for i := range tags {
		tags[i].Items = slices.DeleteFunc(tags[i].Items, func(m Mark) bool { return m.ID == id })
	}
	return tags
}
