package main

// Downloads: the tracks, albums, playlists and mixes the user downloads to
// play without the network, their marks on the tracks' rows, and their
// page in the library.

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
)

// bigDownload is how many tracks, or bytes, a download takes before
// MixMusic asks first.
const (
	bigDownload      = 100
	bigDownloadBytes = 1 << 30
)

// downloadable reports whether t downloads: a track of YouTube Music, or a
// Spotify track, as its recording there.
func downloadable(t media.Track) bool {
	return t.Source == media.SourceYouTube || t.Source == media.SourceSpotify
}

// anyDownloadable reports whether a track of tracks downloads, at the cost
// of one, as a rule: for the head of a page, in each frame.
func anyDownloadable(tracks []media.Track) bool {
	for _, t := range tracks {
		if downloadable(t) {
			return true
		}
	}
	return false
}

// downloadableOf is the tracks of tracks that download.
func downloadableOf(tracks []media.Track) []media.Track {
	out := make([]media.Track, 0, len(tracks))
	for _, t := range tracks {
		if downloadable(t) {
			out = append(out, t)
		}
	}
	return out
}

// confirmDownload runs run, which downloads tracks, after asking when they
// are many, or take a gigabyte or so.
func (s *shell) confirmDownload(tracks []media.Track, run func()) {
	size := s.svc.Downloads.Estimate(tracks)
	if len(tracks) <= bigDownload && size < bigDownloadBytes {
		run()
		return
	}
	s.ask("download "+plural(len(tracks), "track"), "That's about "+byteSize(size)+". They play without the network, and stay until you remove them.",
		"download", run)
}

// download downloads tracks on their own, telling what downloads.
func (s *shell) download(tracks []media.Track) {
	tracks = downloadableOf(tracks)
	if len(tracks) == 0 {
		return
	}
	s.confirmDownload(tracks, func() {
		s.svc.Downloads.Add(tracks)
		msg := "Downloading " + plural(len(tracks), "track")
		if len(tracks) == 1 {
			msg = "Downloading " + tracks[0].Title
			if tracks[0].Source == media.SourceSpotify {
				msg += ": its recording on YouTube Music"
			}
		}
		s.notes = append(s.notes, notice(msg))
	})
}

// downloadItems are the items of a track's menu that download it, stop its
// download, or remove it.
func (s *shell) downloadItems(m *ui.Menu, t media.Track) {
	if !downloadable(t) {
		return
	}
	switch s.kept.State(t.ID).Kind {
	case downloads.Done:
		if m.Item("Remove download").Chosen() {
			s.svc.Downloads.Remove(t.ID)
			s.notes = append(s.notes, notice("Removed the download of "+t.Title))
		}
	case downloads.Waiting, downloads.Downloading:
		if m.Item("Cancel download").Chosen() {
			s.svc.Downloads.Remove(t.ID)
		}
	case downloads.Failed:
		if m.Item("Download again").Chosen() {
			s.svc.Downloads.Retry(t.ID)
		}
		if m.Item("Cancel download").Chosen() {
			s.svc.Downloads.Remove(t.ID)
		}
	default:
		if m.Item("Download").Chosen() {
			s.download([]media.Track{t})
		}
	}
}

// downloadHead is what downloads with an album, a playlist, a mix or the
// liked songs: the collection, and the tracks its page holds now.
type downloadHead struct {
	c      downloads.Collection
	tracks []media.Track
}

func albumDownload(a media.Album) *downloadHead {
	return &downloadHead{c: downloads.Collection{ID: "album:" + a.ID, Kind: "album", Title: a.Title, Source: a.Source,
		Artwork: a.Artwork, Total: a.TrackCount}, tracks: a.Tracks}
}

func playlistDownload(p media.Playlist) *downloadHead {
	kind := "playlist"
	if p.Source == media.SourceLocal {
		kind = "mix"
	}
	return &downloadHead{c: downloads.Collection{ID: "playlist:" + p.ID, Kind: kind, Title: p.Title, Source: p.Source,
		Artwork: p.Artwork, Total: p.TrackCount}, tracks: p.Tracks}
}

func likedDownload(liked []media.Track) *downloadHead {
	return &downloadHead{c: downloads.Collection{ID: "liked", Kind: "liked", Title: "Liked songs", Source: media.SourceLocal}, tracks: liked}
}

