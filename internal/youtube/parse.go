package youtube

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

const (
	pageTypeArtist   = "MUSIC_PAGE_TYPE_ARTIST"
	pageTypeAlbum    = "MUSIC_PAGE_TYPE_ALBUM"
	pageTypePlaylist = "MUSIC_PAGE_TYPE_PLAYLIST"
	// pageTypeChannel is the page of a channel that is not an artist's,
	// such as the uploader of a video.
	pageTypeChannel = "MUSIC_PAGE_TYPE_USER_CHANNEL"

	// videoTypeAudio marks audio tracks, as opposed to music videos.
	videoTypeAudio = "MUSIC_VIDEO_TYPE_ATV"

	// artworkSize is the size of the large artwork, in pixels.
	artworkSize = 544
)

var (
	durationRe = regexp.MustCompile(`^(?:(\d+):)?(\d{1,2}):(\d{2})$`)
	yearRe     = regexp.MustCompile(`^\d{4}$`)
	sizeRe     = regexp.MustCompile(`=w\d+-h\d+`)
)

// kindLabels are the words YouTube Music starts bylines with to say what
// an item is, which are not artist names.
var kindLabels = map[string]bool{
	"Song": true, "Video": true, "Album": true, "Single": true, "EP": true,
	"Playlist": true, "Artist": true, "Episode": true, "Podcast": true, "Profile": true,
}

// parseDuration turns "3:28" or "1:02:03" into milliseconds; 0 if s is not
// a duration.
func parseDuration(s string) int {
	m := durationRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	return ((h*60+min)*60 + sec) * 1000
}

// number returns the node as an int, or 0.
func (n node) number() int {
	f, _ := n.v.(float64)
	return int(f)
}

// artwork returns the images of a thumbnail list: a small and a large
// version of the largest one, when its URL can be resized.
func artwork(thumbs node) []media.Image {
	var best node
	bestWidth := -1
	for _, t := range thumbs.list() {
		if w := t.get("width").number(); w > bestWidth {
			best, bestWidth = t, w
		}
	}
	if bestWidth < 0 {
		return []media.Image{}
	}
	u := best.get("url").str()
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	if !sizeRe.MatchString(u) {
		return []media.Image{{URL: u, Width: bestWidth, Height: best.get("height").number()}}
	}
	large := "=w" + strconv.Itoa(artworkSize) + "-h" + strconv.Itoa(artworkSize)
	return []media.Image{
		{URL: sizeRe.ReplaceAllString(u, "=w120-h120"), Width: 120, Height: 120},
		{URL: sizeRe.ReplaceAllString(u, large), Width: artworkSize, Height: artworkSize},
	}
}

// thumbnailList finds the thumbnail list of a renderer, wherever this kind
// of renderer keeps it.
func thumbnailList(r node) node {
	for _, path := range [][]any{
		{"thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"},
		{"thumbnailRenderer", "musicThumbnailRenderer", "thumbnail", "thumbnails"},
		{"thumbnail", "thumbnails"},
		{"thumbnail", "croppedSquareThumbnailRenderer", "thumbnail", "thumbnails"},
	} {
		if t := r.get(path...); t.exists() {
			return t
		}
	}
	return node{}
}

// pageType returns the page type of a navigation endpoint, such as
// MUSIC_PAGE_TYPE_ALBUM.
func pageType(endpoint node) string {
	return endpoint.get("browseEndpoint", "browseEndpointContextSupportedConfigs", "browseEndpointContextMusicConfig", "pageType").str()
}

// byline is what the runs under an item's title say about it.
type byline struct {
	artists  []media.Ref
	album    *media.Ref
	duration int
	year     string
	// label is the kind word bylines may start with, such as "Single".
	label string
	// plain holds the other segments, such as "5.9M views".
	plain []string
}

