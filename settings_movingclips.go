package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

func init() { addSettings(cacheRows, 10, (*shell).movingClipsRow) }

// movingClipsKept is what the row of the clips of moving covers kept
// always says.
const movingClipsKept = "Albums' moving covers that played loop again without the network."

// movingClipsRow shows the clips of moving covers kept, in Settings ›
// Playback › Offline cache, with Clear moving covers, and bounds them,
// once a source of clips is set (movingclip.go): until then none is kept.
func (s *shell) movingClipsRow(c *ui.Context) {
	if !s.svc.Player.HasClips() {
		return
	}
	// The store's index is read for what it holds in every frame the row
	// shows: the search finds the row by what it always says.
	if s.findable("Moving covers kept", movingClipsKept, "clips animated") {
		use := s.svc.System.MovingCoverUsage()
		s.row(c, "Moving covers kept", fmt.Sprintf("%s in %s. "+movingClipsKept, plural(use.Clips, "clip"), byteSize(use.Bytes)), func() {
			if s.button(c, "clear-moving-covers", "clear", false).Label("Clear moving covers").Disabled(use.Clips == 0).Clicked() {
				s.clearMovingClips()
			}
		}, "clips animated")
	}
	sizes := movingClipSizes
	if !slices.Contains(sizes, s.set.MovingCoversMB) {
		sizes = append(slices.Clone(movingClipSizes), s.set.MovingCoversMB)
		slices.SortFunc(sizes, func(a, b int) int { return clipOrder(a) - clipOrder(b) })
	}
	labels := make([]string, len(sizes))
	for i, mb := range sizes {
		labels[i] = movingClipSizeLabel(mb)
	}
	chosen := movingClipSizeLabel(s.set.MovingCoversMB)
	what := chosen + " at most."
	switch s.set.MovingCoversMB {
	case 0:
		what = "Off: MixMusic keeps only the clip that plays."
	case settings.UnlimitedMovingCovers:
		what = "Unlimited: every clip that played stays."
	}
	s.row(c, "Size of moving covers", what, func() {
		if s.selectOf(c.Key("moving-covers-size"), &chosen, labels).Width(160).Label("Size of moving covers").Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				s.set.MovingCoversMB = sizes[i]
				s.save()
			}
		}
	}, "storage disk clips")
}

// movingClipSizes are the sizes the clips of moving covers take, in MB:
// 0 keeps only the clip that plays, and Unlimited every one.
var movingClipSizes = []int{0, 64, 128, 256, 512, 1024, 2048, 4096, 8192, settings.UnlimitedMovingCovers}

// clipOrder orders the sizes of the clips, Unlimited last.
func clipOrder(mb int) int {
	if mb < 0 {
		return 1 << 30
	}
	return mb
}

// movingClipSizeLabel names a size of the clips of moving covers.
func movingClipSizeLabel(mb int) string {
	if mb < 0 {
		return "Unlimited"
	}
	return cacheSizeLabel(mb)
}

// clearMovingClips removes the clips of moving covers kept, beside the
// main thread, and has the sleeve look for them anew.
func (s *shell) clearMovingClips() {
	go func() {
		err := s.svc.System.ClearMovingCovers()
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			for key := range s.loads {
				if strings.HasPrefix(key, "clip:") {
					s.forget(key)
				}
			}
			s.notes = append(s.notes, notice("Moving covers cleared"))
		})
	}()
}
