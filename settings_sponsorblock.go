package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/locales"
)

// The group of Settings › Playback that skips the parts of videos
// SponsorBlock marks (#548): the switch, the kinds of parts skipped, the
// highlight, the server, and SponsorBlock's credit.

func init() { addSettings("playback", 75, (*shell).skipPartsSettings) }

// partRows are the rows of Settings › Playback › Skip parts of videos,
// which register from their features' files.
const partRows = "playback › Skip parts of videos"

// skipPartsSettings show the rows of skipping parts of videos.
func (s *shell) skipPartsSettings(c *ui.Context) {
	s.group(c, locales.SkipPartsGroup(), func() { s.settingsOf(c, partRows) })
}

func init() { addSettings(partRows, 10, (*shell).skipPartsRow) }

// skipPartsRow turns the skipping on.
func (s *shell) skipPartsRow(c *ui.Context) {
	s.row(c, locales.SkipPartsTitle(), locales.SkipPartsAbout(), func() {
		if s.switchOf(c.Key("skip-parts"), &s.set.SkipParts.On).Label(locales.SkipPartsSwitchLabel()).Changed() {
			s.save()
		}
	}, "sponsorblock sponsor intro outro segments")
}

// partKindAbout tells what each kind of part holds.
func partKindAbout(kind sponsorblock.Kind) string {
	switch kind {
	case sponsorblock.NonMusic:
		return locales.SkipPartsKindNonMusic()
	case sponsorblock.Sponsor:
		return locales.SkipPartsKindSponsor()
	case sponsorblock.SelfPromo:
		return locales.SkipPartsKindSelfPromo()
	case sponsorblock.Interaction:
		return locales.SkipPartsKindInteraction()
	case sponsorblock.Intro:
		return locales.SkipPartsKindIntro()
	case sponsorblock.Outro:
		return locales.SkipPartsKindOutro()
	case sponsorblock.Preview:
		return locales.SkipPartsKindPreview()
	case sponsorblock.Hook:
		return locales.SkipPartsKindHook()
	case sponsorblock.Filler:
		return locales.SkipPartsKindFiller()
	}
	return ""
}

func init() { addSettings(partRows, 20, (*shell).partKindRows) }

// partKindRows tick the kinds of parts skipped, each in a row of its own,
// while parts are skipped.
func (s *shell) partKindRows(c *ui.Context) {
	if !s.set.SkipParts.On {
		return
	}
	for _, kind := range sponsorblock.Kinds {
		on := s.set.SkipParts.Skips(kind)
		s.row(c, partKindName(kind), partKindAbout(kind), func() {
			box := s.pointerKeys(ui.Checkbox(c.Key("skip-part-"+string(kind)), &on, "")).Label(locales.SkipPartsKindSkip(partKindName(kind)))
			if box.Changed() {
				kinds := slices.DeleteFunc(slices.Clone(s.set.SkipParts.Kinds), func(k sponsorblock.Kind) bool { return k == kind })
				if on {
					kinds = append(kinds, kind)
				}
				s.set.SkipParts.Kinds = kinds
				s.save()
			}
		}, "sponsorblock skip part")
	}
}

func init() { addSettings(partRows, 30, (*shell).highlightRow) }

// highlightRow marks the highlight SponsorBlock tells of a video, while
// parts are skipped.
func (s *shell) highlightRow(c *ui.Context) {
	if !s.set.SkipParts.On {
		return
	}
	s.row(c, locales.SkipPartsHighlightTitle(), locales.SkipPartsHighlightAbout(), func() {
		on := s.set.SkipParts.Highlight
		box := s.pointerKeys(ui.Checkbox(c.Key("part-highlight"), &on, "")).Label(locales.SkipPartsHighlightMark())
		if box.Changed() {
			s.set.SkipParts.Highlight = on
			s.save()
		}
	}, "sponsorblock highlight jump")
}

func init() { addSettings(partRows, 40, (*shell).partServerRow) }

// partServerRow names the mirror of SponsorBlock to ask in its place,
// while parts are skipped.
func (s *shell) partServerRow(c *ui.Context) {
	if !s.set.SkipParts.On {
		return
	}
	s.row(c, locales.SkipPartsServerTitle(), locales.SkipPartsServerAbout(), func() {
		in, _ := s.settingInput(c, "part-server", s.set.SkipParts.Server, func(address string) {
			server := sponsorblock.Server(address)
			if address != "" && server == "" {
				s.notes = append(s.notes, notice(locales.SkipPartsServerUse(sponsorblock.ServerURL)))
				return
			}
			s.set.SkipParts.Server = server
			s.save()
		})
		in.Placeholder(sponsorblock.ServerURL).Width(260).Label(locales.SkipPartsServerLabel())
	}, "sponsorblock mirror address")
}

func init() { addSettings(partRows, 50, (*shell).partCredit) }

// partCredit credits SponsorBlock's data, as its license asks.
func (s *shell) partCredit(c *ui.Context) {
	k := &s.lk
	s.found(c, "", sponsorblock.Credit, func() {
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
			ui.Text(c, locales.SkipPartsCreditBefore())
			s.linkOf(c, "CC BY-NC-SA 4.0", "https://creativecommons.org/licenses/by-nc-sa/4.0/")
			ui.Text(c, locales.SkipPartsCreditBetween())
			s.linkOf(c, "https://sponsor.ajay.app/", "https://sponsor.ajay.app/")
			ui.Text(c, locales.SkipPartsCreditAfter())
		})
	})
}

// partKindName is the kind's name, as the settings and the seek bar show
// it, in the language shown.
func partKindName(kind sponsorblock.Kind) string {
	switch kind {
	case sponsorblock.NonMusic:
		return locales.SkipPartsNameNonMusic()
	case sponsorblock.Sponsor:
		return locales.SkipPartsNameSponsor()
	case sponsorblock.SelfPromo:
		return locales.SkipPartsNameSelfPromo()
	case sponsorblock.Interaction:
		return locales.SkipPartsNameInteraction()
	case sponsorblock.Intro:
		return locales.SkipPartsNameIntro()
	case sponsorblock.Outro:
		return locales.SkipPartsNameOutro()
	case sponsorblock.Preview:
		return locales.SkipPartsNamePreview()
	case sponsorblock.Hook:
		return locales.SkipPartsNameHook()
	case sponsorblock.Filler:
		return locales.SkipPartsNameFiller()
	case sponsorblock.Highlight:
		return locales.SkipPartsNameHighlight()
	}
	return kind.Name()
}
