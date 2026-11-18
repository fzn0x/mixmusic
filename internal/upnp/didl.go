package upnp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
)

// didl returns the DIDL-Lite metadata of m, which a renderer with a screen
// shows: its title, artist, album, cover and length, and how its audio is
// fetched.
func didl(m cast.Media) string {
	var b bytes.Buffer
	text := func(tag, v string) {
		if v == "" {
			return
		}
		b.WriteString("<" + tag + ">")
		xml.EscapeText(&b, []byte(v))
		b.WriteString("</" + tag + ">")
	}
	b.WriteString(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/"`)
	b.WriteString(` xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/">`)
	b.WriteString(`<item id="mixmusic" parentID="-1" restricted="1">`)
	title := m.Title
	if title == "" {
		title = "MixMusic"
	}
	text("dc:title", title)
	text("dc:creator", m.Artist)
	text("upnp:artist", m.Artist)
	text("upnp:album", m.Album)
	text("upnp:albumArtURI", m.Image)
	text("upnp:class", "object.item.audioItem.musicTrack")
	b.WriteString(`<res protocolInfo="`)
	xml.EscapeText(&b, []byte(ProtocolInfo(m.ContentType)))
	b.WriteString(`"`)
	if m.Duration > 0 {
		b.WriteString(` duration="` + clock(m.Duration) + `.000"`)
	}
	b.WriteString(">")
	xml.EscapeText(&b, []byte(m.URL))
	b.WriteString("</res></item></DIDL-Lite>")
	return b.String()
}

// ProtocolInfo is the protocol info of audio of the MIME type mime that
// MixMusic serves over HTTP.
func ProtocolInfo(mime string) string { return "http-get:*:" + mime + ":" + Features }

// clock writes d as UPnP's times are, H:MM:SS.
func clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int64(d / time.Second)
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

// parseClock reads a UPnP time, H+:MM:SS with an optional fraction, or
// "NOT_IMPLEMENTED" and the like as 0, false.
func parseClock(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	sec, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil || neg {
		return 0, false
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second)), true
}

// Sink is what a renderer plays, as its ConnectionManager's
// GetProtocolInfo tells it: the protocol infos of its Sink, as
// "http-get:*:audio/mpeg:*".
type Sink []string

// parseSink reads the comma-separated protocol infos of a Sink.
func parseSink(s string) Sink {
	var out Sink
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Type returns the MIME type the renderer names, of those it fetches over
// HTTP, that is one of types, as it names it, in the order of types: ""
// when it names none. A renderer that names any type over HTTP plays the
// first.
func (s Sink) Type(types ...string) string {
	for _, want := range types {
		for _, p := range s {
			fields := strings.Split(p, ":")
			if len(fields) < 3 || !strings.EqualFold(fields[0], "http-get") {
				continue
			}
			mime, _, _ := strings.Cut(strings.TrimSpace(fields[2]), ";")
			if mime == "*" || strings.EqualFold(mime, want) {
				if mime == "*" {
					return want
				}
				return mime
			}
		}
	}
	return ""
}
