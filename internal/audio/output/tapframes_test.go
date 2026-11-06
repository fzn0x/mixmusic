package output

import (
	"slices"
	"testing"
	"unsafe"
)

// twoBuffers is an AudioBufferList of two buffers, as Core Audio lays it
// out.
type twoBuffers struct {
	head   audioBufferList
	second audioBuffer
}

func bufferOf(channels int, samples []float32) audioBuffer {
	return audioBuffer{channels: uint32(channels), size: uint32(4 * len(samples)), data: unsafe.Pointer(&samples[0])}
}

// TestAppendStereo reads a tap's buffers as interleaved stereo: one buffer
// of interleaved channels, a buffer for each channel, and one channel on
// both sides, no more than out has room for.
func TestAppendStereo(t *testing.T) {
	interleaved := []float32{1, -1, 2, -2, 3, -3}
	l := audioBufferList{buffers: 1, first: bufferOf(2, interleaved)}
	if got := appendStereo(make([]float32, 0, 16), &l); !slices.Equal(got, interleaved) {
		t.Errorf("interleaved stereo reads as %v", got)
	}

	left, right := []float32{1, 2, 3}, []float32{-1, -2, -3}
	planar := twoBuffers{head: audioBufferList{buffers: 2, first: bufferOf(1, left)}, second: bufferOf(1, right)}
	if got := appendStereo(make([]float32, 0, 16), &planar.head); !slices.Equal(got, interleaved) {
		t.Errorf("a buffer for each channel reads as %v", got)
	}

	mono := audioBufferList{buffers: 1, first: bufferOf(1, left)}
	if got, want := appendStereo(make([]float32, 0, 16), &mono), []float32{1, 1, 2, 2, 3, 3}; !slices.Equal(got, want) {
		t.Errorf("one channel reads as %v, want %v", got, want)
	}

	// 5.1, of which the first two channels are left and right.
	surround := []float32{1, -1, 9, 9, 9, 9, 2, -2, 9, 9, 9, 9}
	l = audioBufferList{buffers: 1, first: bufferOf(6, surround)}
	if got, want := appendStereo(make([]float32, 0, 16), &l), []float32{1, -1, 2, -2}; !slices.Equal(got, want) {
		t.Errorf("six channels read as %v, want %v", got, want)
	}

	l = audioBufferList{buffers: 1, first: bufferOf(2, interleaved)}
	if got, want := appendStereo(make([]float32, 0, 4), &l), interleaved[:4]; !slices.Equal(got, want) {
		t.Errorf("into room for two frames, read %v, want %v", got, want)
	}

	empty := audioBufferList{buffers: 1}
	if got := appendStereo(make([]float32, 0, 16), &empty); len(got) != 0 {
		t.Errorf("a buffer without data reads as %v", got)
	}
}
