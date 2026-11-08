package app

import "testing"

// TestSurroundSettingTellsTheDirectRoute has the direct route ask YouTube's
// TV app of 5.1 audio only while "Play 5.1 surround when a track has it" is
// on: off by default, then on and off again by edits of settings.json, as
// Settings applies them too.
func TestSurroundSettingTellsTheDirectRoute(t *testing.T) {
	a := fileApp(t, `{}`)
	d := a.resolver.Direct
	if d == nil {
		t.Skip("yt-dlp alone finds the streams (MIXMUSIC_YTDLP_ONLY)")
	}
	if d.Surround() {
		t.Fatal("a new install's direct route asks of 5.1 audio")
	}
	writeSettings(t, a.settings.Path(), `{"surround": true}`)
	a.ReloadSettings()
	if !d.Surround() {
		t.Error("5.1 on, the direct route doesn't ask of it")
	}
	writeSettings(t, a.settings.Path(), `{"surround": false}`)
	a.ReloadSettings()
	if d.Surround() {
		t.Error("5.1 off, the direct route asks of it")
	}
}
