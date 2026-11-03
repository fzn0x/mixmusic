package main

// Podcasts: a show's page with its episodes, the episodes' rows, the
// search's podcasts and Library › Podcasts.

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

func podcastPath(id string) string { return "/podcast/" + url.PathEscape(id) }

// podcastPages is what the podcasts' pages keep between frames.
type podcastPages struct {
	// more are the episodes of the podcasts' pages, by the pages' keys.
	more map[string]*episodePages
	// search is the search's podcasts merged for its scope, of the answer
	// at of of.
	search struct {
		of     *loaded[app.PodcastSearch]
		at     int
		scope  int
		merged media.PodcastResults
	}
	// listening are the episodes in progress, as of the positions'
	// version listeningAt; listened tells that it was read.
	listening   []media.Track
	listeningAt uint64
	listened    bool
}

// episodePages are the episodes a podcast's page lists: its first page, of
// answer at of of, then the pages loaded after it, and what reads the
// next, while loading.
type episodePages struct {
	of       *loaded[media.Podcast]
	at       int
	episodes []media.Track
	more     string
	loading  bool
	// resume is the first of episodes left partway, or -1, as of the
	// positions' version resumeAt and resumeOf episodes; resumed tells
	// that it was found.
	resume   int
	resumeAt uint64
	resumeOf int
	resumed  bool
}

// resumeIndex returns the first of the episodes e lists that the user
// left partway, or -1, found again once they or where they were left
// changed.
func (s *shell) resumeIndex(e *episodePages) int {
	if v := s.svc.Collection.PositionsVersion(); !e.resumed || e.resumeAt != v || e.resumeOf != len(e.episodes) {
		e.resume, e.resumeAt, e.resumeOf, e.resumed = s.resumable(e.episodes), v, len(e.episodes), true
	}
	return e.resume
}

// episodesOf returns the episodes the page of key lists, starting again
// from the first page when that is a new answer, as after a refresh.
func (s *shell) episodesOf(key string, first *loaded[media.Podcast]) *episodePages {
	if s.podcasts.more == nil {
		s.podcasts.more = map[string]*episodePages{}
	}
	e := s.podcasts.more[key]
	if e == nil || e.of != first || e.at != first.answers {
		e = &episodePages{of: first, at: first.answers, episodes: slices.Clip(first.value.Episodes), more: first.value.More}
		s.podcasts.more[key] = e
	}
	return e
}

// loadMoreEpisodes reads the page of episodes after those e holds.
func (s *shell) loadMoreEpisodes(e *episodePages, show media.Ref) {
	e.loading = true
	more := e.more
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		page, err := s.svc.Catalog.MoreEpisodes(ctx, show, more)
		s.update(func() {
			e.loading = false
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case e.more == more:
				e.episodes = append(e.episodes, page.Episodes...)
				e.more = page.More
			}
		})
	}()
}

// podcastPage is a podcast's page: its head, then its episodes, the
// newest first.
func (s *shell) podcastPage(c *ui.Context, id string) {
	key := "podcast:" + id
	res := fetch(s, key, func(ctx context.Context) (media.Podcast, error) { return s.svc.Catalog.Podcast(ctx, id) })
	switch {
	case !res.done:
		s.pageLoading(c, "podcast")
		return
	case res.err != nil:
		s.pageProblem(c, "podcast", res.err, key)
		return
	}
	p := res.value
	pages := s.episodesOf(key, res)
	episodes := pages.episodes
	k := &s.lk
	s.pageScroll(c, func() {
		s.pageHeader(c, p.Title, nil, nil)
		facts := []fact{{text: "podcast"}, {artists: p.Authors}}
		if p.EpisodeCount > 0 {
			facts = append(facts, fact{text: plural(p.EpisodeCount, "episode")})
		}
		following := s.svc.Collection.Follows(p.ID)
		h := headProps{art: p.Artwork, source: p.Source, facts: facts, description: p.Description,
			keep: &following, keepLabel: "follow", keptLabel: "following", keeping: s.keeping[key],
			onKeep: func(on bool) { s.followPodcast(key, p, on) },
			menu:   s.collectionMenu(p.Title, nil, p.URL, p.Source, ""),
		}
		if len(episodes) > 0 {
			h.play = func() { s.p.PlayTracks(episodes, 0) }
			if i := s.resumeIndex(pages); i >= 0 {
				h.playLabel = "resume"
				h.play = func() { s.p.PlayTracks(episodes, i) }
			}
		}
		s.collectionHead(c, h)
		if p.Source == media.SourceSpotify && !s.svc.Catalog.SpotifyEpisodesPlay() {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 16).Children(func() {
				ui.Text(c, "Spotify's podcasts play only from Spotify: choose In MixMusic, from Spotify or On Spotify Connect in ")
				s.linkOf(c, "Settings › Playback", "/settings/playback")
				ui.Text(c, ".")
			})
		}
		s.episodeList(c, "episodes", episodes, episodeListOptions{empty: "No episodes yet."})
		if pages.more != "" {
			ui.Row(c).Margin(16, 0, 0).Children(func() {
				if s.button(c, "more-episodes", "load more episodes", false).Disabled(pages.loading).Clicked() {
					s.loadMoreEpisodes(pages, media.Ref{ID: p.ID, Name: p.Title})
				}
			})
		}
	})
}

