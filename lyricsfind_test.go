package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// harbourLines are made-up lyrics, "light" in four of their eight lines:
// the first, the fourth, the sixth and the last.
var harbourLines = []lyrics.Line{
	{TimeMs: 1000, Text: "Harbour light on the water"}, {TimeMs: 10000, Text: "Ropes and gulls"},
	{TimeMs: 20000, Text: "Café by the pier"}, {TimeMs: 30000, Text: "Lights out over the bay"},
	{TimeMs: 40000, Text: "Nobody's waiting"}, {TimeMs: 50000, Text: "A slow light rising"},
	{TimeMs: 60000, Text: "Gulls again"}, {TimeMs: 70000, Text: "Last LIGHT of the day"},
}

// harbourNames are the words of harbourLines and of the test's track,
// which show as they are in every language.
var harbourNames = func() []string {
	names := []string{"Midnight City", "M83", "Hurry Up, We're Dreaming"}
	for _, l := range harbourLines {
		names = append(names, l.Text)
	}
	return names
}()

// openFind opens the box that finds lines of the lyrics with its button,
// named name, in the panel's head or beside Share lines.
func openFind(t *testing.T, s *shell, tt *ui.Tester, name string) {
	t.Helper()
	pressNamed(t, s, tt, 0, name)
	if !tt.Focused(name) {
		t.Fatalf("the box to find in the lyrics hasn't the focus: %q", tt.Texts())
	}
}

// findWords types text in the box that has the focus, as a user would,
// then lets the window draw.
func findWords(s *shell, tt *ui.Tester, text string) {
	tt.Type(text)
	s.drain()
	tt.Frame()
	tt.Frame()
}

// press presses key with mods in the box, then lets the window draw.
func press(s *shell, tt *ui.Tester, mods ui.Modifiers, key ui.Key) {
	tt.Key(mods, key)
	s.drain()
	tt.Frame()
	tt.Frame()
}

// groundOf is how far the ground of the line of lyrics text shows from the
// panel's own, at its start, where its padding has no glyph: 0 untinted.
func groundOf(t *testing.T, s *shell, tt *ui.Tester, text string) int {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("%q doesn't show: %q", text, tt.Texts())
	}
	p, g := tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2)), s.lk.surface
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	return d(p.R, g.R) + d(p.G, g.G) + d(p.B, g.B)
}

// brightest is the brightest pixel of the line of lyrics text, as the sum
// of its channels: a line that dims shows none as bright as the others.
func brightest(t *testing.T, tt *ui.Tester, text string) int {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("%q doesn't show: %q", text, tt.Texts())
	}
	img, m := tt.Image(), 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			p := img.RGBAAt(x, y)
			m = max(m, int(p.R)+int(p.G)+int(p.B))
		}
	}
	return m
}

// inView tells whether the line of lyrics text shows in the window, not
// only laid out above or below it.
func inView(tt *ui.Tester, text string) bool {
	r, ok := tt.Find(text)
	return ok && r.Y >= 0 && r.Y+r.H <= float32(tt.Image().Bounds().Dy())
}

