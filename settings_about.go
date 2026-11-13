package main

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// The groups of Settings › About: this copy of MixMusic and its licenses
// here, the updates in desktop.go.

func init() { addSettings("about", 10, (*shell).mixmusicSettings) }

// mixmusicSettings tell of this copy of MixMusic, and of a newer one.
func (s *shell) mixmusicSettings(c *ui.Context) {
	k := &s.lk
	info := s.svc.System.Info()
	u := s.svc.System.Updates()
	s.group(c, "MixMusic", func() {
		version := "MixMusic " + info.Version + " · " + info.Platform + " " + info.Arch
		if build, ok := buildName(u.Kind); ok {
			version += " · " + build
		}
		s.row(c, locales.AboutVersion(), version, func() {})
		s.releaseRow(c, u)
		// Its links go under it, as they are many.
		about := locales.AboutDescription(info.DataDir)
		s.found(c, "MixMusic", about+" source code, contribute, report a problem, a bug, open logs folder", func() {
			ui.Column(c).Gap(4).Padding(14, 0).Children(func() {
				ui.Text(c, "MixMusic").FontSize(k.bodySize())
				ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted)
				ui.Row(c).Gap(8).Wrap().Margin(12, 0, 0).Children(func() {
					const repo = "https://github.com/fzn0x/mixmusic"
					if s.button(c, "source", locales.AboutSourceCode(), false).Clicked() {
						go s.svc.System.OpenExternal(repo)
					}
					if s.button(c, "contribute", locales.AboutContribute(), false).Clicked() {
						go s.svc.System.OpenExternal(repo + "/blob/main/CONTRIBUTING.md")
					}
					if s.button(c, "report", locales.AboutReport(), false).Clicked() {
						go func() { _ = s.svc.System.OpenExternal(s.svc.System.ReportURL(context.Background())) }()
					}
					if s.button(c, "logs", locales.AboutOpenLogs(), false).Clicked() {
						if err := s.svc.System.OpenLogsFolder(); err != nil {
							s.notes = append(s.notes, noticeOf(err))
						}
					}
				})
			})
		})
	})
}

func init() { addSettings("about", 30, (*shell).licenseSettings) }

// licenseSettings show the licenses of what MixMusic is made of.
func (s *shell) licenseSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, locales.AboutLicenses(), func() {
		for i, n := range append(s.svc.System.Notices(), fontNotices()...) {
			open := s.licenseOpen == i
			// A license's text shows with its row.
			s.found(c, n.Name, "", func() {
				s.row(c, n.Name, "", func() {
					label := locales.AboutShow()
					if open {
						label = locales.AboutHide()
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
