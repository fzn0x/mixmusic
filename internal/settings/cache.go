package settings

import (
	"path/filepath"
	"strings"
)

// UnlimitedCache is the CacheSizeMB that keeps every track played.
const UnlimitedCache = -1

// MaxCacheSizeMB bounds finite custom audio cache sizes (1 TB).
const MaxCacheSizeMB = 1024 * 1024

// UnlimitedMovingCovers is the MovingCoversMB that keeps every clip.
const UnlimitedMovingCovers = -1

// Cache bounds the offline cache of the tracks played and where it lives.
type Cache struct {
	// CacheSizeMB bounds the audio cache; 0 turns it off, and
	// UnlimitedCache keeps them all.
	CacheSizeMB int `json:"cacheSizeMb"`
	// CacheFolder is where the audio cache is kept, an absolute path;
	// "" is the default folder of the system.
	CacheFolder string `json:"cacheFolder"`
	// CacheWithDownloads keeps cached audio in the downloads folder's
	// audio folder alongside downloads.
	CacheWithDownloads bool `json:"cacheWithDownloads"`
	// MovingCoversMB bounds the clips of moving covers kept, a store of
	// their own beside the covers' (#569): 0 keeps none, and
	// UnlimitedMovingCovers keeps them all.
	MovingCoversMB int `json:"movingCoversMb"`
}

// defaultCache is the cache of a new install.
func defaultCache() Cache {
	var c Cache
	c.CacheSizeMB = 1024
	c.MovingCoversMB = 256
	return c
}

// clean is c with its sizes in range, from UnlimitedCache to
// MaxCacheSizeMB, and its folder cleaned, or "" in place of a relative
// one, which would follow the working directory.
func (c Cache) clean() Cache {
	c.CacheSizeMB = min(max(c.CacheSizeMB, UnlimitedCache), MaxCacheSizeMB)
	if c.CacheFolder = strings.TrimSpace(c.CacheFolder); c.CacheFolder != "" {
		if filepath.IsAbs(c.CacheFolder) {
			c.CacheFolder = filepath.Clean(c.CacheFolder)
		} else {
			c.CacheFolder = ""
		}
	}
	c.MovingCoversMB = min(max(c.MovingCoversMB, UnlimitedMovingCovers), 64*1024)
	return c
}
