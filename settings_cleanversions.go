package main

import "github.com/egoist/mygo/ui"

func init() { addSettings("playback", 45, (*shell).explicitContentSettings) }

// explicitContentSettings choose the version an album that comes explicit
// and clean shows and opens on, in Settings › Playback (versions.go).
func (s *shell) explicitContentSettings(c *ui.Context) {
	s.group(c, "Explicit content", func() {
		s.row(c, "Prefer clean versions", "An album that comes explicit and clean shows its clean version, and its page opens on it. Its version switch still has the explicit one.", func() {
			if s.switchOf(c.Key("clean-versions"), &s.set.CleanVersions).Label("Prefer clean versions").Changed() {
				s.save()
			}
		})
	})
}
