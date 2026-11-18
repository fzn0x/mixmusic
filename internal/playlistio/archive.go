package playlistio

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
)

var (
	// The files of Spotify's data download: the playlists, Your Library,
	// and the streaming histories, of the account and extended.
	spotifyPlaylistsFile = regexp.MustCompile(`(?i)^Playlist\d*\.json$`)
	spotifyLibraryFile   = regexp.MustCompile(`(?i)^YourLibrary\.json$`)
	spotifyHistoryFile   = regexp.MustCompile(`(?i)^(StreamingHistory(_music)?_?\d+|Streaming_History_Audio_.*)\.json$`)
	// googleLibraryFile is the songs of the YouTube Music library in a
	// Google Takeout, whose folders' names are in the account's
	// language.
	googleLibraryFile = regexp.MustCompile(`(?i)^music library songs\.csv$`)
)

// readArchive reads the ZIP archive of a data download: Spotify's
// playlists, Your Library and streaming history, or the playlists and the
// library songs of YouTube Music in a Google Takeout, which name songs by
// their videos' IDs. It reads 1,000 files at most, and 100 MB of them.
func readArchive(data []byte) (Source, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Source{}, ErrNoDataDownload
	}
	if len(z.File) > MaxArchiveFiles {
		return Source{}, ErrArchiveFiles
	}
	files := slices.Clone(z.File)
	slices.SortStableFunc(files, func(a, b *zip.File) int { return strings.Compare(a.Name, b.Name) })
	var src Source
	var histories [][]any
	budget := int64(MaxArchive)
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}
		base := path.Base(f.Name)
		lower := strings.ToLower(base)
		spotify := spotifyPlaylistsFile.MatchString(base) || spotifyLibraryFile.MatchString(base) ||
			spotifyHistoryFile.MatchString(base)
		if !spotify && !strings.HasSuffix(lower, ".csv") {
			continue
		}
		body, err := readZipped(f, &budget)
		if err != nil {
			return Source{}, err
		}
		switch {
		case spotifyHistoryFile.MatchString(base):
			var plays []any
			if json.Unmarshal(body, &plays) == nil {
				histories = append(histories, plays)
				src.Archive = ArchiveSpotify
			}
		case spotify:
			lists, _, err := readJSON(base, Decode(body))
			if err == nil {
				src.Lists = append(src.Lists, lists...)
				src.Archive = ArchiveSpotify
			}
		default:
			if l, ok := googleList(base, Decode(body)); ok {
				src.Lists = append(src.Lists, l)
				src.Archive = ArchiveGoogle
			}
		}
	}
	if len(histories) > 0 {
		if l := historyList(histories); len(l.Entries) > 0 {
			src.Lists = append(src.Lists, l)
		}
	}
	if src.Archive == "" {
		return Source{}, ErrNoDataDownload
	}
	if src.Count() == 0 {
		return src, ErrNoEntries
	}
	return src, nil
}

// readZipped reads a file of an archive, of what is left of budget.
func readZipped(f *zip.File, budget *int64) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, ErrNoDataDownload
	}
	defer r.Close()
	body, err := io.ReadAll(io.LimitReader(r, *budget+1))
	if err != nil {
		return nil, ErrNoDataDownload
	}
	*budget -= int64(len(body))
	if *budget < 0 {
		return nil, ErrArchiveTooBig
	}
	return body, nil
}

// googleList reads a CSV file of a Google Takeout: a playlist's videos,
// named after its file ("Name-videos.csv", or "Name.csv" of older
// takeouts), or the library's songs. A file without a column of videos'
// IDs is none, as the list of the playlists.
func googleList(base, text string) (List, bool) {
	t, err := readTable(strings.TrimSuffix(base, path.Ext(base)), text)
	if err != nil || !t.found() || !slices.Contains(t.Fields, FieldVideoID) {
		return List{}, false
	}
	name := strings.TrimSuffix(t.Name, "-videos")
	if t.Name == strings.TrimSuffix(base, path.Ext(base)) {
		t.Name = name
	}
	lists := t.Lists(t.Fields, t.Header)
	if len(lists) == 0 {
		return List{}, false
	}
	l := lists[0]
	for _, more := range lists[1:] {
		l.Entries = append(l.Entries, more.Entries...)
	}
	if googleLibraryFile.MatchString(base) {
		l.Name, l.Kind = "Your Library", KindLibrary
	}
	return l, true
}
