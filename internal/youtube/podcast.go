package youtube

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// A podcast's page is browsed by "MPSP" and the playlist of its episodes,
// and an episode's by "MPED" and its video ID; YouTube Music plays an
// episode as a video of its own type.
const (
	pageTypePodcast  = "MUSIC_PAGE_TYPE_PODCAST_SHOW_DETAIL_PAGE"
	pageTypeEpisode  = "MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_TRACK_PAGE"
	videoTypeEpisode = "MUSIC_VIDEO_TYPE_PODCAST_EPISODE"
	podcastPrefix    = "MPSP"
)

// libraryPodcasts is the list of the podcasts the user saved.
const libraryPodcasts = "FEmusic_library_non_music_audio_list"

// today is the time the days of episodes are told from, as "8h ago":
// tests set it.
var today = time.Now

// SearchPodcasts returns the podcasts and the episodes YouTube Music finds
// for query. It fails only when both searches fail.
func (c *Client) SearchPodcasts(ctx context.Context, query string) (media.PodcastResults, error) {
	var (
		shows, episodes []media.Item
		showErr, epErr  error
		wg              sync.WaitGroup
	)
	wg.Go(func() { shows, showErr = c.Search(ctx, query, FilterPodcasts) })
	wg.Go(func() { episodes, epErr = c.Search(ctx, query, FilterEpisodes) })
	wg.Wait()
	res := media.EmptyPodcastResults()
	if showErr != nil && epErr != nil {
		return res, errors.Join(showErr, epErr)
	}
	for _, it := range shows {
		if it.Podcast != nil {
			res.Podcasts = append(res.Podcasts, *it.Podcast)
		}
	}
	for _, it := range episodes {
		if it.Track != nil && it.Track.IsEpisode() {
			res.Episodes = append(res.Episodes, *it.Track)
		}
	}
	return res, nil
}

// Podcast returns a podcast with its newest episodes, by its browse ID
// (MPSP…), and what reads the episodes after them.
func (c *Client) Podcast(ctx context.Context, browseID string) (media.Podcast, error) {
	if !strings.HasPrefix(browseID, podcastPrefix) {
		return media.Podcast{}, ErrNotFound
	}
	n, err := c.browse(ctx, browseID)
	if err != nil {
		return media.Podcast{}, err
	}
	return parsePodcast(n, browseID)
}

// parsePodcast reads a podcast's page: its head, as an album's, and the
// first page of its episodes, the newest first.
func parsePodcast(n node, browseID string) (media.Podcast, error) {
	h := responsiveHeader(n)
	if !h.exists() {
		return media.Podcast{}, ErrNotFound
	}
	playlistID := strings.TrimPrefix(browseID, podcastPrefix)
	p := media.Podcast{
		ID:          media.ID(media.SourceYouTube, browseID),
		Source:      media.SourceYouTube,
		Title:       h.get("title").text(),
		Authors:     parseByline(h.get("straplineTextOne").runs()).artists,
		Description: headerDescription(h),
		Artwork:     artwork(thumbnailList(h)),
		URL:         "https://music.youtube.com/playlist?list=" + playlistID,
		PlaylistID:  playlistID,
		Saved:       savedInLibrary(h),
	}
	if p.Authors == nil {
		p.Authors = []media.Ref{}
	}
	_, shelf := secondaryShelf(n)
	contents := shelf.get("contents").list()
	p.Episodes = episodesOf(contents, media.Ref{ID: p.ID, Name: p.Title})
	p.More = shelfToken(shelf, contents)
	return p, nil
}

// PodcastEpisodes returns the page of a podcast's episodes that more, a
// podcast's More, reads, and what reads the page after it: about a
// hundred episodes each. show is the podcast, which the episodes name.
func (c *Client) PodcastEpisodes(ctx context.Context, show media.Ref, more string) (media.EpisodePage, error) {
	n, err := c.call(ctx, "browse", map[string]any{"continuation": more}, nil)
	if err != nil {
		return media.EpisodePage{}, err
	}
	contents, next := listContinuation(n)
	return media.EpisodePage{Episodes: episodesOf(contents, show), More: next}, nil
}

// LibraryPodcasts returns the podcasts the user saved on YouTube Music.
func (c *Client) LibraryPodcasts(ctx context.Context) ([]media.Podcast, error) {
	items, err := c.libraryItems(ctx, libraryPodcasts)
	podcasts := []media.Podcast{}
	for _, it := range items {
		if it.Podcast != nil {
			podcasts = append(podcasts, *it.Podcast)
		}
	}
	return podcasts, err
}

// episodesOf reads the episodes among a show's rows, which the show names.
func episodesOf(contents []node, show media.Ref) []media.Track {
	episodes := []media.Track{}
	for _, c := range contents {
		name, r := c.renderer()
		if name != "musicMultiRowListItemRenderer" {
			continue
		}
		if it, ok := parseMultiRowItem(r, show); ok {
			episodes = append(episodes, *it.Track)
		}
	}
	return episodes
}

