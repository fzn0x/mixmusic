package media

import (
	"net/url"
	"regexp"
	"strings"
)

// LinkKind is what a link to a service names.
type LinkKind string

const (
	// LinkSong names a song or a video, which plays.
	LinkSong LinkKind = "song"
	// LinkRadio names a song, and a YouTube radio that plays after it.
	LinkRadio LinkKind = "radio"
	// LinkPlaylist names a playlist, or on YouTube an album by the
	// playlist of its tracks (OLAK5uy_…).
	LinkPlaylist LinkKind = "playlist"
	// LinkAlbum names an album.
	LinkAlbum LinkKind = "album"
	// LinkArtist names an artist.
	LinkArtist LinkKind = "artist"
	// LinkPodcast names a podcast, a show of Spotify.
	LinkPodcast LinkKind = "podcast"
	// LinkEpisode names an episode of a Spotify show.
	LinkEpisode LinkKind = "episode"
)

// Link is what a link to YouTube, YouTube Music or Spotify names, as
// ParseLink reads it.
type Link struct {
	Kind   LinkKind
	Source Source
	// ID is the ID of what the link names, as ID makes it: of a radio, the
	// ID of its song.
	ID string
	// Radio is the ID of the playlist of a radio link's radio (RD…).
	Radio string
}

// The hosts whose links ParseLink reads.
var (
	youtubeHosts = map[string]bool{"music.youtube.com": true, "youtube.com": true, "www.youtube.com": true,
		"m.youtube.com": true, "youtu.be": true}
	spotifyHost = "open.spotify.com"
)

var (
	// youtubeIDRe matches the IDs of YouTube's videos, playlists, albums
	// and channels; spotifyIDRe Spotify's, which are base 62.
	youtubeIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	spotifyIDRe = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	// intlRe matches the part of Spotify's paths that names the language
	// of its page, as intl-de or intl-pt-br.
	intlRe = regexp.MustCompile(`^intl-[a-z]{2}(?:-[a-z]{2,4})?$`)
)

// spotifyKinds are the kinds of Spotify's links, by the word of their
// path and URIs.
var spotifyKinds = map[string]LinkKind{
	"track": LinkSong, "album": LinkAlbum, "playlist": LinkPlaylist, "artist": LinkArtist,
	"show": LinkPodcast, "episode": LinkEpisode,
}

// ParseLink reads text as a link to a song, an album, a playlist, an
// artist or a podcast of a service, and reports whether it is one:
//
//   - YouTube's and YouTube Music's, on music.youtube.com, youtube.com,
//     www.youtube.com, m.youtube.com and youtu.be: a song (watch?v=,
//     youtu.be/ID, shorts/ID), a playlist or an album by its playlist
//     (list= of a PL…, OLAK5uy_… or RDCLAK5uy_… ID, beside v= too), a
//     radio (v= beside any other RD… list), an album (browse/MPREb_…) and
//     an artist (channel/UC…).
//   - Spotify's, on open.spotify.com: a track, an album, a playlist, an
//     artist, a show or an episode, with the language of the page
//     (intl-de/) or without, and the URIs of the same, as
//     spotify:track:ID.
//
// The scheme may be left out, as in youtu.be/ID. What the link tells
// besides, as the parameters that track who shared it (si, pp, feature),
// is dropped. A link to another host, or to another page, is none.
func ParseLink(text string) (Link, bool) {
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text, " \t\r\n") {
		return Link{}, false
	}
	if rest, ok := cutPrefixFold(text, "spotify:"); ok {
		kind, id, ok := strings.Cut(rest, ":")
		return spotifyLink(kind, id, ok)
	}
	if !strings.Contains(text, "://") {
		text = "https://" + text
	}
	u, err := url.Parse(text)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Link{}, false
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case host == spotifyHost:
		if len(parts) > 0 && intlRe.MatchString(strings.ToLower(parts[0])) {
			parts = parts[1:]
		}
		if len(parts) != 2 {
			return Link{}, false
		}
		return spotifyLink(parts[0], parts[1], true)
	case youtubeHosts[host]:
		return youtubeLink(host, parts, u.Query())
	}
	return Link{}, false
}

