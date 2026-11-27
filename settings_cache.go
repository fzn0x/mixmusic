package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// cacheView holds transient UI state for Settings › Playback › Offline cache.
type cacheView struct {
	moving              bool
	moveDone, moveTotal int64
	customSize          bool
}

// cacheSizePresets are the preset audio cache limits, in MB: 0 turns it off
// (kept first for keyboard selection) and UnlimitedCache keeps every track.
var cacheSizePresets = []int{0, 512, 1024, 4096, 16384, settings.UnlimitedCache}

func init() {
	addSettings(cacheRows, 20, (*shell).cacheSizeRows)
	addSettings(cacheRows, 30, (*shell).cacheLocationRows)
}

// cacheSizeRows renders the cache limit selector and, when Custom is active,
// a numeric size input allowing any limit between 1 MB and 1 TB.
func (s *shell) cacheSizeRows(c *ui.Context) {
	isPreset := slices.Contains(cacheSizePresets, s.set.CacheSizeMB)
	customMode := s.cache.customSize || !isPreset

	labels := make([]string, len(cacheSizePresets)+1)
	for i, mb := range cacheSizePresets {
		labels[i] = cacheSizeLabel(mb)
	}
	customLabel := locales.CacheSettingsSizeCustom()
	labels[len(cacheSizePresets)] = customLabel

	chosen := cacheSizeLabel(s.set.CacheSizeMB)
	if customMode {
		chosen = customLabel
	}
	what := locales.CacheSettingsOff()
	switch {
	case s.set.CacheSizeMB < 0:
		what = locales.CacheSettingsUnlimited()
	case s.set.CacheSizeMB > 0:
		what = locales.CacheSettingsLimit(cacheSizeLabel(s.set.CacheSizeMB))
	}

	s.row(c, locales.CacheSettingsSize(), what, func() {
		if s.selectOf(c.Key("cache-size"), &chosen, labels).Width(160).Label(locales.CacheSettingsSizeLabel()).Changed() {
			if chosen == customLabel {
				s.cache.customSize = true
				delete(s.drafts, "cache-custom-size")
			} else if i := slices.Index(labels[:len(cacheSizePresets)], chosen); i >= 0 {
				s.cache.customSize = false
				s.set.CacheSizeMB = cacheSizePresets[i]
				delete(s.drafts, "cache-custom-size")
				s.save()
			}
		}
	}, "storage disk custom unlimited")

	if !customMode {
		return
	}
	saved := cacheSizeInputText(s.set.CacheSizeMB)
	s.row(c, locales.CacheSettingsCustomTitle(), locales.CacheSettingsCustomHint(), func() {
		in, _ := s.settingInput(c, "cache-custom-size", saved, func(raw string) {
			mb, ok := parseCacheSizeMB(raw)
			if !ok {
				s.notes = append(s.notes, noticeOf(errors.New(locales.CacheSettingsCustomRange())))
				delete(s.drafts, "cache-custom-size")
				return
			}
			s.cache.customSize = !slices.Contains(cacheSizePresets, mb)
			s.set.CacheSizeMB = mb
			delete(s.drafts, "cache-custom-size")
			s.save()
		})
		in.Placeholder(locales.CacheSettingsCustomPlaceholder()).Width(180).Label(locales.CacheSettingsCustomTitle())
	}, "storage disk custom size mb gb tb")
}

func cacheSizeInputText(mb int) string {
	if mb <= 0 {
		return ""
	}
	if mb%1024 == 0 {
		return fmt.Sprintf("%d GB", mb/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

// parseCacheSizeMB parses a user-entered cache size such as "2500",
// "2500 MB", "2.5 GB", or "1 TB" into whole megabytes within [1, MaxCacheSizeMB].
func parseCacheSizeMB(raw string) (int, bool) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return 0, false
	}
	var mult float64 = 1
	for _, u := range []struct {
		suffix string
		factor float64
	}{
		{"tib", 1024 * 1024},
		{"tb", 1024 * 1024},
		{"t", 1024 * 1024},
		{"gib", 1024},
		{"gb", 1024},
		{"g", 1024},
		{"mib", 1},
		{"mb", 1},
		{"m", 1},
	} {
		if num, ok := strings.CutSuffix(s, u.suffix); ok {
			s = strings.TrimSpace(num)
			mult = u.factor
			break
		}
	}
	s = strings.ReplaceAll(s, ",", ".")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) || val <= 0 {
		return 0, false
	}
	mb := math.Round(val * mult)
	if mb < 1 || mb > float64(settings.MaxCacheSizeMB) {
		return 0, false
	}
	return int(mb), true
}

