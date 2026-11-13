package main

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/locales"
)

// toolStatus is what MixMusic knows of yt-dlp and the JavaScript runtime,
// kept until an install changes it.
func (s *shell) toolStatus() *loaded[tools.Status] {
	return fetch(s, "tools", func(ctx context.Context) (tools.Status, error) { return s.svc.System.Tools(ctx), nil })
}

// outdated tells whether yt-dlp is the user's own, and older than the
// latest release.
func outdated(t tools.Tool) bool { return t.Found && !t.Managed && t.Latest != "" }

// installButton installs the latest yt-dlp, and shows how far it came.
func (s *shell) installButton(c *ui.Context, key, label string) {
	k := &s.lk
	if s.installing {
		ui.Row(c).Gap(12).Children(func() {
			v := -1.0
			if s.installTotal > 0 {
				v = float64(s.installDone) / float64(s.installTotal)
			}
			ui.Progress(c, v).Width(200)
			ui.Text(c, k.sentence(locales.ToolsInstalling())).FontSize(k.smallSize()).TextColor(k.muted)
		})
		return
	}
	if s.button(c, key, label, true).Clicked() {
		s.installing, s.installDone, s.installTotal = true, 0, 0
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			st, err := s.a.InstallYTDLP(ctx, func(done, total int64) {
				s.update(func() { s.installDone, s.installTotal = done, total })
			})
			s.update(func() {
				s.installing = false
				s.forget("tools")
				if err != nil {
					s.notes = append(s.notes, noticeOf(errors.New(locales.ToolsInstallFailed(err.Error()))))
					return
				}
				if s.set.YTDLPPath != "" {
					// The yt-dlp the user chose would go on running instead.
					s.set.YTDLPPath = ""
					s.save()
				}
				s.notes = append(s.notes, notice(locales.ToolsReady(st.YTDLP.Version)))
			})
		}()
	}
}

// toolProblems tells, on the home page, that some tracks need yt-dlp, or
// that the user's own is too old.
func (s *shell) toolProblems(c *ui.Context) {
	st := s.toolStatus()
	if !st.done {
		return
	}
	y := st.value.YTDLP
	panel := func(title, message string, actions func()) { s.alert(c, title, message, actions).Margin(0, 0, 32) }
	switch {
	case !y.Found:
		panel(locales.ToolsNeedTitle(), locales.ToolsNeedMessage(),
			func() { s.installButton(c, "install-ytdlp", locales.ToolSettingsInstall()) })
	case outdated(y) && s.set.SkippedYTDLP != y.Latest:
		how := locales.ToolsHowInstalled()
		if y.UpdateCommand != "" {
			how = locales.ToolsHowWithCommand(y.UpdateCommand)
		}
		panel(locales.ToolsOutdatedTitle(), locales.ToolsOutdatedMessage(y.Version, y.Latest, how),
			func() {
				s.installButton(c, "use-ours", locales.ToolSettingsUseOurs())
				if s.button(c, "not-now", locales.ToolsNotNow(), false).Clicked() {
					s.set.SkippedYTDLP = y.Latest
					s.save()
				}
			})
	}
}

func init() { addSettings("playback", 90, (*shell).toolSettings) }

// toolSettings show the tools MixMusic runs, and install yt-dlp.
func (s *shell) toolSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, locales.ToolSettingsGroup(), func() {
		st := s.toolStatus()
		if !st.done {
			// The search of the settings finds the tools once known.
			if !s.find.on() {
				s.loading(c)
			}
			return
		}
		y, js := st.value.YTDLP, st.value.JSRuntime
		desc := locales.ToolSettingsNotFound()
		if y.Found {
			desc = locales.ToolSettingsFound(y.Version, y.Path, yesNo(y.Managed), yesNo(y.Bundled), yesNo(outdated(y)), y.Latest)
		}
		s.row(c, "yt-dlp", desc, func() {
			switch {
			case !y.Found:
				s.installButton(c, "install-ytdlp-settings", locales.ToolSettingsInstall())
			case y.Managed:
				s.installButton(c, "update-ytdlp", locales.ToolSettingsUpdate())
			case outdated(y):
				s.installButton(c, "use-ours-settings", locales.ToolSettingsUseOurs())
			}
		})
		flatpak := s.svc.System.Info().Flatpak
		s.row(c, locales.ToolSettingsJSRuntime(), jsRuntimeHelp(js, flatpak), func() {
			ids := []string{"", "deno", "node", "bun", "quickjs"}
			labels := []string{locales.ToolSettingsFindOne(), "Deno", "Node.js", "Bun", "QuickJS"}
			chosen := s.set.JSRuntime
			if i := slices.Index(ids, s.set.JSRuntime); i >= 0 {
				chosen = labels[i]
			}
			if s.selectOf(c.Key("js-runtime"), &chosen, labels).Width(160).Label(locales.ToolSettingsJSRuntime()).Changed() {
				if i := slices.Index(labels, chosen); i >= 0 {
					s.set.JSRuntime = ids[i]
					s.save()
					// The tools stay shown until the answer comes.
					s.refetch("tools")
				}
			}
		}, "Deno Node.js Bun QuickJS")
		s.row(c, locales.ToolSettingsOwn(), ownYTDLPHelp(flatpak), func() {
			in, _ := s.settingInput(c, "ytdlp-path", s.set.YTDLPPath, func(path string) {
				s.set.YTDLPPath = path
				s.save()
				s.refetch("tools")
			})
			in.Placeholder(locales.ToolSettingsOwnPath()).Width(260).Label(locales.ToolSettingsOwn())
		}, "path to yt-dlp")
		_ = k
	})
}

