package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("plugins", 10, (*shell).pluginSettings) }

// pluginSettings list the plugins installed, which the user enables or
// removes. The web interface runs them, for now.
func (s *shell) pluginSettings(c *ui.Context) {
	s.groupWith(c, locales.PluginSettingsGroup(), func() {
		if s.link(c, "get-plugins", locales.PluginSettingsGetPlugins()) {
			s.navigate("/marketplace/plugins")
		}
		if s.link(c, "plugins-folder", locales.PluginSettingsOpenFolder()) {
			s.openFolder(addons.KindPlugin)
		}
		if s.link(c, "reload-plugins", locales.PluginSettingsReload()) {
			s.plugins.Retry()
			s.inv = s.svc.Addons.Reload()
		}
	}, func() {
		if len(s.inv.Plugins) == 0 {
			s.hint(c, locales.PluginSettingsNone())
		}
		for _, p := range s.inv.Plugins {
			desc := locales.PluginSettingsBy(p.Author)
			if p.Description != "" {
				desc = locales.PluginSettingsDescribed(p.Description, p.Author)
			}
			desc = strings.TrimSpace(desc)
			if f := s.plugins.Failure(p.ID); f != "" {
				desc = locales.PluginSettingsFailed(desc, f)
			}
			s.row(c, p.Name+" "+p.Version, desc, func() {
				on := slices.Contains(s.set.EnabledPlugins, p.ID)
				if s.switchOf(c.Key("plugin-"+p.ID), &on).Label(locales.PluginSettingsEnable(p.Name)).Changed() {
					s.enablePlugin(p, on)
				}
				if !p.Builtin && s.iconButton(c, "remove-plugin-"+p.ID, "trash", locales.PluginSettingsRemoveLabel(p.Name), false, 16).Clicked() {
					s.ask(locales.PluginSettingsRemoveTitle(p.Name), locales.PluginSettingsRemoveBody(), locales.PluginSettingsRemove(), func() {
						s.uninstall(addons.KindPlugin, addons.Entry{ID: p.ID, Name: p.Name})
					})
				}
			})
		}
		s.problems(c, addons.KindPlugin)
	})
}

// enablePlugin enables a plugin, once the user said they trust it, or
// disables it.
func (s *shell) enablePlugin(p addons.Plugin, on bool) {
	set := func() {
		ids := slices.DeleteFunc(slices.Clone(s.set.EnabledPlugins), func(id string) bool { return id == p.ID })
		if on {
			ids = append(ids, p.ID)
		}
		s.set.EnabledPlugins = ids
		s.save()
	}
	if !on || p.Builtin {
		set()
		return
	}
	body := locales.PluginSettingsEnableBody()
	if len(p.Permissions) > 0 {
		body = locales.PluginSettingsEnableAsks(strings.Join(p.Permissions, ", "))
	}
	s.ask(locales.PluginSettingsEnableTitle(p.Name), body, locales.PluginSettingsEnableButton(), set)
}
