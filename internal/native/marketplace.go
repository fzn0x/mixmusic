package native

import (
	"context"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
)

var marketTabs = []string{"themes", "plugins"}

// marketplacePage shows the themes and plugins of the community registry.
func (s *shell) marketplacePage(c *ui.Context, tab string) {
	k := &s.lk
	selected := max(slices.Index(marketTabs, tab), 0)
	reg := fetch(s, "registry", func(ctx context.Context) (addons.Registry, error) { return s.svc.Addons.Registry(ctx, false) })
	s.pageScroll(c, func() {
		before := selected
		var counts []int
		if reg.done && reg.err == nil {
			counts = []int{len(reg.value.Themes), len(reg.value.Plugins)}
		}
		s.pageHeaderCounts(c, "marketplace", marketTabs, counts, &selected)
		if selected != before {
			s.router.Replace("/marketplace/" + marketTabs[selected])
		}
		kind := addons.KindTheme
		if selected == 1 {
			kind = addons.KindPlugin
		}
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 24).Children(func() {
			ui.Text(c, "Made by the community, reviewed in the open. ")
			s.linkOf(c, "Share yours", "https://github.com/fzn0x/mixmusic/blob/main/community/README.md")
			ui.Text(c, ".")
		})
		switch {
		case !reg.done:
			s.loading(c)
			return
		case reg.err != nil:
			s.problem(c, reg.err, func() { s.forget("registry") })
			return
		}
		entries := reg.value.Themes
		if kind == addons.KindPlugin {
			entries = reg.value.Plugins
		}
		if len(entries) == 0 {
			s.emptyLine(c, "Nothing here yet: be the first to share one.")
			return
		}
		ui.Row(c).Wrap().Gap(10).AlignItems(ui.Start).Children(func() {
			for _, e := range entries {
				s.marketCard(c, kind, e, reg.value.BaseURL)
			}
		})
	})
	if s.confirm(c, "install-plugin", &s.confirmingPlugin, "install "+s.pluginToInstall.Name,
		"Plugins can control the player, change your library and reach the internet, whatever they ask for. Install plugins from authors you trust."+permissions(s.pluginToInstall),
		"install") {
		s.install(addons.KindPlugin, s.pluginToInstall)
	}
}

func permissions(e addons.Entry) string {
	if len(e.Permissions) == 0 {
		return ""
	}
	return " It asks for: " + strings.Join(e.Permissions, ", ") + "."
}

// installed returns the version of an add-on installed, and whether it's
// built in.
func (s *shell) installed(kind addons.Kind, id string) (version string, builtin, ok bool) {
	if kind == addons.KindTheme {
		for _, t := range s.inv.Themes {
			if t.ID == id {
				return t.Version, t.Builtin, true
			}
		}
		return "", false, false
	}
	for _, p := range s.inv.Plugins {
		if p.ID == id {
			return p.Version, p.Builtin, true
		}
	}
	return "", false, false
}