// parseByline reads runs such as "DJT • Q-ENA II • 3:28", whose links name
// the artists and the album.
func parseByline(runs []node) byline {
	var b byline
	for _, r := range runs {
		text := strings.TrimSpace(r.get("text").str())
		nav := r.get("navigationEndpoint")
		switch pageType(nav) {
		case pageTypeArtist, pageTypeChannel:
			b.artists = append(b.artists, media.Ref{ID: media.ID(media.SourceYouTube, nav.get("browseEndpoint", "browseId").str()), Name: text})
			continue
		case pageTypeAlbum:
			b.album = &media.Ref{ID: media.ID(media.SourceYouTube, nav.get("browseEndpoint", "browseId").str()), Name: text}
			continue
		}
		switch {
		case text == "" || text == "•" || text == "&" || text == ",":
		case b.duration == 0 && parseDuration(text) > 0:
			b.duration = parseDuration(text)
		case b.year == "" && yearRe.MatchString(text):
			b.year = text
		case b.label == "" && len(b.plain) == 0 && len(b.artists) == 0 && kindLabels[text]:
			b.label = text
		default:
			b.plain = append(b.plain, text)
		}
	}
	return b
}

// fallbackArtists returns the artists of a byline, or for one without
// links, such as "Various Artists", its first plain segment.
func (b byline) fallbackArtists() []media.Ref {
	if len(b.artists) > 0 {
		return b.artists
	}
	if len(b.plain) > 0 && !looksLikeCount(b.plain[0]) {
		return []media.Ref{{Name: b.plain[0]}}
	}
	return []media.Ref{}
}

// plays returns how many times the byline's track played, as a song's
// "1.2B plays" or a video's "905M views" say, or 0 when it says not.
func (b byline) plays() int64 {
	for _, s := range b.plain {
		if n, ok := parsePlays(s); ok {
			return n
		}
	}
	return 0
}

// parsePlays reads a count of plays or views as YouTube Music abbreviates
// it in English, as "1.2B plays", "2.2K plays" or "148 views".
func parsePlays(s string) (int64, bool) {
	num := ""
	for _, noun := range []string{" plays", " play", " views", " view"} {
		if n, ok := strings.CutSuffix(s, noun); ok {
			num = n
			break
		}
	}
	scale := 1.0
	for suffix, v := range map[string]float64{"K": 1e3, "M": 1e6, "B": 1e9} {
		if n, ok := strings.CutSuffix(num, suffix); ok {
			num, scale = n, v
			break
		}
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(num, ",", ""), 64)
	if num == "" || err != nil || v < 0 {
		return 0, false
	}
	return int64(math.Round(v * scale)), true
}