// yesNo is the word of a flag, for the messages that take one.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// jsRuntimeHelp describes yt-dlp's JavaScript runtime, js, or tells how to
// get one. A Flatpak's sandbox has Node.js of its own, and sees no other:
// one chosen in settings brought over from another copy isn't there.
func jsRuntimeHelp(js tools.Tool, flatpak bool) string {
	switch {
	case js.Found:
		return locales.ToolSettingsJSRuntimeFound(js.Name, js.Version)
	case flatpak:
		return locales.ToolSettingsJSRuntimeFlatpak(locales.ToolSettingsFindOne())
	}
	return locales.ToolSettingsJSRuntimeNone()
}

// ownYTDLPHelp describes the setting of a yt-dlp of the user's own, which
// a Flatpak finds only in its own folders.
func ownYTDLPHelp(flatpak bool) string {
	if flatpak {
		return locales.ToolSettingsOwnFlatpak()
	}
	return locales.ToolSettingsOwnHelp()
}

func init() { addSettings("playback", 70, (*shell).cacheSettings) }

// cacheSettings show what the offline cache holds, and bound it.
func (s *shell) cacheSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, locales.CacheSettingsGroup(), func() {
		// The cache's folder is read for what it holds in every frame the
		// row shows: the search of the settings finds the row by what it
		// always says, without reading it.
		kept := locales.CacheSettingsKept()
		if s.findable(locales.CacheSettingsTracks(), kept) {
			use := s.svc.System.CacheUsage()
			s.row(c, locales.CacheSettingsTracks(), locales.CacheSettingsTracksLine(use.Tracks, byteSize(use.Bytes)), func() {
				if s.button(c, "clear-cache", locales.CacheSettingsClear(), false).Label(locales.CacheSettingsClearTracks()).Disabled(use.Tracks == 0).Clicked() {
					if err := s.svc.System.ClearCache(); err != nil {
						s.notes = append(s.notes, noticeOf(err))
					} else {
						s.notes = append(s.notes, notice(locales.CacheSettingsCleared()))
					}
				}
			})
		}
		covers := s.svc.System.ArtworkUsage()
		s.row(c, locales.CacheSettingsCovers(), locales.CacheSettingsCoversLine(covers.Images, byteSize(covers.Bytes)), func() {
			if s.button(c, "clear-covers", locales.CacheSettingsClear(), false).Label(locales.CacheSettingsClearCovers()).Disabled(covers.Images == 0 || s.clearingCovers).Clicked() {
				// Thousands of files may go: beside the main thread.
				s.clearingCovers = true
				go func() {
					err := s.svc.System.ClearArtwork()
					s.update(func() {
						s.clearingCovers = false
						if err != nil {
							s.notes = append(s.notes, noticeOf(err))
						} else {
							s.notes = append(s.notes, notice(locales.CacheSettingsCoversCleared()))
						}
					})
				}()
			}
		})
		// The sizes the web interface offered, and one of the user's own.
		sizes := cacheSizes
		if !slices.Contains(sizes, s.set.CacheSizeMB) {
			sizes = append(slices.Clone(cacheSizes), s.set.CacheSizeMB)
			slices.Sort(sizes)
		}
		labels := make([]string, len(sizes))
		for i, mb := range sizes {
			labels[i] = cacheSizeLabel(mb)
		}
		chosen := cacheSizeLabel(s.set.CacheSizeMB)
		what := locales.CacheSettingsOff()
		if s.set.CacheSizeMB > 0 {
			what = locales.CacheSettingsLimit(chosen)
		}
		s.row(c, locales.CacheSettingsSize(), what, func() {
			if s.selectOf(c.Key("cache-size"), &chosen, labels).Width(160).Label(locales.CacheSettingsSizeLabel()).Changed() {
				if i := slices.Index(labels, chosen); i >= 0 {
					s.set.CacheSizeMB = sizes[i]
					s.save()
				}
			}
		}, "storage disk")
	})
	_ = k
}

// cacheSizes are the sizes the offline cache takes, in MB: 0 turns it off.
var cacheSizes = []int{0, 512, 1024, 4096, 16384}

// cacheSizeLabel names a size of the cache.
func cacheSizeLabel(mb int) string {
	switch {
	case mb == 0:
		return locales.CacheSettingsSizeOff()
	case mb%1024 == 0:
		return strconv.Itoa(mb/1024) + " GB"
	}
	return strconv.Itoa(mb) + " MB"
}