// TestLyricsFindSteps finds words in the lyrics panel: the box comes with
// the focus from the search button of the panel's head, the first line
// found is the first at or after the line sung, and Up and Down step
// through the lines found, wrapping. They show tinted, the current one
// more, and a line sung before doesn't dim. Screen readers hear the count
// once typing pauses, and each line a step goes to.
func TestLyricsFindSteps(t *testing.T) {
	s, tt := sungIn(t, "en", harbourLines, 35*time.Second)
	if tt.HasText(locales.LyricsFindNone()) || s.lyricsView.panel.find.open {
		t.Fatal("the box shows before the button is pressed")
	}
	openFind(t, s, tt, locales.LyricsFind())
	if !lineLit(t, s, tt, "Lights out over the bay") {
		t.Fatal("the fourth line isn't the one sung at 0:35")
	}
	tt.Announcements()
	findWords(s, tt, "light")
	if !tt.HasText("2 of 4") {
		t.Fatalf("the line sung, the fourth, isn't the second of four found: %q", tt.Texts())
	}
	if groundOf(t, s, tt, "Lights out over the bay") <= groundOf(t, s, tt, "Harbour light on the water") ||
		groundOf(t, s, tt, "Harbour light on the water") == 0 || groundOf(t, s, tt, "Ropes and gulls") != 0 {
		t.Errorf("the lines aren't tinted as found: current %d, found %d, not found %d", groundOf(t, s, tt, "Lights out over the bay"),
			groundOf(t, s, tt, "Harbour light on the water"), groundOf(t, s, tt, "Ropes and gulls"))
	}
	if past, next := brightest(t, tt, "Ropes and gulls"), brightest(t, tt, "Nobody's waiting"); past < next*9/10 {
		t.Errorf("a line sung before dims while words are found: %d, a line to come %d", past, next)
	}
	tt.Advance(time.Second)
	if got := tt.Announcements(); !slices.Contains(got, "4 lines hold these words") {
		t.Errorf("screen readers heard %q once typing paused", got)
	}
	volume := s.p.State().Volume
	for _, step := range []struct {
		key  ui.Key
		want string
	}{{ui.KeyDown, "3 of 4"}, {ui.KeyDown, "4 of 4"}, {ui.KeyDown, "1 of 4"}, {ui.KeyUp, "4 of 4"}, {ui.KeyUp, "3 of 4"}} {
		press(s, tt, 0, step.key)
		if !tt.HasText(step.want) {
			t.Errorf("after %v, the box doesn't say %q: %q", step.key, step.want, tt.Texts())
		}
	}
	if groundOf(t, s, tt, "A slow light rising") <= groundOf(t, s, tt, "Lights out over the bay") {
		t.Error("the line stepped to isn't tinted the most")
	}
	if got := tt.Announcements(); !slices.Contains(got, "3 of 4: A slow light rising") {
		t.Errorf("screen readers heard %q after the steps", got)
	}
	// The arrows step in the box: they don't turn the volume.
	if v := s.p.State().Volume; v != volume {
		t.Errorf("the arrows in the box moved the volume from %v to %v", volume, v)
	}
	// A word typed further finds again from the line sung.
	findWords(s, tt, "s")
	if !tt.HasText("1 of 1") {
		t.Errorf("\"lights\" isn't found once, the line sung: %q", tt.Texts())
	}
}

// TestLyricsFindFolds finds words regardless of case, accents on Latin
// letters and the shape of quotes, from the first letter typed; and says
// so when no line holds them, without tinting any.
func TestLyricsFindFolds(t *testing.T) {
	s, tt := sungIn(t, "en", harbourLines, 35*time.Second)
	openFind(t, s, tt, locales.LyricsFind())
	for _, c := range []struct{ typed, want string }{
		{"CAFE", "1 of 1"}, {"nobody’s", "1 of 1"}, {"p", "1 of 2"}, {"lighthouse", locales.LyricsFindNone()},
	} {
		findWords(s, tt, c.typed)
		if !tt.HasText(c.want) {
			t.Errorf("%q: the box doesn't say %q: %q", c.typed, c.want, tt.Texts())
		}
		if c.want == locales.LyricsFindNone() {
			for _, l := range harbourLines {
				if groundOf(t, s, tt, l.Text) != 0 {
					t.Errorf("%q shows tinted with no line found", l.Text)
				}
			}
		}
		for range len([]rune(c.typed)) {
			tt.Key(0, ui.KeyBackspace)
		}
		s.drain()
		tt.Frame()
	}
	tt.Advance(time.Second)
	if got := tt.Announcements(); slices.Contains(got, "1 line holds these words") {
		t.Errorf("screen readers heard a count after the box emptied: %q", got)
	}
}

// TestLyricsFindPlays plays the song from the line found with Enter, as a
// click on it does: at its time less the lyrics' timing, and a song paused
// stays paused. While lines are chosen to share, Enter chooses it instead.
func TestLyricsFindPlays(t *testing.T) {
	s, tt := sungIn(t, "en", harbourLines, 35*time.Second)
	s.svc.Catalog.SetLyricsOffset(testTracks[0], shownLyrics(s), 2*time.Second)
	openFind(t, s, tt, locales.LyricsFind())
	findWords(s, tt, "slow")
	press(s, tt, 0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return st.Position == 48*time.Second })
	tt.Frame()
	if !lineLit(t, s, tt, "A slow light rising") {
		t.Error("the line found doesn't light once played from")
	}
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	for range 4 {
		tt.Key(0, ui.KeyBackspace)
	}
	findWords(s, tt, "gulls")
	if !tt.HasText("2 of 2") {
		t.Fatalf("the first line found after 0:48 isn't the seventh: %q", tt.Texts())
	}
	press(s, tt, 0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return st.Position == 58*time.Second })
	if s.p.State().Playing {
		t.Error("Enter played a song paused")
	}
	// Choosing lines to share, Enter chooses the line found.
	click(t, s, tt, "share lines")
	pressNamed(t, s, tt, 0, locales.LyricsFind())
	openFind(t, s, tt, locales.LyricsFind())
	findWords(s, tt, "gulls")
	press(s, tt, 0, ui.KeyDown)
	press(s, tt, 0, ui.KeyEnter)
	if got := s.lyricPick.chosen(); !slices.Equal(got, []string{"Ropes and gulls"}) {
		t.Errorf("Enter chose %q while lines are chosen to share", got)
	}
	if got := s.p.State().Position; got != 58*time.Second {
		t.Errorf("Enter moved the song to %v while lines are chosen", got)
	}
}

