package settings

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// The ParametricEQ.txt of a correction of headphones (#616), and the
// correction as settings keep it.

// peqProfile is a profile as AutoEq writes them, of made-up values.
const peqProfile = `Preamp: -6.1 dB
Filter 1: ON LSC Fc 105 Hz Gain 5.5 dB Q 0.70
Filter 2: ON PK Fc 180 Hz Gain -2.4 dB Q 0.55
Filter 3: ON PK Fc 1450 Hz Gain 3.1 dB Q 1.90
Filter 4: ON PK Fc 5200 Hz Gain -4.8 dB Q 4.20
Filter 5: ON HSC Fc 10000 Hz Gain -1.5 dB Q 0.70
`

// TestParseParametricEQ reads AutoEq's file: its preamp, and its filters
// in order.
func TestParseParametricEQ(t *testing.T) {
	h, err := ParseParametricEQ([]byte(peqProfile))
	if err != nil {
		t.Fatal(err)
	}
	want := Headphones{Preamp: -6.1, Filters: []HeadphoneFilter{
		{FilterLowShelf, 105, 5.5, 0.7},
		{FilterPeak, 180, -2.4, 0.55},
		{FilterPeak, 1450, 3.1, 1.9},
		{FilterPeak, 5200, -4.8, 4.2},
		{FilterHighShelf, 10000, -1.5, 0.7},
	}}
	if h.Preamp != want.Preamp || !slices.Equal(h.Filters, want.Filters) || h.ID != "" || h.Name != "" {
		t.Errorf("read %+v, want %+v", h, want)
	}
}

// TestParseParametricEQForms reads what Equalizer APO's format allows
// besides: Windows' line ends and BOM, comments, blank lines, tabs and
// runs of spaces, filters unnumbered or off or of the type None, lower
// case, decimal commas, units next to their numbers, frequencies in
// thousands as some files write them, the other names of the types,
// a low-pass or high-pass without a Q or with a gain, and preamps that add
// up.
func TestParseParametricEQForms(t *testing.T) {
	for _, c := range []struct {
		name, file string
		preamp     float64
		filters    []HeadphoneFilter
	}{
		{"windows", "\uFEFFPreamp: -3 dB\r\nFilter 1: ON PK Fc 100 Hz Gain 2 dB Q 1\r\n", -3,
			[]HeadphoneFilter{{FilterPeak, 100, 2, 1}}},
		{"comments and blanks", "# made by hand\n\n  \nFilter: ON PK Fc 100 Hz Gain 2 dB Q 1\n# the end\n", 0,
			[]HeadphoneFilter{{FilterPeak, 100, 2, 1}}},
		{"tabs and spaces", "Filter  1:\tON\tPK   Fc  63.0 Hz  Gain  -5.0 dB  Q  4.00", 0,
			[]HeadphoneFilter{{FilterPeak, 63, -5, 4}}},
		{"off and none", "Filter 1: OFF PK Fc 100 Hz Gain 2 dB Q 1\nFilter 2: ON None\nFilter 3: ON HSC Fc 8000 Hz Gain -2 dB Q 0.7", 0,
			[]HeadphoneFilter{{FilterHighShelf, 8000, -2, 0.7}}},
		{"lower case", "preamp: -2 db\nfilter 1: on pk fc 100 hz gain 2 db q 1", -2,
			[]HeadphoneFilter{{FilterPeak, 100, 2, 1}}},
		{"decimal commas", "Preamp: -2,5 dB\nFilter 1: ON PK Fc 100,5 Hz Gain -1,5 dB Q 0,8", -2.5,
			[]HeadphoneFilter{{FilterPeak, 100.5, -1.5, 0.8}}},
		{"units next to numbers", "Preamp: -2dB\nFilter 1: ON PK Fc 100Hz Gain 1.5dB Q 2", -2,
			[]HeadphoneFilter{{FilterPeak, 100, 1.5, 2}}},
		{"parameters in any order", "Filter 1: ON PK Q 2 Gain 1.5 dB Fc 100 Hz", 0,
			[]HeadphoneFilter{{FilterPeak, 100, 1.5, 2}}},
		{"thousands", "Filter 1: ON PK Fc 1.250 Hz Gain 1 dB Q 2\nFilter 2: ON PK Fc 12,500 Hz Gain 1 dB Q 2", 0,
			[]HeadphoneFilter{{FilterPeak, 1250, 1, 2}, {FilterPeak, 12500, 1, 2}}},
		{"other names", "Filter 1: ON PEQ Fc 100 Hz Gain 1 dB Q 2\nFilter 2: ON LPQ Fc 18000 Hz Q 0.5\nFilter 3: ON HPQ Fc 25 Hz Q 1.2", 0,
			[]HeadphoneFilter{{FilterPeak, 100, 1, 2}, {FilterLowPass, 18000, 0, 0.5}, {FilterHighPass, 25, 0, 1.2}}},
		{"passes", "Filter 1: ON LP Fc 18000 Hz\nFilter 2: ON HP Fc 30 Hz Gain 3 dB", 0,
			[]HeadphoneFilter{{FilterLowPass, 18000, 0, math.Sqrt2 / 2}, {FilterHighPass, 30, 0, math.Sqrt2 / 2}}},
		{"preamps add up", "Preamp: -3 dB\nFilter 1: ON PK Fc 100 Hz Gain 1 dB Q 2\nPreamp: -1.5 dB", -4.5,
			[]HeadphoneFilter{{FilterPeak, 100, 1, 2}}},
		{"no preamp", "Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 2", 0,
			[]HeadphoneFilter{{FilterPeak, 100, 1, 2}}},
		{"the edges of the ranges", "Preamp: 30 dB\nFilter 1: ON PK Fc 1 Hz Gain -30 dB Q 0.01\nFilter 2: ON PK Fc 22000 Hz Gain 30 dB Q 100", 30,
			[]HeadphoneFilter{{FilterPeak, 1, -30, 0.01}, {FilterPeak, 22000, 30, 100}}},
		{"exponents", "Filter 1: ON PK Fc 1e3 Hz Gain -2.5E0 dB Q .5", 0,
			[]HeadphoneFilter{{FilterPeak, 1000, -2.5, 0.5}}},
	} {
		h, err := ParseParametricEQ([]byte(c.file))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if h.Preamp != c.preamp || !slices.Equal(h.Filters, c.filters) {
			t.Errorf("%s: read %v dB and %+v, want %v dB and %+v", c.name, h.Preamp, h.Filters, c.preamp, c.filters)
		}
	}
}

