package app

// Control from outside the window (#590): a link, as mixmusic://next, or a
// flag given to MixMusic started again, as --next, has the MixMusic that
// runs do a command of the player, as the media keys and the global
// shortcuts do, and brings no window forward. So a Stream Deck key, a
// keyboard macro or a script can pause, skip or turn shuffle on.

import (
	"net/url"
	"strings"
)

// namedCommands are the commands of the links and the flags, by name:
// mixmusic://<name>, --<name>. Like and dislike aren't among them: any web
// page can open a link, once the browser asks, and a like changes the
// account.
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

// controlFlag returns the command of a flag of the command line, as
// --next.
func controlFlag(arg string) (PlayerCommand, bool) {
	name, ok := strings.CutPrefix(arg, "--")
	if !ok {
		return PlayerCommand{}, false
	}
	c, ok := namedCommands[name]
	return c, ok
}

// HasControlFlag reports whether args, a command line, give the flag of a
// command, as --next: main hands them over to the MixMusic that runs, and
// starts none.
func HasControlFlag(args []string) bool {
	for _, arg := range args {
		if _, ok := controlFlag(arg); ok {
			return true
		}
	}
	return false
}

// controlLink reports whether arg, an argument of the command line, is
// the link of a command.
func controlLink(arg string) bool {
	u, err := url.Parse(arg)
	if err != nil {
		return false
	}
	_, ok := controlOf(u)
	return ok
}

// controlled does the commands of the flags among args, those of a second
// start of MixMusic, and reports whether args asked for any, with a flag
// or a link, whose command HandleURL does as MyGo hands it the link. The
// main window doesn't come forward then: a key's press pulls none in
// front.
func (a *App) controlled(args []string) bool {
	asked := false
	for _, arg := range args {
		if c, ok := controlFlag(arg); ok {
			a.command(c)
			asked = true
		} else if controlLink(arg) {
			asked = true
		}
	}
	return asked
}
