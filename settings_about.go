package main

import (
	"context"

	"github.com/egoist/mygo/ui"
)

func (s *shell) aboutSettings(c *ui.Context) {
	k := &s.lk
	info := s.svc.System.Info()
	u := s.svc.System.Updates()
	s.group(c, "MixMusic", func() {
		version := "MixMusic " + info.Version + " · " + info.Platform + " " + info.Arch
		if build, ok := buildNames[u.Kind]; ok {
			version += " · " + build
		}
		s.row(c, "Version", version, func() {})
		s.releaseRow(c, u)
		// Its links go under it, as they are many.
		about := "Spotify and YouTube Music in one player, open source under the MIT license or the GNU GPL, version 3. Your data lives in " + info.DataDir + "."
		s.found(c, "MixMusic", about+" source code, contribute, report a problem, a bug, open logs folder", func() {
			ui.Column(c).Gap(4).Padding(14, 0).Children(func() {
				ui.Text(c, "MixMusic").FontSize(k.bodySize())
				ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted)
				ui.Row(c).Gap(8).Wrap().Margin(12, 0, 0).Children(func() {
					const repo = "https://github.com/fzn0x/mixmusic"
					if s.button(c, "source", "source code", false).Clicked() {
						go s.svc.System.OpenExternal(repo)
					}
					if s.button(c, "contribute", "contribute", false).Clicked() {
						go s.svc.System.OpenExternal(repo + "/blob/main/CONTRIBUTING.md")
					}
					if s.button(c, "report", "report a problem", false).Clicked() {
						go func() { _ = s.svc.System.OpenExternal(s.svc.System.ReportURL(context.Background())) }()
					}
					if s.button(c, "logs", "open logs folder", false).Clicked() {
						if err := s.svc.System.OpenLogsFolder(); err != nil {
							s.notes = append(s.notes, noticeOf(err))
						}
					}
				})
			})
		})
	})
	s.updateSettings(c)
	s.group(c, "Licenses", func() {
		for i, n := range append(s.svc.System.Notices(), fontNotices()...) {
			open := s.licenseOpen == i
			// A license's text shows with its row.
			s.found(c, n.Name, "", func() {
				s.row(c, n.Name, "", func() {
					label := "show"
					if open {
						label = "hide"
					}
					if s.button(c, "license-"+count(i), label, false).Clicked() {
						if open {
							s.licenseOpen = -1
						} else {
							s.licenseOpen = i
						}
					}
				})
				if open {
					ui.Text(c, n.Text).FontSize(k.captionSize()).Font(monospace()).TextColor(k.muted).Padding(8, 0).Selectable()
				}
			})
		}
	})
}
