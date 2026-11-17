package player

import "sync/atomic"

// errorWords says the errors the notices tell, as the windows do in their
// language (#800). The logs, the state's Error and a failure's details
// keep the errors' English.
type errorWords struct {
	words atomic.Pointer[func(error) string]
}

// say is err in the words set, or as Go says it.
func (w *errorWords) say(err error) string {
	if fn := w.words.Load(); fn != nil && *fn != nil {
		return (*fn)(err)
	}
	return err.Error()
}

// SetErrorWords has the player say the errors of its notices with words,
// as the windows do in their language; nil says them as Go does.
func (p *Player) SetErrorWords(words func(error) string) { p.errorWords.words.Store(&words) }
