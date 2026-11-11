package main

// Track info (#342): the item of every track's menu that tells, in a
// dialog, what a track is and how it plays: where it comes from, its codec
// and file, its bitrate, sample rate, bit depth, channels and length, and
// of a file on this computer, its path, size and tags; of the track that
// plays, what MixMusic's audio engine plays and how loud. What it tells
// comes from internal/app's TrackInfo.

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// trackInfoView is the dialog of Track info: whether it shows, the track
// it tells of, and when it opened and last asked for what it shows.
type trackInfoView struct {
	open          bool
	track         media.Track
	opened, asked time.Time
}

// trackInfoWait is how long the dialog of the track playing asks again,
// once a second, for what the engine plays of it, until the engine opened
// it.
const trackInfoWait = 10 * time.Second

// showInFolder shows a file in the system's file manager; tests replace
// it.
var showInFolder = func(path string) { mygo.Shell.ShowItemInFolder(path) }

// trackInfoOf tells what MixMusic knows of t, which plays as now tells
// when it's the track playing; tests replace it.
var trackInfoOf = func(s *shell, ctx context.Context, t media.Track, now *player.Source) (app.TrackInfo, error) {
	return s.svc.Player.TrackInfo(ctx, t, now)
}

// trackInfoItem is the item of a track's menu that shows its Track info.
func (s *shell) trackInfoItem(m *ui.Menu, t media.Track) {
	if m.Item("Track info").Chosen() {
		s.showTrackInfo(t)
	}
}

// showTrackInfo shows the Track info of t, read anew.
func (s *shell) showTrackInfo(t media.Track) {
	s.trackInfo = trackInfoView{open: true, track: t}
	s.forget(trackInfoKey(t))
}

func trackInfoKey(t media.Track) string { return "track-info:" + t.ID }

// playingSource is how t plays, while it's the track that plays: nil
// otherwise, and until the player knows.
func (s *shell) playingSource(t media.Track) *player.Source {
	cur, ok := s.st.Current()
	if !ok || cur.Track.ID != t.ID || s.st.Source == nil {
		return nil
	}
	src := *s.st.Source
	return &src
}

// trackInfoDialog shows the Track info of the track the dialog tells of,
// over the window, with Close, and Show in Explorer or Finder for a file.
func (s *shell) trackInfoDialog(c *ui.Context) {
	v := &s.trackInfo
	if !v.open {
		return
	}
	t := v.track
	now := s.playingSource(t)
	key := trackInfoKey(t)
	l := fetch(s, key, func(ctx context.Context) (app.TrackInfo, error) { return trackInfoOf(s, ctx, t, now) })
	at := c.Now()
	if v.opened.IsZero() {
		v.opened, v.asked = at, at
	}
	// The track started or stopped playing, or the engine opens it a
	// moment after it's asked to: what the dialog shows is asked again.
	if l.done && l.err == nil && (l.value.Playing != (now != nil) || waitsForEngine(l.value, now)) &&
		at.Sub(v.asked) >= time.Second && at.Sub(v.opened) < trackInfoWait {
		v.asked = at
		s.refetch(key)
	}
	s.dialog(c, &v.open, dialogOptions{key: "track-info", title: "track info", width: 560}, func() {
		s.trackInfoBody(c, t, l)
	}, func() {
		if f := l.value.File; l.done && f != nil && f.Kind != app.FileCache && f.Problem != "File not found" {
			if s.dialogButton(c, "track-info-show", strings.ToLower(showInFolderLabel()), false).Clicked() {
				showInFolder(f.Path)
			}
		}
		if s.dialogButton(c, "track-info-close", "close", true).AutoFocus().Clicked() {
			v.open = false
		}
	})
}

// waitsForEngine reports whether info, of the track playing as now
// tells, waits for the engine to open it.
func waitsForEngine(info app.TrackInfo, now *player.Source) bool {
	return now != nil && now.Mode == player.ModeStream && info.Plays == app.PlaysEngine &&
		(info.Engine == nil || !info.Engine.Ready)
}

// showInFolderLabel names the action that shows a file in the system's
// file manager.
func showInFolderLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "Show in Finder"
	case "linux":
		return "Show in its folder"
	}
	return "Show in Explorer"
}

// trackInfoBody is what the dialog of Track info tells of t: its title
// and artists, then the facts of l, its tags, and its copies.
func (s *shell) trackInfoBody(c *ui.Context, t media.Track, l *loaded[app.TrackInfo]) {
	k := &s.lk
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, t.Title).FontSize(k.bodySize()).FontWeight(k.leadWeight()).Selectable()
		if line := trackByline(t); line != "" {
			ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted).Selectable()
		}
	})
	switch {
	case !l.done:
		ui.Text(c, "Reading what it holds…").FontSize(k.smallSize()).TextColor(k.muted)
		return
	case l.err != nil:
		ui.Text(c, l.err.Error()).FontSize(k.smallSize()).TextColor(k.danger)
		return
	}
	s.infoRows(c, trackFacts(t, l.value))
	s.streamCopy(c, l.value.Stream)
	if rows := tagRows(l.value); len(rows) > 0 {
		ui.Text(c, k.sentence("its tags")).FontSize(k.smallSize()).FontWeight(k.leadWeight()).Margin(8, 0, 0)
		s.infoRows(c, rows)
	}
	s.trackCopies(c, t, l.value)
}

