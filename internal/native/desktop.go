package native

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (s *shell) desktopSettings(c *ui.Context) {
	s.group(c, "on the desktop", func() {
		toggle := func(key, title, description string, v *bool) {
			s.row(c, title, description, func() {
				if ui.Switch(c.Key(key), v).Label(title).Changed() {
					s.save()
				}
			})
		}
		toggle("tray", "Close to the tray", "Closing the window keeps the music playing; quit from the tray icon.", &s.set.CloseToTray)
		toggle("media-keys", "Media keys", "The keyboard's play, next and previous keys control MixMusic.", &s.set.MediaKeys)
		toggle("notifications", "Notifications", "Tell what starts playing while MixMusic is in the background.", &s.set.Notifications)
		toggle("discord", "Show what plays on Discord", "While Discord runs, your profile says what you listen to, with the artwork and the time left; pausing clears it.", &s.set.DiscordPresence)
		if s.set.DiscordPresence {
			describe := func() {
				ui.Text(c, "Discord shows the name of an application: create one called MixMusic in the ")
				ui.Link(c, "developer portal", "https://discord.com/developers/applications")
				ui.Text(c, " and paste its application ID.")
			}
			s.rowWith(c, "Discord application ID", describe, func() {
				in, _ := s.settingInput(c, "discord-app", s.set.DiscordAppID, func(id string) {
					s.set.DiscordAppID = id
					s.save()
				})
				in.Width(260).Label("Discord application ID")
			})
		}
		s.row(c, "Start at login", "", func() {
			// The system says, beside the main thread.
			login := fetch(s, "openAtLogin", func(context.Context) (bool, error) { return s.svc.Preferences.OpenAtLogin(), nil })
			on := login.value
			if ui.Switch(c.Key("login"), &on).Label("Start at login").Disabled(!login.done).Changed() {
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
		})
		s.row(c, "Reset settings", "Puts back the settings of a new install; your accounts, library and add-ons stay.", func() {
			if s.button(c, "reset", "reset", false).Clicked() {
				s.resetting = true
			}
		})
	})
	if ui.AlertDialog(c, &s.resetting, "Reset settings", "Put back every setting as it was on a new install?", "Cancel", "Reset") == 1 {
		if _, err := s.svc.Preferences.Reset(); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
}

// shortcutSettings list the commands and their keys, which the user can
// change: a shortcut is written as "Mod+Shift+L", Mod being Ctrl, or Cmd on
// macOS.
func (s *shell) shortcutSettings(c *ui.Context) {
	k := &s.lk
	groups := []string{}
	byGroup := map[string][]command{}
	for _, cmd := range s.commands() {
		if _, ok := byGroup[cmd.group]; !ok {
			groups = append(groups, cmd.group)
		}
		byGroup[cmd.group] = append(byGroup[cmd.group], cmd)
	}
	for _, g := range groups {
		s.group(c, g, func() {
			for _, cmd := range byGroup[g] {
				current := s.shortcutOf(cmd)
				s.row(c, cmd.title, "", func() {
					if s.editingShortcut == cmd.id {
						in := ui.TextInput(c.Key("shortcut-"+cmd.id), &s.shortcutDraft).Width(180).AutoFocus().Label("Shortcut of " + cmd.title)
						if in.Submitted() {
							s.setShortcut(cmd, strings.TrimSpace(s.shortcutDraft))
						}
						if s.button(c, "shortcut-cancel-"+cmd.id, "cancel", false).Clicked() {
							s.editingShortcut = ""
						}
						return
					}
					label := shortcutLabel(current)
					if label == "" {
						label = "none"
					}
					ui.Text(c, label).FontSize(k.smallSize()).TextColor(k.muted).Width(120).TextAlign(ui.End)
					if s.button(c, "shortcut-edit-"+cmd.id, "change", false).Clicked() {
						s.editingShortcut, s.shortcutDraft = cmd.id, current
					}
					if _, custom := s.set.Shortcuts[cmd.id]; custom && s.button(c, "shortcut-reset-"+cmd.id, "reset", false).Clicked() {
						delete(s.set.Shortcuts, cmd.id)
						s.save()
					}
				})
			}
		})
	}
}

// setShortcut gives a command a shortcut of the user's, "" for none.
func (s *shell) setShortcut(cmd command, shortcut string) {
	if shortcut != "" {
		if _, _, ok := parseShortcut(shortcut); !ok {
			s.notes = append(s.notes, noticeOf(errShortcut(shortcut)))
			return
		}
	}
	if s.set.Shortcuts == nil {
		s.set.Shortcuts = map[string]string{}
	}
	s.set.Shortcuts[cmd.id] = shortcut
	s.editingShortcut = ""
	s.save()
}

type errShortcut string

func (e errShortcut) Error() string {
	return "“" + string(e) + "” isn't a shortcut: write one as Mod+K, Shift+ArrowRight or Alt+1"
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
			if ui.Switch(c.Key("update-checks"), &checks).Label("Check for updates").Changed() {
				set(checks, u.AutomaticDownloads)
			}
		})
		if u.Enabled {
			s.row(c, "Install updates without asking", "New versions download in the background and run at the next launch.", func() {
				if ui.Switch(c.Key("update-downloads"), &downloads).Label("Install updates without asking").Changed() {
					set(u.AutomaticChecks || downloads, downloads)
				}
			})
		}
		if u.Installed != "" {
			s.row(c, "MixMusic "+u.Installed+" is installed", "It runs once MixMusic restarts.", func() {
				if s.button(c, "restart", "restart now", true).Clicked() {
					go s.svc.System.Restart()
				}
			})
		}
	})
}

// downloadLabels name the download of a release that replaces a copy of
// each kind, as UpdateStatus.Kind says.
var downloadLabels = map[string]string{
	"installer": "download the installer", "portable": "download the portable zip", "app": "download the disk image",
	"archive": "download the archive", "deb": "download the .deb", "rpm": "download the .rpm", "appimage": "download the AppImage",
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
	panel := ui.Modal(c, &open, func() {
		ui.Text(c, "mixmusic "+offer.Version+" is out").FontSize(k.titleSize()).FontWeight(300).Font(k.display)
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
			return
		}
		ui.Row(c).Gap(8).Justify(ui.End).Wrap().Children(func() {
			if s.updated != "" {
				if s.button(c, "offer-later", "later", false).Clicked() {
					s.offer = nil
				}
				if s.button(c, "offer-restart", "restart now", true).Clicked() {
					go s.svc.System.Restart()
				}
				return
			}
			if s.button(c, "offer-skip", "skip this version", false).Clicked() {
				s.dismissOffer(true)
			}
			if offer.Page != "" && s.button(c, "offer-notes", "what's new", false).Clicked() {
				go s.svc.System.OpenExternal(offer.Page)
			}
			if s.button(c, "offer-later", "later", false).Clicked() {
				s.dismissOffer(false)
			}
			switch {
			case u.Enabled:
				if s.button(c, "offer-install", "install "+offer.Version, true).Clicked() {
					s.installUpdate()
				}
			case offer.Download != "":
				if s.button(c, "offer-download", downloadLabel(u.Kind), true).Clicked() {
					go s.svc.System.OpenExternal(offer.Download)
				}
			}
		})
	})
	s.frosted(panel).Width(560).Padding(32).Gap(16).Border(1, k.border)
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
