package app

// Track info (#342): what MixMusic knows of a track and of how it plays,
// as the dialog of Track info in a track's menu shows it. It reads the
// file a track plays from, and what the resolver and the engine hold of
// it, and asks no service.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// TrackPlays is what plays a track.
type TrackPlays string

const (
	// PlaysEngine plays it in MixMusic's own audio engine.
	PlaysEngine TrackPlays = "engine"
	// PlaysSpotify plays it in Spotify's own player, in MixMusic.
	PlaysSpotify TrackPlays = "spotify"
	// PlaysConnect plays it on a Spotify Connect device.
	PlaysConnect TrackPlays = "connect"
)

// FileKind is what the file a track plays from is.
type FileKind string

const (
	// FileMusic is a track's own file, of the music on this computer.
	FileMusic FileKind = "music"
	// FileDownload is the audio of its recording, downloaded.
	FileDownload FileKind = "download"
	// FileCache is the audio of its recording, which the offline cache
	// kept once it played to its end.
	FileCache FileKind = "cache"
)

// TrackInfo is what MixMusic knows of a track, and of how it plays.
type TrackInfo struct {
	// Plays is what plays it: Spotify's tracks play in Spotify's own
	// player while the settings ask for it, which tells MixMusic nothing
	// of their audio, and the rest in MixMusic's engine.
	Plays TrackPlays
	// Playing is set for the track that plays.
	Playing bool
	// Recording is the track of YouTube Music that plays for a Spotify
	// track, once MixMusic found it, or that its download holds.
	Recording *media.Track
	// File is the file its audio plays from, when one holds it: its own,
	// its download, or the offline cache's copy.
	File *TrackFile
	// Stream is YouTube's stream of its audio, when it plays from none of
	// those and MixMusic holds the one it found last.
	Stream *TrackStream
	// Surround is set when it plays YouTube's 5.1 surround audio, as the
	// settings ask.
	Surround bool
	// Engine is what MixMusic's audio engine plays of the track playing,
	// once it loaded it.
	Engine *audio.Loaded
	// LKFS is how loud YouTube measured its audio, when Measured.
	LKFS     float64
	Measured bool
	// EvenLoudness and Levelling tell whether the settings have Even
	// loudness and Volume levelling on, and Trim is the gain, in dB, Even
	// loudness plays the track at: 0, or less for a loud one.
	EvenLoudness, Levelling bool
	Trim                    float64
}

// TrackFile is a file a track plays from.
type TrackFile struct {
	Kind FileKind
	Path string
	// Size is its size, in bytes.
	Size int64
	// Format is what it holds, as the engine reads it, unless Problem
	// tells why it can't.
	Format  audio.Format
	Problem string
	// Tags are the tags of a file of the music on this computer, without
	// its picture.
	Tags *tags.Tags
}

// Bitrate is how many kbit/s the file holds in each second of its audio,
// on average, its tags and pictures among them: 0 when its length is
// unknown.
func (f TrackFile) Bitrate() float64 {
	if f.Format.Duration <= 0 {
		return 0
	}
	return float64(f.Size) * 8 / 1000 / f.Format.Duration.Seconds()
}

// TrackStream is YouTube's stream of a track's audio.
type TrackStream struct {
	// Codec and Container name what it is, as people know them: "Opus"
	// and "WebM".
	Codec, Container string
	// Bitrate is in kbit/s, as YouTube tells it.
	Bitrate float64
	// Size is in bytes, 0 when unknown, as of a live stream.
	Size     int64
	Duration time.Duration
	Live     bool
	// Route is how MixMusic found it, and Command, of a run of yt-dlp, the
	// arguments it ran with, as yt-dlp's command line, without the file of
	// the user's session, tokens, or their home folder (stream.ShownArgs).
	Route   stream.Route
	Command string
	// AsAccount is set when it was asked for as the user's YouTube Music
	// account, and not signed out.
	AsAccount bool
	// URL is the address of the audio, which the engine plays through its
	// proxy. It stops working after a few hours, it is tied to the network
	// that asked for it, and it may carry that network's IP address: it is
	// never logged, and only shown to the user who copies it.
	URL string
}