// cacheLocationRows renders the cache folder row and the Merge with
// downloads toggle in Settings › Playback › Offline cache.
func (s *shell) cacheLocationRows(c *ui.Context) {
	k := &s.lk
	folder, def := s.svc.System.CacheFolder(), s.svc.System.DefaultCacheFolder()
	problem := s.svc.System.CacheProblem()
	s.rowWith(c, locales.CacheSettingsFolder(), func() {
		ui.Text(c, folder)
		if problem != "" {
			ui.Text(c, locales.CacheSettingsProblem(problem))
		}
	}, func() {
		if s.cache.moving {
			v := -1.0
			if s.cache.moveTotal > 0 {
				v = float64(s.cache.moveDone) / float64(s.cache.moveTotal)
			}
			ui.Progress(c, v).Width(160).Label(locales.CacheSettingsMoving())
			ui.Text(c, k.sentence(locales.CacheSettingsMovingNow())).FontSize(k.smallSize()).TextColor(k.muted)
			return
		}
		if s.button(c, "cache-folder-open", locales.CacheSettingsOpen(), false).Label(locales.CacheSettingsOpenLabel()).Clicked() {
			s.openCacheFolder()
		}
		if !s.set.CacheWithDownloads {
			if s.button(c, "cache-folder-change", locales.CacheSettingsChange(), false).Label(locales.CacheSettingsChangeLabel()).Disabled(s.movingDownloads).Clicked() {
				s.chooseCacheFolder()
			}
			if !samePath(folder, def) && !s.movingDownloads && s.link(c, "cache-folder-default", locales.CacheSettingsUseDefault()) {
				s.askMoveCache(def)
			}
		}
	}, folder, problem)

	merged := s.set.CacheWithDownloads
	s.row(c, locales.CacheMergeDownloads(), locales.CacheMergeDownloadsDesc(), func() {
		if s.switchOf(c.Key("cache-merge-downloads"), &merged).Label(locales.CacheMergeDownloads()).Disabled(s.cache.moving || s.movingDownloads).Changed() {
			s.askSetCacheMerged(merged)
		}
	}, "downloads merge folder")
}

func (s *shell) openCacheFolder() {
	go func() {
		if err := s.svc.System.OpenCacheFolder(); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

func (s *shell) chooseCacheFolder() {
	go func() {
		dir, err := s.svc.System.ChooseCacheFolder(s.win)
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case dir != "":
				s.askMoveCache(dir)
			}
		})
	}()
}

func (s *shell) askMoveCache(dir string) {
	use := s.svc.System.CacheUsage()
	if use.Tracks == 0 {
		s.moveCache(dir)
		return
	}
	s.ask(locales.CacheMoveTitle(), locales.CacheMoveQuestion(use.Tracks, byteSize(use.Bytes), dir),
		locales.CacheMove(), func() { s.moveCache(dir) })
}

func (s *shell) moveCache(dir string) {
	s.cache.moving, s.cache.moveDone, s.cache.moveTotal = true, 0, 0
	go func() {
		res, err := s.svc.System.MoveCache(context.Background(), dir, func(done, total int64) {
			s.update(func() { s.cache.moveDone, s.cache.moveTotal = done, total })
		})
		s.update(func() {
			s.cache.moving = false
			if err != nil {
				s.notes = append(s.notes, noticeOf(errors.New(locales.CacheMoveFailed(cacheMoveError(err)))))
				return
			}
			s.set.CacheFolder = res.Folder
			s.set.CacheWithDownloads = res.Merged
			s.save()
			msg := locales.CacheMovedTo(s.svc.System.CacheFolder())
			if res.Merged {
				msg = locales.CacheMergedTo(s.svc.System.CacheFolder())
			}
			s.notes = append(s.notes, notice(msg))
		})
	}()
}

func (s *shell) askSetCacheMerged(merged bool) {
	use := s.svc.System.CacheUsage()
	if use.Tracks == 0 {
		s.setCacheMerged(merged)
		return
	}
	question := ""
	if merged {
		target := filepath.Join(s.svc.Downloads.Folder(), "audio")
		question = locales.CacheMergeQuestion(use.Tracks, byteSize(use.Bytes), target)
	} else {
		target := s.svc.System.UnmergeCacheTarget()
		question = locales.CacheUnmergeQuestion(use.Tracks, byteSize(use.Bytes), target)
	}
	s.ask(locales.CacheMoveTitle(), question, locales.CacheMove(), func() { s.setCacheMerged(merged) })
}

func (s *shell) setCacheMerged(merged bool) {
	s.cache.moving, s.cache.moveDone, s.cache.moveTotal = true, 0, 0
	go func() {
		res, err := s.svc.System.SetCacheMerged(context.Background(), merged, func(done, total int64) {
			s.update(func() { s.cache.moveDone, s.cache.moveTotal = done, total })
		})
		s.update(func() {
			s.cache.moving = false
			if err != nil {
				s.notes = append(s.notes, noticeOf(errors.New(locales.CacheMoveFailed(cacheMoveError(err)))))
				return
			}
			s.set.CacheFolder = res.Folder
			s.set.CacheWithDownloads = res.Merged
			s.save()
			if res.Merged {
				s.notes = append(s.notes, notice(locales.CacheMergedTo(s.svc.System.CacheFolder())))
			} else {
				s.notes = append(s.notes, notice(locales.CacheUnmergedTo(s.svc.System.CacheFolder())))
			}
		})
	}()
}

func cacheMoveError(err error) string {
	var write *stream.CacheWriteError
	switch {
	case errors.Is(err, stream.ErrCacheMoving):
		return locales.CacheMoveMovingAlready()
	case errors.Is(err, stream.ErrCacheSameFolder):
		return locales.CacheMoveSameFolder()
	case errors.Is(err, stream.ErrCacheIntoTheirs):
		return locales.CacheMoveIntoTheirs()
	case errors.Is(err, stream.ErrCacheConflict):
		return locales.CacheMoveConflict()
	case errors.As(err, &write):
		return locales.CacheMoveCantWrite(write.Folder, errorText(write.Err))
	}
	return errorText(err)
}
