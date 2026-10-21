package native

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/tools"
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
			ui.Text(c, "installing yt-dlp…").FontSize(k.smallSize()).TextColor(k.muted)
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
					s.notes = append(s.notes, noticeOf(fmt.Errorf("couldn't install yt-dlp: %w", err)))
					return
				}
				if s.set.YTDLPPath != "" {
					// The yt-dlp the user chose would go on running instead.
					s.set.YTDLPPath = ""
					s.save()
				}
				s.notes = append(s.notes, notice("yt-dlp "+st.YTDLP.Version+" is ready"))
			})
		}()
	}
}

// toolProblems tells, on the home page, that playback needs yt-dlp, or
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
		panel("Playback needs yt-dlp",
			"MixMusic finds the audio of YouTube Music tracks with yt-dlp. Install it once, and it stays up to date from Settings.",
			func() { s.installButton(c, "install-ytdlp", "install yt-dlp") })
	case outdated(y) && s.skippedYTDLP != y.Latest:
		how := "the way you installed it"
		if y.UpdateCommand != "" {
			how = "with " + y.UpdateCommand
		}
		panel("Your yt-dlp is out of date",
			fmt.Sprintf("yt-dlp %s finds the audio of your tracks, and %s is out. As YouTube changes, an old yt-dlp stops finding it. Update it %s, or use MixMusic's own, which it keeps up to date.", y.Version, y.Latest, how),
			func() {
				s.installButton(c, "use-ours", "use MixMusic's yt-dlp")
				if s.button(c, "not-now", "not now", false).Clicked() {
					s.skippedYTDLP = y.Latest
				}
			})
	}
}

// toolSettings show the tools MixMusic runs, and install yt-dlp.
func (s *shell) toolSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, "tools", func() {
		st := s.toolStatus()
		if !st.done {
			s.loading(c)
			return
		}
		y, js := st.value.YTDLP, st.value.JSRuntime
		desc := "Not found. MixMusic needs it to play YouTube Music."
		if y.Found {
			desc = "yt-dlp " + y.Version + " at " + y.Path
			if y.Managed {
				desc += ", MixMusic's own, which it keeps up to date"
			}
			if outdated(y) {
				desc += ". " + y.Latest + " is out."
			}
		}
		s.row(c, "yt-dlp", desc, func() {
			switch {
			case !y.Found:
				s.installButton(c, "install-ytdlp-settings", "install yt-dlp")
			case y.Managed:
				s.installButton(c, "update-ytdlp", "update yt-dlp")
			case outdated(y):
				s.installButton(c, "use-ours-settings", "use MixMusic's yt-dlp")
			}
		})
		jsDesc := "yt-dlp needs Deno, Node.js, Bun or QuickJS to read most YouTube streams. Install one, and MixMusic finds it."
		if js.Found {
			jsDesc = js.Name + " " + js.Version + " · yt-dlp uses it to solve YouTube's challenges."
		}
		s.row(c, "JavaScript runtime", jsDesc, func() {
			ids := []string{"", "deno", "node", "bun", "quickjs"}
			labels := []string{"Find one", "Deno", "Node.js", "Bun", "QuickJS"}
			chosen := s.set.JSRuntime
			if i := slices.Index(ids, s.set.JSRuntime); i >= 0 {
				chosen = labels[i]
			}
			if ui.Select(c.Key("js-runtime"), &chosen, labels).Width(160).Label("JavaScript runtime").Changed() {
				if i := slices.Index(labels, chosen); i >= 0 {
					s.set.JSRuntime = ids[i]
					s.save()
					s.forget("tools")
				}
			}
		})
		s.row(c, "Own yt-dlp", "Use a yt-dlp of your own, such as your package manager's, instead.", func() {
			in, _ := s.settingInput(c, "ytdlp-path", s.set.YTDLPPath, func(path string) {
				s.set.YTDLPPath = path
				s.save()
				s.forget("tools")
			})
			in.Placeholder("path to yt-dlp").Width(260).Label("Own yt-dlp")
		})
		_ = k
	})
}

// cacheSettings show what the offline cache holds, and bound it.
func (s *shell) cacheSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, "offline cache", func() {
		use := s.svc.System.CacheUsage()
		s.row(c, "Tracks kept", fmt.Sprintf("%s in %s. Tracks you played play again without the network.", plural(use.Tracks, "track"), byteSize(use.Bytes)), func() {
			if s.button(c, "clear-cache", "clear", false).Disabled(use.Tracks == 0).Clicked() {
				if err := s.svc.System.ClearCache(); err != nil {
					s.notes = append(s.notes, noticeOf(err))
				}
			}
		})
		s.row(c, "Size", byteSize(int64(s.set.CacheSizeMB)<<20)+" at most.", func() {
			v := float64(s.set.CacheSizeMB)
			if ui.StepSlider(c.Key("cache-size"), &v, 256, 10240, 256).Width(200).Label("Cache size").Changed() {
				s.set.CacheSizeMB = int(v)
				s.save()
			}
		})
	})
	_ = k
}
