package native

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// pluginSettings list the plugins installed, which the user enables or
// removes. The web interface runs them, for now.
func (s *shell) pluginSettings(c *ui.Context) {
	s.groupWith(c, "installed plugins", func() {
		if s.link(c, "get-plugins", "get plugins") {
			s.navigate("/marketplace/plugins")
		}
		if s.link(c, "plugins-folder", "open plugins folder") {
			s.openFolder(addons.KindPlugin)
		}
		if s.link(c, "reload-plugins", "reload") {
			s.plugins.Retry()
			s.inv = s.svc.Addons.Reload()
		}
	}, func() {
		if len(s.inv.Plugins) == 0 {
			s.hint(c, "No plugins yet. The marketplace has some, and docs/plugins.md tells how to write one.")
		}
		for _, p := range s.inv.Plugins {
			desc := strings.TrimSpace(p.Description + " by " + p.Author)
			if f := s.plugins.Failure(p.ID); f != "" {
				desc += " · failed: " + f
			}
			s.row(c, p.Name+" "+p.Version, desc, func() {
				on := slices.Contains(s.set.EnabledPlugins, p.ID)
				if s.switchOf(c.Key("plugin-"+p.ID), &on).Label("Enable " + p.Name).Changed() {
					s.enablePlugin(p, on)
				}
				if !p.Builtin && s.iconButton(c, "remove-plugin-"+p.ID, "trash", "Remove "+p.Name, false, 16).Clicked() {
					s.ask("remove "+p.Name, "The plugin and its files are deleted; what it stored stays.", "remove", func() {
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
	perms := ""
	if len(p.Permissions) > 0 {
		perms = " It asks for: " + strings.Join(p.Permissions, ", ") + "."
	}
	s.ask("enable "+p.Name, "Plugins can control the player, change your library and reach the internet, whatever they ask for. Enable plugins from authors you trust."+perms, "enable", set)
}