// looksLikeCount reports whether s counts something, as "5.9M views" or
// "21K plays" do, rather than naming someone.
func looksLikeCount(s string) bool {
	for _, suffix := range []string{" views", " plays", " subscribers", " monthly audience", " songs", " tracks", " listeners"} {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

// flexRuns returns the runs of a list item's column i.
func flexRuns(item node, i int) []node {
	return item.get("flexColumns", i, "musicResponsiveListItemFlexColumnRenderer", "text").runs()
}

// isExplicit reports whether a renderer carries the explicit badge.
func isExplicit(r node) bool {
	for _, badge := range append(r.get("badges").list(), r.get("subtitleBadges").list()...) {
		if badge.get("musicInlineBadgeRenderer", "icon", "iconType").str() == "MUSIC_EXPLICIT_BADGE" {
			return true
		}
	}
	return false
}

// newTrack makes the track a watch endpoint plays.
func newTrack(videoID, title string, watch node, b byline, thumbs []media.Image, explicit bool) *media.Track {
	videoType := watch.get("watchEndpointMusicSupportedConfigs", "watchEndpointMusicConfig", "musicVideoType").str()
	return &media.Track{
		ID:         media.ID(media.SourceYouTube, videoID),
		Source:     media.SourceYouTube,
		Title:      title,
		Artists:    b.fallbackArtists(),
		Album:      b.album,
		DurationMs: b.duration,
		Artwork:    thumbs,
		Explicit:   explicit,
		Video:      videoType != "" && videoType != videoTypeAudio,
		Plays:      b.plays(),
		URL:        "https://music.youtube.com/watch?v=" + videoID,
	}
}

// parseListItem reads a musicResponsiveListItemRenderer: a row of a
// search result, an album, a playlist or a shelf.
func parseListItem(item node) (media.Item, bool) {
	titleRuns := flexRuns(item, 0)
	if len(titleRuns) == 0 {
		return media.Item{}, false
	}
	title := item.get("flexColumns", 0, "musicResponsiveListItemFlexColumnRenderer", "text").text()
	watch := titleRuns[0].get("navigationEndpoint", "watchEndpoint")
	videoID := item.get("playlistItemData", "videoId").str()
	if videoID == "" {
		videoID = watch.get("videoId").str()
	}
	var runs []node
	for i := 1; i < len(item.get("flexColumns").list()); i++ {
		runs = append(runs, flexRuns(item, i)...)
	}
	for _, col := range item.get("fixedColumns").list() {
		runs = append(runs, col.get("musicResponsiveListItemFixedColumnRenderer", "text").runs()...)
	}
	b := parseByline(runs)
	thumbs := artwork(thumbnailList(item))

	if videoID != "" {
		t := newTrack(videoID, title, watch, b, thumbs, isExplicit(item))
		t.EntryID = item.get("playlistItemData", "playlistSetVideoId").str()
		return media.Item{Kind: media.ItemTrack, Track: t}, true
	}
	nav := item.get("navigationEndpoint")
	return browseItem(pageType(nav), nav.get("browseEndpoint", "browseId").str(), title, b, thumbs)
}

// parseTwoRowItem reads a musicTwoRowItemRenderer: a card of a carousel,
// with artwork above a title and a subtitle.
func parseTwoRowItem(r node) (media.Item, bool) {
	title := r.get("title").text()
	if title == "" {
		return media.Item{}, false
	}
	b := parseByline(r.get("subtitle").runs())
	thumbs := artwork(thumbnailList(r))
	nav := r.get("navigationEndpoint")
	if watch := nav.get("watchEndpoint"); watch.get("videoId").str() != "" {
		return media.Item{Kind: media.ItemTrack, Track: newTrack(watch.get("videoId").str(), title, watch, b, thumbs, isExplicit(r))}, true
	}
	return browseItem(pageType(nav), nav.get("browseEndpoint", "browseId").str(), title, b, thumbs)
}

// browseItem makes the album, artist or playlist an item links to.
func browseItem(pageType, browseID, title string, b byline, thumbs []media.Image) (media.Item, bool) {
	if browseID == "" {
		return media.Item{}, false
	}
	switch pageType {
	case pageTypeArtist, pageTypeChannel:
		return media.Item{Kind: media.ItemArtist, Artist: &media.Artist{
			ID:        media.ID(media.SourceYouTube, browseID),
			Source:    media.SourceYouTube,
			Name:      title,
			Artwork:   thumbs,
			Subtitle:  strings.Join(b.plain, " • "),
			TopTracks: []media.Track{},
			Albums:    []media.Album{},
			Singles:   []media.Album{},
			Related:   []media.Ref{},
			URL:       "https://music.youtube.com/channel/" + browseID,
		}}, true
	case pageTypeAlbum:
		return media.Item{Kind: media.ItemAlbum, Album: &media.Album{
			ID:      media.ID(media.SourceYouTube, browseID),
			Source:  media.SourceYouTube,
			Title:   title,
			Artists: b.fallbackArtists(),
			Kind:    albumKind(b.label),
			Year:    b.year,
			Artwork: thumbs,
			Tracks:  []media.Track{},
			URL:     "https://music.youtube.com/browse/" + browseID,
		}}, true
	case pageTypePlaylist:
		id := strings.TrimPrefix(browseID, "VL")
		owner := ""
		if artists := b.fallbackArtists(); len(artists) > 0 {
			owner = artists[0].Name
		}
		return media.Item{Kind: media.ItemPlaylist, Playlist: &media.Playlist{
			ID:      media.ID(media.SourceYouTube, id),
			Source:  media.SourceYouTube,
			Title:   title,
			Owner:   owner,
			Artwork: thumbs,
			Tracks:  []media.Track{},
			URL:     "https://music.youtube.com/playlist?list=" + id,
		}}, true
	}
	return media.Item{}, false
}

// albumKind maps the label of an album's byline to its kind.
func albumKind(label string) media.AlbumKind {
	switch label {
	case "Single":
		return media.AlbumKindSingle
	case "EP":
		return media.AlbumKindEP
	case "Album":
		return media.AlbumKindAlbum
	}
	return ""
}

// parseItems reads the items of a shelf's contents, skipping renderers it
// does not know.
func parseItems(contents []node) []media.Item {
	items := []media.Item{}
	for _, c := range contents {
		name, r := c.renderer()
		var (
			item media.Item
			ok   bool
		)
		switch name {
		case "musicResponsiveListItemRenderer":
			item, ok = parseListItem(r)
		case "musicTwoRowItemRenderer":
			item, ok = parseTwoRowItem(r)
		}
		if ok {
			items = append(items, item)
		}
	}
	return items
}
