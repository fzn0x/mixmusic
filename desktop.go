package main

import (
	"cmp"
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("desktop", 10, (*shell).desktopSettings) }

// desktopRows are the rows of Settings › On the desktop that register
// from their features' files, below Discord's.
const desktopRows = "desktop › On the desktop"

func (s *shell) desktopSettings(c *ui.Context) {
	s.group(c, locales.DesktopSettingsGroup(), func() {
		toggle := func(key, title, description string, v *bool, words ...string) {
			s.row(c, title, description, func() {
				if s.switchOf(c.Key(key), v).Label(title).Changed() {
					s.save()
				}
			}, words...)
		}
		toggle("tray", locales.DesktopSettingsTrayTitle(), locales.DesktopSettingsTrayAbout(), &s.set.CloseToTray, "minimize background")
		toggle("media-keys", locales.DesktopSettingsMediaKeysTitle(), locales.DesktopSettingsMediaKeysAbout(), &s.set.MediaKeys)
		toggle("notifications", locales.DesktopSettingsNotificationsTitle(), locales.DesktopSettingsNotificationsAbout(), &s.set.Notifications)
		toggle("title", locales.DesktopSettingsTitleTitle(), locales.DesktopSettingsTitleAbout(), &s.set.ShowInTitle)
		toggle("discord", locales.DesktopSettingsDiscordTitle(), locales.DesktopSettingsDiscordAbout(), &s.set.DiscordPresence)
		if s.set.DiscordPresence {
			describe := func() {
				ui.Text(c, locales.DesktopSettingsDiscordBefore())
				s.linkOf(c, locales.DesktopSettingsDiscordPortal(), "https://discord.com/developers/applications")
				ui.Text(c, locales.DesktopSettingsDiscordAfter())
			}
			s.rowWith(c, locales.DesktopSettingsDiscordAppID(), describe, func() {
				in, _ := s.settingInput(c, "discord-app", s.set.DiscordAppID, func(id string) {
					s.set.DiscordAppID = id
					s.save()
				})
				in.Placeholder(locales.DesktopSettingsDiscordPlaceholder()).Width(260).Label(locales.DesktopSettingsDiscordAppID())
			}, "Discord shows the name of MixMusic's application. To show another, create one in the developer portal and paste its application ID.")
		}
		s.settingsOf(c, desktopRows)
		s.row(c, locales.DesktopSettingsLogin(), "", func() {
			// The system says, beside the main thread.
			login := fetch(s, "openAtLogin", func(context.Context) (bool, error) { return s.svc.Preferences.OpenAtLogin(), nil })
			on := login.value
			if s.switchOf(c.Key("login"), &on).Label(locales.DesktopSettingsLogin()).Disabled(!login.done).Changed() {
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
		s.row(c, locales.DesktopSettingsSettingsFile(), locales.DesktopSettingsSettingsFileAbout(), func() {
			if s.button(c, "settings-file-open", locales.DesktopSettingsOpen(), false).Clicked() {
				s.openSettingsFile()
			}
			if s.button(c, "settings-file-show", locales.DesktopSettingsShowInFolder(), false).Clicked() {
				s.showSettingsFile()
			}
		}, "settings.json json config")
		s.row(c, locales.DesktopSettingsReset(), locales.DesktopSettingsResetAbout(), func() {
			if s.button(c, "reset", locales.DesktopSettingsResetButton(), false).Clicked() {
				s.resetting = true
			}
		}, "defaults")
	})
	if s.confirm(c, "reset-settings", &s.resetting, locales.DesktopSettingsResetTitle(), locales.DesktopSettingsResetConfirm(), locales.DesktopSettingsResetButton()) {
		if _, err := s.svc.Preferences.Reset(); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
}

func init() { addSettings("shortcuts", 10, (*shell).shortcutSettings) }

// shortcutSettings list the commands and their keys, which the user
// records by pressing them: Escape cancels, Backspace removes the
// shortcut, and a command that had the keys gives them up.
func (s *shell) shortcutSettings(c *ui.Context) {
	k := &s.lk
	groups := []string{}
	byGroup := map[string][]command{}
	palette := locales.ShortcutsCommandPalette()
	for _, cmd := range s.commands() {
		if _, ok := byGroup[cmd.group]; !ok {
			groups = append(groups, cmd.group)
		}
		byGroup[cmd.group] = append(byGroup[cmd.group], cmd)
		if keys := shortcutLabel(s.shortcutOf(cmd)); cmd.id == "view.palette" && keys != "" {
			palette = locales.ShortcutsCommandPaletteKeys(keys)
		}
	}
	// The search of the settings shows the commands it finds without the
	// section's introduction.
	if !s.find.on() {
		ui.Row(c).Gap(24).AlignItems(ui.Center).Margin(0, 0, 32).Children(func() {
			ui.Text(c, locales.ShortcutsHint(palette)).
				FontSize(k.smallSize()).TextColor(k.muted).Grow(1).MinWidth(0).MaxWidth(640)
			if (len(s.set.Shortcuts) > 0 || len(s.set.GlobalShortcuts) > 0) && s.link(c, "shortcuts-reset", locales.ShortcutsResetAll()) {
				s.set.Shortcuts, s.set.GlobalShortcuts, s.recording = nil, nil, ""
				s.save()
			}
		})
	}
	s.globalShortcutSettings(c)
	for _, g := range groups {
		s.group(c, commandGroup(g), func() {
			for _, cmd := range byGroup[g] {
				current := s.shortcutOf(cmd)
				recording := s.recording == cmd.id
				// The search finds a command by its keys too.
				var keys []string
				if s.find.sifting() {
					keys = []string{shortcutWords(current)}
				}
				s.row(c, cmd.title, "", func() {
					label, ink, action := shortcutLabel(current), k.muted, locales.ShortcutsChange()
					switch {
					case recording:
						label, ink, action = locales.ShortcutsRecording(), k.accent, locales.ShortcutsCancel()
					case label == "":
						label, action = locales.ShortcutsNone(), locales.ShortcutsSet()
					}
					ui.Text(c, label).FontSize(k.smallSize()).TextColor(ink).MinWidth(120).TextAlign(ui.End).AlignSelf(ui.Center)
					b := s.button(c, "shortcut-edit-"+cmd.id, action, false).Label(locales.ShortcutsEditLabel(action, cmd.title))
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
							c.Announce(locales.ShortcutsAnnounce(cmd.title))
						}
					}
					// Reset puts the command's own keys back, which a command
					// the user gave them to gives up, as when they're recorded.
					if _, custom := s.set.Shortcuts[cmd.id]; custom && !recording &&
						s.button(c, "shortcut-reset-"+cmd.id, locales.ShortcutsReset(), false).Label(locales.ShortcutsResetLabel(cmd.title)).Clicked() {
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
		s.giveUpGlobal(cmd.id, keys)
		for _, other := range s.commands() {
			if other.id != cmd.id && sameShortcut(s.shortcutOf(other), keys) {
				s.set.Shortcuts[other.id] = ""
				s.notes = append(s.notes, notice(locales.ShortcutsDisplaced(other.title)))
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

func init() { addSettings("about", 20, (*shell).updateSettings) }

// updateSettings check for new versions, and install them without asking.
func (s *shell) updateSettings(c *ui.Context) {
	u := s.svc.System.Updates()
	if u.Kind == "development" {
		return
	}
	s.group(c, locales.UpdatesGroup(), func() {
		asks := newVersionDoes(u)
		last := ""
		if t, err := time.Parse(time.RFC3339, u.LastCheck); err == nil {
			last = locales.UpdatesLastChecked(t.Local().Format("2 Jan 2006, 15:04"))
		}
		checks, downloads := u.AutomaticChecks, u.AutomaticChecks && u.AutomaticDownloads
		set := func(checks, downloads bool) {
			if _, err := s.svc.System.SetUpdates(checks, downloads); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		}
		s.row(c, locales.UpdatesCheckTitle(), locales.UpdatesCheckAbout(asks, last), func() {
			if s.switchOf(c.Key("update-checks"), &checks).Label(locales.UpdatesCheckTitle()).Changed() {
				set(checks, u.AutomaticDownloads)
			}
		})
		if u.Enabled {
			s.row(c, locales.UpdatesInstallTitle(), locales.UpdatesInstallAbout(), func() {
				if s.switchOf(c.Key("update-downloads"), &downloads).Label(locales.UpdatesInstallTitle()).Changed() {
					set(u.AutomaticChecks || downloads, downloads)
				}
			})
		}
	})
}

// newVersionDoes tells, after the row of the checks, what a new version
// found does in a copy of u's: Flathub's comes through the software
// center.
func newVersionDoes(u app.UpdateStatus) string {
	switch {
	case u.Enabled && !u.AutomaticDownloads:
		return locales.UpdatesAsks()
	case u.Kind == "flathub":
		return locales.UpdatesFlathub()
	case !u.Enabled:
		return locales.UpdatesOffers()
	}
	return ""
}

// buildName tells how a copy of kind was installed, as UpdateStatus.Kind
// says; ok is false for a kind it doesn't know.
func buildName(kind string) (name string, ok bool) {
	switch kind {
	case "installer":
		return locales.UpdatesBuildInstaller(), true
	case "portable":
		return locales.UpdatesBuildPortable(), true
	case "app":
		return locales.UpdatesBuildApp(), true
	case "archive":
		return locales.UpdatesBuildArchive(), true
	case "deb":
		return locales.UpdatesBuildDeb(), true
	case "rpm":
		return locales.UpdatesBuildRpm(), true
	case "pacman":
		return locales.UpdatesBuildPacman(), true
	case "appimage":
		return locales.UpdatesBuildAppImage(), true
	case "flatpak":
		return locales.UpdatesBuildFlatpak(), true
	case "flathub":
		return locales.UpdatesBuildFlathub(), true
	case "development":
		return locales.UpdatesBuildDevelopment(), true
	}
	return "", false
}

// releaseRow tells whether a newer version is out, and installs it, or
// offers its download where this copy can't replace itself; once one is
// installed, it restarts MixMusic.
func (s *shell) releaseRow(c *ui.Context, u app.UpdateStatus) {
	if installed := cmp.Or(s.updated, u.Installed); installed != "" {
		s.row(c, locales.UpdatesInstalledTitle(installed), locales.UpdatesRestartHint(), func() {
			if s.button(c, "restart", locales.UpdatesRestartNow(), true).Clicked() {
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
		if !s.updating && s.link(c, "release-again", locales.UpdatesCheckAgain()) {
			s.forget("release")
		}
	}
	switch rel := r.value; {
	case !r.done:
		s.row(c, locales.UpdatesLooking(), "", func() {}, words)
	case r.err != nil:
		s.row(c, locales.UpdatesCouldntLook(), r.err.Error(), again, words)
	case rel.Newer:
		var date string
		if t, err := time.Parse(time.RFC3339, rel.Date); err == nil {
			date = t.Local().Format("2 Jan 2006")
		}
		var describe string
		switch {
		case date != "" && !u.Enabled:
			describe = locales.UpdatesOutSinceHow(date, replacing(u.Kind, rel.File))
		case date != "":
			describe = locales.UpdatesOutSince(date)
		case !u.Enabled:
			describe = replacing(u.Kind, rel.File)
		}
		s.row(c, locales.UpdatesOut(rel.Version), describe, func() {
			again()
			if rel.Page != "" && s.button(c, "release-notes", locales.UpdatesWhatsNew(), false).Clicked() {
				go s.svc.System.OpenExternal(rel.Page)
			}
			switch {
			case u.Enabled:
				label := locales.UpdatesInstallVersion(rel.Version)
				switch {
				case s.updating && s.updateTotal > 0:
					label = locales.UpdatesDownloadingPercent(int(100 * s.updateDone / s.updateTotal))
				case s.updating:
					label = locales.UpdatesDownloading()
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
		s.row(c, locales.UpdatesNewest(), "", again, words)
	}
}

// downloadLabel names the download that replaces a copy of kind, as
// UpdateStatus.Kind says.
func downloadLabel(kind string) string {
	switch kind {
	case "installer":
		return locales.UpdatesDownloadInstaller()
	case "portable":
		return locales.UpdatesDownloadPortable()
	case "app":
		return locales.UpdatesDownloadApp()
	case "archive":
		return locales.UpdatesDownloadArchive()
	case "deb":
		return locales.UpdatesDownloadDeb()
	case "rpm":
		return locales.UpdatesDownloadRpm()
	case "appimage":
		return locales.UpdatesDownloadAppImage()
	case "flatpak":
		return locales.UpdatesDownloadFlatpak()
	}
	return locales.UpdatesDownloadAny()
}

// replacing tells how file, a release's download, takes the place of a
// copy of kind, which can't replace itself.
func replacing(kind, file string) string {
	switch kind {
	case "deb":
		return locales.UpdatesReplaceDeb(file)
	case "rpm":
		return locales.UpdatesReplaceRpm(file)
	case "pacman":
		// The AUR's mixmusic-bin updates with the system, as pacman's
		// packages do: no download of the release replaces it.
		return locales.UpdatesReplacePacman()
	case "appimage":
		return locales.UpdatesReplaceAppImage(file)
	case "flatpak":
		// A bundle's copy updates only from the next bundle: flatpak
		// update doesn't know where it came from.
		if file == "" {
			return ""
		}
		return locales.UpdatesReplaceFlatpak(file)
	case "flathub":
		// Flathub builds each release, and the software center installs
		// it with the updates of the other apps: no download replaces it.
		return locales.UpdatesReplaceFlathub()
	case "development":
		return locales.UpdatesReplaceDevelopment()
	}
	if file == "" {
		return ""
	}
	return locales.UpdatesReplaceOther(file)
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
			ui.Text(c, locales.UpdateOfferInstalled(s.updated)).FontSize(k.bodySize())
		} else {
			if offer.Notes != "" {
				ui.Text(c, offer.Notes).FontSize(k.bodySize()).MaxLines(8).MaxWidth(520)
			}
			var hint string
			if u.Enabled {
				hint = locales.UpdateOfferHaveAuto(s.svc.System.Info().Version)
			} else {
				hint = locales.UpdateOfferHave(s.svc.System.Info().Version, replacing(u.Kind, offer.File))
			}
			ui.Text(c, hint).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(520)
		}
		if s.updating {
			v := -1.0
			if s.updateTotal > 0 {
				v = float64(s.updateDone) / float64(s.updateTotal)
			}
			ui.Progress(c, v).Width(320)
			ui.Text(c, locales.UpdateOfferDownloading()).FontSize(k.smallSize()).TextColor(k.muted)
		}
	}
	// The offer comes unasked, and takes the focus from where it was:
	// later has it, which only puts the offer off, so that a Space
	// pressed for the music or typed in a search skips nothing.
	actions := func() {
		if s.updated != "" {
			if s.dialogButton(c, "offer-later", locales.UpdateOfferLater(), false).AutoFocus().Clicked() {
				s.offer = nil
			}
			if s.dialogButton(c, "offer-restart", locales.UpdatesRestartNow(), true).Clicked() {
				go s.svc.System.Restart()
			}
			return
		}
		if s.dialogButton(c, "offer-skip", locales.UpdateOfferSkip(), false).Clicked() {
			s.dismissOffer(true)
		}
		if offer.Page != "" && s.dialogButton(c, "offer-notes", locales.UpdatesWhatsNew(), false).Clicked() {
			go s.svc.System.OpenExternal(offer.Page)
		}
		if s.dialogButton(c, "offer-later", locales.UpdateOfferLater(), false).AutoFocus().Clicked() {
			s.dismissOffer(false)
		}
		switch {
		case u.Enabled:
			if s.dialogButton(c, "offer-install", locales.UpdateOfferInstall(offer.Version), true).Clicked() {
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
	s.dialog(c, &open, dialogOptions{key: "offer", title: locales.UpdateOfferTitle(offer.Version), width: 560}, body, actions)
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
