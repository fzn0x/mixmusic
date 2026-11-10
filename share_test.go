package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/sharecard"
	"github.com/fzn0x/mixmusic/lyrics"
)

// savesTo makes the save dialog answer path, and returns what it was asked.
func savesTo(t *testing.T, path string) chan mygo.SaveDialogOptions {
	t.Helper()
	asked := make(chan mygo.SaveDialogOptions, 4)
	was := saveFileDialog
	saveFileDialog = func(opts mygo.SaveDialogOptions) (string, error) {
		asked <- opts
		return path, nil
	}
	t.Cleanup(func() { saveFileDialog = was })
	return asked
}

// decodedCard waits for the file at path, and decodes the PNG in it.
func decodedCard(t *testing.T, s *shell, path string) image.Image {
	t.Helper()
	var img image.Image
	for deadline := time.Now().Add(30 * time.Second); img == nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no image at %s; the toasts say %v", path, s.notes)
		}
		s.drain()
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		img, _ = png.Decode(f)
		f.Close()
	}
	return img
}

// TestShareMenus offers Share in the menus of songs, albums, playlists
// and mixes: it copies their links, and saves their image, as a square
// card or a story, in the theme's colours.
func TestShareMenus(t *testing.T) {
	var copied []string
	was := copyText
	copyText = func(text string) { copied = append(copied, text) }
	t.Cleanup(func() { copyText = was })
	s := newTestShell(t)
	tr := testTracks[0]
	tr.URL = "https://music.youtube.com/watch?v=glassharbor"
	album := media.Album{ID: "al-x", Title: "Paper Lanterns", Source: media.SourceYouTube, Year: "2011",
		Artists: []media.Ref{{Name: "North Pier"}}, URL: "https://music.youtube.com/browse/MPREb_lanterns", Tracks: []media.Track{tr}}
	list := media.Playlist{ID: "pl-x", Title: "Rainy Day Mix", Source: media.SourceYouTube, URL: "https://music.youtube.com/playlist?list=PLrainy"}
	bare := media.Playlist{ID: "pl-y", Title: "Mixtape"}
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: album}
	s.loads["search:pier"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{YouTube: media.SearchResults{
		Albums: []media.Album{album}, Playlists: []media.Playlist{list, bare}}}}
	tt := testerOf(t, s, 1280, 1800)
	dir := t.TempDir()
	asked := savesTo(t, filepath.Join(dir, "card"))

	copyLink := func(open func() error, want string) {
		t.Helper()
		if err := open(); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem("Share", "Copy link"); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		tt.Frame()
		if len(copied) == 0 || copied[len(copied)-1] != want {
			t.Errorf("copied %q, not %q", copied, want)
		}
	}
	rightClick := func(label string) func() error { return func() error { return tt.RightClick(label) } }
	more := func() error { return tt.Click("More") }
	show(s, tt, "/album/al-x")
	copyLink(rightClick(tr.Title), tr.URL)
	copyLink(more, album.URL)
	show(s, tt, searchPath("pier"))
	copyLink(rightClick(album.Title), album.URL)
	copyLink(rightClick(list.Title), list.URL)

	// A playlist without a link has an image to save, and no link to copy.
	if err := tt.RightClick(bare.Title); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); !slices.Contains(got, "Share") {
		t.Fatalf("the menu of a playlist without a link is %q", got)
	}
	if err := tt.ChooseMenuItem("Share", "Copy link"); err == nil {
		t.Error("a playlist without a link offers to copy it")
	}
	tt.CloseMenu()

	// The image of an album is a square, with no .png asked of the dialog.
	if err := tt.RightClick(album.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Share", "Save image…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	select {
	case opts := <-asked:
		if opts.DefaultPath != "Paper Lanterns.png" || len(opts.Filters) != 1 || !slices.Equal(opts.Filters[0].Extensions, []string{"png"}) {
			t.Errorf("the dialog asks %+v", opts)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no save dialog opened")
	}
	img := decodedCard(t, s, filepath.Join(dir, "card.png"))
	if got := img.Bounds().Size(); got != image.Pt(sharecard.Square.W, sharecard.Square.H) {
		t.Errorf("the card is %v", got)
	}
	// The foot of the card is the theme's ground.
	bg := s.lk.bg
	r, g, b, _ := img.At(10, sharecard.Square.H-10).RGBA()
	if uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(b>>8) != bg.B {
		t.Errorf("the card's ground is %d %d %d, not the theme's %d %d %d", r>>8, g>>8, b>>8, bg.R, bg.G, bg.B)
	}
	until(t, s, tt, "the toast", func() bool { return tt.HasText("Saved the image as card.png") })

	// A story is tall.
	if err := tt.RightClick(list.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Share", "Save image for a story…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	<-asked
	img = decodedCard(t, s, filepath.Join(dir, "card.png"))
	until(t, s, tt, "the story", func() bool {
		f, err := os.Open(filepath.Join(dir, "card.png"))
		if err != nil {
			return false
		}
		defer f.Close()
		c, err := png.DecodeConfig(f)
		return err == nil && c.Height == sharecard.Story.H && c.Width == sharecard.Story.W
	})
}

// TestShareCancelled saves nothing when the user closes the save dialog.
func TestShareCancelled(t *testing.T) {
	s := newTestShell(t)
	asked := savesTo(t, "")
	tt := tester(t, s)
	show(s, tt, "/")
	s.saveCard("Glass Harbor", nil, sharecard.Square, s.titleCard(shareTarget{kind: "Song", title: "Glass Harbor"}))
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("no save dialog opened")
	}
	time.Sleep(50 * time.Millisecond)
	s.drain()
	tt.Frame()
	if tt.HasText("Saved the image") || len(s.notes) != 0 {
		t.Errorf("the toasts say %v", s.notes)
	}
}

// TestShareFailed tells why the image couldn't be written.
func TestShareFailed(t *testing.T) {
	s := newTestShell(t)
	savesTo(t, filepath.Join(t.TempDir(), "missing", "card.png"))
	tt := tester(t, s)
	show(s, tt, "/")
	s.saveCard("Glass Harbor", nil, sharecard.Square, s.titleCard(shareTarget{kind: "Song", title: "Glass Harbor"}))
	until(t, s, tt, "the failure", func() bool {
		return slices.ContainsFunc(tt.Texts(), func(text string) bool { return strings.Contains(text, "missing") })
	})
}

// TestSongShareLinks links a Spotify song's page, and its page on YouTube
// Music too when MixMusic matched it there.
func TestSongShareLinks(t *testing.T) {
	s := newTestShell(t)
	tr := testTracks[1]
	tr.URL = "https://open.spotify.com/track/saltflats"
	if got := s.songShare(tr).links; !slices.Equal(got, []shareLink{{"Spotify", tr.URL}}) {
		t.Errorf("links without a match: %v", got)
	}
	if got := youtubeLink(media.Track{ID: "yt:abcDEF12345"}); got != "https://music.youtube.com/watch?v=abcDEF12345" {
		t.Errorf("a track without an address links to %q", got)
	}
	if got := youtubeLink(media.Track{ID: "spotify:2"}); got != "" {
		t.Errorf("a Spotify track links to %q on YouTube Music", got)
	}
	if got := playlistShare(media.Playlist{Title: "Night drive", Source: media.SourceLocal}); got.kind != "Mix" || len(got.links) != 0 {
		t.Errorf("a mix shares as %+v", got)
	}
	if got := albumShare(media.Album{Title: "Salt Flats", Kind: media.AlbumKindEP, Year: "1998", Artists: []media.Ref{{Name: "Orchard Radio"}}}); got.kind != "EP" ||
		got.subtitle != "Orchard Radio · 1998" {
		t.Errorf("an album shares as %+v", got)
	}
}

// TestCardFileName proposes names the systems' dialogs take.
func TestCardFileName(t *testing.T) {
	for in, want := range map[string]string{
		"Glass Harbor":          "Glass Harbor.png",
		`AC/DC: "Back"?`:        "AC-DC- -Back.png",
		"   ":                   "MixMusic.png",
		"...":                   "MixMusic.png",
		"Ñandú blanco":          "Ñandú blanco.png",
		"a\nb":                  "a-b.png",
		string(make([]rune, 0)): "MixMusic.png",
	} {
		if got := cardFileName(in); got != want {
			t.Errorf("cardFileName(%q) = %q, want %q", in, got, want)
		}
	}
	long := cardFileName(string(slices.Repeat([]rune("é"), 200)))
	if len([]rune(long)) != 84 {
		t.Errorf("a long name has %d letters", len([]rune(long)))
	}
}

// TestLyricsShare chooses lines of the lyrics, which the click doesn't
// play from, and saves them as a card.
func TestLyricsShare(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Source: "LRCLIB",
		Lines: []lyrics.Line{{Text: "Paper kites"}, {Text: "Over the river"}, {Text: "Under the bridge"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 900)
	dir := t.TempDir()
	asked := savesTo(t, filepath.Join(dir, "lyrics.png"))
	s.open("/lyrics")
	tt.Frame()
	if tt.HasText("save image") {
		t.Fatal("the lyrics offer to save lines before any is chosen")
	}
	click(t, s, tt, "share lines")
	if !tt.HasText("Click the lines to share.") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	click(t, s, tt, "Paper kites")
	click(t, s, tt, "Under the bridge")
	if !tt.HasText("2 lines chosen.") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	// A line chosen again lets go.
	click(t, s, tt, "Under the bridge")
	if !tt.HasText("1 line chosen.") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	click(t, s, tt, "Under the bridge")
	click(t, s, tt, "save image")
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("no save dialog opened")
	}
	img := decodedCard(t, s, filepath.Join(dir, "lyrics.png"))
	if got := img.Bounds().Size(); got != image.Pt(sharecard.Square.W, sharecard.Square.H) {
		t.Errorf("the card is %v", got)
	}
	if got := s.lyricPick.chosen(); !slices.Equal(got, []string{"Paper kites", "Under the bridge"}) {
		t.Errorf("chosen %q", got)
	}
	click(t, s, tt, "done")
	if tt.HasText("save image") || s.lyricPick.on {
		t.Error("done leaves lines chosen")
	}
}

// TestLyricsPick holds as many lines as a card fits.
func TestLyricsPick(t *testing.T) {
	var p lyricsPick
	for i := range lyricsPickMax + 3 {
		p.toggle(i, "line")
	}
	if len(p.lines) != lyricsPickMax {
		t.Errorf("%d lines chosen", len(p.lines))
	}
	if p.toggle(lyricsPickMax, "more") {
		t.Error("a line past the most was chosen")
	}
	if p.toggle(0, "line") {
		t.Error("a line chosen twice stayed chosen")
	}
}
