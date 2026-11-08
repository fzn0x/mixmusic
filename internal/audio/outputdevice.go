package audio

// ReopenOutput stops the output that plays and starts it again, as when
// the device it should play on changed, which the engine's OutputStarter
// picks: the track plays on from where it was, on the new device. An output
// that doesn't play starts on the new device when it next plays. One that
// fails to start fails as one that stops of itself does.
func (e *Engine) ReopenOutput() { e.restartOutput(true) }