// downloadAction is where the download of a collection is, as its head's
// button and its menu say, and what choosing it does.
type downloadAction struct {
	label, item string
	done        bool
	run         func()
}

// downloadOf tells where the download of d is: not asked for, under way,
// missing tracks its page holds now, or done.
func (s *shell) downloadOf(d *downloadHead) downloadAction {
	v, held := s.kept.Collection(d.c.ID)
	title := d.c.Title
	remove := func(question string) func() {
		return func() {
			s.ask("remove download", question, "remove", func() {
				s.svc.Downloads.RemoveCollection(d.c.ID)
				s.notes = append(s.notes, notice("Removed the download of "+title))
			})
		}
	}
	switch {
	case !held:
		return downloadAction{"download", "Download", false, func() { s.downloadCollection(d) }}
	case v.Waiting > 0:
		return downloadAction{fmt.Sprintf("downloading %d of %d", v.Kept, len(v.Tracks)), "Cancel download", false,
			remove("Stop downloading " + title + "? What downloaded of it goes too, unless another download keeps it.")}
	}
	if more := s.missing(d) + v.Failed; more > 0 {
		return downloadAction{fmt.Sprintf("download %d more", more), fmt.Sprintf("Download %d more", more), false,
			func() { s.downloadCollection(d) }}
	}
	return downloadAction{"downloaded", "Remove download", true,
		remove("Remove the download of " + title + "? Its tracks won't play offline any more, unless another download keeps them.")}
}

// missingMemo is how many tracks of a page its download misses, as counted
// last: of the snapshot of version, for the tracks starting at first.
type missingMemo struct {
	version uint64
	first   *media.Track
	n, of   int
}

// missing counts the tracks of d's page its download doesn't hold, once for
// each change of the downloads, not in each frame: a playlist holds
// thousands.
func (s *shell) missing(d *downloadHead) int {
	var first *media.Track
	if len(d.tracks) > 0 {
		first = &d.tracks[0]
	}
	v := s.kept.Version()
	if m, ok := s.missingMemos[d.c.ID]; ok && m.version == v && m.first == first && m.of == len(d.tracks) {
		return m.n
	}
	n := s.kept.Missing(d.c.ID, downloadableOf(d.tracks))
	s.missingMemos[d.c.ID] = missingMemo{version: v, first: first, n: n, of: len(d.tracks)}
	return n
}

// downloadCollection downloads the tracks of d's page, as its collection.
func (s *shell) downloadCollection(d *downloadHead) {
	tracks := downloadableOf(d.tracks)
	if len(tracks) == 0 {
		s.notes = append(s.notes, notice("Nothing here downloads"))
		return
	}
	s.confirmDownload(tracks, func() {
		s.svc.Downloads.AddCollection(d.c, tracks)
		msg := "Downloading " + d.c.Title
		if d.c.Total > len(d.tracks) {
			msg += fmt.Sprintf(": the %d of its %d tracks MixMusic reads", len(d.tracks), d.c.Total)
		}
		s.notes = append(s.notes, notice(msg))
	})
}

// downloadButton is the button of a collection's head that downloads it.
func (s *shell) downloadButton(c *ui.Context, d *downloadHead) {
	a := s.downloadOf(d)
	if s.button(c, "head-download", a.label, a.done).Clicked() {
		a.run()
	}
}

// withDownload adds the download of d to menu, a collection's.
func (s *shell) withDownload(menu func(m *ui.Menu), d *downloadHead) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		menu(m)
		if !anyDownloadable(d.tracks) {
			return
		}
		m.Separator()
		if a := s.downloadOf(d); m.Item(a.item).Chosen() {
			a.run()
		}
	}
}

// downloadAlbum and downloadPlaylist fetch what a collection holds, and
// download it.
func (s *shell) downloadAlbum(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		a, err := s.svc.Catalog.Album(ctx, id)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.downloadCollection(albumDownload(a))
		})
	}()
}

func (s *shell) downloadPlaylist(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		p, err := s.svc.Catalog.Playlist(ctx, id)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.downloadCollection(playlistDownload(p))
		})
	}()
}

