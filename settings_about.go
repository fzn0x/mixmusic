package main

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
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
			s.found(c, noticeName(n), "", func() {
				s.row(c, noticeName(n), "", func() {
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

// noticeNames say the names of the notices of internal/app, which are
// English there, in the catalogs.
var noticeNames = map[string]func() string{
	"libopus, which decodes Opus audio":                                                          locales.AboutNoticeOpus,
	"SharpJaad, which decodes AAC audio":                                                         locales.AboutNoticeAac,
	"go-mp3, which decodes the MP3 files on this computer":                                       locales.AboutNoticeMp3,
	"jfreymuth/vorbis, which decodes the Vorbis files on this computer":                          locales.AboutNoticeVorbis,
	"Chromium's Web Audio, which shapes the sound":                                               locales.AboutNoticeWebAudio,
	"Material Color Utilities, which make the Material themes' colours":                          locales.AboutNoticeMaterialColor,
	"SponsorBlock's data, the parts of videos MixMusic skips":                                    locales.AboutNoticeSponsorBlock,
	"Unihan, whose readings of Chinese characters write Chinese lyrics in pinyin":                locales.AboutNoticeUnihan,
	"pinyin-data and phrase-pinyin-data, the readings of Chinese characters and words in pinyin": locales.AboutNoticePinyin,
	"kagome, which reads the kanji of Japanese lyrics for their romaji":                          locales.AboutNoticeKagome,
	"The IPA dictionary (mecab-ipadic), which romaji downloads the first time it's turned on":    locales.AboutNoticeIpadic,
}

// noticeName is a notice's name in the language shown, or as it's given.
func noticeName(n app.Notice) string {
	if say, ok := noticeNames[n.Name]; ok {
		return say()
	}
	return n.Name
}
