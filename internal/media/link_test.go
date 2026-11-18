package media

import "testing"

// TestParseLink reads the links people share of YouTube, YouTube Music
// and Spotify: what each names, without what tracks who shared it.
func TestParseLink(t *testing.T) {
	const (
		video = "dQw4w9WgXcQ"
		sp    = "4uLU6hMCjMI75M1A2tKUQC"
	)
	song := Link{Kind: LinkSong, Source: SourceYouTube, ID: "yt:" + video}
	spotify := func(kind LinkKind, id string) Link { return Link{Kind: kind, Source: SourceSpotify, ID: "sp:" + id} }
	playlist := func(id string) Link { return Link{Kind: LinkPlaylist, Source: SourceYouTube, ID: "yt:" + id} }
	radio := func(list string) Link {
		return Link{Kind: LinkRadio, Source: SourceYouTube, ID: "yt:" + video, Radio: "yt:" + list}
	}
	for _, c := range []struct {
		text string
		want Link
	}{
		// A song or a video of YouTube.
		{"https://music.youtube.com/watch?v=" + video, song},
		{"https://music.youtube.com/watch?v=" + video + "&si=AbCdEfGh", song},
		{"https://www.youtube.com/watch?v=" + video, song},
		{"https://youtube.com/watch?v=" + video + "&feature=shared", song},
		{"https://m.youtube.com/watch?v=" + video + "&pp=ygUFaGVsbG8%3D", song},
		{"http://www.youtube.com/watch?feature=youtu.be&v=" + video + "&t=42s", song},
		{"https://youtu.be/" + video, song},
		{"https://youtu.be/" + video + "?si=AbCdEfGh", song},
		{"https://www.youtube.com/shorts/" + video, song},
		{"https://youtube.com/shorts/" + video + "?feature=share", song},
		{"youtu.be/" + video, song},
		{"  music.youtube.com/watch?v=" + video + "\n", song},
		{"HTTPS://WWW.YOUTUBE.COM/watch?v=" + video, song},
		// Beside a list that is no playlist, as the liked songs: the song.
		{"https://music.youtube.com/watch?v=" + video + "&list=LM", song},
		// A playlist or an album, by its list, beside a song too.
		{"https://music.youtube.com/playlist?list=PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG", playlist("PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG")},
		{"https://www.youtube.com/playlist?list=PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG&si=AbC", playlist("PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG")},
		{"https://music.youtube.com/playlist?list=OLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg", playlist("OLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg")},
		{"https://music.youtube.com/playlist?list=RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk", playlist("RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk")},
		{"https://www.youtube.com/watch?v=" + video + "&list=PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG&index=3", playlist("PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG")},
		{"https://music.youtube.com/watch?v=" + video + "&list=OLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg", playlist("OLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg")},
		{"https://youtu.be/" + video + "?list=RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk", playlist("RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk")},
		// A radio: a song beside any other RD list.
		{"https://music.youtube.com/watch?v=" + video + "&list=RDAMVM" + video, radio("RDAMVM" + video)},
		{"https://www.youtube.com/watch?v=" + video + "&list=RD" + video + "&start_radio=1", radio("RD" + video)},
		{"https://music.youtube.com/watch?v=" + video + "&list=RDEMhe2sl9h4F3oBTwCDoU8Q3w&si=x", radio("RDEMhe2sl9h4F3oBTwCDoU8Q3w")},
		// An album and an artist.
		{"https://music.youtube.com/browse/MPREb_I0lA998lMLT", Link{Kind: LinkAlbum, Source: SourceYouTube, ID: "yt:MPREb_I0lA998lMLT"}},
		{"https://music.youtube.com/browse/MPREb_I0lA998lMLT?si=AbC", Link{Kind: LinkAlbum, Source: SourceYouTube, ID: "yt:MPREb_I0lA998lMLT"}},
		{"https://music.youtube.com/channel/UCZh6nYJx1Tq_0yW7RWo8BVg", Link{Kind: LinkArtist, Source: SourceYouTube, ID: "yt:UCZh6nYJx1Tq_0yW7RWo8BVg"}},
		{"https://www.youtube.com/channel/UCZh6nYJx1Tq_0yW7RWo8BVg/videos", Link{Kind: LinkArtist, Source: SourceYouTube, ID: "yt:UCZh6nYJx1Tq_0yW7RWo8BVg"}},
		// Spotify's six kinds, in its pages' languages or not, and as URIs.
		{"https://open.spotify.com/track/" + sp, spotify(LinkSong, sp)},
		{"https://open.spotify.com/track/" + sp + "?si=1a2b3c4d5e6f", spotify(LinkSong, sp)},
		{"https://open.spotify.com/intl-de/track/" + sp + "?si=1a2b3c4d5e6f", spotify(LinkSong, sp)},
		{"https://open.spotify.com/intl-pt-br/track/" + sp, spotify(LinkSong, sp)},
		{"open.spotify.com/track/" + sp, spotify(LinkSong, sp)},
		{"https://open.spotify.com/album/4m2880jivSbbyEGAKfITCa", spotify(LinkAlbum, "4m2880jivSbbyEGAKfITCa")},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M?si=abc", spotify(LinkPlaylist, "37i9dQZF1DXcBWIGoYBM5M")},
		{"https://open.spotify.com/artist/4tZwfgrHOc3mvqYlEYSvVi", spotify(LinkArtist, "4tZwfgrHOc3mvqYlEYSvVi")},
		{"https://open.spotify.com/intl-ja/artist/4tZwfgrHOc3mvqYlEYSvVi/", spotify(LinkArtist, "4tZwfgrHOc3mvqYlEYSvVi")},
		{"https://open.spotify.com/show/2MAi0BvDc6GTFvKFPXnkCL", spotify(LinkPodcast, "2MAi0BvDc6GTFvKFPXnkCL")},
		{"https://open.spotify.com/episode/512ojhOuo1ktJprKbVcKyQ?si=x", spotify(LinkEpisode, "512ojhOuo1ktJprKbVcKyQ")},
		{"spotify:track:" + sp, spotify(LinkSong, sp)},
		{"spotify:album:4m2880jivSbbyEGAKfITCa", spotify(LinkAlbum, "4m2880jivSbbyEGAKfITCa")},
		{"spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", spotify(LinkPlaylist, "37i9dQZF1DXcBWIGoYBM5M")},
		{"spotify:artist:4tZwfgrHOc3mvqYlEYSvVi", spotify(LinkArtist, "4tZwfgrHOc3mvqYlEYSvVi")},
		{"spotify:show:2MAi0BvDc6GTFvKFPXnkCL", spotify(LinkPodcast, "2MAi0BvDc6GTFvKFPXnkCL")},
		{"spotify:episode:512ojhOuo1ktJprKbVcKyQ", spotify(LinkEpisode, "512ojhOuo1ktJprKbVcKyQ")},
	} {
		got, ok := ParseLink(c.text)
		if !ok || got != c.want {
			t.Errorf("ParseLink(%q) = %+v, %v; want %+v", c.text, got, ok, c.want)
		}
	}
}

