package playlistio

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ownApp is what the "app" of MixMusic's own JSON files says.
const ownApp = "MixMusic"

// heard is how long a streaming history must have played a song for the
// play to count.
const heard = 30 * time.Second

// readJSON reads JSON: MixMusic's own export, the files of Spotify's data
// download, or an array of objects of a song each, whose keys name what
// they hold as the columns of a table do.
func readJSON(name, text string) ([]List, []*Table, error) {
	var v any
	d := json.NewDecoder(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, nil, ErrNotAList
	}
	switch v := v.(type) {
	case map[string]any:
		if str(v["app"]) == ownApp {
			return ownLists(v["tracks"]), nil, nil
		}
		if ps, ok := v["playlists"].([]any); ok {
			return spotifyPlaylists(ps), nil, nil
		}
		if ts, ok := v["tracks"].([]any); ok && len(ts) > 0 && hasKey(ts[0], "uri") {
			return spotifyLibrary(ts), nil, nil
		}
		for _, key := range []string{"tracks", "songs", "items"} {
			if arr, ok := v[key].([]any); ok {
				return objects(name, arr)
			}
		}
	case []any:
		if len(v) > 0 && (hasKey(v[0], "msPlayed") || hasKey(v[0], "ms_played")) {
			if l := historyList([][]any{v}); len(l.Entries) > 0 {
				return []List{l}, nil, nil
			}
			return nil, nil, ErrNoEntries
		}
		return objects(name, v)
	}
	return nil, nil, ErrNotAList
}

func hasKey(v any, key string) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, ok = m[key]
	return ok
}

// str is a JSON value as text: a string, or a number.
func str(v any) string {
	switch v := v.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// ownLists reads the tracks of MixMusic's own JSON (WriteJSON).
func ownLists(v any) []List {
	arr, _ := v.([]any)
	var lists []List
	index := map[string]int{}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		e := Entry{Title: str(m["title"]), Album: str(m["album"]), ISRC: isrc(str(m["isrc"])), Service: str(m["service"]),
			URL: str(m["url"])}
		switch a := m["artists"].(type) {
		case []any:
			for _, x := range a {
				if s := str(x); s != "" {
					e.Artists = append(e.Artists, s)
				}
			}
		case string:
			e.Artists = splitArtists(a, true)
		}
		if ms, err := strconv.ParseInt(str(m["durationMs"]), 10, 64); err == nil && ms > 0 {
			e.Duration = time.Duration(ms) * time.Millisecond
		}
		if id := str(m["mixmusicId"]); ownID(id) {
			e.MixMusicID = id
		}
		if strings.HasPrefix(strings.ToLower(e.URL), "file:") {
			e.Path = e.URL
		} else {
			e.VideoID, e.SpotifyID = link(e.URL, false)
		}
		playlist := str(m["playlist"])
		j, ok := index[playlist]
		if !ok {
			j = len(lists)
			index[playlist] = j
			lists = append(lists, List{Name: playlist, Own: true})
		}
		lists[j].Entries = append(lists[j].Entries, e)
	}
	return lists
}