// spotifyLink is the link to the item of kind, as Spotify's paths and URIs
// name it, whose ID is id.
func spotifyLink(kind, id string, ok bool) (Link, bool) {
	k, known := spotifyKinds[strings.ToLower(kind)]
	if !ok || !known || !spotifyIDRe.MatchString(id) {
		return Link{}, false
	}
	return Link{Kind: k, Source: SourceSpotify, ID: ID(SourceSpotify, id)}, true
}

// youtubeLink is the link of a page of YouTube or YouTube Music on host:
// the parts of its path, and its query.
func youtubeLink(host string, parts []string, q url.Values) (Link, bool) {
	video := ""
	switch {
	case host == "youtu.be" && len(parts) == 1:
		video = parts[0]
	case len(parts) == 1 && parts[0] == "watch":
		video = q.Get("v")
	case len(parts) == 2 && parts[0] == "shorts":
		video = parts[1]
	case len(parts) == 1 && parts[0] == "playlist":
	case len(parts) == 2 && parts[0] == "browse" && strings.HasPrefix(parts[1], "MPREb_"):
		return youtubeItem(LinkAlbum, parts[1])
	case len(parts) >= 2 && parts[0] == "channel" && strings.HasPrefix(parts[1], "UC"):
		// Its tabs too, as channel/UC…/videos.
		return youtubeItem(LinkArtist, parts[1])
	default:
		return Link{}, false
	}
	if video != "" && !youtubeIDRe.MatchString(video) {
		return Link{}, false
	}
	list := q.Get("list")
	if !youtubeIDRe.MatchString(list) {
		list = ""
	}
	switch {
	case playlistList(list):
		return youtubeItem(LinkPlaylist, list)
	case video != "" && strings.HasPrefix(list, "RD"):
		l, _ := youtubeItem(LinkRadio, video)
		l.Radio = ID(SourceYouTube, list)
		return l, true
	case video != "":
		return youtubeItem(LinkSong, video)
	}
	return Link{}, false
}

// playlistList reports whether list, the list= of a link, is a playlist's
// or an album's that its page shows: one of a user's or a channel's
// (PL…), of an album's tracks (OLAK5uy_…), or one of YouTube Music's own
// (RDCLAK5uy_…). Its other radios (RD…) are made for whoever plays them.
func playlistList(list string) bool {
	for _, prefix := range []string{"PL", "OLAK5uy_", "RDCLAK5uy_"} {
		if strings.HasPrefix(list, prefix) && len(list) > len(prefix) {
			return true
		}
	}
	return false
}

func youtubeItem(kind LinkKind, id string) (Link, bool) {
	if !youtubeIDRe.MatchString(id) {
		return Link{}, false
	}
	return Link{Kind: kind, Source: SourceYouTube, ID: ID(SourceYouTube, id)}, true
}

// URL is the link's own address, on YouTube Music or Spotify, without
// what tracked who shared it.
func (l Link) URL() string {
	_, id, _ := ParseID(l.ID)
	if l.Source == SourceSpotify {
		for word, kind := range spotifyKinds {
			if kind == l.Kind {
				return "https://open.spotify.com/" + word + "/" + id
			}
		}
		return ""
	}
	q := url.Values{}
	switch l.Kind {
	case LinkSong:
		q.Set("v", id)
	case LinkRadio:
		_, list, _ := ParseID(l.Radio)
		q.Set("v", id)
		q.Set("list", list)
	case LinkPlaylist:
		return "https://music.youtube.com/playlist?list=" + url.QueryEscape(id)
	case LinkAlbum:
		return "https://music.youtube.com/browse/" + id
	case LinkArtist:
		return "https://music.youtube.com/channel/" + id
	default:
		return ""
	}
	return "https://music.youtube.com/watch?" + q.Encode()
}

// IsLink reports whether text is a link: one ParseLink reads, or the
// address of any web page.
func IsLink(text string) bool {
	_, ok := ParseLink(text)
	return ok || WebLink(text) != ""
}

// cutPrefixFold is strings.CutPrefix, that ignores case.
func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}
