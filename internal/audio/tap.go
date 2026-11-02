package audio

// Tap analyses audio that plays outside the engine, as Spotify's when it
// plays in MixMusic's own page, which a loopback capture hears: through the
// engine's analyser, so that what draws it moves alike.
type Tap struct{ an *analyser }

// NewTap returns a tap that has heard nothing yet.
func NewTap() *Tap { return &Tap{an: newAnalyser()} }

// Write adds interleaved stereo frames to what the tap heard.
func (t *Tap) Write(frames []float32) { t.an.writeInput(frames, 2) }

// Silence makes the tap hear silence from now on: its spectrum then falls
// to nothing, as the engine's does while nothing plays.
func (t *Tap) Silence() { t.an.silence() }

// Analyse writes what the tap's analyser gives now into out.
func (t *Tap) Analyse(out *Analysis) { t.an.analyse(out) }