// TestParseParametricEQErrors stops at the first line that doesn't read,
// and tells which, and why.
func TestParseParametricEQErrors(t *testing.T) {
	for _, c := range []struct {
		file  string
		line  int
		why   ParametricEQWhy
		value string
	}{
		{"Channel: L\nFilter 1: ON PK Fc 100 Hz Gain 1 dB Q 2", 1, NotParametricEQ, ""},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 2\nDevice: Speakers", 2, NotParametricEQ, ""},
		{"Filter Settings file\nFilter 1: ON PK Fc 100 Hz Gain 1 dB Q 2", 1, NotParametricEQ, ""},
		{"Filter 1: PK Fc 100 Hz Gain 1 dB Q 2", 1, NotParametricEQ, ""},
		{"Filter 1: ON", 1, NotParametricEQ, ""},
		{"Filter 1:", 1, NotParametricEQ, ""},
		{"Filter one: ON PK Fc 100 Hz Gain 1 dB Q 2", 1, NotParametricEQ, ""},
		{"Filter 1: ON BP Fc 1000 Hz Q 1", 1, UnknownFilter, "BP"},
		{"Filter 1: ON LS Fc 100 Hz Gain 3 dB", 1, UnknownFilter, "LS"},
		{"Filter 1: ON NO Fc 800 Hz", 1, UnknownFilter, "NO"},
		{"Filter 1: ON IIR Order 1 Coefficients 1 0 1 0", 1, UnknownFilter, "IIR"},
		{"Filter 1: ON PK Gain 1 dB Q 2", 1, MissingParameter, "Fc"},
		{"Filter 1: ON LSC Fc 100 Hz Q 0.7", 1, MissingParameter, "Gain"},
		{"\n\nFilter 3: ON PK Fc 100 Hz Gain 1 dB", 3, MissingParameter, "Q"},
		{"Filter 1: ON LP", 1, MissingParameter, "Fc"},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 2 Fc 200 Hz", 1, ParameterTwice, "Fc"},
		{"Filter 1: ON PK Fc 30000 Hz Gain 1 dB Q 2", 1, OutOfRange, "Fc"},
		{"Filter 1: ON PK Fc 0 Hz Gain 1 dB Q 2", 1, OutOfRange, "Fc"},
		{"Filter 1: ON PK Fc 105.500 Hz Gain 1 dB Q 2", 1, OutOfRange, "Fc"},
		{"Filter 1: ON PK Fc 100 Hz Gain 31 dB Q 2", 1, OutOfRange, "Gain"},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 0", 1, OutOfRange, "Q"},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 101", 1, OutOfRange, "Q"},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q -1", 1, OutOfRange, "Q"},
		{"Filter 1: ON PK Fc 1.2.3 Hz Gain 1 dB Q 1", 1, LeftOver, "Fc 1.2.3 Hz Gain 1 dB Q 1"},
		{"Preamp: loud", 1, OutOfRange, "Preamp"},
		{"Preamp: -31 dB\nFilter 1: ON PK Fc 100 Hz Gain 1 dB Q 2", 1, OutOfRange, "Preamp"},
		{"Preamp: -20 dB\nPreamp: -20 dB", 2, OutOfRange, "Preamp"},
		{"Preamp: -3 dB extra", 1, OutOfRange, "Preamp"},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB BW Oct 0.5", 1, Bandwidth, ""},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 2 T60 target 100 ms", 1, LeftOver, "T60 target 100 ms"},
		{"Filter 1: ON LSC 12 dB Fc 300 Hz Gain 5.0 dB", 1, LeftOver, "12 dB Fc 300 Hz Gain 5.0 dB"},
		{"Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 2x", 1, LeftOver, "Q 2x"},
	} {
		h, err := ParseParametricEQ([]byte(c.file))
		var pe *ParametricEQError
		if !errors.As(err, &pe) {
			t.Errorf("%q: %v, want a line's error", c.file, err)
			continue
		}
		if pe.Line != c.line || pe.Why != c.why || pe.Value != c.value {
			t.Errorf("%q: line %d, why %d, of %q; want line %d, why %d, of %q", c.file, pe.Line, pe.Why, pe.Value, c.line, c.why, c.value)
		}
		if h.On() || len(h.Filters) != 0 || h.Preamp != 0 {
			t.Errorf("%q: read %+v besides", c.file, h)
		}
	}
}