// downloadMark tells, after a track's source mark, where its download is:
// downloaded, waiting, downloading as a ring that fills, or failed, with
// the reason in its label and tooltip. It takes its room in every row
// while the downloads hold anything, so that the columns stay in line, and
// none otherwise.
func (s *shell) downloadMark(c *ui.Context, key string, t media.Track, on bool) {
	if !s.kept.Any() {
		return
	}
	k := &s.lk
	st := s.kept.State(t.ID)
	box := ui.Box(c.Key(key)).Size(14, 14).Shrink(0).Center()
	ink, track := k.muted, k.muted.Alpha(0.5)
	if on {
		ink, track = k.accentText, k.accentText.Alpha(0.4)
	}
	switch st.Kind {
	case downloads.Done:
		box.Label("Downloaded").Tooltip("Downloaded").Children(func() {
			ui.Icon(c, icon("download")).FontSize(13).TextColor(ink)
		})
	case downloads.Waiting:
		box.Label("Waiting to download").Tooltip("Waiting to download").Draw(func(p *ui.Painter, r ui.Rect) {
			var ring ui.Path
			ring.Circle(r.X+7, r.Y+7, 5.5)
			p.StrokePath(&ring, 1.5, track)
		})
	case downloads.Downloading:
		label := "Downloading"
		if st.Progress >= 0 {
			label = fmt.Sprintf("Downloading, %d%%", int(math.Round(st.Progress*100)))
		}
		accent := k.accent
		if on {
			accent = k.accentText
		}
		v := float32(max(st.Progress, 0.04))
		box.Label(label).Tooltip(label).Draw(func(p *ui.Painter, r ui.Rect) {
			var ring, arc ui.Path
			ring.Circle(r.X+7, r.Y+7, 5.5)
			p.StrokePath(&ring, 1.5, track)
			circleFromTop(&arc, r.X+7, r.Y+7, 5.5)
			arc.Trim(0, v)
			p.StrokePath(&arc, 1.5, accent)
		})
	case downloads.Failed:
		label := "Couldn't download: " + st.Why
		danger := k.danger
		if on {
			danger = k.accentText
		}
		box.Label(label).Tooltip(label).Children(func() {
			ui.Icon(c, icon("alert")).FontSize(13).TextColor(danger)
		})
	}
}

// circleFromTop adds a circle to p, drawn clockwise from its top, as a
// ring of progress fills.
func circleFromTop(p *ui.Path, cx, cy, r float32) {
	const k = 0.5522847498
	p.MoveTo(cx, cy-r)
	p.CubeTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy)
	p.CubeTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r)
	p.CubeTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy)
	p.CubeTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r)
}

// downloadsView is the downloaded tracks the page lists, as made last: of
// the snapshot of version, all of them or those of the collection of.
type downloadsView struct {
	version uint64
	of      string
	tracks  []media.Track
}

// downloadedTracks returns the tracks downloaded, or those of the
// collection the page shows, made again only once the downloads change.
func (s *shell) downloadedTracks() []media.Track {
	v := s.kept.Version()
	if s.downloadsList.version == v && s.downloadsList.of == s.downloadsOf && s.downloadsList.tracks != nil {
		return s.downloadsList.tracks
	}
	tracks := s.kept.Tracks()
	if col, ok := s.kept.Collection(s.downloadsOf); ok {
		tracks = make([]media.Track, 0, len(col.Tracks))
		for _, id := range col.Tracks {
			if t, ok := s.kept.Track(id); ok {
				tracks = append(tracks, t)
			}
		}
	}
	if tracks == nil {
		tracks = []media.Track{}
	}
	s.downloadsList = downloadsView{version: v, of: s.downloadsOf, tracks: tracks}
	return tracks
}

