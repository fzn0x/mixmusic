package app

// Control from outside the window (#590): a link, as mixmusic://next, has
// the MixMusic that runs do a command of the player, as the media keys and
// the global shortcuts do, and brings no window forward. So a Stream Deck
// key, a keyboard macro or a script can pause, skip or turn shuffle on.

import "net/url"

// namedCommands are the commands of the links, by name:
// mixmusic://<name>. Like and dislike aren't among them: any web page can
// open a link, once the browser asks, and a like changes the account.
var namedCommands = map[string]PlayerCommand{
	"toggle":      {Command: CommandToggle},
	"play":        {Command: CommandPlay},
	"pause":       {Command: CommandPause},
	"next":        {Command: CommandNext},
	"previous":    {Command: CommandPrevious},
	"mute":        {Command: CommandMute},
	"shuffle":     {Command: CommandShuffle},
	"repeat":      {Command: CommandRepeat},
	"volume-up":   {Command: CommandVolumeBy, Value: globalVolumeStep},
	"volume-down": {Command: CommandVolumeBy, Value: -globalVolumeStep},
}

// controlOf returns the command of a link, as mixmusic://next: its name
// alone, with a slash after it at most. mixmusic://play/<track id> plays a
// track, which isn't one.
func controlOf(u *url.URL) (PlayerCommand, bool) {
	if u.Scheme != URLScheme || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return PlayerCommand{}, false
	}
	c, ok := namedCommands[u.Host]
	return c, ok
}

// followControl does the command of u, a link HandleURL was given, if it
// is one, and reports whether it was: it shows no window.
func (a *App) followControl(u *url.URL) bool {
	c, ok := controlOf(u)
	if ok {
		a.command(c)
	}
	return ok
}