// resumable returns the first of episodes the user left partway, or -1.
func (s *shell) resumable(episodes []media.Track) int {
	for i, t := range episodes {
		if pos, ok := s.svc.Collection.Position(t.ID); ok && pos.Started() {
			return i
		}
	}
	return -1
}

// followPodcast follows the podcast of the page of key, or stops: in
// MixMusic, and on its service where the user signed in.
func (s *shell) followPodcast(key string, p media.Podcast, on bool) {
	s.keeping[key] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := s.svc.Collection.SetPodcastFollowed(ctx, p, on)
		s.update(func() {
			delete(s.keeping, key)
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case on:
				s.notes = append(s.notes, notice(p.Title+": following"))
			default:
				s.notes = append(s.notes, notice(p.Title+": no longer following"))
			}
			s.forget(string(p.Source) + ":podcasts")
		})
	}()
}

// episodeListOptions say what an episode list shows.
type episodeListOptions struct {
	// show names each episode's show, for a list of several shows'.
	show  bool
	empty string
}

// episodeList lists episodes: their artwork, title, day, length and what
// is left of them, the playing one flooded with the accent. Playing one
// plays the list from it.
func (s *shell) episodeList(c *ui.Context, key string, episodes []media.Track, o episodeListOptions) {
	if len(episodes) == 0 {
		s.emptyLine(c, o.empty)
		return
	}
	row := s.episodeRowHeight()
	cur, hasCur := s.st.Current()
	list := ui.Column(c.Key(key))
	from, to := s.nearRows(c, key, list, len(episodes), row)
	now := time.Now()
	list.Children(func() {
		if from > 0 {
			ui.Box(c).Height(float32(from) * row)
		}
		for i := from; i < to; i++ {
			s.episodeRow(c, key, i, episodes, hasCur && cur.Track.ID == episodes[i].ID, o, now)
		}
		if to < len(episodes) {
			ui.Box(c).Height(float32(len(episodes)-to) * row)
		}
	})
}

// episodeRowHeight is the height of an episode's row: a track row's, and
// room for the second line.
func (s *shell) episodeRowHeight() float32 { return max(s.lk.row, 64) }

func (s *shell) episodeRow(c *ui.Context, key string, i int, episodes []media.Track, current bool, o episodeListOptions, now time.Time) {
	k := &s.lk
	t := episodes[i]
	row := ui.Row(c.Key(key+"-"+count(i)+"-"+t.ID)).Height(s.episodeRowHeight()).Padding(0, 8).Gap(16).Radius(k.radiusSm)
	ink, sub := k.text, k.muted
	switch {
	case current:
		row.Background(k.selected)
		ink, sub = k.selectedText, k.selectedSub
	case row.Hovered():
		row.Background(k.hover)
	}
	play := func() {
		if current {
			s.p.Toggle()
		} else {
			s.p.PlayTracks(episodes, i)
		}
	}
	if row.DoubleClicked() {
		play()
	}
	menu := func(m *ui.Menu) { s.trackMenu(m, t) }
	row.ContextMenu(menu)
	hovered := row.Hovered()
	const art = 48
	row.Children(func() {
		lead := s.buttonBase(c.Key(key+"-lead-"+count(i))).Size(art, art).Shrink(0).Cursor(ui.CursorPointer)
		label := "Play " + t.Title
		if current && s.st.Playing {
			label = "Pause " + t.Title
		}
		if lead.Label(label).Clicked() {
			play()
		}
		lead.Children(func() {
			s.cover(c, key+"-cover-"+count(i), t.Artwork, art, false)
			if hovered || current {
				name := "play"
				if current && s.st.Playing {
					name = "pause"
				}
				ui.Box(c).Absolute().Top(0).Left(0).Size(art, art).Background(k.scrim).Radius(k.scrimRadius()).Center().Children(func() {
					ui.Icon(c, icon(name)).FontSize(16).TextColor(k.text)
				})
			}
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, t.Title).FontSize(k.bodySize()).TextColor(ink).MaxLines(1).MinWidth(0)
			ui.Text(c, s.episodeLine(t, o.show, now)).FontSize(k.smallSize()).TextColor(sub).SingleLine().MinWidth(0)
		})
		s.sourceMark(c, t.Source, current)
		duration := ""
		if t.DurationMs > 0 {
			duration = clock(ms(t.DurationMs))
		}
		ui.Text(c, duration).FontSize(k.smallSize()).TextColor(sub).FontFeatures("tnum").Width(64).TextAlign(ui.End).Shrink(0)
		more := s.iconButton(c, key+"-more-"+count(i), "more", "More for "+t.Title, false, 18).Menu(menu)
		if !hovered && !more.FocusVisible() {
			more.Opacity(0)
		}
	})
}