// TestParseParametricEQFiles turns down a file of no filters, of more than
// twenty, or too big; up to twenty read.
func TestParseParametricEQFiles(t *testing.T) {
	filters := func(n int) string {
		var b strings.Builder
		b.WriteString("Preamp: -6 dB\n")
		for i := range n {
			fmt.Fprintf(&b, "Filter %d: ON PK Fc %d Hz Gain 1 dB Q 1\n", i+1, 100+i*100)
		}
		return b.String()
	}
	if h, err := ParseParametricEQ([]byte(filters(MaxHeadphoneFilters))); err != nil || len(h.Filters) != MaxHeadphoneFilters {
		t.Errorf("twenty filters: %d, %v", len(h.Filters), err)
	}
	var many *TooManyFiltersError
	if _, err := ParseParametricEQ([]byte(filters(MaxHeadphoneFilters + 4))); !errors.As(err, &many) || many.Count != 24 {
		t.Errorf("24 filters: %v", err)
	}
	for _, file := range []string{"", "\n\n", "# nothing\n", "Preamp: -3 dB\n", "Filter 1: OFF PK Fc 100 Hz Gain 1 dB Q 2\nFilter 2: ON None"} {
		if _, err := ParseParametricEQ([]byte(file)); !errors.Is(err, ErrNoFilters) {
			t.Errorf("%q: %v, want no filters", file, err)
		}
	}
	big := strings.Repeat("# "+strings.Repeat("x", 98)+"\n", 700)
	if _, err := ParseParametricEQ([]byte(big)); !errors.Is(err, ErrParametricEQSize) {
		t.Errorf("a file of %d bytes: %v", len(big), err)
	}
}

// TestParametricEQErrorWords tells the line, cut to 60 characters, and
// valid UTF-8 of a binary file's.
func TestParametricEQErrorWords(t *testing.T) {
	_, err := ParseParametricEQ([]byte("Filter 1: ON PK Fc 100 Hz Gain 1 dB Q 2\nFilter 2: ON PK Fc 99999 Hz Gain 1 dB Q 2"))
	if want := `line 2, "Filter 2: ON PK Fc 99999 Hz Gain 1 dB Q 2", has a Fc that isn't a number from 1 Hz to 22000 Hz`; err == nil || err.Error() != want {
		t.Errorf("%v, want %s", err, want)
	}
	_, err = ParseParametricEQ([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\xff", 100)))
	var pe *ParametricEQError
	if !errors.As(err, &pe) || pe.Line != 1 || !strings.HasPrefix(pe.Text, "\uFFFDPNG") {
		t.Fatalf("a PNG: %v", err)
	}
	long := "Device: " + strings.Repeat("y", 100)
	_, err = ParseParametricEQ([]byte(long))
	if !errors.As(err, &pe) || len([]rune(pe.Text)) != 60 || !strings.HasSuffix(pe.Text, "…") {
		t.Errorf("a long line: %q", pe.Text)
	}
	for _, c := range []struct {
		name   string
		lo, hi string
	}{{"Fc", "1 Hz", "22000 Hz"}, {"Gain", "-30 dB", "30 dB"}, {"Preamp", "-30 dB", "30 dB"}, {"Q", "0.01", "100"}} {
		if lo, hi := ParameterRange(c.name); lo != c.lo || hi != c.hi {
			t.Errorf("%s: from %s to %s", c.name, lo, hi)
		}
	}
}
