package player

// SetRepeat repeats as r tells: the queue, the track, or nothing, as a
// computer that controls this one asks (remote control). A queue that
// repeats has no radio after it (radioahead.go).
func (p *Player) SetRepeat(r Repeat) {
	p.do(func() {
		if r != RepeatAll && r != RepeatOne {
			r = RepeatOff
		}
		if p.s.Repeat == r {
			return
		}
		p.s.Repeat = r
		if r != RepeatOff {
			p.dropRadioAhead()
		}
		p.report()
		p.emit(StateChange)
	})
}

// SetMuted mutes, or unmutes, as a computer that controls this one asks.
func (p *Player) SetMuted(muted bool) {
	p.do(func() {
		if p.s.Muted == muted {
			return
		}
		p.s.Muted = muted
		p.apply()
	})
}
