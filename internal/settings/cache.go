package settings

// Cache bounds the offline cache of the tracks played.
type Cache struct {
	// CacheSizeMB bounds the audio cache; 0 turns it off.
	CacheSizeMB int `json:"cacheSizeMb"`
}

// defaultCache is the cache of a new install.
func defaultCache() Cache {
	var c Cache
	c.CacheSizeMB = 1024
	return c
}

// clean is c with its size in range, from 0 to 64 GB.
func (c Cache) clean() Cache {
	c.CacheSizeMB = min(max(c.CacheSizeMB, 0), 64*1024)
	return c
}