// TestLyricsFindHoldsTheLine keeps the line found in view while the box
// holds text, as the song goes on: synced lyrics follow it again once the
// box is empty. Escape empties the box, then closes it, never the panel.
func TestLyricsFindHoldsTheLine(t *testing.T) {
	var lines []lyrics.Line
	for i := range 60 {
		lines = append(lines, lyrics.Line{TimeMs: i * 3000, Text: fmt.Sprintf("Wave number %d", i)})
	}
	lines[2].Text = "The lantern by the door"
	s, tt := sungIn(t, "en", lines, 170*time.Second)
	tt.Frame()
	tt.Frame()
	if inView(tt, "The lantern by the door") {
		t.Fatal("the third line shows while the 57th is sung")
	}
	openFind(t, s, tt, locales.LyricsFind())
	findWords(s, tt, "lantern")
	tt.Advance(time.Second)
	if !inView(tt, "The lantern by the door") || !tt.HasText("1 of 1") {
		t.Fatalf("the line found, wrapping to the top, doesn't show: %q", tt.Texts())
	}
	s.p.Time(176*time.Second, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 176*time.Second })
	tt.Frame()
	tt.Advance(lyricsPause + time.Second)
	if !inView(tt, "The lantern by the door") {
		t.Error("the lyrics followed the song away from the line found")
	}
	press(s, tt, 0, ui.KeyEscape)
	tt.Advance(time.Second)
	if s.lyricsView.panel.find.query != "" || !s.lyricsView.panel.find.open {
		t.Fatalf("Escape left %q in the box, open %v", s.lyricsView.panel.find.query, s.lyricsView.panel.find.open)
	}
	if inView(tt, "The lantern by the door") || !inView(tt, "Wave number 58") {
		t.Error("the lyrics don't follow the song again once the box is empty")
	}
	press(s, tt, 0, ui.KeyEscape)
	if s.lyricsView.panel.find.open || s.panel != "lyrics" {
		t.Errorf("a second Escape left the box open %v, the panel %q", s.lyricsView.panel.find.open, s.panel)
	}
	if tt.Focused(locales.LyricsFind()) {
		t.Error("the box closed with the focus")
	}
}

// TestLyricsFindPlain finds words in plain lyrics, and in synced lyrics
// shown as plain text, line by line: there Enter only scrolls, as the box
// says.
func TestLyricsFindPlain(t *testing.T) {
	s, tt := sungIn(t, "en", harbourLines, 35*time.Second)
	ly := shownLyrics(s)
	ly.Synced, ly.Lines = false, slices.Clone(ly.Lines)
	for i := range ly.Lines {
		ly.Lines[i].TimeMs = 0
	}
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: ly}
	tt.Frame()
	openFind(t, s, tt, locales.LyricsFind())
	if !tt.HasText(locales.LyricsFindPlain()) {
		t.Errorf("the box doesn't say Enter only scrolls in plain lyrics: %q", tt.Texts())
	}
	findWords(s, tt, "light")
	if !tt.HasText("1 of 4") || groundOf(t, s, tt, "Harbour light on the water") <= groundOf(t, s, tt, "A slow light rising") {
		t.Fatalf("plain lyrics start at the first line found: %q", tt.Texts())
	}
	press(s, tt, 0, ui.KeyDown)
	press(s, tt, 0, ui.KeyEnter)
	s.drain()
	if got := s.p.State().Position; got != 35*time.Second {
		t.Errorf("Enter moved a song of plain lyrics to %v", got)
	}
	// Synced lyrics, shown as plain text.
	s, tt = sungIn(t, "en", harbourLines, 35*time.Second)
	s.set.LyricsPlain = true
	tt.Frame()
	openFind(t, s, tt, locales.LyricsFind())
	findWords(s, tt, "light")
	press(s, tt, 0, ui.KeyEnter)
	s.drain()
	if got := s.p.State().Position; got != 35*time.Second || !tt.HasText(locales.LyricsFindPlain()) {
		t.Errorf("Enter moved synced lyrics shown as plain text to %v: %q", got, tt.Texts())
	}
}

