package settings

import (
	"cmp"
	"regexp"
	"strings"
)

// DefaultDiscordAppID is MixMusic's own Discord application, whose name
// Discord shows beside what plays when the user set no other.
const DefaultDiscordAppID = "1562074343691788378"

// discordIDRe matches a Discord application ID, a number.
var discordIDRe = regexp.MustCompile(`^[0-9]{15,22}$`)

// Desktop is how MixMusic fits in the desktop: Settings › Desktop.
type Desktop struct {
	// CloseToTray keeps the music playing as the window closes, from the
	// tray icon. It is off at first.
	CloseToTray bool `json:"closeToTray"`
	// Notifications tell what starts playing while MixMusic is in the
	// background.
	Notifications bool `json:"notifications"`
	// ShowInTitle names what plays in the main window's title.
	ShowInTitle bool `json:"showInTitle"`
	// KeepOnTop keeps the main window above the other windows (#350). It
	// is off at first; the mini player stays on top either way.
	KeepOnTop bool `json:"keepOnTop"`
	// MiniPanel is the panel the mini player shows beside it as it opens
	// (#371): MiniQueue, MiniLyrics or MiniOutput, or "" for none.
	MiniPanel string `json:"miniPanel"`
	// MiniZoom is the zoom the mini player's window was last sized for:
	// the size the window keeps of its last run suits it, and the mini
	// player scales it to the zoom as it opens (#717). 0 before it opened.
	MiniZoom float64 `json:"miniZoom"`
	// MediaKeys has the keyboard's play, next and previous keys control
	// MixMusic.
	MediaKeys bool `json:"mediaKeys"`
	// AssistantControl lets the AI assistants of this computer control
	// MixMusic over MCP (#477), through the command they start. It is off
	// at first.
	AssistantControl bool `json:"assistantControl"`
	// RoomName is the user's name in the rooms of Listen with friends
	// (#561), which the host and the other guests show, of at most 50
	// characters: "" is the computer's name.
	RoomName string `json:"roomName"`
	// RoomSongs is what becomes of the songs the guests of the user's room
	// add (#734): RoomSongsAdd, RoomSongsAsk, as at first, or RoomSongsOff.
	RoomSongs string `json:"roomSongs"`
	// RoomGuestControl lets the guests of the user's room play, pause and
	// change its tracks. It is off at first.
	RoomGuestControl bool `json:"roomGuestControl"`
	// RoomAskJoin has the user let in each computer that joins their room.
	// It is off at first.
	RoomAskJoin bool `json:"roomAskJoin"`
	// RoomFollowVolume has a guest of a room play at the host's volume,
	// rather than its own. It is off at first.
	RoomFollowVolume bool `json:"roomFollowVolume"`
	// RoomAnywhere lets friends outside this network join the user's room
	// (#562): MixMusic asks the router to forward the room's port, and the
	// invite link carries where they reach it. It is off at first.
	RoomAnywhere bool `json:"roomAnywhere"`
	// RoomPort is the TCP port rooms listen on, kept so that a forward
	// made by hand keeps working: picked at random from 1024 to 65535 as
	// the first room opens, 0 before.
	RoomPort int `json:"roomPort"`
	// RoomAddress is the address invites carry in place of those MixMusic
	// finds, as a VPN's both sides run or a forward made by hand: a host,
	// with a port or not. "" has MixMusic find them.
	RoomAddress string `json:"roomAddress"`
	// DiscordPresence shows what plays on the user's Discord profile, as
	// the Discord application of DiscordAppID, or MixMusic's own when it
	// is empty.
	DiscordPresence bool `json:"showOnDiscord"`
	// DiscordAppID is the ID of the user's own Discord application, which
	// DiscordPresence shows as; "" is MixMusic's.
	DiscordAppID string `json:"discordAppId"`
	// DiscordFirstLine is the first line of the Discord status, a template
	// whose placeholders, as {title}, name the track's (#352). Empty, the
	// line shows nothing.
	DiscordFirstLine string `json:"discordFirstLine"`
	// DiscordSecondLine is the status's second line, a template as
	// DiscordFirstLine is.
	DiscordSecondLine string `json:"discordSecondLine"`
	// DiscordCover shows the track's cover in the status, where its
	// service has it online.
	DiscordCover bool `json:"discordCover"`
	// DiscordServiceBadge shows a badge of the track's service, YouTube
	// Music, Spotify or this computer, at the cover's corner. It is off
	// at first.
	DiscordServiceBadge bool `json:"discordServiceBadge"`
	// DiscordTimeLeft shows the time played and the time left in the
	// status.
	DiscordTimeLeft bool `json:"discordTimeLeft"`
	// DiscordListenButton shows a button in the status that opens the
	// track on its service. It is off at first.
	DiscordListenButton bool `json:"discordListenButton"`
	// DiscordKeepWhilePaused keeps the status while the track is paused,
	// marked as paused and without its times, where pausing clears it at
	// first.
	DiscordKeepWhilePaused bool `json:"discordKeepWhilePaused"`
	// NotchPlayer shows what plays around the camera housing of a
	// MacBook's screen, and opens a player there (#46). It is on at
	// first, and does nothing on other screens and systems.
	NotchPlayer bool `json:"notchPlayer"`
	// BackupEvery is how often MixMusic backs itself up (#592):
	// BackupOff, as at first, BackupDaily, BackupWeekly, BackupMonthly or
	// BackupAtQuit.
	BackupEvery string `json:"backupEvery"`
	// BackupFolder is the folder the automatic backups go to, an absolute
	// path: "" is "MixMusic backups" in the Documents folder, but in the
	// Flatpak, which asks for one.
	BackupFolder string `json:"backupFolder"`
	// BackupKeep is how many automatic backups the folder keeps: one of
	// BackupKeeps, 5 at first, all of them for 0, and one file written
	// over for 1.
	BackupKeep int `json:"backupKeep"`
}