// episodeLine is what an episode's row says under its title: its show,
// when the list is of several shows', the day it came out, its length,
// and what is left of it, or that it was played.
func (s *shell) episodeLine(t media.Track, show bool, now time.Time) string {
	var parts []string
	if e := t.Episode; e != nil {
		if show && e.Show.Name != "" {
			parts = append(parts, e.Show.Name)
		}
		if day := releasedLabel(e.Released, now); day != "" {
			parts = append(parts, day)
		}
	}
	if t.DurationMs > 0 {
		parts = append(parts, longDuration(ms(t.DurationMs)))
	}
	if pos, ok := s.svc.Collection.Position(t.ID); ok {
		switch {
		case pos.Played:
			parts = append(parts, "played")
		case pos.Left() > 0:
			parts = append(parts, longDuration(pos.Left())+" left")
		}
	} else if e := t.Episode; e != nil && e.Played {
		parts = append(parts, "played")
	}
	return strings.Join(parts, " · ")
}

// releasedLabel writes the day an episode came out, as Oct 26, with its
// year when it isn't now's, or its month or year alone when that is all
// its service tells.
func releasedLabel(day string, now time.Time) string {
	if t, err := time.Parse(time.DateOnly, day); err == nil {
		if t.Year() == now.Year() {
			return t.Format("Jan 2")
		}
		return t.Format("Jan 2, 2006")
	}
	if t, err := time.Parse("2006-01", day); err == nil {
		return t.Format("Jan 2006")
	}
	return day
}

// episodeItems are the items of an episode's menu: go to its show, and
// mark it played or not.
func (s *shell) episodeItems(m *ui.Menu, t media.Track) {
	if t.Episode == nil {
		return
	}
	if id := t.Episode.Show.ID; id != "" && m.Item("Go to podcast").Chosen() {
		s.navigate(podcastPath(id))
	}
	played := t.Episode.Played
	if pos, ok := s.svc.Collection.Position(t.ID); ok {
		played = pos.Played
	}
	label := "Mark as played"
	if played {
		label = "Mark as unplayed"
	}
	if m.Item(label).Chosen() {
		s.svc.Collection.SetPlayed(t, !played)
	}
}

// podcastTile is a podcast's tile: it opens its page, and its play circle
// plays its newest episode, or the one the user left partway.
func (s *shell) podcastTile(c *ui.Context, key string, p media.Podcast) {
	sub := names(p.Authors)
	if sub == "" {
		sub = "Podcast"
	}
	open, play := s.artTile(c, key, p.Artwork, p.Title, sub, p.Source, false, false, true, func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			s.navigate(podcastPath(p.ID))
		}
		if m.Item("Play").Chosen() {
			s.playPodcast(p.ID)
		}
		s.copyLinkItem(m, p.URL)
	})
	switch {
	case play:
		s.playPodcast(p.ID)
	case open:
		s.navigate(podcastPath(p.ID))
	}
}

// playPodcast fetches a podcast's newest episodes and plays them: from the
// first the user left partway, or from the newest.
func (s *shell) playPodcast(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		p, err := s.svc.Catalog.Podcast(ctx, id)
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case len(p.Episodes) == 0:
				s.notes = append(s.notes, notice("Nothing to play here"))
			default:
				s.p.PlayTracks(p.Episodes, max(s.resumable(p.Episodes), 0))
			}
		})
	}()
}

