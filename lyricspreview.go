package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The preview in Settings › Appearance › Lyrics style shows the style, the
// size of its text, the ball and its motion between lines as the full-screen
// lyrics will, in small: made-up lines, sung one after another in a loop,
// over the ground of the cover that plays, so that the user sees what each
// setting does as it is changed. It draws its lines as the stage does
// (stageLineView), and follows a made-up song's clock rather than the
// player's: a clock that goes on from line to line, as the ball's flight from
// one line to the next needs.

// previewLines are the made-up lines in English, which the tests count the
// lines the preview shows by; the app sings previewLinesOf.
var previewLines = []string{"Slow wind over the pier", "Lanterns drifting home", "Quiet as the harbour light", "Carry me back to shore"}

// previewLinesOf are the made-up lines, in the app's language.
func previewLinesOf() []string {
	return []string{locales.LyricsPreviewLineOne(), locales.LyricsPreviewLineTwo(), locales.LyricsPreviewLineThree(), locales.LyricsPreviewLineFour()}
}

const (
	// previewLineFor is how long a line of the preview is sung, and
	// previewSung how much of that its words take, one after another: the rest
	// is the ball's, to go on to the next line in.
	previewLineFor = 4000 * time.Millisecond
	previewSung    = 2200 * time.Millisecond
	// previewScale is how much smaller the preview's text is than the
	// stage's, and previewHeight how tall the preview is.
	previewScale  = 0.36
	previewHeight = 176
	// previewTick is how often the preview is built anew: as often as the
	// player tells the time.
	previewTick = 250 * time.Millisecond
)

// previewWordsOf returns the words of the made-up line of that place in the
// song, which starts as the place's turn comes: line 0 starts at 0, line 1
// previewLineFor after.
// Its words are sung one after another over previewSung, from the line's start.
func previewWordsOf(line int) []lyrics.Word {
	lines := previewLinesOf()
	base := int(time.Duration(line) * previewLineFor / time.Millisecond)
	fields := strings.Fields(lines[line%len(lines)])
	step := previewSung / time.Duration(len(fields))
	out := make([]lyrics.Word, len(fields))
	for j, f := range fields {
		start := time.Duration(j) * step
		out[j] = lyrics.Word{Text: f, StartMs: int(start/time.Millisecond) + base, EndMs: int((start+step*9/10)/time.Millisecond) + base}
	}
	return out
}

// lyricPreview shows the preview: the line sung and the ones around it, in
// the style and size set, with the ball if it's on, on the ground of the
// cover playing. It asks to be built anew every previewTick while it shows,
// and when a line ends.
func (s *shell) lyricPreview(c *ui.Context) {
	k := &s.lk
	style := lyricStyleOf(s.set.LyricsStyle)
	box := ui.Box(c.Key("lyric-preview")).Height(previewHeight).MaxWidth(560).Margin(14, 0).Radius(14).Clip().Label(locales.LyricsPreviewLabel())
	var b *ui.Bitmap
	if cur, ok := s.st.Current(); ok && len(cur.Track.Artwork) > 0 {
		b = s.art.get(c, artworkURL(cur.Track.Artwork, 64), 64)
	}
	accent := k.accent
	box.Background(ui.Color{A: 255}).Draw(func(p *ui.Painter, r ui.Rect) { paintStageGround(p, r, b, accent, style.scrim) })
	// The made-up song's time, and how far into it the preview is: it starts as
	// the preview is first built.
	ls := &s.lyricsView
	if ls.previewAt.IsZero() {
		ls.previewAt = c.Now()
	}
	pos := max(c.Now().Sub(ls.previewAt), 0)
	if s.inView(c, box) {
		// Built anew as often as a player tells the time, and as a line ends,
		// when the next is sung.
		c.After(min(previewTick, previewLineFor-pos%previewLineFor))
	}
	lines := previewLinesOf()
	n := len(lines)
	active := int(pos / previewLineFor)
	state := &player.State{Playing: true, Position: pos, Speed: 1}
	box.Children(func() {
		ui.Column(c).Height(previewHeight).Justify(ui.Center).Padding(0, 14).Children(func() {
			for d := -1; d <= 1; d++ {
				line := active + d
				if line < 0 {
					// Before the first, the last: the preview goes round.
					line += n * (1 + (-line)/n)
				}
				dist := d
				if dist < 0 {
					dist = -dist
				}
				v := stageLineView{
					style: style, scale: previewScale, text: lines[line%n], key: "preview-" + strconv.Itoa(line%n),
					opacity: style.opacity(dist), blur: style.blurAt(dist), state: state, ball: s.set.LyricsBall,
				}
				if d == 0 {
					v.words, v.active = previewWordsOf(line), true
					v.rig, v.line, v.motion = &ls.previewBall, line, s.set.LyricsBallMotion
					next := previewWordsOf(line + 1)
					v.next = ballNext{ok: true, text: lines[(line+1)%n], word: next[0]}
				}
				s.stageLineView(c, v)
			}
		})
		// The ball flies over the lines, inside the preview.
		ballOverlay(c, &ls.previewBall)
	})
}
