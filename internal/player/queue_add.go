package player

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// AddToQueue is the window's Add to queue (#618): tracks go at the end of
// the queue, before the radio's, as Enqueue puts them, and when no track is
// current, nor one loading, the first of them the queue doesn't pass over
// plays too, as PlayNext starts playback. The plugins and the AI
// assistants Enqueue, which plays nothing.
func (p *Player) AddToQueue(tracks []media.Track) {
	if len(tracks) == 0 {
		return
	}
	tracks = slices.Clone(tracks)
	p.do(func() {
		at := p.enqueue(p.chosenItems(tracks))
		if _, ok := p.s.Current(); ok || p.s.Loading {
			return
		}
		for i := at; i < at+len(tracks); i++ {
			if p.s.Queue[i].Skip == "" {
				p.playAt(i, 0, true)
				return
			}
		}
	})
}