// podcastSearch shows the podcasts and the episodes the search finds, of
// the services of scope.
func (s *shell) podcastSearch(c *ui.Context, q string) {
	key := "search-podcasts:" + q
	res := fetch(s, key, func(ctx context.Context) (app.PodcastSearch, error) { return s.svc.Catalog.SearchPodcasts(ctx, q) })
	switch {
	case !res.done:
		s.loading(c)
		return
	case res.err != nil:
		s.problem(c, res.err, func() { s.forget(key) })
		return
	}
	k := &s.lk
	if len(res.value.Errors) > 0 {
		ui.Text(c, strings.Join(res.value.Errors, " · ")).FontSize(k.smallSize()).TextColor(k.danger).Margin(0, 0, 16)
	}
	r := s.mergedPodcasts(res)
	if len(r.Podcasts) > 0 {
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.tileWrap(c, func() {
				for i, p := range r.Podcasts {
					s.podcastTile(c, "podcast-"+count(i), p)
				}
			})
		})
	}
	s.sectionTitle(c, "Episodes")
	s.episodeList(c, "search-episodes", r.Episodes, episodeListOptions{show: true, empty: "No episodes match."})
}

// mergedPodcasts are the podcasts of res of the services of the search's
// scope, made once for each answer and scope.
func (s *shell) mergedPodcasts(res *loaded[app.PodcastSearch]) media.PodcastResults {
	m := &s.podcasts.search
	if m.of != res || m.at != res.answers || m.scope != s.searchScope {
		m.of, m.at, m.scope = res, res.answers, s.searchScope
		switch r := res.value; s.searchScope {
		case 1:
			m.merged = r.Spotify
		case 2:
			m.merged = r.YouTube
		default:
			m.merged = media.PodcastResults{
				Podcasts: interleave(r.Spotify.Podcasts, r.YouTube.Podcasts),
				Episodes: interleave(r.Spotify.Episodes, r.YouTube.Episodes),
			}
		}
	}
	return m.merged
}

// podcastCount is how many podcasts and episodes the search of q found,
// once it did, for its pivot; -1 before, as the search asks only once the
// pivot shows.
func (s *shell) podcastCount(q string) int {
	res, ok := s.loads["search-podcasts:"+q].(*loaded[app.PodcastSearch])
	if !ok || !res.done || res.err != nil {
		return -1
	}
	r := s.mergedPodcasts(res)
	return len(r.Podcasts) + len(r.Episodes)
}

// podcastLibrary is Library › Podcasts: the episodes the user left
// partway, the podcasts they follow, and those they saved on the services
// they signed in to.
func (s *shell) podcastLibrary(c *ui.Context) {
	k := &s.lk
	if listening := s.listening(); len(listening) > 0 {
		s.sectionTitle(c, "Keep listening")
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.episodeList(c, "keep-listening", listening, episodeListOptions{show: true})
		})
	}
	following := s.svc.Collection.Podcasts()
	if len(following) == 0 {
		ui.Text(c, k.words("Nothing followed yet")).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, "Follow a podcast from its page, from Spotify or YouTube Music, and it waits here.").
			FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640).Margin(0, 0, 48)
	} else {
		s.sectionTitle(c, "Following")
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.tileWrap(c, func() {
				for i, p := range following {
					s.podcastTile(c, "following-"+count(i), p)
				}
			})
		})
	}
	accounts := s.accounts()
	col := s.svc.Collection
	if accounts.Spotify.SignedIn {
		s.savedPodcasts(c, "spotify", "Saved on Spotify", fetch(s, "spotify:podcasts", col.SpotifyPodcasts))
	}
	if accounts.YouTube.SignedIn {
		s.savedPodcasts(c, "youtube", "Saved on YouTube Music", fetch(s, "youtube:podcasts", col.YouTubePodcasts))
	}
}

// savedPodcasts is a shelf of the podcasts saved on a service, past those
// the user follows in MixMusic, which show above.
func (s *shell) savedPodcasts(c *ui.Context, key, title string, saved *loaded[[]media.Podcast]) {
	switch {
	case !saved.done:
		return
	case saved.err != nil:
		s.problem(c, saved.err, func() { s.forget(key + ":podcasts") })
		return
	}
	n := 0
	for _, p := range saved.value {
		if !s.svc.Collection.Follows(p.ID) {
			n++
		}
	}
	if n == 0 {
		return
	}
	s.shelf(c, key+"-podcasts", title, func() {
		for i, p := range saved.value {
			if !s.svc.Collection.Follows(p.ID) {
				s.podcastTile(c, key+"-podcast-"+count(i), p)
			}
		}
	})
}

// listening returns the episodes in progress, the latest first, read again
// once where they were left changed.
func (s *shell) listening() []media.Track {
	l := &s.podcasts
	if v := s.svc.Collection.PositionsVersion(); !l.listened || l.listeningAt != v {
		l.listened, l.listeningAt = true, v
		// A new list: the player may hold the one before.
		l.listening = nil
		for _, pos := range s.svc.Collection.InProgress(20) {
			l.listening = append(l.listening, pos.Track)
		}
	}
	return l.listening
}
