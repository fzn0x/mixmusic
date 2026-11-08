package stream

import (
	"iter"
	"strings"
)

// bothFailed is yt-dlp's error, after YouTube's player API found no stream
// either: it reads as yt-dlp's, and is what yt-dlp's is to errors.Is and
// errors.As, and its steps tell both, for the details of a track that
// couldn't play (#351). The player API's error is told as the log has it:
// without the addresses it may quote.
type bothFailed struct{ direct, ytdlp error }

func (e bothFailed) Error() string { return e.ytdlp.Error() }
func (e bothFailed) Unwrap() error { return e.ytdlp }

// Steps tells what was tried, in turn, and why each failed.
func (e bothFailed) Steps() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		if !yield("YouTube's player API", strings.TrimPrefix(loggable(e.direct).Error(), "stream: ")) {
			return
		}
		yield("yt-dlp", strings.TrimPrefix(e.ytdlp.Error(), "yt-dlp: "))
	}
}
