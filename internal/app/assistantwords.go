package app

import (
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// The toasts of what an AI assistant did are whole sentences of the
// catalog, in the interface's language (#800); the tools' answers to the
// assistant, and the logs, stay English.

// namedSaid is a track as a toast names it: its title, and its artists.
func namedSaid(t media.Track) string {
	r := remoteTrackOf(t)
	if len(r.Artists) == 0 {
		return locales.AssistantsNameTitle(oneLine(r.Title))
	}
	return locales.AssistantsNameTrackBy(oneLine(r.Title), oneLine(strings.Join(r.Artists, ", ")))
}

// foundSaid names what a tool told of, of the ID id, as a toast does.
func foundSaid(id string, f assistantFound) string {
	switch f.Kind {
	case kindSong, kindVideo:
		return namedSaid(f.Track)
	case kindAlbum:
		return locales.AssistantsNameAlbum(oneLine(f.Title))
	case kindArtist:
		return locales.AssistantsNameArtistSongs(oneLine(f.Title))
	}
	if id == likedID {
		return locales.AssistantsNameLiked()
	}
	return locales.AssistantsNameTitle(oneLine(f.Title))
}