// defaultDesktop is how a new install fits in the desktop.
func defaultDesktop() Desktop {
	var d Desktop
	d.Notifications = true
	d.ShowInTitle = true
	d.MediaKeys = true
	d.RoomSongs = RoomSongsAsk
	d.DiscordPresence = true
	d.DiscordFirstLine, d.DiscordSecondLine = DefaultDiscordFirstLine, DefaultDiscordSecondLine
	d.DiscordCover, d.DiscordTimeLeft = true, true
	d.NotchPlayer = true
	d.BackupKeep = 5
	return d
}

// clean is d without a Discord application ID that isn't one, with lines of
// the Discord status that Discord takes, and with a panel of the mini
// player it has, and a zoom of it that is one.
func (d Desktop) clean() Desktop {
	d.MiniPanel = cleanMiniPanel(d.MiniPanel)
	d.MiniZoom = zoomStep(d.MiniZoom, 0)
	d.RoomPort, d.RoomAddress = cleanRoomPort(d.RoomPort), cleanRoomAddress(d.RoomAddress)
	d.RoomName = cleanRoomName(d.RoomName)
	d.RoomSongs = cleanRoomSongs(d.RoomSongs)
	if d.DiscordAppID = strings.TrimSpace(d.DiscordAppID); !discordIDRe.MatchString(d.DiscordAppID) {
		d.DiscordAppID = ""
	}
	d.DiscordFirstLine, d.DiscordSecondLine = cleanDiscordLine(d.DiscordFirstLine), cleanDiscordLine(d.DiscordSecondLine)
	d.BackupEvery, d.BackupFolder, d.BackupKeep = cleanBackups(d.BackupEvery, d.BackupFolder, d.BackupKeep)
	return d
}

// DiscordApp returns the ID of the Discord application that shows what
// plays: the user's, or MixMusic's own.
func (d Desktop) DiscordApp() string { return cmp.Or(d.DiscordAppID, DefaultDiscordAppID) }
