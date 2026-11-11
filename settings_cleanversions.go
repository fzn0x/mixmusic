package main

import "github.com/egoist/mygo/ui"

func init() { addSettings("playback", 45, (*shell).explicitContentSettings) }

// explicitRows are the rows of Settings › Playback › Explicit content,
// which register from their features' files.
const explicitRows = "playback › Explicit content"

// explicitContentSettings show the rows of explicit content: the version
// an album that comes explicit and clean shows, and whether explicit
// content shows at all.
func (s *shell) explicitContentSettings(c *ui.Context) {
	s.group(c, "Explicit content", func() { s.settingsOf(c, explicitRows) })
}

func init() { addSettings(explicitRows, 10, (*shell).cleanVersionsRow) }

// cleanVersionsRow chooses the version an album that comes explicit and
// clean shows and opens on (versions.go): the clean one, and no other,
// while explicit content is hidden (explicit.go).
func (s *shell) cleanVersionsRow(c *ui.Context) {
	description := "An album that comes explicit and clean shows its clean version, and its page opens on it. Its version switch still has the explicit one."
	on, fixed := s.set.PrefersClean(), s.set.HideExplicit
	if fixed {
		description = "On while explicit content is hidden: an album that comes explicit and clean shows its clean version alone."
	}
	s.row(c, "Prefer clean versions", description, func() {
		if s.switchOf(c.Key("clean-versions"), &on).Label("Prefer clean versions").Disabled(fixed).Changed() && !fixed {
			s.set.CleanVersions = on
			s.save()
		}
	})
}
