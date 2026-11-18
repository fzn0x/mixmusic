package localfiles

// Music files that are elsewhere now, as on another computer a backup is
// restored to (#592): the IDs of their tracks and songs, and the songs
// with copies of local-copies.json, move to where they are.

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// MoveID returns the ID that the track of a music file, or a song with
// copies, has once its file is where move tells, of its path as the ID
// holds it, which may be another system's: ok is false for another ID,
// as an album's, and for a path move leaves where it is.
func MoveID(id string, move func(path string) (string, bool)) (string, bool) {
	source, native, err := media.ParseID(id)
	if err != nil || source != media.SourceFile {
		return "", false
	}
	if key, ok := strings.CutPrefix(native, songPrefix); ok {
		// A song's key may end in ".<n>", which base64url never holds.
		key, n, numbered := strings.Cut(key, ".")
		to, ok := movedPath(key, move)
		if !ok {
			return "", false
		}
		moved := songID(b64.EncodeToString([]byte(to)))
		if numbered {
			moved += "." + n
		}
		return moved, true
	}
	if strings.Contains(native, ":") {
		return "", false
	}
	to, ok := movedPath(native, move)
	if !ok {
		return "", false
	}
	return TrackID(to), true
}

// movedPath returns where move tells the file of key, a path in base64,
// is now, cleaned.
func movedPath(key string, move func(string) (string, bool)) (string, bool) {
	b, err := b64.DecodeString(key)
	if err != nil || len(b) == 0 {
		return "", false
	}
	to, ok := move(string(b))
	if !ok || to == "" {
		return "", false
	}
	return filepath.Clean(to), true
}

// MoveLinks returns data, the text of local-copies.json, with the paths
// of its songs' copies where move tells they are now, and the IDs of its
// songs moved as MoveID moves them.
func MoveLinks(data []byte, move func(path string) (string, bool)) ([]byte, error) {
	var s savedLinks
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	s = mapPaths(decodePaths(s), func(p string) string {
		if to, ok := move(p); ok && to != "" {
			return filepath.Clean(to)
		}
		return p
	})
	for i, sg := range s.Songs {
		if id, ok := MoveID(sg.ID, move); ok {
			s.Songs[i].ID = id
		}
	}
	out, err := json.MarshalIndent(encodePaths(s), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
