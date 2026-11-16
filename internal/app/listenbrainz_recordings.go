package app

import (
	"cmp"
	"encoding/json"
	"log"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/scrobble"
	"github.com/fzn0x/mixmusic/internal/store"
)

// maxRecordings is how many recordings' IDs the book keeps at most, the
// oldest dropped first.
const maxRecordings = 5000

// recordingBook keeps the MessyBrainz IDs that ListenBrainz gave the
// recordings of the tracks MixMusic announced as playing, by the tracks'
// IDs, in a file of the data folder: a like of one sends its love by it.
// Its methods are safe for concurrent use.
type recordingBook struct {
	file string
	mu   sync.Mutex
	ids  map[string]keptRecording
}

// keptRecording is the ID of a track's recording, and when it was kept.
type keptRecording struct {
	MSID string `json:"msid"`
	At   int64  `json:"at"`
}

// openRecordings reads the book kept in file, empty when there is none.
func openRecordings(file string) *recordingBook {
	b := &recordingBook{file: file, ids: map[string]keptRecording{}}
	if data, err := os.ReadFile(file); err == nil {
		json.Unmarshal(data, &b.ids)
	}
	return b
}

// keep takes in the MessyBrainz ID of the recording of t, and saves the
// book.
func (b *recordingBook) keep(t scrobble.Track, msid string) {
	if t.ID == "" || msid == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ids[t.ID].MSID == msid {
		return
	}
	b.ids[t.ID] = keptRecording{MSID: msid, At: time.Now().Unix()}
	if over := len(b.ids) - maxRecordings; over > 0 {
		ids := slices.SortedFunc(maps.Keys(b.ids), func(x, y string) int { return cmp.Compare(b.ids[x].At, b.ids[y].At) })
		for _, id := range ids[:over] {
			delete(b.ids, id)
		}
	}
	data, err := json.Marshal(b.ids)
	if err == nil {
		err = store.WriteFile(b.file, data)
	}
	if err != nil {
		log.Printf("mixmusic: keeping the recordings of ListenBrainz: %v", err)
	}
}

// msid returns the MessyBrainz ID of the recording of the track of id, ""
// when the book has none.
func (b *recordingBook) msid(id string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ids[id].MSID
}
