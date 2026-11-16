package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("playback", 45, (*shell).explicitContentSettings) }

// explicitRows are the rows of Settings › Playback › Explicit content,
// which register from their features' files.
const explicitRows = "playback › Explicit content"

// explicitContentSettings show the rows of explicit content: the version
// an album that comes explicit and clean shows, and whether explicit
// content shows at all.
func (s *shell) explicitContentSettings(c *ui.Context) {
	s.group(c, locales.CleanVersionsGroup(), func() { s.settingsOf(c, explicitRows) })
}

func init() { addSettings(explicitRows, 10, (*shell).cleanVersionsRow) }

// cleanVersionsRow chooses the version an album that comes explicit and
// clean shows and opens on (versions.go): the clean one, and no other,
// while explicit content is hidden (explicit.go).
func (s *shell) cleanVersionsRow(c *ui.Context) {
	description := locales.CleanVersionsDesc()
	on, fixed := s.set.PrefersClean(), s.set.HideExplicit
	if fixed {
		description = locales.CleanVersionsFixed()
	}
	s.row(c, locales.CleanVersionsPrefer(), description, func() {
		if s.switchOf(c.Key("clean-versions"), &on).Label(locales.CleanVersionsPrefer()).Disabled(fixed).Changed() && !fixed {
			s.set.CleanVersions = on
			s.save()
		}
	})
}
