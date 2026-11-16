package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
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
	s.group(c, "Skip parts of videos", func() { s.settingsOf(c, partRows) })
}

func init() { addSettings(partRows, 10, (*shell).skipPartsRow) }

// skipPartsRow turns the skipping on.
func (s *shell) skipPartsRow(c *ui.Context) {
	s.row(c, "Skip the parts SponsorBlock marks", "Passes over the parts of music videos, uploads and episodes that SponsorBlock's community marks, and shows them on the seek bar. While it's on, MixMusic asks SponsorBlock of each video it plays from YouTube Music, sending the first 4 characters of a hash of its ID, never the ID.", func() {
		if s.switchOf(c.Key("skip-parts"), &s.set.SkipParts.On).Label("Skip the parts SponsorBlock marks").Changed() {
			s.save()
		}
	}, "sponsorblock sponsor intro outro segments")
}

// partKinds tell what each kind of part holds, in the order of
// sponsorblock.Kinds.
var partKinds = map[sponsorblock.Kind]string{
	sponsorblock.NonMusic:    "In music videos, what isn't the song: a spoken scene, a skit, the credits.",
	sponsorblock.Sponsor:     "A paid read or plug for a product or a service.",
	sponsorblock.SelfPromo:   "The creator's own merch, donations, other channels and shows.",
	sponsorblock.Interaction: "Asking to like, subscribe or follow, in the middle of a video.",
	sponsorblock.Intro:       "An intro animation, a title card or a pause, without the content.",
	sponsorblock.Outro:       "End cards and credits after the content.",
	sponsorblock.Preview:     "Clips of what comes later in the video, or of what came before.",
	sponsorblock.Hook:        "A teaser or a hello before the content starts.",
	sponsorblock.Filler:      "Tangents and jokes beside the topic. It skips a lot, so it's off at first.",
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
		s.row(c, kind.Name(), partKinds[kind], func() {
			box := s.pointerKeys(ui.Checkbox(c.Key("skip-part-"+string(kind)), &on, "")).Label("Skip " + kind.Name())
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
	s.row(c, "Highlight", "Marks the moment SponsorBlock's community calls a video's best with a dot on the seek bar. Jump to the highlight, on the now playing page and in the command palette, plays from there.", func() {
		on := s.set.SkipParts.Highlight
		box := s.pointerKeys(ui.Checkbox(c.Key("part-highlight"), &on, "")).Label("Mark the highlight")
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
	s.row(c, "Server", "SponsorBlock's own, or the https address of a mirror of it.", func() {
		in, _ := s.settingInput(c, "part-server", s.set.SkipParts.Server, func(address string) {
			server := sponsorblock.Server(address)
			if address != "" && server == "" {
				s.notes = append(s.notes, notice("Use the https address of the server, as "+sponsorblock.ServerURL))
				return
			}
			s.set.SkipParts.Server = server
			s.save()
		})
		in.Placeholder(sponsorblock.ServerURL).Width(260).Label("SponsorBlock server")
	}, "sponsorblock mirror address")
}

func init() { addSettings(partRows, 50, (*shell).partCredit) }

// partCredit credits SponsorBlock's data, as its license asks.
func (s *shell) partCredit(c *ui.Context) {
	k := &s.lk
	s.found(c, "", sponsorblock.Credit, func() {
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
			ui.Text(c, "Uses SponsorBlock data licensed under ")
			s.linkOf(c, "CC BY-NC-SA 4.0", "https://creativecommons.org/licenses/by-nc-sa/4.0/")
			ui.Text(c, " from ")
			s.linkOf(c, "https://sponsor.ajay.app/", "https://sponsor.ajay.app/")
			ui.Text(c, ". Skipped parts count for nothing in plays and Last.fm scrobbles; each is skipped once a play, so seeking back into one plays it.")
		})
	})
}
