package engine

import (
	"slices"
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDesktopAdapter_CaptureSwapsItsBuffers(t *testing.T) {
	a := &DesktopAdapter{}
	key := func(k gpucontext.Key) gpucontext.InputEvent {
		return gpucontext.KeyEvent{Key: k, Pressed: true}
	}
	collect := func(evs ...gpucontext.InputEvent) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.pending = append(a.pending, evs...)
	}
	frame := func(during ...gpucontext.InputEvent) []gpucontext.InputEvent {
		var got []gpucontext.InputEvent
		a.drain(func(ev gpucontext.InputEvent) {
			got = append(got, ev)
			collect(during...)
			during = nil
		})
		return got
	}
	a1, b1 := key(gpucontext.KeyA), key(gpucontext.KeyB)
	collect(a1, b1)
	if got := frame(); !slices.Equal(got, []gpucontext.InputEvent{a1, b1}) {
		t.Fatalf("the first frame delivers %v, want A and B", got)
	}
	if got := frame(); len(got) != 0 {
		t.Fatalf("a frame without input delivers %v again", got)
	}
	kept := a.pending[:cap(a.pending)]
	if len(kept) != 2 {
		t.Fatalf("the array to fill next has room for %d, want the 2 drained before", len(kept))
	}
	c1, d1 := key(gpucontext.KeyC), key(gpucontext.KeyD)
	collect(c1)
	if &a.pending[0] != &kept[0] {
		t.Error("the next frame's events go into a fresh array, want the one drained before")
	}
	if got := frame(d1); !slices.Equal(got, []gpucontext.InputEvent{c1}) {
		t.Errorf("the frame delivers %v, want C alone: D came while it was consumed", got)
	}
	if got := frame(); !slices.Equal(got, []gpucontext.InputEvent{d1}) {
		t.Errorf("the next frame delivers %v, want the D that came during the last", got)
	}
}
