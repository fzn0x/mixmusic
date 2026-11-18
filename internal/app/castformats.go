package app

// The formats of the audio a renderer or an FCast receiver plays: Opus in
// WebM where it plays it, else YouTube's AAC in MP4, even of a track kept
// as WebM; and the songs on this computer, in their own format, where it
// plays that. A Google Cast device plays the streams as they come, and
// the songs on this computer in their own format, within the bounds of
// its default media receiver.

import (
	"encoding/binary"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/flac"
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
		return castFile(native, rawURL, server, name, plays, nil)
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
		server.serve(at, castServed{path: path, contentType: t, of: rawURL})
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

// castFile returns the path a device fetches the file of the song native,
// on this computer, at, past its server's token, and its type, of the
// formats plays tells the device plays, unless beyond tells why it can't
// play this file of its format. The file is served while its stream,
// rawURL, is cast or queued.
func castFile(native, rawURL string, server *castServer, name string, plays func(castFormat) string,
	beyond func(file string, f castFormat) error) (string, string, error) {
	file, ok := localfiles.PathOf(media.ID(media.SourceFile, native))
	f, known := fileFormats[strings.ToLower(filepath.Ext(file))]
	if !ok || !known {
		return "", "", errLocalCast
	}
	t := plays(f)
	if t == "" {
		return "", "", errFormatRefused(name, f.name)
	}
	if beyond != nil {
		if err := beyond(file, f); err != nil {
			return "", "", err
		}
	}
	at := "/file/" + native + f.ext
	server.serve(at, castServed{file: file, contentType: t, of: rawURL})
	return at, t, nil
}

// googleSource is how a Google Cast device's audio is picked: a song on
// this computer in its own format, as the default media receiver plays
// every kind the library plays, within its bounds (googleBeyond); a stream
// as it comes (castContentType).
func (a *App) googleSource(server *castServer, name string) func(string) (string, string, error) {
	return func(rawURL string) (string, string, error) {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", "", err
		}
		if native, ok := strings.CutPrefix(u.Path, "/file/"); ok {
			return castFile(native, rawURL, server, name, func(f castFormat) string { return f.types[0] },
				func(file string, f castFormat) error { return googleBeyond(file, f, name) })
		}
		videoID, surround, err := videoOf(rawURL)
		if err != nil {
			return "", "", err
		}
		return u.Path, a.castContentType(videoID, surround), nil
	}
}

// googleBeyond tells why the Google Cast device named name can't play the
// song in file, of the format f, as its default media receiver plays FLAC
// up to 96 kHz and 24 bits, and WAV as LPCM, whose samples are whole
// numbers. It is nil for the rest, and for a file it can't read, whose
// failure the device tells.
func googleBeyond(file string, f castFormat, name string) error {
	switch f.ext {
	case formatFLAC.ext:
		in, err := os.Open(file)
		if err != nil {
			return nil
		}
		defer in.Close()
		r, err := flac.NewReader(in)
		if err != nil {
			return nil
		}
		if info := r.Info(); info.SampleRate > 96000 || info.BitsPerSample > 24 {
			return wordedLater(name+" plays FLAC only up to 96 kHz and 24 bits, which this file goes beyond",
				func() string { return locales.CastFLACBeyond(name) }, nil)
		}
	case formatWAV.ext:
		if wavFloat(file) {
			return wordedLater(name+" doesn't play WAV files of floating-point samples",
				func() string { return locales.CastWAVFloat(name) }, nil)
		}
	}
	return nil
}

// wavFloat reports whether the WAV file holds floating-point samples, as
// its format chunk tells, of its own or of an extensible format.
func wavFloat(file string) bool {
	in, err := os.Open(file)
	if err != nil {
		return false
	}
	defer in.Close()
	var h [12]byte
	if _, err := io.ReadFull(in, h[:]); err != nil || string(h[:4]) != "RIFF" || string(h[8:]) != "WAVE" {
		return false
	}
	for range 256 {
		var ch [8]byte
		if _, err := io.ReadFull(in, ch[:]); err != nil {
			return false
		}
		size := int64(binary.LittleEndian.Uint32(ch[4:]))
		if string(ch[:4]) != "fmt " {
			// Chunks are padded to an even size.
			if _, err := in.Seek(size+size&1, io.SeekCurrent); err != nil {
				return false
			}
			continue
		}
		if size < 16 || size > 1024 {
			return false
		}
		b := make([]byte, size)
		if _, err := io.ReadFull(in, b); err != nil {
			return false
		}
		format := binary.LittleEndian.Uint16(b)
		if format == 0xFFFE && size >= 26 {
			// WAVE_FORMAT_EXTENSIBLE: the first two bytes of its subformat.
			format = binary.LittleEndian.Uint16(b[24:])
		}
		return format == 3 // WAVE_FORMAT_IEEE_FLOAT
	}
	return false
}