// parseMultiRowItem reads a musicMultiRowListItemRenderer: an episode's
// row, with its title, "58K views • 8h ago" and its length. show is the
// podcast whose page holds it, if known.
func parseMultiRowItem(r node, show media.Ref) (media.Item, bool) {
	watch := r.get("onTap", "watchEndpoint")
	if !watch.exists() {
		watch = overlayWatch(r)
	}
	videoID := watch.get("videoId").str()
	title := r.get("title").text()
	if videoID == "" || title == "" {
		return media.Item{}, false
	}
	line := parseEpisodeRuns(append(r.get("subtitle").runs(),
		r.get("playbackProgress", "musicPlaybackProgressRenderer", "durationText").runs()...))
	if show.Name != "" {
		line.show = show
	}
	t := newEpisode(videoID, title, line, episodeArt(thumbnailList(r)))
	readProgress(t, r.get("playbackProgress", "musicPlaybackProgressRenderer"))
	return media.Item{Kind: media.ItemTrack, Track: t}, true
}

// parseEpisodeListItem reads a row that is an episode, of a search,
// "Sep 1 • <its show>", or of a playlist, with its length: its title opens
// the episode's page, or its play button plays an episode. ok is false for
// other rows.
func parseEpisodeListItem(item node, titleRuns []node) (media.Item, bool) {
	watch := overlayWatch(item)
	if pageType(titleRuns[0].get("navigationEndpoint")) != pageTypeEpisode && videoType(watch) != videoTypeEpisode {
		return media.Item{}, false
	}
	videoID := item.get("playlistItemData", "videoId").str()
	if videoID == "" {
		videoID = watch.get("videoId").str()
	}
	if videoID == "" {
		return media.Item{}, false
	}
	var runs []node
	for i := 1; i < len(item.get("flexColumns").list()); i++ {
		runs = append(runs, flexRuns(item, i)...)
	}
	// A playlist's row tells the length in a column of its own.
	for _, col := range item.get("fixedColumns").list() {
		runs = append(runs, col.get("musicResponsiveListItemFixedColumnRenderer", "text").runs()...)
	}
	title := item.get("flexColumns", 0, "musicResponsiveListItemFlexColumnRenderer", "text").text()
	t := newEpisode(videoID, title, parseEpisodeRuns(runs), episodeArt(thumbnailList(item)))
	t.EntryID = item.get("playlistItemData", "playlistSetVideoId").str()
	return media.Item{Kind: media.ItemTrack, Track: t}, true
}

// parseEpisodeCard reads a card of a carousel that opens an episode's
// page, "5 min 51 sec • <its artist>", with the episode under its play
// button.
func parseEpisodeCard(r node, title string) (media.Item, bool) {
	videoID := overlayWatch(r).get("videoId").str()
	if videoID == "" {
		return media.Item{}, false
	}
	line := parseEpisodeRuns(r.get("subtitle").runs())
	return media.Item{Kind: media.ItemTrack, Track: newEpisode(videoID, title, line, episodeArt(thumbnailList(r)))}, true
}

// overlayWatch returns the watch endpoint of the play button over a
// renderer's artwork.
func overlayWatch(r node) node {
	for _, key := range []string{"overlay", "thumbnailOverlay"} {
		if w := r.get(key, "musicItemThumbnailOverlayRenderer", "content", "musicPlayButtonRenderer", "playNavigationEndpoint", "watchEndpoint"); w.exists() {
			return w
		}
	}
	return node{}
}

// videoType returns the type of video a watch endpoint plays.
func videoType(watch node) string {
	return watch.get("watchEndpointMusicSupportedConfigs", "watchEndpointMusicConfig", "musicVideoType").str()
}

// podcastItem makes the podcast an item links to, by its browse ID.
func podcastItem(browseID, title string, b byline, thumbs []media.Image) media.Item {
	playlistID := strings.TrimPrefix(browseID, podcastPrefix)
	authors := b.artists
	if authors == nil {
		authors = []media.Ref{}
	}
	return media.Item{Kind: media.ItemPodcast, Podcast: &media.Podcast{
		ID:         media.ID(media.SourceYouTube, browseID),
		Source:     media.SourceYouTube,
		Title:      title,
		Authors:    authors,
		Artwork:    thumbs,
		Episodes:   []media.Track{},
		URL:        "https://music.youtube.com/playlist?list=" + playlistID,
		PlaylistID: playlistID,
	}}
}

// episodeLine is what the runs about an episode tell.
type episodeLine struct {
	show     media.Ref
	artists  []media.Ref
	released string
	duration int
	plays    int64
}