// marketCard shows an add-on of the registry, a theme in its own colours.
func (s *shell) marketCard(c *ui.Context, kind addons.Kind, e addons.Entry, baseURL string) {
	k := &s.lk
	version, builtin, has := s.installed(kind, e.ID)
	outdated := has && !builtin && version != e.Version
	ui.Column(c.Key(string(kind)+"-"+e.ID)).Width(300).Border(1, k.border).Radius(k.radius).Clip().Children(func() {
		preview := ui.Box(c.Key("preview")).Height(120).FillWidth().Center()
		shows := s.inView(c, preview)
		if kind == addons.KindTheme {
			if bg, ok := parseColor(e.Colors["bg"]); ok {
				preview.Background(col(bg))
			}
		} else {
			preview.Background(k.surface2)
		}
		preview.Children(func() {
			switch {
			case kind == addons.KindPlugin:
				ui.Icon(c, icon("puzzle")).FontSize(40).TextColor(k.muted)
			case e.Preview != "":
				// Only a preview in view downloads, and tries again.
				if shows {
					if b := s.art.get(c, baseURL+e.Path+"/"+e.Preview, 600); b != nil {
						ui.Image(c, b).Fill().Fit(ui.Cover)
					}
				}
			default:
				ui.Row(c).Gap(4).Children(func() {
					for i := 1; i <= 4; i++ {
						if t, ok := parseColor(e.Colors["tile-"+count(i)]); ok {
							ui.Box(c).Size(22, 22).Background(col(t))
						}
					}
				})
			}
		})
		ui.Column(c).Padding(16).Gap(6).Children(func() {
			ui.Text(c, strings.ToLower(e.Name)).FontSize(k.size(1.5)).FontWeight(300).Font(k.display).SingleLine()
			ui.Text(c, e.Version+" · by "+e.Author).FontSize(k.captionSize()).TextColor(k.muted)
			if e.Description != "" {
				ui.Text(c, e.Description).FontSize(k.smallSize()).MaxLines(3)
			}
			if len(e.Permissions) > 0 {
				ui.Text(c, "asks for "+strings.Join(e.Permissions, ", ")).FontSize(k.captionSize()).TextColor(k.muted)
			}
			ui.Row(c).Gap(8).Margin(8, 0, 0).Children(func() {
				busy := s.installingAddon == e.ID
				switch {
				case builtin:
					ui.Text(c, "built in").FontSize(k.smallSize()).TextColor(k.muted)
				case has && !outdated:
					if s.button(c, "remove-"+e.ID, "remove", false).Disabled(busy).Clicked() {
						s.uninstall(kind, e)
					}
				case kind == addons.KindPlugin && e.API != addons.PluginAPI:
					ui.Text(c, "made for an older MixMusic").FontSize(k.smallSize()).TextColor(k.muted)
				default:
					label := "install"
					switch {
					case busy:
						label = "installing…"
					case outdated:
						label = "update to " + e.Version
					}
					if s.button(c, "install-"+e.ID, label, true).Disabled(busy).Clicked() {
						if kind == addons.KindPlugin && !outdated {
							s.pluginToInstall, s.confirmingPlugin = e, true
						} else {
							s.install(kind, e)
						}
					}
				}
				if has && kind == addons.KindTheme && s.set.Theme != e.ID {
					use := s.buttonBase(c.Key("use-" + e.ID)).Label("Use " + e.Name).Cursor(ui.CursorPointer)
					if use.Clicked() {
						s.set.Theme = e.ID
						s.save()
					}
					use.Children(func() { ui.Text(c, "use it").FontSize(k.smallSize()).TextColor(k.accent) })
				}
			})
		})
	})
}

// install installs or updates an add-on of the registry.
func (s *shell) install(kind addons.Kind, e addons.Entry) {
	s.installingAddon = e.ID
	_, _, had := s.installed(kind, e.ID)
	go func() {
		_, err := s.svc.Addons.Install(context.Background(), kind, e.ID)
		s.update(func() {
			s.installingAddon = ""
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			verb := "Installed "
			if had {
				verb = "Updated "
			}
			// What was installed is one click away from use.
			id := e.ID
			use := actionToast{message: verb + e.Name, action: "use it", run: func() { s.chooseTheme(id) }}
			if kind == addons.KindPlugin {
				use.action, use.run = "enable", func() {
					if !slices.Contains(s.set.EnabledPlugins, id) {
						s.set.EnabledPlugins = append(slices.Clone(s.set.EnabledPlugins), id)
						s.save()
					}
				}
			}
			s.actionToasts = append(s.actionToasts, use)
		})
	}()
}

// uninstall removes an add-on, after making sure nothing uses it.
func (s *shell) uninstall(kind addons.Kind, e addons.Entry) {
	switch kind {
	case addons.KindTheme:
		if s.set.Theme == e.ID {
			s.set.Theme = "mix"
			s.save()
		}
	case addons.KindPlugin:
		s.set.EnabledPlugins = slices.DeleteFunc(slices.Clone(s.set.EnabledPlugins), func(id string) bool { return id == e.ID })
		s.save()
	}
	if _, err := s.svc.Addons.Uninstall(kind, e.ID); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	s.notes = append(s.notes, notice("Removed "+e.Name))
}