// downloadsLibrary is the library's downloads: what they hold and where,
// what downloads, the albums, playlists and mixes downloaded, and every
// track.
func (s *shell) downloadsLibrary(c *ui.Context) {
	k := &s.lk
	st := s.kept.Status()
	if _, ok := s.kept.Collection(s.downloadsOf); !ok {
		s.downloadsOf = ""
	}
	problem := func() {
		if st.Problem != "" {
			s.alert(c, "Your downloads can't be reached", st.Problem+". Tracks stream as before meanwhile.", nil).Margin(0, 0, 32)
		}
	}
	if st.Tracks == 0 && st.Waiting == 0 && st.Failed == 0 {
		// A folder that can't be reached tells nothing of what it holds.
		if st.Problem == "" {
			ui.Text(c, "nothing downloaded yet").FontSize(k.size(3.5)).FontWeight(300).Font(k.display).TextColor(k.accent)
			ui.Text(c, "Choose Download in the menu of a track, an album or a playlist, and it plays here without the network.").
				FontSize(k.subtitleSize()).FontWeight(300).MaxWidth(640).Margin(0, 0, 24)
		}
		s.downloadsFolderLine(c, st)
		problem()
		return
	}
	s.downloadsFolderLine(c, st)
	problem()
	if st.Waiting > 0 || st.Failed > 0 {
		s.downloadQueue(c, st)
	}
	if cols := s.kept.Collections(); len(cols) > 0 {
		s.shelf(c, "downloaded", "albums & playlists", func() {
			// The tiles far to either side take their room, and build
			// nothing.
			n := len(cols)
			from, to := s.nearTiles(c, "downloaded", n)
			if from > 0 {
				ui.Box(c).Width(float32(from)*(artTileSize+shelfGap) - shelfGap).Shrink(0)
			}
			for i := from; i < to; i++ {
				s.downloadedTile(c, "downloaded-"+count(i), cols[i])
			}
			if to < n {
				ui.Box(c).Width(float32(n-to)*(artTileSize+shelfGap) - shelfGap).Shrink(0)
			}
		})
	}
	tracks := s.downloadedTracks()
	title := "tracks"
	if col, ok := s.kept.Collection(s.downloadsOf); ok {
		title = col.Title
	}
	s.sectionHead(c, title, func() {
		if s.downloadsOf != "" && s.link(c, "downloads-all", "show all") {
			s.downloadsOf = ""
		}
	})
	if len(tracks) > 0 {
		ui.Row(c).Gap(8).Margin(0, 0, 16).Children(func() {
			if s.button(c, "downloads-play", "play", true).Clicked() {
				s.p.PlayTracks(tracks, 0)
			}
			if s.button(c, "downloads-shuffle", "shuffle", false).Clicked() {
				s.shuffled(tracks)
			}
		})
	}
	view := s.trackTools(c, "downloads", "list", tracks)
	empty := "Nothing downloaded here yet."
	if q := s.searching("downloads"); q != "" {
		empty = "No download matches “" + q + "”."
	}
	s.trackList(c, "downloads", view.tracks, trackListOptions{art: true, album: true, empty: empty})
	if st.Tracks > 0 {
		ui.Row(c).Margin(32, 0, 0).Children(func() {
			if s.button(c, "downloads-remove-all", "remove all downloads", false).Clicked() {
				s.ask("remove all downloads", fmt.Sprintf("Remove all %s, %s? They won't play offline any more.", plural(st.Tracks, "download"), byteSize(st.Bytes)),
					"remove", func() {
						s.svc.Downloads.RemoveAll()
						s.notes = append(s.notes, notice("Removed every download"))
					})
			}
		})
	}
}

// downloadsFolderLine tells what the downloads hold and where, with links
// to their folder and their settings.
func (s *shell) downloadsFolderLine(c *ui.Context, st downloads.Status) {
	k := &s.lk
	ui.Row(c).Gap(16).Wrap().AlignItems(ui.Center).Margin(0, 0, 24).Children(func() {
		ui.Text(c, plural(st.Tracks, "track")+" · "+byteSize(st.Bytes)+" · in "+st.Folder).FontSize(k.smallSize()).TextColor(k.muted).MinWidth(0)
		if s.link(c, "downloads-open", "open folder") {
			s.openDownloads()
		}
		if s.link(c, "downloads-settings", "settings") {
			s.navigate("/settings/playback")
		}
	})
}

