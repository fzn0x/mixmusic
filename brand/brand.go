// Package brand holds MixMusic's brand files: the app's icon, which
// mygo.config.ts names, the tray's, which the app embeds, and the mark of
// its logo, the pink rings with a play button, for the code that draws it.
// The app builds them in, so they stay out of resources/, whose every file
// `mygo build` ships beside the app (#303).
//
// discord/ holds the badges of the Discord status, which tools/discordbadges
// draws: Discord reads them from GitHub, and the app doesn't build them in.
package brand

import _ "embed"

// Mark is MixMusic's mark, an SVG of one path, without the black tile of
// the app's icon.
//
//go:embed mark.svg
var Mark []byte