// parseEpisodeRuns reads the runs about an episode, as "58K views • 8h
// ago", "Sep 1 • <its show>" or "5 min 51 sec • <its artist>".
func parseEpisodeRuns(runs []node) episodeLine {
	var l episodeLine
	for _, r := range runs {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(r.get("text").str()), "•"))
		nav := r.get("navigationEndpoint")
		switch pageType(nav) {
		case pageTypePodcast:
			l.show = media.Ref{ID: media.ID(media.SourceYouTube, nav.get("browseEndpoint", "browseId").str()), Name: text}
			continue
		case pageTypeArtist, pageTypeChannel:
			l.artists = append(l.artists, media.Ref{ID: media.ID(media.SourceYouTube, nav.get("browseEndpoint", "browseId").str()), Name: text})
			continue
		}
		if text == "" {
			continue
		}
		if n, ok := parsePlays(text); ok && l.plays == 0 {
			l.plays = n
			continue
		}
		if d := parseLength(text); d > 0 && l.duration == 0 {
			l.duration = d
			continue
		}
		if day := parseReleased(text, today()); day != "" && l.released == "" {
			l.released = day
		}
	}
	return l
}

// newEpisode makes the track of an episode. Its artists name its show, or
// when the runs name none, the artists they name.
func newEpisode(videoID, title string, l episodeLine, thumbs []media.Image) *media.Track {
	artists := l.artists
	if l.show.Name != "" {
		artists = []media.Ref{{Name: l.show.Name}}
	}
	if artists == nil {
		artists = []media.Ref{}
	}
	return &media.Track{
		ID:         media.ID(media.SourceYouTube, videoID),
		Source:     media.SourceYouTube,
		Title:      title,
		Artists:    artists,
		DurationMs: l.duration,
		Artwork:    thumbs,
		Video:      true,
		VideoType:  videoTypeEpisode,
		Plays:      l.plays,
		URL:        "https://music.youtube.com/watch?v=" + videoID,
		Episode:    &media.Episode{Show: l.show, Released: l.released},
	}
}

// episodeArt returns every size of an episode's artwork, the smallest
// first: its pictures are frames of its video, which can't be resized.
func episodeArt(thumbs node) []media.Image {
	out := []media.Image{}
	for _, t := range thumbs.list() {
		u := t.get("url").str()
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		}
		if u != "" {
			out = append(out, media.Image{URL: u, Width: t.get("width").number(), Height: t.get("height").number()})
		}
	}
	slices.SortStableFunc(out, func(a, b media.Image) int { return a.Width - b.Width })
	return out
}

var (
	lengthRe = regexp.MustCompile(`^(?:(\d+) hrs?)?\s*(?:(\d+) mins?)?\s*(?:(\d+) secs?)?$`)
	agoRe    = regexp.MustCompile(`^(\d+)\s*(s|m|h|d|w|mo|y|seconds?|minutes?|hours?|days?|weeks?|months?|years?) ago$`)
)

// parseLength reads the length of an episode, as "2 hr 18 min", "34 min"
// or "5 min 51 sec", or "3:28", into milliseconds: 0 if s is none.
func parseLength(s string) int {
	if d := parseDuration(s); d > 0 {
		return d
	}
	m := lengthRe.FindStringSubmatch(s)
	if m == nil || (m[1] == "" && m[2] == "" && m[3] == "") {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	return ((h*60+min)*60 + sec) * 1000
}

// parseReleased reads the day an episode came out, as YouTube Music tells
// it in English: "8h ago", "4d ago", "Oct 26" of this year, "Dec 29,
// 2025", "Yesterday". It returns the day, as 2026-10-30, or "" if s tells
// none.
func parseReleased(s string, now time.Time) string {
	const day = "2006-01-02"
	switch strings.ToLower(s) {
	case "today", "just now":
		return now.Format(day)
	case "yesterday":
		return now.AddDate(0, 0, -1).Format(day)
	}
	if m := agoRe.FindStringSubmatch(strings.ToLower(s)); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch unit := m[2]; {
		case unit == "s" || unit == "m" || unit == "h" || strings.HasPrefix(unit, "second") ||
			strings.HasPrefix(unit, "minute") || strings.HasPrefix(unit, "hour"):
			d := map[byte]time.Duration{'s': time.Second, 'm': time.Minute, 'h': time.Hour}[unit[0]]
			return now.Add(-time.Duration(n) * d).Format(day)
		case unit == "d" || strings.HasPrefix(unit, "day"):
			return now.AddDate(0, 0, -n).Format(day)
		case unit == "w" || strings.HasPrefix(unit, "week"):
			return now.AddDate(0, 0, -7*n).Format(day)
		case unit == "mo" || strings.HasPrefix(unit, "month"):
			return now.AddDate(0, -n, 0).Format(day)
		default:
			return now.AddDate(-n, 0, 0).Format(day)
		}
	}
	if t, err := time.Parse("Jan 2, 2006", s); err == nil {
		return t.Format(day)
	}
	if t, err := time.Parse("Jan 2", s); err == nil {
		t = t.AddDate(now.Year(), 0, 0)
		// A day of this year still to come is of the year before.
		if t.After(now.AddDate(0, 0, 1)) {
			t = t.AddDate(-1, 0, 0)
		}
		return t.Format(day)
	}
	return ""
}
