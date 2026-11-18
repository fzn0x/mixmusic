package settings

import (
	"slices"
	"testing"
)

// TestHeadphonesClean keeps the filters of a type a correction plays, each
// value in its range, up to twenty of them, and turns a correction of none
// into none.
func TestHeadphonesClean(t *testing.T) {
	h := Headphones{ID: "a/b/c", Name: "c", Preamp: -40, Filters: []HeadphoneFilter{
		{"BP", 1000, 1, 1},
		{FilterPeak, 0, 40, 0},
		{FilterLowPass, 30000, 6, 200},
	}}
	got := h.clean()
	want := []HeadphoneFilter{{FilterPeak, MinFilterHz, MaxFilterGain, MinFilterQ}, {FilterLowPass, MaxFilterHz, 0, MaxFilterQ}}
	if got.Preamp != -MaxFilterGain || !slices.Equal(got.Filters, want) || got.ID != "a/b/c" {
		t.Errorf("cleaned to %+v", got)
	}
	h.Filters = make([]HeadphoneFilter, 25)
	for i := range h.Filters {
		h.Filters[i] = HeadphoneFilter{FilterPeak, 100, 1, 1}
	}
	if got := h.clean(); len(got.Filters) != MaxHeadphoneFilters {
		t.Errorf("25 filters cleaned to %d", len(got.Filters))
	}
	for _, h := range []Headphones{
		{Name: "no ID", Filters: []HeadphoneFilter{{FilterPeak, 100, 1, 1}}},
		{ID: "x", Filters: []HeadphoneFilter{{"BP", 100, 1, 1}}},
		{ID: "x", Preamp: -3},
	} {
		if got := h.clean(); got.On() || got.Name != "" || got.Filters == nil || len(got.Filters) != 0 {
			t.Errorf("%+v cleaned to %+v", h, got)
		}
	}
	if d := Default().Equalizer.Headphones; d.On() || d.Filters == nil {
		t.Errorf("the default: %+v", d)
	}
}
