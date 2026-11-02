package settings

import (
	"slices"
	"strings"
)

// DefaultRegistryURL is the community registry in MixMusic's repository.
const DefaultRegistryURL = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/community/registry-2.json"

// WebRegistryURL is the registry of the add-ons of the web interface,
// which the versions of MixMusic before its own interface read, and kept
// in their settings.
const WebRegistryURL = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/community/registry.json"

// Addons are the plugins that run, and where more come from.
type Addons struct {
	// EnabledPlugins are the IDs of the plugins that run.
	EnabledPlugins []string `json:"enabledPlugins"`
	// RegistryURL is the address of the community registry of themes and
	// plugins.
	RegistryURL string `json:"registryUrl"`
}

// defaultAddons are the add-ons of a new install: no plugin runs.
func defaultAddons() Addons {
	var a Addons
	a.EnabledPlugins = []string{}
	a.RegistryURL = DefaultRegistryURL
	return a
}

// clean is a with each plugin once, and d's registry in place of one that
// isn't on HTTPS, or is the web interface's.
func (a Addons) clean(d Addons) Addons {
	plugins := []string{}
	for _, id := range a.EnabledPlugins {
		if ValidID(id) && !slices.Contains(plugins, id) {
			plugins = append(plugins, id)
		}
	}
	a.EnabledPlugins = plugins
	if !strings.HasPrefix(a.RegistryURL, "https://") || a.RegistryURL == WebRegistryURL {
		a.RegistryURL = d.RegistryURL
	}
	return a
}
