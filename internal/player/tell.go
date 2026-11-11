package player

// Tell has the windows show n, as they show the player's own notices,
// after what was asked of the player before it: what an AI assistant had
// it do (#477).
func (p *Player) Tell(n Notice) { p.do(func() { p.notify(n) }) }