// engineLoaded returns what MixMusic's audio engine plays, while the
// player plays in it: tests replace it.
var engineLoaded = func(a *App) (audio.Loaded, bool) {
	if a.goPlayer.Load() == nil || a.cast.remote.Load() {
		return audio.Loaded{}, false
	}
	e := a.nativeEngine()
	if e == nil {
		return audio.Loaded{}, false
	}
	return e.Loaded()
}

// TrackInfo tells what MixMusic knows of a track, and of how it plays:
// now is how it plays when it's the track that plays, nil otherwise. It
// reads the file the track plays from, and asks no service: of a track
// that didn't play, it may know no more than where it comes from.
func (p *Player) TrackInfo(ctx context.Context, t media.Track, now *player.Source) (TrackInfo, error) {
	a := p.a
	st := a.settings.Get()
	info := TrackInfo{Plays: PlaysEngine, Playing: now != nil, EvenLoudness: st.Loudness, Levelling: st.Normalize}
	if t.Source == media.SourceFile {
		path, ok := a.localFiles.FileOf(t.ID)
		if !ok {
			return TrackInfo{}, errors.New("not a file of this computer: " + t.ID)
		}
		info.File = readTrackFile(FileMusic, path, true)
		info.engine(a, now)
		return info, ctx.Err()
	}
	if plays, ok := spotifyPlayer(t, st, now); ok {
		info.Plays = plays
		return info, nil
	}
	rec, video := recordingOf(a, t, now)
	if rec.ID != "" && rec.ID != t.ID {
		info.Recording = &rec
	}
	if video == "" {
		return info, nil
	}
	info.Surround = now != nil && st.Surround && now.SurroundURL != ""
	key := video
	if info.Surround {
		key = stream.SurroundKey(video)
	}
	// What plays of a video, as the proxy serves it: its download, which
	// holds its stereo audio, else the cache's copy, else its stream.
	if path, ok := a.downloads.Snapshot().VideoFile(video); ok && !info.Surround {
		info.File = readTrackFile(FileDownload, path, false)
	} else if path, _, ok := a.cache.Find(key); ok {
		info.File = readTrackFile(FileCache, path, false)
	} else {
		known := a.resolver.Known
		if info.Surround {
			known = a.resolver.KnownSurround
		}
		if s, ok := known(video); ok {
			info.Stream = trackStream(s)
		}
	}
	if lkfs, ok := a.loudness.lkfs(video); ok {
		info.LKFS, info.Measured = lkfs, true
		if info.EvenLoudness {
			info.Trim = audio.LoudnessTrim(lkfs)
		}
	}
	info.engine(a, now)
	return info, ctx.Err()
}

// spotifyPlayer tells whether Spotify's own player plays a track, and
// which: the track playing, as now tells, or else a track of Spotify,
// while the settings ask for Spotify to play its tracks.
func spotifyPlayer(t media.Track, st settings.Settings, now *player.Source) (TrackPlays, bool) {
	if now != nil {
		switch now.Mode {
		case player.ModeConnect:
			return PlaysConnect, true
		case player.ModeSpotify:
			return PlaysSpotify, true
		}
		return "", false
	}
	if t.Source != media.SourceSpotify {
		return "", false
	}
	switch {
	case st.SpotifyPlayback == settings.SpotifyPlaybackConnect:
		return PlaysConnect, true
	case st.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere:
		return PlaysSpotify, true
	}
	return "", false
}

