package app

// The system's media controls show the covers of the artwork store, which
// the windows show too: each cover downloads once, and shows offline once
// kept.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/nowplaying"
	"github.com/fzn0x/mixmusic/internal/store"
)

const (
	// coverWait bounds the wait for a cover.
	coverWait = 30 * time.Second
	// coverRetry is how long the controls go without a cover that didn't
	// load before they ask for it again.
	coverRetry = time.Minute
)

// controlsCovers hands the system's controls the cover of the track that
// shows, from the artwork store, which it reads beside them.
type controlsCovers struct {
	// load returns the image at an address, as the artwork store does.
	load func(ctx context.Context, rawURL string) ([]byte, error)
	// dir is the folder of the covers' files, for controls that take a
	// file; "" for those that take the bytes.
	dir string
	// changed is called once a cover asked for loaded, or didn't.
	changed func()
	now     func() time.Time

	mu sync.Mutex
	// url is the address of the cover kept: data, in file, or neither
	// when it didn't load at failed.
	url    string
	data   []byte
	file   string
	failed time.Time
	// want is the address asked for last, and loading those that load.
	want    string
	loading map[string]bool
	// told is set once a file didn't write.
	told bool
	// writing is held while files are written and removed.
	writing sync.Mutex
}

// newControlsCovers returns the covers of the artwork store, which call
// changed once one loaded.
func (a *App) newControlsCovers(changed func()) *controlsCovers {
	cc := &controlsCovers{load: a.artStore.Bytes, changed: changed, now: time.Now}
	if nowplaying.CoverFiles {
		// Desktops take the file's address whole.
		dir := filepath.Join(a.opts.CacheDir, "now-playing")
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		cc.dir = dir
	}
	return cc
}

// get returns the cover at rawURL, and its file, once kept. Until then it
// returns nothing, and loads it beside the caller.
func (cc *controlsCovers) get(rawURL string) (data []byte, file string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.want = rawURL
	if rawURL == "" {
		return nil, ""
	}
	if cc.url == rawURL && (cc.data != nil || cc.now().Sub(cc.failed) < coverRetry) {
		return cc.data, cc.file
	}
	if !cc.loading[rawURL] {
		if cc.loading == nil {
			cc.loading = map[string]bool{}
		}
		cc.loading[rawURL] = true
		go cc.fetch(rawURL)
	}
	return nil, ""
}

// fetch reads the cover at rawURL from the store, which downloads it unless
// it has it, and keeps it if it's still the one asked for.
func (cc *controlsCovers) fetch(rawURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), coverWait)
	data, err := cc.load(ctx, rawURL)
	cancel()
	if err != nil || len(data) == 0 {
		data = nil // the store logs why
	}
	if cc.keep(rawURL, data) {
		cc.changed()
	}
}

// keep keeps data, the cover at rawURL, or that it didn't load, if it's
// still the one asked for, and reports whether it was. Where the controls
// take a file, it writes one, and removes the others, but the kept cover's.
func (cc *controlsCovers) keep(rawURL string, data []byte) bool {
	// One fetch at a time writes and removes files.
	cc.writing.Lock()
	defer cc.writing.Unlock()
	cc.mu.Lock()
	wanted := cc.want == rawURL
	cc.mu.Unlock()
	file := ""
	if data != nil && wanted && cc.dir != "" {
		var err error
		if file, err = cc.write(rawURL, data); err != nil {
			cc.mu.Lock()
			told := cc.told
			cc.told = true
			cc.mu.Unlock()
			if !told {
				log.Printf("mixmusic: the system's media controls: keeping their cover in %s: %v", cc.dir, err)
			}
		}
	}
	cc.mu.Lock()
	delete(cc.loading, rawURL)
	if wanted = cc.want == rawURL; wanted {
		cc.url, cc.data, cc.file, cc.failed = rawURL, data, file, time.Time{}
		if data == nil {
			cc.failed = cc.now()
		}
	}
	kept := cc.file
	cc.mu.Unlock()
	if cc.dir != "" {
		entries, _ := os.ReadDir(cc.dir)
		for _, e := range entries {
			if path := filepath.Join(cc.dir, e.Name()); path != kept {
				os.Remove(path)
			}
		}
	}
	return wanted
}

// write keeps data, the cover at rawURL, in a file of cc.dir, whose path
// it returns.
func (cc *controlsCovers) write(rawURL string, data []byte) (string, error) {
	sum := sha256.Sum256([]byte(rawURL))
	path := filepath.Join(cc.dir, hex.EncodeToString(sum[:8])+coverExt(data))
	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(data)) {
		return path, nil
	}
	if err := store.WriteFile(path, data); err != nil {
		return "", err
	}
	return path, nil
}

// coverExt is the extension of a cover's file: desktops may go by it.
func coverExt(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ""
}
