package app

import (
	"errors"
	"fmt"
	"slices"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/locales"
)

// The tags of the library (library/tags.go): the user tags their mixes,
// the playlists of their services' libraries, their saved albums and the
// albums on this computer, and filters the Library by them. Tags stay in
// MixMusic, never reaching a service.

// Tags returns the tags, with what each is on, in the user's order.
func (c *Collection) Tags() []library.Tag { return c.a.library.Tags() }

// TagsOf returns the IDs of the tags on the item id, in the tags' order.
func (c *Collection) TagsOf(id string) []string { return c.a.library.TagsOf(id) }

// NewTag makes a tag named name, of the colour color, last of the tags,
// or returns the tag of that name, regardless of case and spacing.
func (c *Collection) NewTag(name, color string) (library.Tag, error) {
	t, err := c.a.library.NewTag(name, color)
	return t, tagError(err)
}

// RenameTag names the tag of id name: one with the name of another
// becomes one with it, whose ID into is.
func (c *Collection) RenameTag(id, name string) (into string, err error) {
	into, err = c.a.library.RenameTag(id, name)
	return into, tagError(err)
}

// RecolorTag gives the tag of id the colour color.
func (c *Collection) RecolorTag(id, color string) error {
	return tagError(c.a.library.RecolorTag(id, color))
}

// DeleteTag deletes the tag of id, which comes off all it was on.
func (c *Collection) DeleteTag(id string) error { return tagError(c.a.library.DeleteTag(id)) }

// MoveTag moves the tag of id onto the tag of onto, among the tags of
// among: every other tag keeps its place.
func (c *Collection) MoveTag(id, onto string, among []string) error {
	return tagError(c.a.library.MoveTag(id, onto, among))
}

// ChangeTagged puts the tag of id on the items of on, mixes, playlists
// and albums, and takes it off the items of the IDs of off. Those of a
// service are kept with the account of it the user is signed in to, as a
// pin is, and follow its whole lists as pins do.
func (c *Collection) ChangeTagged(id string, on []library.Mark, off []string) error {
	on = slices.Clone(on)
	for i := range on {
		on[i].Account = c.a.markAccount(on[i].ID)
	}
	return tagError(c.a.library.ChangeTagged(id, on, off))
}

// tagError is err, an error of the library's tags, as the windows show
// it, in the interface's language.
func tagError(err error) error {
	switch {
	case errors.Is(err, library.ErrTagName):
		return wordedLater("a tag needs a name", locales.LibraryTagsNeedName, err)
	case errors.Is(err, library.ErrNoTag):
		return wordedLater("the tag is gone", locales.LibraryTagsGone, err)
	case errors.Is(err, library.ErrTooManyTags):
		return wordedLater(fmt.Sprintf("there are %d tags already, the most there can be", library.MaxTags),
			func() string { return locales.LibraryTagsTooMany(library.MaxTags) }, err)
	}
	return err
}
