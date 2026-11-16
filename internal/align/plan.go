package align

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// What a shift is taken on (#565).
const (
	// Reach is how far the shifts searched go either way: from Reach
	// before the song starting at the video's start, or at its end less
	// the song's length, whichever is earlier, to Reach after the later.
	Reach = 120 * time.Second
	// MinScore is the normalized correlation a window's best match
	// reaches at least.
	MinScore = 0.4
	// Clearance is how much lower than the best match's score the next
	// best is at least, for the best to stand clear of it: that, and a
	// fifth of what the best lacks of 1 more, so that a weaker match stands
	// further clear. The same recording matches itself at 0.95 or more,
	// and a song of one loop each bar of it at nearly as much.
	Clearance = 0.04
	// Agree is how far apart the shifts of the windows that match lie at
	// most.
	Agree = 100 * time.Millisecond
	// MaxSpeed is how much faster or slower the song plays in the video at
	// most, as the shifts of its first and last windows tell.
	MaxSpeed = 0.01
	// MinOverlap is how long a window is at least: the song's and the
	// video's audio overlap that much at each shift searched.
	MinOverlap = 3 * time.Second
)

const (
	// window is how long a window of the song is, unless a sixth of the
	// song is shorter.
	window = 10 * time.Second
	// settle is read before each stretch compared, for the envelope's
	// filters to settle.
	settle = 100 * time.Millisecond
	// longest is the length of the longest song or video measured.
	longest = 20 * time.Minute
)

// windowsAt are where the windows of the song lie, as shares of its
// length: far apart, so that a verse the video cuts between two of them
// moves the second.
var windowsAt = []float64{0.2, 0.5, 0.8}

// ErrNoMatch is a song the video doesn't hold as it is, as a live take,
// or one with a verse cut, or whose match isn't clear.
var ErrNoMatch = errors.New("align: no clear match")

// Stretch is a stretch of a recording, from From to To.
type Stretch struct{ From, To time.Duration }

// Plan is what to read of a song and of its music video to find where the
// song starts in the video.
type Plan struct {
	// Windows are the stretches of the song to read, each a little longer
	// than the window it compares, which starts settle after it.
	Windows []Stretch
	// Video is the stretch of the video to read.
	Video Stretch
	// Min and Max bound the shifts searched.
	Min, Max time.Duration

	starts []time.Duration
	length time.Duration
}

// NewPlan returns the plan of a song and of its music video as long as
// song and video, or false when there is too little to compare: a song
// under 18 seconds, of windows under MinOverlap, or a song or video over
// 20 minutes.
func NewPlan(song, video time.Duration) (Plan, bool) {
	if song <= 0 || video <= 0 || song > longest || video > longest {
		return Plan{}, false
	}
	length := min(window, song/6).Truncate(Hop)
	if length < MinOverlap {
		return Plan{}, false
	}
	p := Plan{length: length, Min: min(0, video-song) - Reach, Max: max(0, video-song) + Reach}
	for _, at := range windowsAt {
		start := (time.Duration(at*float64(song)) - length/2).Truncate(Hop)
		p.starts = append(p.starts, start)
		p.Windows = append(p.Windows, Stretch{start - settle, start + length})
	}
	p.Video = Stretch{max(0, p.starts[0]+p.Min-settle), min(video, p.starts[len(p.starts)-1]+length+p.Max)}
	if p.Video.To-p.Video.From < length+settle {
		return Plan{}, false
	}
	return p, true
}

// Result is where a song starts in its music video.
type Result struct {
	// Shift is the time of the video the song's 0:00 plays at: less than
	// 0 when the video leaves the song's first seconds out.
	Shift time.Duration
	// Windows tells how each window matched, for the log.
	Windows string
}

// Shift finds where the song starts in the video, from the envelopes of
// the plan's Windows, in their order, and of its Video, as Levels made
// them from where each was read. It is ErrNoMatch unless two windows or
// more match clearly: their best match reaches MinScore, stands clear of
// the next best and isn't at the edge of the shifts searched; and the
// shifts of those that do agree within Agree, and tell the song plays at
// a speed within MaxSpeed of its own. The shift is then the mean of
// theirs.
func (p Plan) Shift(windows []Envelope, video Envelope) (Result, error) {
	if video.At > 0 {
		video = video.From(video.At + settle)
	}
	in := searchIn(video)
	n := int(p.length / Hop)
	type matched struct {
		start, shift time.Duration
	}
	var clear []matched
	var told []string
	for i, w := range windows[:min(len(windows), len(p.starts))] {
		start := p.starts[i]
		part := w.From(start).first(n)
		if part.Frames() < n {
			told = append(told, "short")
			continue // the song ended first
		}
		m, ok := in.find(part, start+p.Min, start+p.Max)
		if !ok {
			told = append(told, "none")
			continue
		}
		shift := m.at - start
		edge := ""
		if m.edge {
			edge = ", at the edge"
		}
		told = append(told, fmt.Sprintf("%+.3fs (%.2f, next %.2f%s)", shift.Seconds(), m.score, m.next, edge))
		if m.score >= MinScore && m.next <= m.score-Clearance-(1-m.score)/5 && !m.edge {
			clear = append(clear, matched{start, shift})
		}
	}
	r := Result{Windows: strings.Join(told, ", ")}
	why := func(what string) error {
		return fmt.Errorf("%w: %s: %s", ErrNoMatch, what, r.Windows)
	}
	if len(clear) < 2 {
		return r, why("too few windows match clearly")
	}
	least, most, sum := clear[0].shift, clear[0].shift, time.Duration(0)
	for _, m := range clear {
		least, most, sum = min(least, m.shift), max(most, m.shift), sum+m.shift
	}
	if most-least > Agree {
		return r, why("the windows disagree")
	}
	first, last := clear[0], clear[len(clear)-1]
	if speed := float64(last.shift-first.shift) / float64(last.start-first.start); speed <= -MaxSpeed || speed >= MaxSpeed {
		return r, why("the speeds differ")
	}
	r.Shift = (sum / time.Duration(len(clear))).Round(time.Millisecond)
	return r, nil
}
