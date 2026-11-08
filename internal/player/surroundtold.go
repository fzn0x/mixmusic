package player

// toldSurround is the 5.1 surround audio YouTube told of after a stream
// was found: the URL of the stream, and that of its 5.1 audio.
type toldSurround struct{ url, surroundURL string }

// SurroundFound tells the player that the stream at url has 5.1 surround
// audio, at surroundURL, which YouTube told of only after the stream was
// found, as its track started without waiting for it. The current entry,
// when it plays that stream, takes it, which offers its 5.1 audio, and
// plays it while 5.1 surround is on, from where it is, as turning 5.1 on
// does; an entry whose stream still loads takes it as it loads.
func (p *Player) SurroundFound(url, surroundURL string) {
	p.do(func() {
		p.told = toldSurround{url, surroundURL}
		src := p.s.Source
		if src == nil {
			return
		}
		s := p.withTold(*src)
		if s.SurroundURL == src.SurroundURL {
			return
		}
		p.s.Source = &s
		if p.loaded && p.surround && !p.stereoOnly {
			_ = p.stream.Load(p.audioOf(s), p.cue(p.s.Position, !p.paused))
		}
		p.emit(StateChange)
	})
}

// withTold returns src with the 5.1 surround audio YouTube told of last,
// when that is of src's stream, which told of none.
func (p *Player) withTold(src Source) Source {
	t := p.told
	if t.url != "" && src.Mode == ModeStream && !src.Live && src.SurroundURL == "" && src.URL == t.url {
		src.SurroundURL = t.surroundURL
	}
	return src
}
