package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(explicitRows, 20, (*shell).hideExplicitRow) }

// hideExplicitDescription says what Hide explicit content does.
const hideExplicitDescription = "Songs, albums and videos marked explicit don't show in the search, on Home, on artists' pages or in radios, and never play. Your playlists, liked songs, library and queue list them dimmed."

// hideExplicitRow hides explicit content, in Settings › Playback ›
// Explicit content (explicit.go).
func (s *shell) hideExplicitRow(c *ui.Context) {
	s.row(c, "Hide explicit content", hideExplicitDescription, func() {
		if s.switchOf(c.Key("hide-explicit"), &s.set.HideExplicit).Label("Hide explicit content").Changed() {
			s.save()
		}
	}, "parental", "children", "kids", "family", "clean", "mature", "advisory")
}
