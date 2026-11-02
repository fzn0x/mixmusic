package settings

// Keys are the keys of the commands: Settings › Shortcuts.
type Keys struct {
	// Shortcuts replaces the keys of commands, by command ID; an empty
	// value takes a command's shortcut away.
	Shortcuts map[string]string `json:"shortcuts"`
	// GlobalShortcuts are the keys that run commands whichever window is
	// in front, by command ID, written as Shortcuts are: "Mod+Alt+P".
	// A new install has none.
	GlobalShortcuts map[string]string `json:"globalShortcuts"`
}

// defaultKeys are the keys of a new install: the commands' own.
func defaultKeys() Keys {
	var k Keys
	k.Shortcuts = map[string]string{}
	k.GlobalShortcuts = map[string]string{}
	return k
}

// clean is k without entries too long or too many.
func (k Keys) clean() Keys {
	shortcuts := map[string]string{}
	for id, keys := range k.Shortcuts {
		if id != "" && len(id) <= 128 && len(keys) <= 64 && len(shortcuts) < 500 {
			shortcuts[id] = keys
		}
	}
	k.Shortcuts = shortcuts
	// IDs this version doesn't know stay, for a version that does.
	global := map[string]string{}
	for id, keys := range k.GlobalShortcuts {
		if id != "" && len(id) <= 128 && keys != "" && len(keys) <= 64 && len(global) < 32 {
			global[id] = keys
		}
	}
	k.GlobalShortcuts = global
	return k
}
