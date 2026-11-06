package app

import "fmt"

// ControlsArtwork tells what MixMusic hands the system's media controls of
// the cover of what plays, and what they tell back of the cover they hold,
// where they tell it: for capture mode's act:nowplaying.
func (a *App) ControlsArtwork() string {
	c := &a.controls
	c.mu.Lock()
	ctl, info := c.c, c.shown()
	c.mu.Unlock()
	if ctl == nil || info == nil {
		return "the system's media controls show nothing"
	}
	handed := fmt.Sprintf("handed %q: %d bytes", info.Artwork, len(info.Cover))
	if info.CoverFile != "" {
		handed += ", in " + info.CoverFile
	}
	w, h, ok, told := ctl.Artwork()
	switch {
	case !told:
		return handed
	case !ok:
		return handed + "; they hold no cover"
	}
	return fmt.Sprintf("%s; they hold a cover of %.0fx%.0f", handed, w, h)
}
