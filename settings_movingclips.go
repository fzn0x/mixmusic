package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(cacheRows, 10, (*shell).movingClipsRow) }

// movingClipsRow shows the clips of moving covers kept, in Settings ›
// Playback › Offline cache, with Clear moving covers, and bounds them,
// once a source of clips is set (movingclip.go): until then none is kept.
func (s *shell) movingClipsRow(c *ui.Context) {
	if !s.svc.Player.HasClips() {
		return
	}
	// The store's index is read for what it holds in every frame the row
	// shows: the search finds the row by what it always says.
	if s.findable(locales.MovingClipsTitle(), locales.MovingClipsKept(), "clips animated") {
		use := s.svc.System.MovingCoverUsage()
		s.row(c, locales.MovingClipsTitle(), locales.MovingClipsLine(use.Clips, byteSize(use.Bytes)), func() {
			if s.button(c, "clear-moving-covers", locales.CacheSettingsClear(), false).Label(locales.MovingClipsClearLabel()).Disabled(use.Clips == 0).Clicked() {
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
	what := locales.MovingClipsAtMost(chosen)
	switch s.set.MovingCoversMB {
	case 0:
		what = locales.MovingClipsOff()
	case settings.UnlimitedMovingCovers:
		what = locales.MovingClipsUnlimitedDesc()
	}
	s.row(c, locales.MovingClipsSize(), what, func() {
		if s.selectOf(c.Key("moving-covers-size"), &chosen, labels).Width(160).Label(locales.MovingClipsSize()).Changed() {
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
		return locales.MovingClipsUnlimited()
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
			s.notes = append(s.notes, notice(locales.MovingClipsCleared()))
		})
	}()
}