// openDownloads opens the folder of the downloads in the file manager.
func (s *shell) openDownloads() {
	go func() {
		if err := s.svc.Downloads.OpenFolder(); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// downloadQueue shows what downloads: how far they came, each under way and
// next with a way to stop it, and those that failed, with why.
func (s *shell) downloadQueue(c *ui.Context, st downloads.Status) {
	k := &s.lk
	ui.Column(c).Margin(0, 0, 48).Children(func() {
		s.sectionHead(c, "downloading", func() {
			if st.Waiting > 0 {
				label := "pause"
				if st.Paused {
					label = "resume"
				}
				if s.link(c, "downloads-pause", label) {
					s.svc.Downloads.SetPaused(!st.Paused)
				}
				if s.link(c, "downloads-cancel", "cancel all") {
					s.ask("cancel all downloads", "Stop every download under way, and those that wait? What downloaded stays.", "stop", s.svc.Downloads.CancelAll)
				}
			}
			if st.Failed > 0 && s.link(c, "downloads-retry", "retry failed") {
				s.svc.Downloads.RetryFailed()
			}
		})
		if st.Waiting > 0 {
			total := st.BatchDone + st.Waiting
			ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(0, 0, 12).Children(func() {
				ui.Progress(c, float64(st.BatchDone)/float64(total)).Width(240).Label("Downloads")
				ui.Text(c, fmt.Sprintf("%d of %d", st.BatchDone, total)).FontSize(k.smallSize()).TextColor(k.muted).FontFeatures("tnum")
			})
		}
		if st.Hold != "" {
			ui.Text(c, st.Hold).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 12).MaxWidth(720)
		}
		for _, j := range st.Jobs {
			s.jobRow(c, j)
		}
		if listed := len(st.Jobs); st.Waiting > listed {
			ui.Text(c, fmt.Sprintf("and %d more", st.Waiting-listed)).FontSize(k.smallSize()).TextColor(k.muted).Padding(8, 8)
		}
		for _, j := range st.FailedJobs {
			s.jobRow(c, j)
		}
	})
}

// jobRow is a track that downloads, waits to, or failed.
func (s *shell) jobRow(c *ui.Context, j downloads.Job) {
	k := &s.lk
	t := j.Track
	st := s.kept.State(t.ID)
	ui.Row(c.Key("job-"+t.ID)).Height(56).Padding(0, 8).Gap(12).AlignItems(ui.Center).Children(func() {
		s.cover(c, "job-cover-"+t.ID, t.Artwork, 40, false)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, t.Title).FontSize(k.bodySize()).SingleLine()
			ui.Text(c, names(t.Artists)).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
		})
		status, ink := "waiting", k.muted
		switch st.Kind {
		case downloads.Downloading:
			status = "downloading"
			if j.Phase != "" {
				status = j.Phase
			} else if st.Progress >= 0 {
				status = fmt.Sprintf("%d%%", int(math.Round(st.Progress*100)))
			}
		case downloads.Failed:
			status, ink = "couldn't download: "+st.Why, k.danger
		}
		ui.Text(c, status).FontSize(k.smallSize()).TextColor(ink).MaxWidth(360).MaxLines(2).Shrink(1)
		if st.Kind == downloads.Failed && s.link(c, "job-retry-"+t.ID, "retry") {
			s.svc.Downloads.Retry(t.ID)
		}
		if s.iconButton(c, "job-cancel-"+t.ID, "close", "Cancel the download of "+t.Title, false, 14).Clicked() {
			s.svc.Downloads.Remove(t.ID)
		}
	})
}

// downloadedTile is an album, a playlist or a mix downloaded, which shows
// its tracks below when chosen.
func (s *shell) downloadedTile(c *ui.Context, key string, v downloads.CollectionView) {
	sub := plural(len(v.Tracks), "track")
	if v.Kept < len(v.Tracks) {
		sub = fmt.Sprintf("%d of %d tracks", v.Kept, len(v.Tracks))
	}
	if v.Total > len(v.Tracks) {
		sub += fmt.Sprintf(", of its %d", v.Total)
	}
	art := v.Artwork
	open, play := s.artTile(c, key, art, v.Title, sub, v.Source, false, s.downloadsOf == v.ID, true, func(m *ui.Menu) {
		if m.Item("Show its tracks").Chosen() {
			s.downloadsOf = v.ID
		}
		if m.Item("Remove download").Chosen() {
			s.ask("remove download", "Remove the download of "+v.Title+"? Its tracks won't play offline any more, unless another download keeps them.",
				"remove", func() { s.svc.Downloads.RemoveCollection(v.ID) })
		}
	})
	switch {
	case play:
		var tracks []media.Track
		for _, id := range v.Tracks {
			if t, ok := s.kept.Track(id); ok {
				tracks = append(tracks, t)
			}
		}
		if len(tracks) > 0 {
			s.p.PlayTracks(tracks, 0)
		}
	case open && s.downloadsOf == v.ID:
		s.downloadsOf = ""
	case open:
		s.downloadsOf = v.ID
	}
}

