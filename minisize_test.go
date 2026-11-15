package main

import "testing"

// TestMiniSize: the mini player's window opens at its size at the zoom,
// with its least and largest, the first time it opens, where it took its
// size at 100% and its cover became a strip at 125% (#717).
func TestMiniSize(t *testing.T) {
	type size struct{ w, h, minW, minH, maxH int }
	for z, want := range map[float64]size{
		1:    {420, 132, 340, 112, 180},
		1.25: {525, 165, 425, 140, 225},
		2:    {840, 264, 680, 224, 360},
		0.8:  {336, 106, 272, 90, 144},
		0:    {420, 132, 340, 112, 180},
	} {
		var got size
		got.w, got.h, got.minW, got.minH, got.maxH = miniSize(z)
		if got != want {
			t.Errorf("at %v%% the mini player opens at %+v, want %+v", z*100, got, want)
		}
	}
}
