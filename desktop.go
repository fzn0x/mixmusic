package main

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

func (s *shell) desktopSettings(c *ui.Context) {
	s.group(c, "on the desktop", func() {
		toggle := func(key, title, description string, v *bool, words ...string) {
			s.row(c, title, description, func() {
				if s.switchOf(c.Key(key), v).Label(title).Changed() {
					s.save()
				}
			}, words...)
		}
		toggle("tray", "Close to the tray", "Closing the window keeps the music playing; quit from the tray icon.", &s.set.CloseToTray, "minimize background")
		toggle("media-keys", "Media keys", "The keyboard's play, next and previous keys control MixMusic.", &s.set.MediaKeys)
		toggle("notifications", "Notifications", "Tell what starts playing while MixMusic is in the background.", &s.set.Notifications)
		toggle("title", "Show what plays in the window title", "The window's title names the track that plays, for the taskbar, the window switcher and screen readers.", &s.set.ShowInTitle)
		toggle("discord", "Show what plays on Discord", "While Discord runs, your profile says what you listen to, with the artwork and the time left; pausing clears it.", &s.set.DiscordPresence)
		if s.set.DiscordPresence {
			describe := func() {
				ui.Text(c, "Discord shows the name of MixMusic's application. To show another, create one in the ")
				s.linkOf(c, "developer portal", "https://discord.com/developers/applications")
				ui.Text(c, " and paste its application ID.")
			}
			s.rowWith(c, "Discord application ID", describe, func() {
				in, _ := s.settingInput(c, "discord-app", s.set.DiscordAppID, func(id string) {
					s.set.DiscordAppID = id
					s.save()
				})
				in.Placeholder("MixMusic's").Width(260).Label("Discord application ID")
			}, "Discord shows the name of MixMusic's application. To show another, create one in the developer portal and paste its application ID.")
		}
		s.row(c, "Start at login", "", func() {
			// The system says, beside the main thread.
			login := fetch(s, "openAtLogin", func(context.Context) (bool, error) { return s.svc.Preferences.OpenAtLogin(), nil })
			on := login.value
			if s.switchOf(c.Key("login"), &on).Label("Start at login").Disabled(!login.done).Changed() {
				login.value = on
				go func() {
					err := s.svc.Preferences.SetOpenAtLogin(on)
					s.update(func() {
						if err != nil {
							s.notes = append(s.notes, noticeOf(err))
						}
						s.forget("openAtLogin")
					})
				}()
			}
		}, "startup launch")
		s.row(c, "Settings file", "Your settings, as settings.json in your data folder. Save it while MixMusic runs, and they apply at once.", func() {
			if s.button(c, "settings-file-open", "open", false).Clicked() {
				s.openSettingsFile()
			}
			if s.button(c, "settings-file-show", "show in folder", false).Clicked() {
				s.showSettingsFile()
			}
		}, "settings.json json config")
		s.row(c, "Reset settings", "Puts back the settings of a new install; your accounts, library and add-ons stay.", func() {
			if s.button(c, "reset", "reset", false).Clicked() {
				s.resetting = true
			}
		}, "defaults")
	})
	if s.confirm(c, "reset-settings", &s.resetting, "reset settings", "Put back every setting as it was on a new install?", "reset") {
		if _, err := s.svc.Preferences.Reset(); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
}

// shortcutSettings list the commands and their keys, which the user
// records by pressing them: Escape cancels, Backspace removes the
// shortcut, and a command that had the keys gives them up.
func (s *shell) shortcutSettings(c *ui.Context) {
	k := &s.lk
	groups := []string{}
	byGroup := map[string][]command{}
	palette := "the command palette"
	for _, cmd := range s.commands() {
		if _, ok := byGroup[cmd.group]; !ok {
			groups = append(groups, cmd.group)
		}
		byGroup[cmd.group] = append(byGroup[cmd.group], cmd)
		if keys := shortcutLabel(s.shortcutOf(cmd)); cmd.id == "view.palette" && keys != "" {
			palette += ", " + keys + ","
		}
	}
	// The search of the settings shows the commands it finds without the
	// section's introduction.
	if !s.find.on() {
		ui.Row(c).Gap(24).AlignItems(ui.Center).Margin(0, 0, 32).Children(func() {
			ui.Text(c, "Shortcuts of a single key work while you are not typing; "+palette+" lists every command, those of plugins too.").
				FontSize(k.smallSize()).TextColor(k.muted).Grow(1).MinWidth(0).MaxWidth(640)
			if len(s.set.Shortcuts) > 0 && s.link(c, "shortcuts-reset", "reset all") {
				s.set.Shortcuts, s.recording = nil, ""
				s.save()
			}
		})
	}
	for _, g := range groups {
		s.group(c, g, func() {
			for _, cmd := range byGroup[g] {
				current := s.shortcutOf(cmd)
				recording := s.recording == cmd.id
				// The search finds a command by its keys too.
				var keys []string
				if s.find.sifting() {
					keys = []string{shortcutWords(current)}
				}
				s.row(c, cmd.title, "", func() {
					label, ink, action := shortcutLabel(current), k.muted, "change"
					switch {
					case recording:
						label, ink, action = "press the keys · esc cancels · backspace removes", k.accent, "cancel"
					case label == "":
						label, action = "none", "set"
					}
					ui.Text(c, label).FontSize(k.smallSize()).TextColor(ink).MinWidth(120).TextAlign(ui.End).AlignSelf(ui.Center)
					b := s.button(c, "shortcut-edit-"+cmd.id, action, false).Label(action + " the shortcut of " + cmd.title)
					if recording {
						// The keys pressed name the shortcut rather than run
						// commands, until the button loses the focus.
						b.HandleInput(func(ev ui.InputEvent) bool { return s.recordKey(cmd, ev) })
						if !b.Focused() && s.recordingFocused {
							s.recording = ""
						}
						s.recordingFocused = b.Focused()
					}
					if b.Clicked() {
						s.recording, s.recordingFocused = "", false
						if !recording {
							s.recording = cmd.id
							c.Announce("Press the keys of the shortcut of " + cmd.title + ". Escape cancels, Backspace removes it.")
						}
					}
					// Reset puts the command's own keys back, which a command
					// the user gave them to gives up, as when they're recorded.
					if _, custom := s.set.Shortcuts[cmd.id]; custom && !recording &&
						s.button(c, "shortcut-reset-"+cmd.id, "reset", false).Label("reset the shortcut of "+cmd.title).Clicked() {
						s.setShortcut(cmd, cmd.shortcut)
					}
				}, keys...)
			}
		})
	}
}

// recordKey takes a key pressed as the user records cmd's shortcut; Tab
// goes on, as the focus leaves.
func (s *shell) recordKey(cmd command, ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown || ev.Key == ui.KeyTab {
		return false
	}
	switch {
	case ev.Repeat:
	case ev.Key == ui.KeyEscape:
		s.recording = ""
	case ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete:
		s.setShortcut(cmd, "")
	default:
		// A modifier alone waits for its key.
		if keys := shortcutName(ev.Mods, ev.Key); keys != "" {
			s.setShortcut(cmd, keys)
		}
	}
	return true
}

// setShortcut gives a command the keys of a shortcut, "" for none; a
// command that had them gives them up.
func (s *shell) setShortcut(cmd command, keys string) {
	s.recording = ""
	if s.set.Shortcuts == nil {
		s.set.Shortcuts = map[string]string{}
	}
	if keys != "" {
		for _, other := range s.commands() {
			if other.id != cmd.id && sameShortcut(s.shortcutOf(other), keys) {
				s.set.Shortcuts[other.id] = ""
				s.notes = append(s.notes, notice(other.title+" has no shortcut now"))
			}
		}
	}
	if sameShortcut(keys, cmd.shortcut) {
		delete(s.set.Shortcuts, cmd.id)
	} else {
		s.set.Shortcuts[cmd.id] = keys
	}
	s.save()
}

// updateSettings check for new versions, and install them without asking.
func (s *shell) updateSettings(c *ui.Context) {
	u := s.svc.System.Updates()
	if u.Kind == "development" {
		return
	}
	s.group(c, "updates", func() {
		asks := ""
		switch {
		case u.Enabled && !u.AutomaticDownloads:
			asks = " A new version asks to install."
		case !u.Enabled:
			asks = " A new version offers its download."
		}
		last := ""
		if t, err := time.Parse(time.RFC3339, u.LastCheck); err == nil {
			last = " Last checked " + t.Local().Format("2 Jan 2006, 15:04") + "."
		}
		checks, downloads := u.AutomaticChecks, u.AutomaticChecks && u.AutomaticDownloads
		set := func(checks, downloads bool) {
			if _, err := s.svc.System.SetUpdates(checks, downloads); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		}
		s.row(c, "Check for updates", "At start and every few hours, from the releases on GitHub."+asks+last, func() {
			if s.switchOf(c.Key("update-checks"), &checks).Label("Check for updates").Changed() {
				set(checks, u.AutomaticDownloads)
			}
		})
		if u.Enabled {
			s.row(c, "Install updates without asking", "New versions download in the background and run at the next launch.", func() {
				if s.switchOf(c.Key("update-downloads"), &downloads).Label("Install updates without asking").Changed() {
					set(u.AutomaticChecks || downloads, downloads)
				}
			})
		}
	})
}

// buildNames tell how a copy of each kind was installed, as
// UpdateStatus.Kind says.
var buildNames = map[string]string{
	"installer": "installed with the setup", "portable": "portable", "app": "app", "archive": "installed with install.sh",
	"deb": ".deb package", "rpm": ".rpm package", "appimage": "AppImage", "flatpak": "Flatpak", "development": "development build",
}

// releaseRow tells whether a newer version is out, and installs it, or
// offers its download where this copy can't replace itself; once one is
// installed, it restarts MixMusic.
func (s *shell) releaseRow(c *ui.Context, u app.UpdateStatus) {
	if installed := cmp.Or(s.updated, u.Installed); installed != "" {
		s.row(c, "MixMusic "+installed+" is installed", "It runs once MixMusic restarts.", func() {
			if s.button(c, "restart", "restart now", true).Clicked() {
				go s.svc.System.Restart()
			}
		})
		return
	}
	// The search of the settings finds the newest version by these words,
	// without asking GitHub for it.
	const words = "newest newer version update release"
	if _, asked := s.loads["release"]; !asked && !s.findable("", words) {
		return
	}
	r := fetch(s, "release", s.svc.System.LatestRelease)
	again := func() {
		if !s.updating && s.link(c, "release-again", "check again") {
			s.forget("release")
		}
	}
	switch rel := r.value; {
	case !r.done:
		s.row(c, "Looking for a newer version…", "", func() {}, words)
	case r.err != nil:
		s.row(c, "Couldn't look for a newer version", r.err.Error(), again, words)
	case rel.Newer:
		var describe []string
		if t, err := time.Parse(time.RFC3339, rel.Date); err == nil {
			describe = append(describe, "Out since "+t.Local().Format("2 Jan 2006")+".")
		}
		if !u.Enabled {
			describe = append(describe, replacing(u.Kind, rel.File))
		}
		s.row(c, "MixMusic "+rel.Version+" is out", strings.Join(describe, " "), func() {
			again()
			if rel.Page != "" && s.button(c, "release-notes", "what's new", false).Clicked() {
				go s.svc.System.OpenExternal(rel.Page)
			}
			switch {
			case u.Enabled:
				label := "install " + rel.Version
				switch {
				case s.updating && s.updateTotal > 0:
					label = fmt.Sprintf("downloading %d%%", 100*s.updateDone/s.updateTotal)
				case s.updating:
					label = "downloading…"
				}
				if s.button(c, "release-install", label, true).Disabled(s.updating).Clicked() {
					s.installUpdate()
				}
			case rel.Download != "":
				if s.button(c, "release-download", downloadLabel(u.Kind), true).Clicked() {
					go s.svc.System.OpenExternal(rel.Download)
				}
			}
		}, words)
	default:
		s.row(c, "This is the newest version", "", again, words)
	}
}

// downloadLabels name the download of a release that replaces a copy of
// each kind, as UpdateStatus.Kind says.
var downloadLabels = map[string]string{
	"installer": "download the installer", "portable": "download the portable zip", "app": "download the disk image",
	"archive": "download the archive", "deb": "download the .deb", "rpm": "download the .rpm", "appimage": "download the AppImage",
	"flatpak": "download the Flatpak",
}

// downloadLabel names the download that replaces a copy of kind.
func downloadLabel(kind string) string {
	if l, ok := downloadLabels[kind]; ok {
		return l
	}
	return "download it"
}

// replacing tells how file, a release's download, takes the place of a
// copy of kind, which can't replace itself.
func replacing(kind, file string) string {
	switch kind {
	case "deb":
		return "Open " + file + " with your software center, or run sudo apt install ./" + file + ". Your library and settings stay."
	case "rpm":
		return "Open " + file + " with your software center, or run sudo dnf install ./" + file + ". Your library and settings stay."
	case "appimage":
		return "Make " + file + " executable and use it in place of this AppImage. Your library and settings stay."
	case "flatpak":
		// A bundle's copy updates only from the next bundle: flatpak
		// update doesn't know where it came from.
		if file == "" {
			return ""
		}
		return "Open " + file + " with your software center, or run flatpak install ./" + file + ". Your library and settings stay."
	case "development":
		return "Development builds don't update themselves."
	}
	if file == "" {
		return ""
	}
	return "MixMusic can't write where it is installed, so it can't replace itself: run " + file + ". Your library and settings stay."
}

// updateOffer offers a new version: to install it, where this copy
// replaces itself, or else to download the file that replaces it.
func (s *shell) updateOffer(c *ui.Context) {
	k := &s.lk
	offer := s.offer
	if offer == nil {
		return
	}
	u := s.svc.System.Updates()
	open := true
	body := func() {
		if s.updated != "" {
			ui.Text(c, "MixMusic "+s.updated+" is installed. It runs once MixMusic restarts.").FontSize(k.bodySize())
		} else {
			if offer.Notes != "" {
				ui.Text(c, offer.Notes).FontSize(k.bodySize()).MaxLines(8).MaxWidth(520)
			}
			hint := "You have " + s.svc.System.Info().Version + ". "
			if u.Enabled {
				hint += "To install new versions without this question, turn on install updates without asking in Settings › About."
			} else {
				hint += replacing(u.Kind, offer.File)
			}
			ui.Text(c, hint).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(520)
		}
		if s.updating {
			v := -1.0
			if s.updateTotal > 0 {
				v = float64(s.updateDone) / float64(s.updateTotal)
			}
			ui.Progress(c, v).Width(320)
			ui.Text(c, "Downloading…").FontSize(k.smallSize()).TextColor(k.muted)
		}
	}
	// The offer comes unasked, and takes the focus from where it was:
	// later has it, which only puts the offer off, so that a Space
	// pressed for the music or typed in a search skips nothing.
	actions := func() {
		if s.updated != "" {
			if s.dialogButton(c, "offer-later", "later", false).AutoFocus().Clicked() {
				s.offer = nil
			}
			if s.dialogButton(c, "offer-restart", "restart now", true).Clicked() {
				go s.svc.System.Restart()
			}
			return
		}
		if s.dialogButton(c, "offer-skip", "skip this version", false).Clicked() {
			s.dismissOffer(true)
		}
		if offer.Page != "" && s.dialogButton(c, "offer-notes", "what's new", false).Clicked() {
			go s.svc.System.OpenExternal(offer.Page)
		}
		if s.dialogButton(c, "offer-later", "later", false).AutoFocus().Clicked() {
			s.dismissOffer(false)
		}
		switch {
		case u.Enabled:
			if s.dialogButton(c, "offer-install", "install "+offer.Version, true).Clicked() {
				s.installUpdate()
			}
		case offer.Download != "":
			if s.dialogButton(c, "offer-download", downloadLabel(u.Kind), true).Clicked() {
				go s.svc.System.OpenExternal(offer.Download)
			}
		}
	}
	if s.updating {
		actions = nil
	}
	s.dialog(c, &open, dialogOptions{key: "offer", title: "mixmusic " + offer.Version + " is out", width: 560}, body, actions)
	if !open && !s.updating {
		s.dismissOffer(false)
	}
}

func (s *shell) dismissOffer(skip bool) {
	s.offer = nil
	if err := s.svc.System.DismissUpdate(skip); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

func (s *shell) installUpdate() {
	s.updating, s.updateDone, s.updateTotal = true, 0, 0
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		version, err := s.a.InstallUpdate(ctx, func(done, total int64) {
			s.update(func() { s.updateDone, s.updateTotal = done, total })
		})
		s.update(func() {
			s.updating = false
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.updated = version
		})
	}()
}
