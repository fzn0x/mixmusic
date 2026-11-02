package stream

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Video is a stream of the picture of a video, without its sound, as
// YouTube serves it beside the audio: a DASH stream, whose index is at its
// start, for a page to play through Media Source Extensions.
type Video struct {
	Itag   string
	URL    string
	Header http.Header
	// MimeType names its container and codec, as Media Source Extensions
	// take them: video/webm; codecs="vp9".
	MimeType string
	// Family is the family of its codec: "vp9", "avc1" or "av01".
	Family        string
	Width, Height int
	FPS           float64
	// Size is the length of the stream in bytes, 0 when unknown.
	Size int64
	// Bitrate is in kbit/s.
	Bitrate float64
}

// videoFamily returns the family of a video codec, as YouTube and yt-dlp
// name it, or "" for one MixMusic doesn't play.
func videoFamily(codec string) string {
	switch {
	case codec == "vp9" || strings.HasPrefix(codec, "vp09."):
		return "vp9"
	case strings.HasPrefix(codec, "avc1."):
		return "avc1"
	case strings.HasPrefix(codec, "av01."):
		return "av01"
	}
	return ""
}

// infoFormat is what MixMusic reads of a format yt-dlp lists.
type infoFormat struct {
	FormatID string            `json:"format_id"`
	URL      string            `json:"url"`
	Ext      string            `json:"ext"`
	Protocol string            `json:"protocol"`
	VCodec   string            `json:"vcodec"`
	ACodec   string            `json:"acodec"`
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	FPS      float64           `json:"fps"`
	TBR      float64           `json:"tbr"`
	Filesize int64             `json:"filesize"`
	Headers  map[string]string `json:"http_headers"`
}

// ytdlpVideos returns the video streams of the formats yt-dlp lists: those
// served whole over HTTPS, without sound. YouTube's storyboards, its HLS
// streams and its streams with sound go.
func ytdlpVideos(formats []infoFormat) []Video {
	var out []Video
	for _, f := range formats {
		family := videoFamily(f.VCodec)
		if f.Protocol != "https" || f.URL == "" || family == "" || f.ACodec != "none" && f.ACodec != "" ||
			f.Ext != "webm" && f.Ext != "mp4" || f.Width <= 0 || f.Height <= 0 {
			continue
		}
		v := Video{
			Itag:     f.FormatID,
			URL:      f.URL,
			Header:   http.Header{},
			MimeType: "video/" + f.Ext + `; codecs="` + f.VCodec + `"`,
			Family:   family,
			Width:    f.Width,
			Height:   f.Height,
			FPS:      f.FPS,
			Size:     f.Filesize,
			Bitrate:  f.TBR,
		}
		for k, val := range f.Headers {
			v.Header.Set(k, val)
		}
		out = append(out, v)
	}
	return out
}

// directVideos returns the video streams of the formats YouTube's player
// API gave the direct route: those that play as given, unencrypted, and
// whole. length is the video's, 0 when unknown.
func directVideos(formats []playerFormat, length time.Duration) []Video {
	var out []Video
	for _, f := range formats {
		if !strings.HasPrefix(f.MimeType, "video/") || !f.direct() || len(f.DrmFamilies) > 0 ||
			f.Type == "FORMAT_STREAM_TYPE_OTF" || f.TargetDurationSec != 0 || f.Width <= 0 || f.Height <= 0 {
			continue
		}
		family := videoFamily(f.codec())
		if family == "" {
			continue
		}
		bitrate := f.AverageBitrate
		if bitrate == 0 {
			bitrate = f.Bitrate
		}
		v := Video{
			Itag:     strconv.Itoa(f.Itag),
			URL:      f.URL,
			Header:   http.Header{},
			MimeType: f.MimeType,
			Family:   family,
			Width:    int(f.Width),
			Height:   int(f.Height),
			FPS:      float64(f.FPS),
			Size:     int64(f.ContentLength),
			Bitrate:  float64(bitrate) / 1000,
		}
		v.Header.Set("User-Agent", visionAgent)
		out = append(out, v)
	}
	return out
}

// maxVideoLines bounds the shorter side of the streams PickVideo takes:
// 480 lines are more than the now playing page's frame shows at the scale
// of most displays, and a stream of more costs several times the data and
// the decoding for nothing to see.
const maxVideoLines = 480

// PickVideo picks the stream of a video to show in a frame of w × h
// pixels: of the first of families, the codecs the page plays in the order
// it prefers them, that the video has a stream of, the smallest at least
// that large, else the largest, of at most maxVideoLines lines; with
// saver, the smallest. Of two of a size, it takes the one of at most 30
// frames a second, which the page decodes more easily, else the smaller.
func PickVideo(vs []Video, families []string, w, h int, saver bool) (Video, bool) {
	for _, family := range families {
		var of []Video
		for _, v := range vs {
			if v.Family == family && v.Width > 0 && v.Height > 0 && min(v.Width, v.Height) <= maxVideoLines {
				of = append(of, v)
			}
		}
		if len(of) == 0 {
			continue
		}
		slices.SortStableFunc(of, func(a, b Video) int {
			return cmp.Or(cmp.Compare(a.Width*a.Height, b.Width*b.Height), compareBool(a.FPS > 30, b.FPS > 30), cmp.Compare(a.Bitrate, b.Bitrate))
		})
		if saver {
			return of[0], true
		}
		for _, v := range of {
			if v.Width >= w && v.Height >= h {
				return v, true
			}
		}
		largest := of[len(of)-1].Width * of[len(of)-1].Height
		for _, v := range of {
			if v.Width*v.Height == largest {
				return v, true
			}
		}
	}
	return Video{}, false
}

// compareBool orders false before true.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}