// TestLyricsFindShown finds lines by their words in Latin letters and by
// their translation while those show, and by their words as written, from
// one kanji.
func TestLyricsFindShown(t *testing.T) {
	refused := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(refused.Close)
	s := newTestShellOf(t, func(o *app.Options) { o.RomajiURL = refused.URL })
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "ja", Lines: kiteLines}}
	s.loads["lyrics-translation:"+testTracks[0].ID+":en"] = &loaded[translated]{done: true, value: kiteTranslation}
	s.loads[romajiKey(testTracks[0])] = &loaded[romaji.Lines]{done: true, value: kiteRomaji}
	s.set.LyricsTranslate = []string{"ja"}
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeUnder}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	tt.Frame()
	openFind(t, s, tt, locales.LyricsFind())
	for _, c := range []struct{ typed, want string }{
		{"凧", "1 of 1"}, {"TAKO", "1 of 1"}, {"sky", "1 of 1"}, {"か", locales.LyricsFindNone()},
	} {
		findWords(s, tt, c.typed)
		if !tt.HasText(c.want) {
			t.Errorf("%q: the box doesn't say %q: %q", c.typed, c.want, tt.Texts())
		}
		for range len([]rune(c.typed)) {
			tt.Key(0, ui.KeyBackspace)
		}
	}
	// Without the translation, it isn't found by it.
	s.set.LyricsTranslate = nil
	s.set.LyricsRomanize = nil
	findWords(s, tt, "sky")
	if !tt.HasText(locales.LyricsFindNone()) {
		t.Errorf("a translation that doesn't show is found: %q", tt.Texts())
	}
}

// TestLyricsFindNextTrack works the lines found out again as the next track
// plays, so that Enter plays a line of its lyrics, not of those before.
func TestLyricsFindNextTrack(t *testing.T) {
	s, tt := sungIn(t, "en", harbourLines, 35*time.Second)
	s.loads["lyrics:"+testTracks[1].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "en",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Sand and stone"}, {TimeMs: 20000, Text: "A light at the end"}}}}
	openFind(t, s, tt, locales.LyricsFind())
	findWords(s, tt, "light")
	if !tt.HasText("2 of 4") {
		t.Fatalf("%q", tt.Texts())
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 && st.Playing && !st.Loading })
	tt.Frame()
	tt.Frame()
	if !tt.HasText("1 of 1") || !tt.HasText("A light at the end") {
		t.Fatalf("the next track's lyrics aren't found: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return st.Position == 20*time.Second })
}

// TestLyricsFindPalette opens the box from the palette's Find in the
// lyrics: the lyrics panel's, which opens with it, and the now playing
// page's there. Nothing playing, it says to play something first.
func TestLyricsFindPalette(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 900)
	runCommand(t, s, tt, "view.lyricsFind")
	if !tt.HasText(locales.CmdPlayFirst()) || s.panel == "lyrics" {
		t.Errorf("with nothing playing, the panel is %q: %q", s.panel, tt.Texts())
	}
	s, tt = sungOn(t, "/", "en", harbourLines, 35*time.Second)
	if s.panel == "lyrics" {
		t.Fatal("the lyrics panel shows at first")
	}
	runCommand(t, s, tt, "view.lyricsFind")
	tt.Frame()
	if s.panel != "lyrics" || !s.lyricsView.panel.find.open || !tt.Focused(locales.LyricsFind()) {
		t.Fatalf("the palette left the panel %q, its box open %v: %q", s.panel, s.lyricsView.panel.find.open, tt.Texts())
	}
	findWords(s, tt, "light")
	if !tt.HasText("2 of 4") {
		t.Errorf("%q", tt.Texts())
	}
	show(s, tt, "/now-playing")
	runCommand(t, s, tt, "view.lyricsFind")
	tt.Frame()
	if !s.lyricsView.page.find.open {
		t.Fatal("the palette didn't open the box of the now playing page")
	}
	findWords(s, tt, "gulls")
	if s.lyricsView.page.find.query != "gulls" || s.lyricsView.panel.find.query != "light" {
		t.Errorf("the page's box holds %q, the panel's %q", s.lyricsView.page.find.query, s.lyricsView.panel.find.query)
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, locales.LyricsFind()); !ok {
		t.Error("no search button beside Share lines")
	}
	// Ctrl+F stays the music search.
	for _, cmd := range s.commands() {
		if cmd.id == "view.lyricsFind" && s.shortcutOf(cmd) != "" {
			t.Errorf("Find in the lyrics has the keys %q", s.shortcutOf(cmd))
		}
	}
}

// TestLyricsFindRightToLeft finds words in Arabic lyrics, laid out right to
// left, without the vowel signs or the hamza they're written with, and
// plays from the line found.
func TestLyricsFindRightToLeft(t *testing.T) {
	s, tt := sungIn(t, "ar", []lyrics.Line{
		{TimeMs: 0, Text: "يَا قَمَرَ اللَّيْلِ"}, {TimeMs: 30000, Text: "أَنْتَ نُورِي"}, {TimeMs: 60000, Text: "سَلَامٌ عَلَى الْبَحْرِ"},
	}, 10*time.Second)
	tt.SetDirection(ui.RightToLeft)
	tt.Frame()
	openFind(t, s, tt, locales.LyricsFind())
	findWords(s, tt, "قمر")
	if !tt.HasText("1 of 1") {
		t.Errorf("\"moon\" without its vowels isn't found: %q", tt.Texts())
	}
	for range 3 {
		tt.Key(0, ui.KeyBackspace)
	}
	findWords(s, tt, "انت نوري")
	if !tt.HasText("1 of 1") {
		t.Fatalf("\"you are my light\" without its hamza isn't found: %q", tt.Texts())
	}
	press(s, tt, 0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return st.Position == 30*time.Second })
}

