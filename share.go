package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/sharecard"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// Share, in the menu of a song, an album, a playlist or a mix, gives what
// to send to a friend: the link to its page on its service, or to its pages
// on both services for a Spotify song MixMusic found on YouTube Music too,
// and an image: a card with its cover, title and artist in the colours of
// the theme, which the system's dialog saves as a PNG, square for a chat
// or tall for a story. Lyrics have their own (lyricsshare.go). There is no
// server: the links go to the services' pages, and the image is made on
// this computer (internal/sharecard).

// shareLink is a page of a service.
type shareLink struct{ service, url string }

// shareTarget is what Share offers to send: the card's kind, title and
// second line, its artwork, and the links to its pages.
type shareTarget struct {
	kind, title, subtitle string
	art                   []media.Image
	links                 []shareLink
	// onService, if set, adds the items that put a mix on a service, to
	// share its link there.
	onService func(m *ui.Menu)
}

// songShare is the share of the song t: its link, and where MixMusic
// matched it on YouTube Music, that one too.
func (s *shell) songShare(t media.Track) shareTarget {
	out := shareTarget{kind: "Song", title: t.Title, subtitle: names(t.Artists), art: t.Artwork}
	if t.IsEpisode() {
		out.kind = "Episode"
	}
	if t.URL != "" {
		out.links = append(out.links, shareLink{sourceName(t.Source), t.URL})
	}
	if t.Source == media.SourceSpotify {
		if found, ok := s.svc.Player.Matched(t); ok {
			if u := youtubeLink(found); u != "" {
				out.links = append(out.links, shareLink{sourceName(media.SourceYouTube), u})
			}
		}
	}
	return out
}

// youtubeLink is the page of a track of YouTube Music: its own address, or
// the one its video's ID makes.
func youtubeLink(t media.Track) string {
	if t.URL != "" {
		return t.URL
	}
	source, video, err := media.ParseID(t.ID)
	if err != nil || source != media.SourceYouTube || !stream.ValidVideoID(video) {
		return ""
	}
	return "https://music.youtube.com/watch?v=" + video
}

// albumShare is the share of the album a.
func albumShare(a media.Album) shareTarget {
	kind := "Album"
	switch a.Kind {
	case media.AlbumKindSingle:
		kind = "Single"
	case media.AlbumKindEP:
		kind = "EP"
	}
	sub := names(a.Artists)
	if a.Year != "" && sub != "" {
		sub += " · " + a.Year
	}
	out := shareTarget{kind: kind, title: a.Title, subtitle: sub, art: a.Artwork}
	if a.URL != "" {
		out.links = []shareLink{{sourceName(a.Source), a.URL}}
	}
	return out
}

// playlistShare is the share of the playlist or mix p.
func playlistShare(p media.Playlist) shareTarget {
	out := shareTarget{kind: "Playlist", title: p.Title, subtitle: p.Owner, art: p.Artwork}
	if p.Source == media.SourceLocal {
		out.kind, out.subtitle = "Mix", ""
	}
	if p.URL != "" {
		out.links = []shareLink{{sourceName(p.Source), p.URL}}
	}
	return out
}

// shareItem is Share, in a menu: its submenu copies the links of t and
// saves its image.
func (s *shell) shareItem(m *ui.Menu, t shareTarget) {
	m.Submenu("Share", func(m *ui.Menu) {
		for _, l := range t.links {
			label := "Copy link"
			if len(t.links) > 1 {
				label = "Copy the link on " + l.service
			}
			if m.Item(label).Chosen() {
				copyText(l.url)
				s.notes = append(s.notes, notice("Copied the link"))
			}
		}
		if t.onService != nil {
			t.onService(m)
		}
		if len(t.links) > 0 || t.onService != nil {
			m.Separator()
		}
		if m.Item("Save image…").Chosen() {
			s.saveCard(t.title, t.art, sharecard.Square, s.titleCard(t))
		}
		if m.Item("Save image for a story…").Chosen() {
			s.saveCard(t.title, t.art, sharecard.Story, s.titleCard(t))
		}
	})
}

// withShare is menu with Share after it.
func (s *shell) withShare(menu func(m *ui.Menu), t shareTarget) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		menu(m)
		s.shareItem(m, t)
	}
}

// cardColors are the colours of the theme, as a card wears them.
func (k *look) cardColors() sharecard.Colors {
	solid := func(c ui.Color) color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255} }
	return sharecard.Colors{Background: solid(k.bg), Text: solid(k.text), Muted: solid(k.muted), Accent: solid(k.accent)}
}