// trackByline is the artists and the album of t, as "M83 · Hurry Up,
// We're Dreaming".
func trackByline(t media.Track) string {
	parts := []string{}
	if a := names(t.Artists); a != "" {
		parts = append(parts, a)
	}
	if t.Album != nil && t.Album.Name != "" {
		parts = append(parts, t.Album.Name)
	}
	return strings.Join(parts, " · ")
}

// infoRow is a fact of Track info: its label, and its value, which a
// problem shows in the danger colour.
type infoRow struct {
	label, value string
	problem      bool
}

// infoRows shows rows of facts, each label beside its value, which can be
// selected and copied.
func (s *shell) infoRows(c *ui.Context, rows []infoRow) {
	k := &s.lk
	ui.Column(c).Gap(8).Children(func() {
		for _, r := range rows {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Text(c, r.label).FontSize(k.smallSize()).TextColor(k.muted).Width(112).Shrink(0)
				v := ui.Text(c, r.value).FontSize(k.smallSize()).Grow(1).Shrink(1).MinWidth(0).Selectable()
				if r.problem {
					v.TextColor(k.danger)
				}
			})
		}
	})
}

// trackFacts are the rows of Track info of t: where it comes from, what
// plays it, what its audio is, the file it plays from, what the engine
// plays of it while it plays, and how loud.
func trackFacts(t media.Track, info app.TrackInfo) []infoRow {
	rows := []infoRow{{"Source", sourceLine(t, info), false}}
	if r := info.Recording; r != nil {
		rows = append(rows, infoRow{"Plays as", r.Title + " · " + names(r.Artists) + ", on YouTube Music", false})
	}
	switch info.Plays {
	case app.PlaysSpotify, app.PlaysConnect:
		where := "In Spotify's own player, in MixMusic"
		if info.Plays == app.PlaysConnect {
			where = "On a Spotify Connect device"
		}
		rows = append(rows, infoRow{"Plays", where, false},
			infoRow{"Quality", "As your Spotify account and Spotify's settings choose: Spotify tells MixMusic nothing of its audio.", false})
		if t.DurationMs > 0 {
			rows = append(rows, infoRow{"Length", clock(ms(t.DurationMs)), false})
		}
		return append(rows, infoRow{"Loudness", "Spotify plays it at its own loudness.", false})
	}
	f, ok := formatOf(info)
	st := info.Stream
	switch {
	case ok:
		rows = append(rows, infoRow{"Format", formatName(f.Codec, f.Container), false})
	case st != nil:
		rows = append(rows, infoRow{"Format", formatName(st.Codec, st.Container), false})
	case info.File == nil && t.Source == media.SourceSpotify && info.Recording == nil:
		rows = append(rows, infoRow{"Format", "MixMusic finds its recording on YouTube Music as it plays, and tells its format then.", false})
	case info.File == nil:
		rows = append(rows, infoRow{"Format", "MixMusic finds its stream as it plays, and tells its format then.", false})
	}
	switch file := info.File; {
	case st != nil && st.Bitrate > 0:
		rows = append(rows, infoRow{"Bitrate", strconv.Itoa(int(st.Bitrate+0.5)) + " kbit/s", false})
	case file != nil && file.Bitrate() > 0:
		rows = append(rows, infoRow{"Bitrate", "About " + strconv.Itoa(int(file.Bitrate()+0.5)) + " kbit/s, on average", false})
	}
	if ok && f.SampleRate > 0 {
		rows = append(rows, infoRow{"Sample rate", kilohertz(f.SampleRate), false})
	}
	if ok && f.BitDepth > 0 {
		depth := strconv.Itoa(f.BitDepth) + "-bit"
		if f.Float {
			depth += " floating point"
		}
		rows = append(rows, infoRow{"Bit depth", depth, false})
	}
	if ok && f.Channels > 0 {
		rows = append(rows, infoRow{"Channels", channelsName(f.Channels), false})
	}
	if length := lengthOf(t, info, f); length != "" {
		rows = append(rows, infoRow{"Length", length, false})
	}
	if file := info.File; file != nil {
		value := file.Path
		if file.Problem != "" {
			value += " · " + file.Problem
		}
		rows = append(rows, infoRow{fileLabel(file.Kind), value, file.Problem != ""})
		if file.Size > 0 {
			rows = append(rows, infoRow{"Size", byteSize(file.Size), false})
		}
	} else if st != nil && st.Size > 0 {
		rows = append(rows, infoRow{"Size", byteSize(st.Size), false})
	}
	rows = append(rows, streamRows(st)...)
	if info.Playing {
		rows = append(rows, infoRow{"Playing", playingLine(info), false})
	}
	return append(rows, infoRow{"Loudness", loudnessLine(t, info), false})
}

