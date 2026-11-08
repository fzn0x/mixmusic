package app

// PlayCounts returns how many times each track was played in MixMusic, by
// its ID, as the history counts them, for lists sorted by plays.
func (c *Collection) PlayCounts() map[string]int { return c.a.library.PlayCounts() }
