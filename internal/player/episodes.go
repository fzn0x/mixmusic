package player

// The end of a show's episode, with autoplay on (#589): the queue ends on
// an episode, and the show's next episode that the user hasn't heard
// follows it, read on its service.

import (
	"context"
	"errors"
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// errNoNextEpisode is why a queue that ended on an episode stops: the show
// has no newer episode left to hear.
var errNoNextEpisode = errors.New("no next episode")

// playNextEpisode adds the show's next episode the user hasn't heard at
// the end of the queue, and the move m goes on to it. A show with no such
// episode stops the queue as it does today; one whose episodes can't be
// read stops it saying why.
func (p *Player) playNextEpisode(seed Item, m *move) {
	token := p.loadToken
	await(p, func(ctx context.Context) (media.Track, error) {
		next, ok, err := p.svc.NextEpisode(ctx, seed.Track)
		switch {
		case err != nil:
			return media.Track{}, err
		case !ok:
			return media.Track{}, errNoNextEpisode
		}
		return next, nil
	}, func(next media.Track, err error) {
		switch {
		case token != p.loadToken:
		case err == nil:
			p.s.Queue = append(slices.Clone(p.s.Queue), p.item(next, true))
			p.emit(QueueChange)
			p.step(m, p.s.Index+1)
		case errors.Is(err, errNoNextEpisode):
			p.stopMove(m, "")
		default:
			p.notify(Notice{Message: locales.PlayerNextEpisodeUnread(err.Error())})
			p.stopMove(m, "")
		}
	})
}