// TestParseLinkNone reads no link of what isn't one MixMusic opens: a
// search, another host, another page of the services, or an ID that
// can't be one. These are searched, as before.
func TestParseLinkNone(t *testing.T) {
	for _, text := range []string{
		"", "daft punk", "get lucky https://youtu.be/dQw4w9WgXcQ",
		"https://example.com/watch?v=dQw4w9WgXcQ", "https://soundcloud.com/artist/track",
		"https://notyoutube.com/watch?v=dQw4w9WgXcQ", "https://youtube.com.example.org/watch?v=dQw4w9WgXcQ",
		"https://music.youtube.com/", "https://music.youtube.com/explore", "https://www.youtube.com/results?search_query=daft",
		"https://music.youtube.com/watch", "https://music.youtube.com/watch?v=", "https://youtu.be/",
		"https://music.youtube.com/playlist?list=RDAMVMdQw4w9WgXcQ", "https://music.youtube.com/playlist?list=LM",
		"https://music.youtube.com/playlist?list=PL", "https://music.youtube.com/watch?v=bad%20id",
		"https://music.youtube.com/browse/FEmusic_home", "https://www.youtube.com/@daftpunk",
		"https://open.spotify.com/", "https://open.spotify.com/track/", "https://open.spotify.com/user/someone",
		"https://open.spotify.com/track/abc-def", "https://open.spotify.com/intl-de/", "https://open.spotify.com/genre/0JQ5DAqbMKFEC4WFtoNRpw",
		"spotify:track:", "spotify:podcast:2MAi0BvDc6GTFvKFPXnkCL", "spotify:user:someone", "ftp://youtu.be/dQw4w9WgXcQ",
		"mixmusic://play/yt:dQw4w9WgXcQ",
	} {
		if got, ok := ParseLink(text); ok {
			t.Errorf("ParseLink(%q) = %+v, want none", text, got)
		}
	}
}

// TestLinkURL gives a link's own address, which tracks no one.
func TestLinkURL(t *testing.T) {
	for text, want := range map[string]string{
		"https://youtu.be/dQw4w9WgXcQ?si=AbC":                                     "https://music.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=RDdQw4w9WgXcQ&pp=x":     "https://music.youtube.com/watch?list=RDdQw4w9WgXcQ&v=dQw4w9WgXcQ",
		"https://www.youtube.com/playlist?list=PLx0sYbCqOb8&feature=share":        "https://music.youtube.com/playlist?list=PLx0sYbCqOb8",
		"https://music.youtube.com/browse/MPREb_I0lA998lMLT":                      "https://music.youtube.com/browse/MPREb_I0lA998lMLT",
		"https://music.youtube.com/channel/UCZh6nYJx1Tq_0yW7RWo8BVg?si=x":         "https://music.youtube.com/channel/UCZh6nYJx1Tq_0yW7RWo8BVg",
		"https://open.spotify.com/intl-de/track/4uLU6hMCjMI75M1A2tKUQC?si=1a2b3c": "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC",
		"spotify:show:2MAi0BvDc6GTFvKFPXnkCL":                                     "https://open.spotify.com/show/2MAi0BvDc6GTFvKFPXnkCL",
	} {
		l, ok := ParseLink(text)
		if got := l.URL(); !ok || got != want {
			t.Errorf("the URL of %q = %q, want %q", text, got, want)
		}
	}
}

// TestIsLink tells a link, which the search box opens or searches without
// keeping it among the recent searches, from a search.
func TestIsLink(t *testing.T) {
	for text, want := range map[string]bool{
		"https://youtu.be/dQw4w9WgXcQ": true, "spotify:track:4uLU6hMCjMI75M1A2tKUQC": true,
		"https://example.com/some/page": true, "http://example.org": true,
		"daft punk": false, "AC/DC": false, "mixmusic://search?q=x": false, "": false,
	} {
		if got := IsLink(text); got != want {
			t.Errorf("IsLink(%q) = %v, want %v", text, got, want)
		}
	}
}