// formatOf is what a track's audio is: what the engine plays of the track
// playing, once it opened it, else what its file holds.
func formatOf(info app.TrackInfo) (audio.Format, bool) {
	if e := info.Engine; e != nil && e.Ready {
		return e.Format, true
	}
	if f := info.File; f != nil && f.Problem == "" {
		return f.Format, true
	}
	return audio.Format{}, false
}

// sourceLine tells where t comes from: its service, and whether it plays
// from its download, or this computer.
func sourceLine(t media.Track, info app.TrackInfo) string {
	if t.Source == media.SourceFile {
		return "A file on this computer"
	}
	line := sourceName(t.Source)
	if f := info.File; f != nil && f.Kind == app.FileDownload {
		line += ", downloaded"
	}
	return line
}

// formatName names a codec in its kind of file, as "Opus in WebM", or
// "FLAC" alone, which is both.
func formatName(codec, container string) string {
	switch container {
	case "", codec:
		return codec
	case "ADTS":
		container = "HLS packed audio"
	}
	return codec + " in " + container
}

// kilohertz writes a sample rate as "44.1 kHz".
func kilohertz(hz int) string {
	return strconv.FormatFloat(float64(hz)/1000, 'f', -1, 64) + " kHz"
}

// channelsName names a count of channels: "Mono", "Stereo", "5.1
// surround", or "4 channels".
func channelsName(n int) string {
	switch n {
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1 surround"
	}
	return strconv.Itoa(n) + " channels"
}

// lengthOf is how long t plays: as its audio tells, else its stream, else
// its service; "Live" for a live stream.
func lengthOf(t media.Track, info app.TrackInfo, f audio.Format) string {
	switch st := info.Stream; {
	case st != nil && st.Live:
		return "Live"
	case f.Duration > 0:
		return clock(f.Duration)
	case st != nil && st.Duration > 0:
		return clock(st.Duration)
	case t.DurationMs > 0:
		return clock(ms(t.DurationMs))
	}
	return ""
}

// fileLabel labels the file a track plays from, of its kind.
func fileLabel(kind app.FileKind) string {
	switch kind {
	case app.FileDownload:
		return "Download"
	case app.FileCache:
		return "Offline copy"
	}
	return "File"
}

// playingLine tells what MixMusic's audio engine plays of the track
// playing, and from where, once it opened it.
func playingLine(info app.TrackInfo) string {
	e := info.Engine
	if e == nil || !e.Ready {
		return "MixMusic's audio engine is opening it…"
	}
	line := "In MixMusic's audio engine, "
	if info.Surround {
		line += "its 5.1 surround audio, "
	}
	switch f := info.File; {
	case f != nil && f.Kind == app.FileMusic:
		line += "from its file"
	case f != nil && f.Kind == app.FileDownload:
		line += "from its download"
	case f != nil && f.Kind == app.FileCache:
		line += "from the offline cache"
	case info.Stream != nil && info.Stream.Live:
		line += "live from YouTube"
	default:
		line += "streamed from YouTube"
	}
	if r := e.Format.SampleRate; r > 0 && r != audio.Rate {
		line += ", converted to " + kilohertz(audio.Rate)
	}
	return line
}

// loudnessLine tells how loud t plays: as YouTube measured it, and what
// Even loudness and Volume levelling make of it.
func loudnessLine(t media.Track, info app.TrackInfo) string {
	var line string
	switch {
	case info.Measured && !info.EvenLoudness:
		line = "YouTube measured it at " + lkfs(info.LKFS) + ". Even loudness is off: it plays at its own level."
	case info.Measured && info.Trim < -0.05:
		line = fmt.Sprintf("YouTube measured it at %s. Even loudness turns it down by %.1f dB.", lkfs(info.LKFS), -info.Trim)
	case info.Measured:
		line = "YouTube measured it at " + lkfs(info.LKFS) + ". Even loudness leaves it as it is: it isn't louder than " + lkfs(-14) + "."
	case t.Source == media.SourceFile:
		line = "It plays at its own level: Even loudness turns tracks down from YouTube's measure of each, which a file on this computer hasn't."
	default:
		line = "MixMusic has no measure of it yet: it plays at its own level."
	}
	if info.Levelling {
		line += " Volume levelling squeezes its loud and quiet moments closer together."
	}
	return line
}

// lkfs writes a loudness as "−9.2 LKFS", with a minus sign.
func lkfs(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), "-", "−", 1) + " LKFS"
}

// tagRows are the rows of the tags of a file on this computer: those it
// has.
func tagRows(info app.TrackInfo) []infoRow {
	f := info.File
	if f == nil || f.Tags == nil {
		return nil
	}
	tg := f.Tags
	var rows []infoRow
	add := func(label, value string) {
		if value != "" {
			rows = append(rows, infoRow{label, value, false})
		}
	}
	number := func(n int) string {
		if n <= 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	add("Title", tg.Title)
	add("Artist", strings.Join(tg.Artists, ", "))
	add("Album", tg.Album)
	add("Album artist", strings.Join(tg.AlbumArtists, ", "))
	add("Track", number(tg.Track))
	add("Disc", number(tg.Disc))
	add("Year", number(tg.Year))
	return rows
}