// recordingOf returns the track of YouTube Music that plays for t, and
// its video's ID, when MixMusic knows it: the one that plays, as now
// tells, else the one its download holds, else of a Spotify track, the
// one found for it before.
func recordingOf(a *App, t media.Track, now *player.Source) (media.Track, string) {
	rec := t
	switch k, ok := a.downloads.Lookup(t.ID); {
	case now != nil && now.Track.ID != "":
		rec = now.Track
	case ok:
		rec = k.Track
	case t.Source == media.SourceSpotify:
		found, ok := a.matcher.Matched(t)
		if !ok {
			return media.Track{}, ""
		}
		rec = found
	}
	source, video, err := media.ParseID(rec.ID)
	if err != nil || source != media.SourceYouTube || !stream.ValidVideoID(video) {
		return media.Track{}, ""
	}
	return rec, video
}

// engine adds what the engine plays of the track playing, as now tells
// it plays: its loudness as the engine plays it too.
func (info *TrackInfo) engine(a *App, now *player.Source) {
	if now == nil || now.Mode != player.ModeStream {
		return
	}
	l, ok := engineLoaded(a)
	if !ok || l.URL == "" || l.URL != now.URL && l.URL != now.SurroundURL {
		return
	}
	info.Engine = &l
	info.Surround = now.SurroundURL != "" && l.URL == now.SurroundURL
	info.EvenLoudness, info.Levelling = l.EvenLoudness, l.Levelling
	info.LKFS, info.Measured, info.Trim = l.LKFS, l.Measured, l.Trim
}

// readTrackFile reads a file a track plays from: its size, what it holds,
// and its tags, when withTags.
func readTrackFile(kind FileKind, path string, withTags bool) *TrackFile {
	tf := &TrackFile{Kind: kind, Path: path}
	f, err := os.Open(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		tf.Problem = "File not found"
		return tf
	case err != nil:
		tf.Problem = "Can't read the file"
		return tf
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil {
		tf.Size = fi.Size()
	}
	if tf.Format, err = audio.Inspect(f); err != nil {
		tf.Problem = "MixMusic can't play this file"
		if !errors.Is(err, audio.ErrCantPlay) {
			tf.Problem = "Can't read the file"
		}
	}
	if withTags {
		if tg, err := tags.Read(f); err == nil {
			tg.Picture = nil
			tf.Tags = &tg
		}
	}
	return tf
}

// trackStream is what Track info tells of a stream, in the words people
// know its codec and its file by.
func trackStream(s stream.Stream) *TrackStream {
	ts := &TrackStream{Codec: codecName(s.Codec), Container: containerName(s.MimeType), Bitrate: s.Bitrate,
		Size: s.Size, Duration: s.Duration, Live: s.Live, Route: s.Route, AsAccount: s.AsAccount, URL: s.URL}
	if len(s.Args) > 0 {
		ts.Command = shellLine(append([]string{"yt-dlp"}, stream.ShownArgs(s.Args)...))
	}
	if s.Live && ts.Container == "" {
		// The packed audio of HLS.
		ts.Codec, ts.Container = "AAC", "ADTS"
	}
	return ts
}

// shellLine writes words as a command line: those with a space, a
// semicolon or a quote in double quotes.
func shellLine(words []string) string {
	parts := make([]string, len(words))
	for i, w := range words {
		if w == "" || strings.ContainsAny(w, " \t;&|<>\"'") {
			w = `"` + strings.ReplaceAll(w, `"`, `\"`) + `"`
		}
		parts[i] = w
	}
	return strings.Join(parts, " ")
}

// codecName names a codec as yt-dlp and YouTube's player API name it, as
// "opus" or "mp4a.40.2": "Opus" or "AAC".
func codecName(c string) string {
	switch {
	case c == "opus":
		return "Opus"
	case c == "ac-3":
		return "AC-3"
	case c == "ec-3":
		return "E-AC-3"
	case strings.HasPrefix(c, "mp4a"):
		return "AAC"
	}
	return c
}

// containerName names the kind of file of a content type, as
// `audio/webm; codecs="opus"`: "WebM", or "MP4".
func containerName(mime string) string {
	switch {
	case strings.HasPrefix(mime, "audio/webm"):
		return "WebM"
	case strings.HasPrefix(mime, "audio/mp4"):
		return "MP4"
	}
	return ""
}