// cardFaces are the files of the weights of each face MixMusic carries.
var cardFaces = map[string][]struct {
	file   string
	weight int
}{
	face:         {{"fonts/selawkl.ttf", 300}, {"fonts/selawk.ttf", 400}, {"fonts/selawksb.ttf", 600}, {"fonts/selawkb.ttf", 700}},
	faceRound:    {{"fonts/quicksand-light.ttf", 300}, {"fonts/quicksand-regular.ttf", 400}, {"fonts/quicksand-bold.ttf", 700}},
	faceMaterial: {{"fonts/robotoflex-regular.ttf", 400}, {"fonts/robotoflex-medium.ttf", 500}, {"fonts/robotoflex-bold.ttf", 700}},
	faceArchivo:  {{"fonts/archivo-semicondensed-extrabold.ttf", 800}},
}

// cardFamily is the family a card is set in: the theme's face when
// MixMusic carries it, else the interface's, and the system's fonts for
// the letters it lacks.
func (k *look) cardFamily() sharecard.Family {
	files, ok := cardFaces[k.body]
	if !ok {
		files = cardFaces[face]
	}
	var fam sharecard.Family
	for _, f := range files {
		data, err := fontFiles.ReadFile(f.file)
		if err != nil {
			continue
		}
		font, err := sharecard.ParseFont(data)
		if err != nil {
			continue
		}
		fam.Faces = append(fam.Faces, sharecard.Face{Weight: f.weight, Font: font})
	}
	fam.Fallback = sharecard.SystemFallback()
	return fam
}

// cardBuilder makes a card of a size, with the cover that came for it, if
// one did.
type cardBuilder func(cover image.Image, size sharecard.Size) sharecard.Card

// titleCard builds the card of t in the colours of the current theme: its
// kind, title and second line under its cover.
func (s *shell) titleCard(t shareTarget) cardBuilder {
	colors := s.lk.cardColors()
	return func(cover image.Image, size sharecard.Size) sharecard.Card {
		c := sharecard.Card{
			Size: size, Colors: colors, Cover: cover,
			Lines: []sharecard.Line{
				{Text: strings.ToUpper(t.kind), Size: 34, Weight: 600, Ink: colors.Accent},
				{Text: t.title, Size: 88, Weight: 700, MaxLines: 3, Gap: 18},
			},
			Foot: []sharecard.Line{{Text: "MixMusic", Size: 34, Weight: 600, Ink: colors.Accent}},
		}
		if t.subtitle != "" {
			c.Lines = append(c.Lines, sharecard.Line{Text: t.subtitle, Size: 52, Weight: 400, Ink: colors.Muted, MaxLines: 2, Gap: 20})
		}
		return c
	}
}

// saveFileDialog asks the user where to save a file, as MyGo's dialog
// does; tests replace it.
var saveFileDialog = func(opts mygo.SaveDialogOptions) (string, error) { return mygo.Dialog.Save(opts) }

// cardCoverWait is how long the cover of a card may take to come.
const cardCoverWait = 20 * time.Second

// saveCard asks where to save the card build makes, at size, beside the
// main thread, then draws it with the cover art gives, if it comes, and
// writes it as a PNG. name is what to call the file at first.
func (s *shell) saveCard(name string, art []media.Image, size sharecard.Size, build cardBuilder) {
	win := s.win
	fam := s.lk.cardFamily()
	go func() {
		path, err := saveFileDialog(mygo.SaveDialogOptions{Parent: win, Title: "Save the image", DefaultPath: cardFileName(name),
			ButtonLabel: "Save", Filters: []mygo.FileFilter{{Name: "PNG image", Extensions: []string{"png"}}}, CreateDirectories: true})
		if err == nil && path == "" {
			return
		}
		if err == nil {
			path, err = s.writeCard(path, art, size, fam, build)
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, notice("Saved the image as "+filepath.Base(path)))
		})
	}()
}

// writeCard draws the card and writes it to path, with .png at its end,
// and returns where it wrote. A cover that doesn't come leaves the card
// without it.
func (s *shell) writeCard(path string, art []media.Image, size sharecard.Size, fam sharecard.Family, build cardBuilder) (string, error) {
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		path += ".png"
	}
	var cover image.Image
	if u := artworkURL(art, 600); u != "" {
		ctx, cancel := context.WithTimeout(context.Background(), cardCoverWait)
		cover, _ = s.art.src.Image(ctx, u)
		cancel()
	}
	var buf bytes.Buffer
	if err := build(cover, size).PNG(&buf, fam); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, buf.Bytes(), 0o644)
}

// cardFileName is a file name for the card of what is called title.
func cardFileName(title string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 0x20 {
			return '-'
		}
		return r
	}, strings.TrimSpace(title))
	name = strings.Trim(name, " .-")
	if name == "" {
		name = "MixMusic"
	}
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	return name + ".png"
}
