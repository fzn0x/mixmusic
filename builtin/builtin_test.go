package builtin

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// TestThemesAreValid loads the built-in themes as the app does.
func TestThemesAreValid(t *testing.T) {
	inv := addons.NewManager("", FS).List()
	for _, p := range inv.Problems {
		t.Errorf("%s %s: %s", p.Kind, p.Folder, p.Error)
	}
	ids := map[string]bool{}
	for _, th := range inv.Themes {
		ids[th.ID] = true
		if !th.Builtin {
			t.Errorf("%s is not marked built-in", th.ID)
		}
	}
	if !ids["mix"] {
		t.Error("the default theme, mix, is missing")
	}
	if len(ids) < 2 {
		t.Errorf("only %d built-in themes", len(ids))
	}
}
