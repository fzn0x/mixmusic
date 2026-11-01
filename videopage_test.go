package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/grafana/sobek"
)

// pageLogic runs public/video.js without a document, which defines only
// mvLogic, and returns a function that evaluates an expression of it and
// decodes what it gives into out.
func pageLogic(t *testing.T) func(expr string, out any) {
	t.Helper()
	src, err := os.ReadFile("public/video.js")
	if err != nil {
		t.Fatal(err)
	}
	vm := sobek.New()
	if _, err := vm.RunScript("video.js", string(src)); err != nil {
		t.Fatal(err)
	}
	return func(expr string, out any) {
		t.Helper()
		v, err := vm.RunString("JSON.stringify(" + expr + ")")
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if err := json.Unmarshal([]byte(v.String()), out); err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
	}
}

// TestPageFetchesWhatPlaysNext has the music video's page fetch the parts
// of the picture that play next, ahead of what shows by half a minute at
// most, and none it holds.
func TestPageFetchesWhatPlaysNext(t *testing.T) {
	eval := pageLogic(t)
	// Twelve parts of five seconds: [time, duration, first byte, last byte].
	const parts = `Array.from({length: 12}, (_, i) => [5 * i, 5, 1000 * i, 1000 * i + 999])`
	for _, c := range []struct {
		name   string
		ranges string
		t      float64
		skip   string
		want   int
	}{
		{"nothing held", `[]`, 0, "", 0},
		{"the first two held", `[[0, 10]]`, 0, "", 2},
		{"held to half a minute ahead", `[[0, 35]]`, 0, "", -1},
		{"held to just short of it", `[[0, 31]]`, 0, "", 6},
		{"after a seek", `[[0, 10]]`, 32, "", 6},
		{"in the middle of a part not held", `[[0, 10]]`, 12, "", 2},
		{"late in a part held up to now", `[[0, 14.5]]`, 14, "", 3},
		{"a gap ahead", `[[0, 10], [15, 40]]`, 1, "", 2},
		{"a part left out", `[]`, 0, "(i) => i === 0", 1},
		{"past the end", `[]`, 60, "", -1},
		{"near the end", `[[0, 57]]`, 50, "", 11},
	} {
		skip := "undefined"
		if c.skip != "" {
			skip = c.skip
		}
		var got int
		eval("mvLogic.nextPart("+parts+", "+c.ranges+", "+jsNum(c.t)+", mvLogic.AHEAD, "+skip+")", &got)
		if got != c.want {
			t.Errorf("%s: part %d, want %d", c.name, got, c.want)
		}
	}
	var before float64
	for _, c := range []struct {
		ranges string
		t      float64
		want   float64
	}{
		{`[[0, 50]]`, 40, 10},
		{`[[0, 50]]`, 30, 0},
		{`[[9.5, 50]]`, 40, 0},
		{`[]`, 40, 0},
	} {
		eval("mvLogic.stale("+c.ranges+", "+jsNum(c.t)+", mvLogic.BEHIND)", &before)
		if before != c.want {
			t.Errorf("held %s at %v: drops before %v, want %v", c.ranges, c.t, before, c.want)
		}
	}
}

// TestPageFollowsTheSound keeps the picture in time with the time the Go
// side sends, which moves on from when it was read while the sound plays.
func TestPageFollowsTheSound(t *testing.T) {
	eval := pageLogic(t)
	var at float64
	for _, c := range []struct {
		clock string
		now   float64
		want  float64
	}{
		{`{at: 10, sentAt: 1000, playing: true}`, 1500, 10.5},
		{`{at: 10, sentAt: 1000, playing: false}`, 1500, 10},
		{`{at: 10, sentAt: 1000, playing: true}`, 900, 10},
	} {
		eval("mvLogic.target("+c.clock+", "+jsNum(c.now)+")", &at)
		if at != c.want {
			t.Errorf("%s at %v: %v, want %v", c.clock, c.now, at, c.want)
		}
	}
	type fix struct {
		Seek bool
		Rate float64
	}
	for _, c := range []struct {
		d    float64
		want fix
	}{
		{0.5, fix{true, 1}},
		{-0.4, fix{true, 1}},
		{0.1, fix{false, 0.95}},  // ahead: slower
		{-0.1, fix{false, 1.05}}, // behind: faster
		{0.3, fix{false, 0.92}},  // at most 8%
		{-0.3, fix{false, 1.08}},
		{0.01, fix{false, 1}},
		{-0.02, fix{false, 1}},
	} {
		var got fix
		eval("mvLogic.correction("+jsNum(c.d)+")", &got)
		if got != c.want {
			t.Errorf("%v ahead: %+v, want %+v", c.d, got, c.want)
		}
	}
}

func jsNum(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