// TestPseudoLocaleLyricsFind shows the box that finds lines in en-XA: only
// the lyrics, the track's names and the words typed are English.
func TestPseudoLocaleLyricsFind(t *testing.T) {
	usePseudoLocale(t)
	s, tt := sungIn(t, "en", harbourLines, 35*time.Second)
	tt.SetLocale("en-XA")
	xa := locales.In("en-XA")
	tt.Frame()
	openFind(t, s, tt, xa.LyricsFind())
	findWords(s, tt, "light")
	if !tt.HasText(xa.LyricsFindCount(2, 4)) {
		t.Errorf("no count in en-XA: %q", tt.Texts())
	}
	checkPseudo(t, "The lyrics found", tt.Texts(), append(harbourNames, "light")...)
	findWords(s, tt, "house")
	if !tt.HasText(xa.LyricsFindNone()) {
		t.Errorf("no \"none\" in en-XA: %q", tt.Texts())
	}
	s.set.LyricsPlain = true
	tt.Frame()
	if !tt.HasText(xa.LyricsFindPlain()) {
		t.Errorf("no word of plain lyrics in en-XA: %q", tt.Texts())
	}
	checkPseudo(t, "The lyrics found as plain text", tt.Texts(), append(harbourNames, "lighthouse")...)
}

// TestLyricsFold folds what Find in the lyrics compares: case, accents on
// Latin, Greek and Cyrillic letters only, quotes, spaces, width, and
// Arabic's and Hebrew's vowel signs.
func TestLyricsFold(t *testing.T) {
	for _, c := range []struct {
		line, typed string
		found       bool
	}{
		{"Café au lait", "CAFE", true},
		{"Nobody’s home", "nobody's", true},
		{"“Run”, she said", "\"run\"", true},
		{"hold   on", " hold on ", true},
		{"ＡＢＣ", "abc", true},
		{"がっこう", "か", false},
		{"ｶﾞｯｺｳ", "ガ", true},
		{"하늘로", "하", true},
		{"하늘로", "한", false},
		{"Ελλάδα", "ΕΛΛΑΔΑ", true},
		{"στο δρόμος", "ΔΡΟΜΟΣ", true},
		{"ёжик в тумане", "ежик", true},
		{"мой дом", "мои", false},
		{"Straße", "strasse", true},
		{"IŞIK", "ışık", true},
		{"İstanbul", "istanbul", true},
		{"Łódź", "lodz", true},
		{"cœur", "coeur", true},
		{"أَنْتَ نُورِي", "انت", true},
		{"قـمـر", "قمر", true},
		{"שָׁלוֹם", "שלום", true},
		{"‏سلام", "سلام", true},
		{"नमस्ते", "नमस्ते", true},
	} {
		if got := strings.Contains(lyricsFold(c.line), lyricsFold(c.typed)); got != c.found {
			t.Errorf("%q in %q: %v, folded %q in %q", c.typed, c.line, got, lyricsFold(c.typed), lyricsFold(c.line))
		}
	}
	if got := lyricsFold("  "); got != "" {
		t.Errorf("spaces fold to %q", got)
	}
}
