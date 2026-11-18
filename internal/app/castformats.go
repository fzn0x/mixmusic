package app

// The formats of the audio a renderer or an FCast receiver plays: Opus in
// WebM where it plays it, else YouTube's AAC in MP4, even of a track kept
// as WebM; and the songs on this computer, in their own format, where it
// plays that.

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/internal/fcast"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/upnp"
	"github.com/fzn0x/mixmusic/locales"
)

// castFormat is a format of audio: the MIME types it goes by, MixMusic's
// own first, its name, as the windows name it, the token an FCast
// receiver names it by, and the extension of the address it's served at.
type castFormat struct {
	types       []string
	name, token string
	ext         string
}

var (
	formatWebM   = castFormat{[]string{"audio/webm", "video/webm"}, "Opus", "opus", ".webm"}
	formatAAC    = castFormat{[]string{"audio/mp4", "audio/x-m4a", "audio/m4a", "audio/x-mp4", "video/mp4"}, "AAC", "aac", ".m4a"}
	formatMP3    = castFormat{[]string{"audio/mpeg", "audio/mp3", "audio/x-mpeg"}, "MP3", "mp3", ".mp3"}
	formatFLAC   = castFormat{[]string{"audio/flac", "audio/x-flac"}, "FLAC", "flac", ".flac"}
	formatVorbis = castFormat{[]string{"audio/ogg", "application/ogg", "audio/x-ogg", "audio/vorbis"}, "Ogg Vorbis", "vorbis", ".ogg"}
	formatOpus   = castFormat{[]string{"audio/ogg", "audio/opus", "application/ogg"}, "Opus", "opus", ".opus"}
	formatWAV    = castFormat{[]string{"audio/wav", "audio/x-wav", "audio/wave", "audio/vnd.wave"}, "WAV", "pcm", ".wav"}
)

// fileFormats are the formats of the songs on this computer, by the
// extension of their file, as localfiles finds them.
var fileFormats = map[string]castFormat{".mp3": formatMP3, ".flac": formatFLAC, ".ogg": formatVorbis, ".oga": formatVorbis,
	".opus": formatOpus, ".m4a": formatAAC, ".mp4": formatAAC, ".wav": formatWAV, ".wave": formatWAV}

// errFormatRefused tells that the device plays none of the formats MixMusic
// has a track in: it names the format.
func errFormatRefused(device, format string) error {
	return wordedLater(device+" doesn't play "+format+" audio",
		func() string { return locales.CastRendererFormat(device, format) }, nil)
}

// rendererSource is how a renderer's audio is picked, of the formats its
// sink tells it plays: one that tells none plays MP4 and MP3, as most do.
func (a *App) rendererSource(sink upnp.Sink, server *castServer, name string) func(string) (string, string, error) {
	if len(sink) == 0 {
		sink = upnp.Sink{"http-get:*:audio/mp4:*", "http-get:*:audio/mpeg:*"}
	}
	return func(rawURL string) (string, string, error) {
		return a.castSource(rawURL, server, name, func(f castFormat) string { return sink.Type(f.types...) })
	}
}

// receiverSource is how an FCast receiver's audio is picked: WebM only
// where its v4 capabilities list it, AAC otherwise, as any receiver plays
// it; a song on this computer where it lists its format, or on a v3
// receiver, which lists none.
func (a *App) receiverSource(s *fcast.Session, server *castServer) func(string) (string, string, error) {
	return func(rawURL string) (string, string, error) {
		return a.castSource(rawURL, server, s.Receiver().Name, func(f castFormat) string {
			switch {
			case f.ext == formatWebM.ext:
				if s.Plays("webm", "opus") {
					return f.types[0]
				}
				return ""
			case f.ext == formatAAC.ext, s.Version() < 4, s.PlaysAudio(f.token):
				return f.types[0]
			}
			return ""
		})
	}
}

// castSource returns the path a device fetches the audio of a stream at,
// past its server's token, and its type, of the formats plays tells the
// device plays, as their type it knows: the path is served from then on.
func (a *App) castSource(rawURL string, server *castServer, name string, plays func(castFormat) string) (string, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	if native, ok := strings.CutPrefix(u.Path, "/file/"); ok {
		file, ok := localfiles.PathOf(media.ID(media.SourceFile, native))
		f, known := fileFormats[strings.ToLower(filepath.Ext(file))]
		if !ok || !known {
			return "", "", errLocalCast
		}
		t := plays(f)
		if t == "" {
			return "", "", errFormatRefused(name, f.name)
		}
		at := "/file/" + native + f.ext
		server.serve(at, castServed{file: file, contentType: t})
		return at, t, nil
	}
	// A renderer gets the stereo audio of a track that plays in 5.1.
	videoID, _, err := videoOf(rawURL)
	if err != nil {
		return "", "", err
	}
	have := a.castContentType(videoID, false)
	pick := func(path string, f castFormat) (string, string, bool) {
		t := plays(f)
		if t == "" {
			return "", "", false
		}
		at := path + f.ext
		server.serve(at, castServed{path: path, contentType: t})
		return at, t, true
	}
	switch {
	case have == "audio/webm":
		if at, t, ok := pick(stream.Path(videoID), formatWebM); ok {
			return at, t, nil
		}
	case have == "audio/mp4":
		if at, t, ok := pick(stream.Path(videoID), formatAAC); ok {
			return at, t, nil
		}
	}
	if at, t, ok := pick(stream.AACPath(videoID), formatAAC); ok {
		return at, t, nil
	}
	return "", "", errFormatRefused(name, formatAAC.name)
}
