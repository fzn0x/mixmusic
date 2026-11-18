package settings

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestCollapsedFoldersClean keeps each folder closed once, of a thousand
// and more, without the empty IDs and those too long.
func TestCollapsedFoldersClean(t *testing.T) {
	ids := []string{"a", "", "a", strings.Repeat("x", maxFolderID+1), "b"}
	for i := range maxCollapsedFolders + 10 {
		ids = append(ids, fmt.Sprintf("f%d", i))
	}
	l := defaultLibrary()
	l.CollapsedFolders = ids
	got := l.clean(defaultLibrary()).CollapsedFolders
	if len(got) != maxCollapsedFolders || !slices.Equal(got[:3], []string{"a", "b", "f0"}) {
		t.Errorf("kept %d folders closed, the first %q", len(got), got[:3])
	}
	if &got[0] == &ids[0] {
		t.Error("clean shares the slice it cleaned")
	}
	if got := defaultLibrary().clean(defaultLibrary()).CollapsedFolders; got == nil || len(got) != 0 {
		t.Errorf("none closed cleans to %#v", got)
	}
}
