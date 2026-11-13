package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// BenchmarkGround times a frame of the home page as it scrolls over the
// grounds of the themes that paint one under the whole window, and over
// Mix's, which paints none, for a measure: a ground that shades the
// window many times over makes every frame slow to draw (#325).
func BenchmarkGround(b *testing.B) {
	for _, theme := range []string{"mix", "aurora", "glasshouse", "greenroom", "sakura", "terminal", "anime", "mochi"} {
		b.Run(theme, func(b *testing.B) {
			s := newTestShell(b)
			wear(s, theme, nil)
			tt := testerOf(b, s, 1280, 820)
			s.loads["home:netease"] = &loaded[[]media.Shelf]{done: true, value: shelves(10, 25)}
			show(s, tt, "/")
			scrollFrames(b, tt)
		})
	}
}
