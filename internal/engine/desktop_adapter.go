package engine

import (
	"sync"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

// InputAdapter captures one frame's raw input into events.
type InputAdapter interface {
	Capture(e *control.InputEvents)
}

// DesktopAdapter captures the input of the engine's window (gogpu); without a window it captures
// nothing. The window hands out its events a pass of its loop at a time and forgets them at the
// next pass, drawn or not: Collect keeps them, every pass, for the tick's Capture.
type DesktopAdapter struct {
	app      *gogpu.App
	captured bool // the cursor is caught: its moves come as deltas, its place stays put

	mu      sync.Mutex
	pending []gpucontext.InputEvent
}

// Collect keeps the events of the window's pass for the next Capture; the engine calls it every
// pass of the window's loop.
func (a *DesktopAdapter) Collect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ev, ok := a.app.PollInputEvent(); ok; ev, ok = a.app.PollInputEvent() {
		a.pending = append(a.pending, ev)
	}
}

func (a *DesktopAdapter) Capture(e *control.InputEvents) {
	e.CursorDelta = geom.Vec{}
	e.ScrollDelta = 0
	if a.app == nil {
		return
	}
	a.mu.Lock()
	events := a.pending
	a.pending = nil
	a.mu.Unlock()
	pos := e.MousePos
	var moved, scroll geom.Vec
	for _, ev := range events {
		switch ev := ev.(type) {
		case gpucontext.KeyEvent:
			if k := keyOf(ev.Key); k != control.KeyUnknown {
				action := control.ActionRelease
				if ev.Pressed {
					action = control.ActionPress
				}
				e.AddKeyEvent(k, action)
			}
		case gpucontext.PointerEvent:
			moved = geom.NewVec(moved.X+ev.DeltaX, moved.Y+ev.DeltaY)
			if !a.captured {
				pos = geom.NewVec(ev.X, ev.Y)
			}
			if b, ok := buttonOf(ev.Button); ok && (ev.Type == gpucontext.PointerDown || ev.Type == gpucontext.PointerUp) {
				action := control.ActionRelease
				if ev.Type == gpucontext.PointerDown {
					action = control.ActionPress
				}
				e.AddClickEvent(int(ev.X), int(ev.Y), b, action)
			}
		case gpucontext.ScrollEvent:
			scroll = geom.NewVec(scroll.X+ev.DeltaX, scroll.Y+ev.DeltaY)
		}
	}
	if a.captured {
		e.CursorDelta = moved
	} else {
		e.CursorDelta = geom.NewVec(pos.X-e.MousePos.X, pos.Y-e.MousePos.Y)
		e.MousePos = pos
	}
	e.ScrollDelta = -scroll.Y
	kb, mouse := a.app.Input().Keyboard(), a.app.Input().Mouse()
	e.Modifiers.Shift = kb.Pressed(input.KeyShiftLeft) || kb.Pressed(input.KeyShiftRight)
	e.Modifiers.Ctrl = kb.Pressed(input.KeyControlLeft) || kb.Pressed(input.KeyControlRight)
	e.Modifiers.Alt = kb.Pressed(input.KeyAltLeft) || kb.Pressed(input.KeyAltRight)
	e.MiddleDown = mouse.Pressed(input.MouseButtonMiddle)
	e.WindowFillsScreen = a.app.IsFullscreen() || a.app.IsMaximized()
}

// capture catches the window's cursor, or lets it go.
func (a *DesktopAdapter) capture(on bool) {
	if a.app == nil || a.captured == on {
		return
	}
	a.captured = on
	if on {
		a.app.SetCursorMode(gpucontext.CursorModeLocked)
		return
	}
	a.app.SetCursorMode(gpucontext.CursorModeNormal)
	// gogpu on Wayland leaves the cursor hidden until it re-enters the window and skips a shape it
	// holds already: another shape and back shows it again
	a.app.SetCursor(gpucontext.CursorPointer)
	a.app.SetCursor(gpucontext.CursorDefault)
}

func buttonOf(b gpucontext.Button) (control.MouseButton, bool) {
	switch b {
	case gpucontext.ButtonLeft:
		return control.MouseButtonLeft, true
	case gpucontext.ButtonRight:
		return control.MouseButtonRight, true
	case gpucontext.ButtonMiddle:
		return control.MouseButtonMiddle, true
	}
	return 0, false
}

// keyOf is control's key for the window's; KeyUnknown for one control has not.
func keyOf(k gpucontext.Key) control.Key {
	switch {
	case k >= gpucontext.KeyA && k <= gpucontext.KeyZ:
		return control.KeyA + control.Key(k-gpucontext.KeyA)
	case k >= gpucontext.Key0 && k <= gpucontext.Key9:
		return control.Key0 + control.Key(k-gpucontext.Key0)
	case k >= gpucontext.KeyF1 && k <= gpucontext.KeyF12:
		return control.KeyF1 + control.Key(k-gpucontext.KeyF1)
	case k >= gpucontext.KeyNumpad0 && k <= gpucontext.KeyNumpad9:
		return control.KeyNumpad0 + control.Key(k-gpucontext.KeyNumpad0)
	}
	return keys[k]
}

var keys = map[gpucontext.Key]control.Key{
	gpucontext.KeyEscape: control.KeyEscape, gpucontext.KeyEnter: control.KeyEnter, gpucontext.KeySpace: control.KeySpace,
	gpucontext.KeyTab: control.KeyTab, gpucontext.KeyBackspace: control.KeyBackspace, gpucontext.KeyDelete: control.KeyDelete,
	gpucontext.KeyInsert: control.KeyInsert, gpucontext.KeyHome: control.KeyHome, gpucontext.KeyEnd: control.KeyEnd,
	gpucontext.KeyPageUp: control.KeyPageUp, gpucontext.KeyPageDown: control.KeyPageDown,
	gpucontext.KeyUp: control.KeyArrowUp, gpucontext.KeyDown: control.KeyArrowDown, gpucontext.KeyLeft: control.KeyArrowLeft, gpucontext.KeyRight: control.KeyArrowRight,
	gpucontext.KeyLeftShift: control.KeyShift, gpucontext.KeyRightShift: control.KeyShift,
	gpucontext.KeyLeftControl: control.KeyControl, gpucontext.KeyRightControl: control.KeyControl,
	gpucontext.KeyLeftAlt: control.KeyAlt, gpucontext.KeyRightAlt: control.KeyAlt,
	gpucontext.KeyMinus: control.KeyMinus, gpucontext.KeyEqual: control.KeyEqual,
	gpucontext.KeyLeftBracket: control.KeyBracketLeft, gpucontext.KeyRightBracket: control.KeyBracketRight,
	gpucontext.KeyBackslash: control.KeyBackslash, gpucontext.KeySemicolon: control.KeySemicolon, gpucontext.KeyApostrophe: control.KeyQuote,
	gpucontext.KeyGrave: control.KeyBackquote, gpucontext.KeyComma: control.KeyComma, gpucontext.KeyPeriod: control.KeyPeriod,
	gpucontext.KeySlash: control.KeySlash, gpucontext.KeyCapsLock: control.KeyCapsLock,
	gpucontext.KeyNumpadAdd: control.KeyNumpadAdd, gpucontext.KeyNumpadSubtract: control.KeyNumpadSubtract,
	gpucontext.KeyNumpadMultiply: control.KeyNumpadMultiply, gpucontext.KeyNumpadDivide: control.KeyNumpadDivide,
	gpucontext.KeyNumpadDecimal: control.KeyNumpadDecimal, gpucontext.KeyNumpadEnter: control.KeyNumpadEnter,
}