// downloadSettings show the downloads in Settings › Playback: what they
// hold, their folder, and whether they go on over a metered connection.
func (s *shell) downloadSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, "downloads", func() {
		st := s.kept.Status()
		s.row(c, "Downloads", fmt.Sprintf("%s in %s. They take the quality above, play without the network, and stay until you remove them.",
			plural(st.Tracks, "track"), byteSize(st.Bytes)), func() {
			if s.button(c, "show-downloads", "show", false).Label("Show downloads").Clicked() {
				s.navigate("/library/downloads")
			}
		})
		folder, def := st.Folder, s.svc.Downloads.Default()
		s.rowWith(c, "Folder", func() {
			ui.Text(c, folder)
			if st.Problem != "" {
				ui.Text(c, ". "+st.Problem)
			}
		}, func() {
			if s.movingDownloads {
				v := -1.0
				if s.moveTotal > 0 {
					v = float64(s.moveDone) / float64(s.moveTotal)
				}
				ui.Progress(c, v).Width(160).Label("Moving the downloads")
				ui.Text(c, "moving…").FontSize(k.smallSize()).TextColor(k.muted)
				return
			}
			if s.button(c, "downloads-folder-open", "open", false).Label("Open the folder of the downloads").Clicked() {
				s.openDownloads()
			}
			if s.button(c, "downloads-folder-change", "change", false).Label("Change the folder of the downloads").Clicked() {
				s.chooseDownloadsFolder(st)
			}
			if !samePath(folder, def) && s.link(c, "downloads-folder-default", "use the default") {
				s.askMoveDownloads(def, st)
			}
		})
		s.row(c, "Download on metered connections", "Off: downloads wait on a metered connection, such as a phone's hotspot.", func() {
			if s.switchOf(c.Key("download-metered"), &s.set.DownloadOnMetered).Label("Download on metered connections").Changed() {
				s.save()
			}
		})
	})
}

// samePath reports whether two folders are the same, as their names tell:
// regardless of case on Windows.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// chooseDownloadsFolder asks for the folder to move the downloads to.
func (s *shell) chooseDownloadsFolder(st downloads.Status) {
	go func() {
		dir, err := s.svc.Downloads.ChooseFolder(s.win)
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case dir != "":
				s.askMoveDownloads(dir, st)
			}
		})
	}()
}

// askMoveDownloads asks before moving the downloads to dir, when there are
// some to move.
func (s *shell) askMoveDownloads(dir string, st downloads.Status) {
	if st.Tracks == 0 && st.Waiting == 0 {
		s.moveDownloads(dir)
		return
	}
	s.ask("move downloads", fmt.Sprintf("Move %s, %s, to %s? They wait meanwhile.", plural(st.Tracks, "download"), byteSize(st.Bytes), dir),
		"move", func() { s.moveDownloads(dir) })
}

// moveDownloads moves the downloads to dir, showing how far it came, and
// keeps their folder in the settings once moved, on the main thread: the
// settings the window saves are its own copy.
func (s *shell) moveDownloads(dir string) {
	s.movingDownloads, s.moveDone, s.moveTotal = true, 0, 0
	go func() {
		got, err := s.svc.Downloads.Move(context.Background(), dir, func(done, total int64) {
			s.update(func() { s.moveDone, s.moveTotal = done, total })
		})
		s.update(func() {
			s.movingDownloads = false
			if err != nil {
				s.notes = append(s.notes, noticeOf(fmt.Errorf("couldn't move the downloads: %w", err)))
				return
			}
			s.set.DownloadsFolder = got
			s.save()
			s.notes = append(s.notes, notice("The downloads are in "+s.svc.Downloads.Folder()+" now"))
		})
	}()
}

// keptArt is the covers the downloads keep, which show without the
// network, else the covers of src.
type keptArt struct {
	local func(url string) (string, bool)
	src   artSource
}

func (k keptArt) Image(ctx context.Context, rawURL string) (image.Image, error) {
	if p, ok := k.local(rawURL); ok {
		if f, err := os.Open(p); err == nil {
			img, _, err := image.Decode(f)
			f.Close()
			if err == nil {
				return img, nil
			}
		}
	}
	return k.src.Image(ctx, rawURL)
}

// followNetwork tells once the network goes, while the downloads hold
// something, that they still play.
func (s *shell) followNetwork(online bool) {
	if online || s.svc.Downloads.Snapshot().Status().Tracks == 0 {
		return
	}
	s.actionToasts = append(s.actionToasts, actionToast{"You're offline: your downloads still play", "downloads",
		func() { s.navigate("/library/downloads") }})
}