// spotifyPlaylists reads the playlists of Spotify's data download
// (Playlist1.json): their tracks, and the files of the user's own of
// them, whose URIs name their artist, album, title and length.
func spotifyPlaylists(ps []any) []List {
	var out []List
	for _, p := range ps {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		l := List{Name: str(m["name"])}
		items, _ := m["items"].([]any)
		for _, it := range items {
			im, ok := it.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := im["track"].(map[string]any); ok {
				e := Entry{Title: str(t["trackName"]), Album: str(t["albumName"])}
				if a := str(t["artistName"]); a != "" {
					e.Artists = []string{a}
				}
				_, e.SpotifyID = link(str(t["trackUri"]), true)
				l.Entries = append(l.Entries, e)
			} else if t, ok := im["localTrack"].(map[string]any); ok {
				if e, ok := spotifyLocal(str(t["uri"])); ok {
					l.Entries = append(l.Entries, e)
				}
			}
		}
		if len(l.Entries) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// spotifyLocal reads the URI Spotify gives a file of the user's own:
// spotify:local:artist:album:title:seconds, of words joined by +.
func spotifyLocal(uri string) (Entry, bool) {
	parts := strings.Split(strings.TrimPrefix(uri, "spotify:local:"), ":")
	if !strings.HasPrefix(uri, "spotify:local:") || len(parts) < 3 {
		return Entry{}, false
	}
	unescape := func(s string) string {
		u, err := url.QueryUnescape(s)
		if err != nil {
			return s
		}
		return strings.TrimSpace(u)
	}
	e := Entry{Title: unescape(parts[2]), Album: unescape(parts[1])}
	if a := unescape(parts[0]); a != "" {
		e.Artists = []string{a}
	}
	if len(parts) > 3 {
		if n, err := strconv.Atoi(parts[3]); err == nil && n > 0 {
			e.Duration = time.Duration(n) * time.Second
		}
	}
	return e, e.Title != ""
}

// spotifyLibrary reads the songs of Your Library in Spotify's data
// download (YourLibrary.json).
func spotifyLibrary(ts []any) []List {
	l := List{Name: "Your Library", Kind: KindLibrary}
	for _, t := range ts {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		e := Entry{Title: str(m["track"]), Album: str(m["album"])}
		if a := str(m["artist"]); a != "" {
			e.Artists = []string{a}
		}
		_, e.SpotifyID = link(str(m["uri"]), true)
		if e.Title != "" || e.SpotifyID != "" {
			l.Entries = append(l.Entries, e)
		}
	}
	if len(l.Entries) == 0 {
		return nil
	}
	return []List{l}
}

// historyList is the songs that the streaming histories of Spotify's data
// download played for 30 seconds or more, the most played first, then in
// the order they first played: of the account's history
// (StreamingHistory_music_0.json), or its extended one
// (Streaming_History_Audio_2024_0.json).
func historyList(files [][]any) List {
	l := List{Name: "Streaming history", Kind: KindHistory}
	index := map[string]int{}
	for _, plays := range files {
		for _, p := range plays {
			m, ok := p.(map[string]any)
			if !ok {
				continue
			}
			ms, _ := strconv.ParseInt(cmp.Or(str(m["msPlayed"]), str(m["ms_played"])), 10, 64)
			if time.Duration(ms)*time.Millisecond < heard {
				continue
			}
			e := Entry{Title: cmp.Or(str(m["trackName"]), str(m["master_metadata_track_name"])),
				Album: str(m["master_metadata_album_album_name"])}
			if a := cmp.Or(str(m["artistName"]), str(m["master_metadata_album_artist_name"])); a != "" {
				e.Artists = []string{a}
			}
			_, e.SpotifyID = link(str(m["spotify_track_uri"]), true)
			if e.Title == "" {
				continue // an episode's, or a video's
			}
			key := e.SpotifyID
			if key == "" {
				key = strings.ToLower(fmt.Sprint(e.Artists, "\x00", e.Title))
			}
			i, ok := index[key]
			if !ok {
				i = len(l.Entries)
				index[key] = i
				l.Entries = append(l.Entries, e)
			}
			l.Entries[i].Plays++
			if l.Entries[i].SpotifyID == "" {
				l.Entries[i].SpotifyID = e.SpotifyID
			}
		}
	}
	slices.SortStableFunc(l.Entries, func(a, b Entry) int { return cmp.Compare(b.Plays, a.Plays) })
	return l
}

// objects reads an array of objects of a song each as a table, whose
// columns are their keys, in the order they first come.
func objects(name string, arr []any) ([]List, []*Table, error) {
	var keys []string
	seen := map[string]bool{}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, nil, ErrNotAList
		}
		// A map's keys come in no order: they're sorted, which keeps
		// tables of the same file alike.
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		slices.Sort(ks)
		for _, k := range ks {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	if len(keys) == 0 {
		return nil, nil, ErrNoEntries
	}
	rows := [][]string{keys}
	for _, item := range arr {
		m := item.(map[string]any)
		row := make([]string, len(keys))
		for i, k := range keys {
			switch v := m[k].(type) {
			case []any:
				var parts []string
				for _, x := range v {
					if s := str(x); s != "" {
						parts = append(parts, s)
					}
				}
				row[i] = strings.Join(parts, "; ")
			default:
				row[i] = str(v)
			}
		}
		rows = append(rows, row)
	}
	t := &Table{Name: name, Rows: rows, Header: -1}
	if fields, millis := fieldsOf(keys); slices.ContainsFunc(fields, func(f Field) bool {
		return f == FieldTitle || f == FieldVideoID || f == FieldURL || f == FieldSpotifyURI
	}) {
		t.Header, t.Fields, t.millis = 0, fields, millis
	}
	lists, tables := t.listsOrSelf()
	return lists, tables, nil
}
