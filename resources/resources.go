// Package resources holds MixMusic's brand files for the code that draws
// them: the mark of its logo, the pink rings with a play button.
package resources

import _ "embed"

// Mark is MixMusic's mark, an SVG of one path, without the black tile of
// the app's icon.
//
//go:embed mark.svg
var Mark []byte
