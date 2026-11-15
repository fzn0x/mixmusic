package app

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The captions of a video on YouTube as a source of lyrics, for live
// versions and uploads no lyrics service has (settings.LyricsCaptions),
// and as the transcript of a podcast episode.

// captionsWait bounds a lookup of a video's captions, as Better Lyrics'
// and Unison's are: lyrics never wait long on it.
const captionsWait = 3 * time.Second

// captionsSource names the captions where their lyrics show.
const captionsSource = "YouTube captions"

// captionLyrics returns the lyrics of a track from the captions of its
// video on YouTube, in the language Settings chooses when the video has
// it: those of the video itself, or, for a track of another service, of
// its recording there when it is as long, give or take timeSlack.
// Captions YouTube made from speech are left out, and a file of the
// computer isn't looked for.
func (c *Catalog) captionLyrics(ctx context.Context, track media.Track, _ lyrics.Query) (lyrics.Lyrics, error) {
	if track.Source == media.SourceLocal || c.a.captions == nil {
		return lyrics.Lyrics{}, errNotAsked
	}
	ctx, cancel := context.WithTimeout(ctx, captionsWait)
	defer cancel()
	lines, lang, err := c.captionsOf(ctx, track, false)
	if err != nil {
		return lyrics.Lyrics{}, err
	}
	l := lyrics.Timed(lines, captionsSource)
	l.Language = languageOf(lang)
	return l, nil
}

// Transcript returns the transcript of a podcast episode from its
// captions on YouTube, timed, made from speech when it has no other,
// while YouTube captions is a source of lyrics; lyrics.ErrNotFound when
// it has none, or isn't of YouTube.
func (c *Catalog) Transcript(ctx context.Context, episode media.Track) (lyrics.Lyrics, error) {
	on := false
	for _, s := range c.a.settings.Get().SourceOrder() {
		on = on || s.Key == settings.LyricsCaptions && !s.Off
	}
	if !on || episode.Source != media.SourceYouTube || c.a.captions == nil {
		return lyrics.Lyrics{}, lyrics.ErrNotFound
	}
	lines, lang, err := c.captionsOf(ctx, episode, true)
	if err != nil {
		if lyricsNone(err) {
			err = lyrics.ErrNotFound
		}
		return lyrics.Lyrics{}, err
	}
	l := lyrics.Timed(lines, captionsSource)
	l.Language = languageOf(lang)
	return l, nil
}

// captionsOf returns the lines of the captions of track's recording on
// YouTube that lyrics come from (captionTrack), and their language.
func (c *Catalog) captionsOf(ctx context.Context, track media.Track, speech bool) ([]lyrics.Line, string, error) {
	yt, err := c.youtubeRecording(ctx, track)
	if err != nil {
		return nil, "", err
	}
	_, id, err := media.ParseID(yt.ID)
	if err != nil {
		return nil, "", err
	}
	caps, err := c.a.captions.Captions(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if track.Source != media.SourceYouTube && caps.Length > 0 && track.DurationMs > 0 &&
		(caps.Length-time.Duration(track.DurationMs)*time.Millisecond).Abs() > timeSlack {
		// Another recording: its captions wouldn't keep time with the track.
		return nil, "", lyrics.ErrNotFound
	}
	t, ok := captionTrack(caps.Tracks, c.a.settings.Get().LyricsCaptionLanguage, speech)
	if !ok {
		return nil, "", lyrics.ErrNotFound
	}
	text, err := c.a.captions.CaptionText(ctx, t)
	if err != nil {
		return nil, "", err
	}
	lines := captionLines(text)
	if len(lines) == 0 {
		return nil, "", lyrics.ErrNotFound
	}
	return lines, t.Language, nil
}

// captionTrack returns the captions of tracks that lyrics come from:
// those in the language lang when there are, else the video's own, which
// YouTube shows of it, or else those in the language it speaks, or the
// first. Those YouTube made from speech come only where speech allows it
// and there are no others.
func captionTrack(tracks []stream.CaptionTrack, lang string, speech bool) (stream.CaptionTrack, bool) {
	var made, spoken []stream.CaptionTrack
	for _, t := range tracks {
		if t.Speech {
			spoken = append(spoken, t)
		} else {
			made = append(made, t)
		}
	}
	said := ""
	if len(spoken) > 0 {
		said = spoken[0].Language
	}
	if len(made) == 0 {
		if !speech || len(spoken) == 0 {
			return stream.CaptionTrack{}, false
		}
		made = spoken
	}
	for _, exact := range []bool{true, false} {
		for _, t := range made {
			if lang != "" && sameLanguage(t.Language, lang, exact) {
				return t, true
			}
		}
	}
	for _, t := range made {
		if t.Default {
			return t, true
		}
	}
	for _, t := range made {
		if said != "" && sameLanguage(t.Language, said, false) {
			return t, true
		}
	}
	return made[0], true
}

// sameLanguage reports whether the languages a and b, as "pt-BR" and
// "pt", are the same: exactly, or else by their language alone.
func sameLanguage(a, b string, exact bool) bool {
	if exact {
		return strings.EqualFold(a, b)
	}
	return strings.EqualFold(languageOf(a), languageOf(b))
}

// languageOf returns the language of a code of YouTube's, as "pt" of
// "pt-BR".
func languageOf(code string) string {
	lang, _, _ := strings.Cut(code, "-")
	return strings.ToLower(lang)
}

// soundRe matches a caption that only tells a sound, as "[Music]".
var soundRe = regexp.MustCompile(`^\[[^\]]*\]$`)

// captionLines are the lines of lyrics of captions: each caption a line at
// its time, without the notes of music around it, and without those that
// only tell a sound.
func captionLines(caps []stream.Caption) []lyrics.Line {
	lines := []lyrics.Line{}
	for _, cp := range caps {
		text := strings.TrimSpace(strings.Trim(cp.Text, "♪♫🎵🎶 "))
		if text == "" || soundRe.MatchString(text) {
			continue
		}
		lines = append(lines, lyrics.Line{TimeMs: cp.StartMs, Text: text})
	}
	return lines
}
