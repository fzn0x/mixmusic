package library

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Podcasts returns the podcasts the user follows, the last followed first.
func (l *Library) Podcasts() []media.Podcast { return orEmpty(l.file.Get().Podcasts) }

// Follows reports whether the user follows the podcast id.
func (l *Library) Follows(id string) bool {
	return slices.ContainsFunc(l.file.Get().Podcasts, func(p media.Podcast) bool { return p.ID == id })
}

// SetFollowed follows a podcast, or stops following it. A podcast
// followed again moves first.
func (l *Library) SetFollowed(p media.Podcast, on bool) error {
	return l.file.Update(func(d *Data) error {
		d.Podcasts = slices.DeleteFunc(d.Podcasts, func(x media.Podcast) bool { return x.ID == p.ID })
		if on {
			d.Podcasts = slices.Insert(d.Podcasts, 0, p.Summary())
		}
		return nil
	})
}
