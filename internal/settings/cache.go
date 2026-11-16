package settings

// Cache bounds the offline cache of the tracks played.
type Cache struct {
	// CacheSizeMB bounds the audio cache; 0 turns it off.
	CacheSizeMB int `json:"cacheSizeMb"`
	// MovingCoversMB bounds the clips of moving covers kept, a store of
	// their own beside the covers' (#569): 0 keeps none, and
	// UnlimitedMovingCovers keeps them all.
	MovingCoversMB int `json:"movingCoversMb"`
}

// UnlimitedMovingCovers is the MovingCoversMB that keeps every clip.
const UnlimitedMovingCovers = -1

// defaultCache is the cache of a new install.
func defaultCache() Cache {
	var c Cache
	c.CacheSizeMB = 1024
	c.MovingCoversMB = 256
	return c
}

// clean is c with its sizes in range, from 0 to 64 GB, and the clips'
// unlimited too.
func (c Cache) clean() Cache {
	c.CacheSizeMB = min(max(c.CacheSizeMB, 0), 64*1024)
	c.MovingCoversMB = min(max(c.MovingCoversMB, UnlimitedMovingCovers), 64*1024)
	return c
}
