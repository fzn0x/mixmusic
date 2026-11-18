package settings

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestTagFilterClean keeps each tag chosen once, of two hundred at most,
// without the empty IDs and those too long, and shows the tags' chips on
// a new install.
func TestTagFilterClean(t *testing.T) {
	ids := []string{"a", "", "a", strings.Repeat("x", maxTagID+1), "b"}
	for i := range maxTagFilter + 10 {
		ids = append(ids, fmt.Sprintf("t%d", i))
	}
	l := defaultLibrary()
	l.TagFilter = ids
	got := l.clean(defaultLibrary()).TagFilter
	if len(got) != maxTagFilter || !slices.Equal(got[:3], []string{"a", "b", "t0"}) {
		t.Errorf("kept %d tags chosen, the first %q", len(got), got[:3])
	}
	if &got[0] == &ids[0] {
		t.Error("clean shares the slice it cleaned")
	}
	d := defaultLibrary().clean(defaultLibrary())
	if d.TagFilter == nil || len(d.TagFilter) != 0 || !d.TagFilters || d.TagsMatchAll {
		t.Errorf("a new install chooses %#v, shows the chips %v, matches all %v", d.TagFilter, d.TagFilters, d.TagsMatchAll)
	}
}
