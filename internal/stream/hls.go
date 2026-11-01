package stream

// The HLS playlists of live streams, as RFC 8216 has them: YouTube serves
// the audio of a live video as an HLS media playlist that grows as the
// stream goes on, which a master playlist lists among the video's
// renditions.

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// errPlaylist is wrapped by the errors of a playlist the live route can't
// read, or can't play. They never quote the playlist, whose URIs are
// stream URLs.
var errPlaylist = errors.New("stream: an HLS playlist MixMusic can't read")

// mediaPlaylist is what the live route reads of an HLS media playlist
// (RFC 8216, section 4.3.3): its segments, and how to go on with it.
type mediaPlaylist struct {
	// target is the target duration: what each segment lasts at most.
	target time.Duration
	// sequence is the media sequence number of the first segment, from
	// which those of the others follow.
	sequence int64
	segments []segment
	// ended is set once the playlist has its last segment, which
	// EXT-X-ENDLIST tells: the stream ended.
	ended bool
}

// segment is a media segment of a playlist.
type segment struct {
	// seq is its media sequence number.
	seq int64
	// uri is where it is, resolved against the playlist's URL.
	uri      string
	duration time.Duration
	// discontinuity is set when it doesn't follow on from the segment
	// before: EXT-X-DISCONTINUITY.
	discontinuity bool
}

// end is the media sequence number after the playlist's last segment.
func (m mediaPlaylist) end() int64 { return m.sequence + int64(len(m.segments)) }

// parseMediaPlaylist reads an HLS media playlist, whose URIs are relative
// to base. It reads what the live route needs and passes over the tags it
// doesn't, as clients must: among them EXT-X-PROGRAM-DATE-TIME, and the ad
// cues of EXT-X-DATERANGE and EXT-X-CUEPOINT. Segments it can't fetch whole
// and play as they come are errors: encrypted ones, those of fragmented
// MP4, and byte ranges.
func parseMediaPlaylist(text string, base *url.URL) (mediaPlaylist, error) {
	var m mediaPlaylist
	text, ok := strings.CutPrefix(strings.TrimPrefix(text, "\ufeff"), "#EXTM3U")
	if !ok {
		return m, fmt.Errorf("%w: no #EXTM3U", errPlaylist)
	}
	var (
		duration      time.Duration
		discontinuity bool
	)
	for text != "" {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "#EXT"):
			tag, value, _ := strings.Cut(line, ":")
			switch tag {
			case "#EXT-X-TARGETDURATION":
				n, err := strconv.ParseInt(value, 10, 64)
				if err != nil || n <= 0 || n > 3600 {
					return m, fmt.Errorf("%w: a target duration of %q", errPlaylist, value)
				}
				m.target = time.Duration(n) * time.Second
			case "#EXT-X-MEDIA-SEQUENCE":
				n, err := strconv.ParseInt(value, 10, 64)
				if err != nil || n < 0 {
					return m, fmt.Errorf("%w: a media sequence of %q", errPlaylist, value)
				}
				m.sequence = n
			case "#EXTINF":
				d, _, _ := strings.Cut(value, ",")
				s, err := strconv.ParseFloat(strings.TrimSpace(d), 64)
				if err != nil || !(s >= 0 && s <= 3600) { // NaN too
					return m, fmt.Errorf("%w: a segment of %q seconds", errPlaylist, d)
				}
				duration = time.Duration(s * float64(time.Second))
			case "#EXT-X-DISCONTINUITY":
				discontinuity = true
			case "#EXT-X-ENDLIST":
				m.ended = true
			case "#EXT-X-KEY":
				if method := attributes(value)["METHOD"]; method != "NONE" {
					return m, fmt.Errorf("%w: segments encrypted with %s", errPlaylist, method)
				}
			case "#EXT-X-MAP":
				return m, fmt.Errorf("%w: segments of fragmented MP4", errPlaylist)
			case "#EXT-X-BYTERANGE":
				return m, fmt.Errorf("%w: segments in byte ranges", errPlaylist)
			case "#EXT-X-STREAM-INF", "#EXT-X-MEDIA", "#EXT-X-I-FRAME-STREAM-INF":
				return m, fmt.Errorf("%w: a master playlist", errPlaylist)
			}
		case strings.HasPrefix(line, "#"): // a comment
		default:
			uri, err := resolveURI(base, line)
			if err != nil {
				return m, err
			}
			m.segments = append(m.segments, segment{uri: uri, duration: duration, discontinuity: discontinuity})
			duration, discontinuity = 0, false
		}
	}
	if m.target <= 0 {
		return m, fmt.Errorf("%w: no target duration", errPlaylist)
	}
	// EXT-X-MEDIA-SEQUENCE comes before the first segment, but numbers it
	// wherever it is.
	for i := range m.segments {
		m.segments[i].seq = m.sequence + int64(i)
	}
	return m, nil
}

// audioRenditions returns the URIs of the audio renditions an HLS master
// playlist lists (RFC 8216, section 4.3.4.1), EXT-X-MEDIA tags of
// TYPE=AUDIO, by their GROUP-ID: for a YouTube live stream, the itag of
// the rendition's format.
func audioRenditions(text string, base *url.URL) (map[string]string, error) {
	text, ok := strings.CutPrefix(strings.TrimPrefix(text, "\ufeff"), "#EXTM3U")
	if !ok {
		return nil, fmt.Errorf("%w: no #EXTM3U", errPlaylist)
	}
	out := map[string]string{}
	for text != "" {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "#EXT-X-MEDIA:")
		if !ok {
			continue
		}
		a := attributes(value)
		if a["TYPE"] != "AUDIO" || a["URI"] == "" || a["GROUP-ID"] == "" {
			continue
		}
		uri, err := resolveURI(base, a["URI"])
		if err != nil {
			return nil, err
		}
		if _, seen := out[a["GROUP-ID"]]; !seen {
			out[a["GROUP-ID"]] = uri
		}
	}
	return out, nil
}

// resolveURI resolves a URI of a playlist against the playlist's URL. The
// error doesn't quote it.
func resolveURI(base *url.URL, uri string) (string, error) {
	if strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://") {
		return uri, nil // as YouTube's are: no copy of the long URI
	}
	ref, err := url.Parse(uri)
	if err != nil || base == nil {
		return "", fmt.Errorf("%w: a URI that doesn't parse", errPlaylist)
	}
	return base.ResolveReference(ref).String(), nil
}

// attributes reads an attribute list (RFC 8216, section 4.2): names and
// values, separated by commas, a value quoted when it may hold a comma.
// Quoted values come without their quotes.
func attributes(list string) map[string]string {
	out := map[string]string{}
	for list != "" {
		name, rest, ok := strings.Cut(list, "=")
		if !ok {
			break
		}
		var value string
		if quoted, ok := strings.CutPrefix(rest, `"`); ok {
			value, rest, _ = strings.Cut(quoted, `"`)
			_, rest, _ = strings.Cut(rest, ",")
		} else {
			value, rest, _ = strings.Cut(rest, ",")
		}
		out[strings.TrimSpace(name)] = strings.TrimSpace(value)
		list = rest
	}
	return out
}
