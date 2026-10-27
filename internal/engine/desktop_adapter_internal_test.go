package engine

import (
	"slices"
	"testing"

	"github.com/gogpu/gpucontext"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
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

func pointerAt(x, y, dx, dy float64) gpucontext.PointerEvent {
	return gpucontext.PointerEvent{Type: gpucontext.PointerMove, X: x, Y: y, DeltaX: dx, DeltaY: dy}
}

// a pass of the pointer's events, folded into e as Capture folds them
func pass(p *pointer, e *control.InputEvents, w, h int, fills bool,
	evs ...gpucontext.PointerEvent) {
	p.begin()
	for _, ev := range evs {
		p.take(ev, w, h, fills)
	}
	p.apply(e)
}

func TestPointer_FreeItMovesByItsPlace(t *testing.T) {
	var p pointer
	e := &control.InputEvents{MousePos: geom.NewVec(10, 10)}
	pass(&p, e, 800, 600, false, pointerAt(30, 25, 0, 0))
	if e.MousePos != geom.NewVec(30, 25) || e.CursorDelta != geom.NewVec(20, 15) {
		t.Errorf("the cursor at %v moved by %v, want (30, 25) by (20, 15)", e.MousePos,
			e.CursorDelta)
	}
	pass(&p, e, 800, 600, false)
	if e.MousePos != geom.NewVec(30, 25) {
		t.Errorf("a pass without events moved the cursor to %v", e.MousePos)
	}
}

func TestPointer_CaughtItMovesByTheWindowsDeltasAndStaysPut(t *testing.T) {
	var p pointer
	e := &control.InputEvents{MousePos: geom.NewVec(400, 300)}
	p.catch(true)
	pass(&p, e, 800, 600, false, pointerAt(403, 298, 3, -2), pointerAt(400, 300, 0, 0))
	if e.CursorDelta != geom.NewVec(3, -2) || e.MousePos != geom.NewVec(400, 300) || !p.locks {
		t.Errorf("caught, the cursor moved by %v to %v, the window locking %v; want (3, -2), "+
			"the place kept, the lock noted", e.CursorDelta, e.MousePos, p.locks)
	}
	// the window's warp back to the middle, without a delta, is no movement
	pass(&p, e, 800, 600, false, pointerAt(400, 300, 0, 0))
	if e.CursorDelta != (geom.Vec{}) {
		t.Errorf("an event at the middle without a delta moved the cursor by %v", e.CursorDelta)
	}
}

func TestPointer_CaughtWithoutDeltasItFollowsItsPlaceFromTheSecondPass(t *testing.T) {
	var p pointer
	e := &control.InputEvents{MousePos: geom.NewVec(400, 300)}
	p.catch(true)
	pass(&p, e, 800, 600, false, pointerAt(100, 100, 0, 0))
	if e.CursorDelta != (geom.Vec{}) {
		t.Errorf("the first place caught moved the cursor by %v, want nothing", e.CursorDelta)
	}
	pass(&p, e, 800, 600, false, pointerAt(105, 100, 0, 0), pointerAt(110, 95, 0, 0))
	if e.CursorDelta != geom.NewVec(10, -5) || e.MousePos != geom.NewVec(400, 300) || p.locks {
		t.Errorf("the place moved: the cursor by %v to %v, the window locking %v; want (10, -5), "+
			"the place kept, no lock", e.CursorDelta, e.MousePos, p.locks)
	}
	p.catch(false)
	p.catch(true)
	pass(&p, e, 800, 600, false, pointerAt(50, 50, 0, 0))
	if e.CursorDelta != (geom.Vec{}) {
		t.Errorf("caught again, the first place moved the cursor by %v, want nothing",
			e.CursorDelta)
	}
}

func TestPointer_TheWindowHasTheCursorFromItsFirstEventTillItLeaves(t *testing.T) {
	var p pointer
	e := &control.InputEvents{}
	pass(&p, e, 800, 600, false)
	if e.CursorInWindow {
		t.Fatal("the window has the cursor before any event of it")
	}
	pass(&p, e, 800, 600, false, pointerAt(30, 25, 0, 0))
	if !e.CursorInWindow {
		t.Fatal("a move came and the window has no cursor")
	}
	leave := gpucontext.PointerEvent{Type: gpucontext.PointerLeave, X: 30, Y: 25}
	pass(&p, e, 800, 600, false, leave)
	if e.CursorInWindow || e.MousePos != geom.NewVec(30, 25) || e.CursorDelta != (geom.Vec{}) {
		t.Fatalf("after the leave the window has the cursor %v at %v, moved by %v; want none, "+
			"where it was last seen, unmoved", e.CursorInWindow, e.MousePos, e.CursorDelta)
	}
	pass(&p, e, 800, 600, false)
	if e.CursorInWindow {
		t.Fatal("a pass without events gave the window the cursor back")
	}
	enter := gpucontext.PointerEvent{Type: gpucontext.PointerEnter, X: 50, Y: 40}
	pass(&p, e, 800, 600, false, enter)
	if !e.CursorInWindow || e.MousePos != geom.NewVec(50, 40) {
		t.Errorf("after the enter the window has the cursor %v at %v, want it at (50, 40)",
			e.CursorInWindow, e.MousePos)
	}
}

func TestPointer_FillingTheScreenTheFarEdgeIsTheLastRowAndColumn(t *testing.T) {
	var p pointer
	e := &control.InputEvents{}
	if at := p.take(pointerAt(800, 600, 0, 0), 800, 600, true); at != geom.NewVec(799, 599) {
		t.Errorf("the place (800, 600) in a window 800 by 600 filling the screen is %v, want "+
			"(799, 599)", at)
	}
	p.apply(e)
	if e.MousePos != geom.NewVec(799, 599) {
		t.Errorf("the cursor at %v, want (799, 599)", e.MousePos)
	}
	if at := p.take(pointerAt(800, 600, 0, 0), 800, 600, false); at != geom.NewVec(800, 600) {
		t.Errorf("in a window, the place (800, 600) is %v, want it as reported", at)
	}
}
