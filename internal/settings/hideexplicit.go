package settings

// PrefersClean reports whether an album that comes explicit and clean
// shows and opens its clean version: as Prefer clean versions asks, and
// always while Hide explicit content is on, which leaves CleanVersions as
// the user set it for when it's off again.
func (p Playback) PrefersClean() bool { return p.CleanVersions || p.HideExplicit }
