package engine

import (
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

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
// nothing.
type DesktopAdapter struct {
	app     *gogpu.App
	pointer pointer

	mu      sync.Mutex
	pending []gpucontext.InputEvent
	spare   []gpucontext.InputEvent // the array drained last, for the next Collect to fill

	// the switches of full screen asked for, performed by Collect on the window's thread: AppKit
	// takes them on no other; start is the one asked for as the game starts (GRAM_FULLSCREEN), a
	// wish the window's config does not carry everywhere
	toggles atomic.Int32
	start   bool
	frames  int

	logged time.Time // when the input was last logged (GRAM_INPUT_LOG)
	moved  geom.Vec  // the cursor's movement since
}

var logInput = os.Getenv("GRAM_INPUT_LOG") != ""

// Collect keeps the events of the window's pass for the next Capture, which the window would
// forget at its next pass, and switches full screen where asked.
func (a *DesktopAdapter) Collect() {
	a.switchFullscreen()
	a.mu.Lock()
	defer a.mu.Unlock()
	for ev, ok := a.app.PollInputEvent(); ok; ev, ok = a.app.PollInputEvent() {
		a.pending = append(a.pending, ev)
	}
}

// ToggleFullscreen asks for the window switched to full screen, or back, at the window's next
// pass.
func (a *DesktopAdapter) ToggleFullscreen() { a.toggles.Add(1) }

// the window asked for full screen as the game starts is asked again every startRetry frames,
// startTries times, until it says it is
const (
	startRetry = 120
	startTries = 3
)

func (a *DesktopAdapter) switchFullscreen() {
	if a.start {
		switch {
		case a.app.IsFullscreen(), a.frames >= startRetry*startTries:
			a.start = false
		case a.frames%startRetry == 0:
			a.app.SetFullscreen(true)
		}
		a.frames++
	}
	if a.toggles.Swap(0)%2 == 1 {
		a.app.ToggleFullscreen()
	}
}

func (a *DesktopAdapter) Capture(e *control.InputEvents) {
	e.CursorDelta = geom.Vec{}
	e.ScrollDelta = 0
	if a.app == nil {
		return
	}
	fills := a.app.IsFullscreen() || a.app.IsMaximized()
	w, h := a.app.Size()
	var scroll geom.Vec
	a.pointer.begin()
	a.drain(func(ev gpucontext.InputEvent) {
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
			at := a.pointer.take(ev, w, h, fills)
			if b, ok := buttonOf(ev.Button); ok && (ev.Type == gpucontext.PointerDown || ev.Type == gpucontext.PointerUp) {
				action := control.ActionRelease
				if ev.Type == gpucontext.PointerDown {
					action = control.ActionPress
				}
				e.AddClickEvent(int(at.X), int(at.Y), b, action)
			}
		case gpucontext.ScrollEvent:
			scroll = geom.NewVec(scroll.X+float32(ev.DeltaX), scroll.Y+float32(ev.DeltaY))
		}
	})
	a.pointer.apply(e)
	e.ScrollDelta = -scroll.Y
	kb, mouse := a.app.Input().Keyboard(), a.app.Input().Mouse()
	e.Modifiers.Shift = kb.Pressed(input.KeyShiftLeft) || kb.Pressed(input.KeyShiftRight)
	e.Modifiers.Ctrl = kb.Pressed(input.KeyControlLeft) || kb.Pressed(input.KeyControlRight)
	e.Modifiers.Alt = kb.Pressed(input.KeyAltLeft) || kb.Pressed(input.KeyAltRight)
	e.MiddleDown = mouse.Pressed(input.MouseButtonMiddle)
	if logInput {
		a.log(e, w, h, fills)
	}
}

// log writes every click as it comes and, once a second, where the cursor is in what window
func (a *DesktopAdapter) log(e *control.InputEvents, w, h int, fills bool) {
	for _, c := range e.ClickQueue {
		action := "released"
		if c.Action == control.ActionPress {
			action = "pressed"
		}
		log.Printf("gram: input: button %d %s at (%.0f, %.0f)", c.Button, action, c.Pos.X, c.Pos.Y)
	}
	a.moved = a.moved.Add(e.CursorDelta)
	if time.Since(a.logged) < time.Second {
		return
	}
	fw, fh := a.app.PhysicalSize()
	log.Printf("gram: input: the cursor at (%.0f, %.0f) in a window %dx%d (framebuffer %dx%d, "+
		"scale %.2f), fullscreen %v, maximized %v, fills the screen %v, in the window %v, "+
		"caught %v (the window gives deltas %v), moved (%.0f, %.0f) since the last line",
		e.MousePos.X, e.MousePos.Y, w, h, fw, fh, a.app.ScaleFactor(), a.app.IsFullscreen(),
		a.app.IsMaximized(), fills, a.pointer.in, a.pointer.captured, a.pointer.locks, a.moved.X,
		a.moved.Y)
	a.logged, a.moved = time.Now(), geom.Vec{}
}

// drain hands consume every event Collect kept, then keeps their array for Collect to fill
// again: the two arrays swap under the lock, so a frame allocates none.
func (a *DesktopAdapter) drain(consume func(gpucontext.InputEvent)) {
	a.mu.Lock()
	events := a.pending
	a.pending = a.spare[:0]
	a.mu.Unlock()
	for _, ev := range events {
		consume(ev)
	}
	a.spare = events
}

func (a *DesktopAdapter) capture(on bool) {
	if a.app == nil || a.pointer.captured == on {
		return
	}
	a.pointer.catch(on)
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

// pointer folds a pass's pointer events into the frame's cursor: whether the window has it at
// all, its place while free; caught, the deltas the window gives, or, where it gives none (macOS:
// gogpu's lock is a stub there, the cursor stays visible and stops at the screen's edge), the
// movement of its place
type pointer struct {
	in       bool // the window has the cursor: an event of it came and it has not left since
	captured bool
	locks    bool     // the window has given a delta while caught: it locks the cursor for real
	known    bool     // abs is a place of this capture, a movement measured from it
	abs      geom.Vec // the last place the window reported, caught
	last     geom.Vec // the last place reported this pass
	summed   geom.Vec // the deltas of this pass
	moved    bool     // a pointer event came this pass
	told     bool     // the fallback's word given once
}

// catch has the pointer caught, or let go: the place it is caught at is no movement
func (p *pointer) catch(on bool) { p.captured, p.known = on, false }

func (p *pointer) begin() { p.summed, p.moved = geom.Vec{}, false }

// take folds one event in and gives its place, pinned to the last row and column where the window
// fills the screen: Cocoa's bottom-left origin flipped puts a cursor pinned on the last row at h.
// A leave's place is where the cursor was last seen
func (p *pointer) take(ev gpucontext.PointerEvent, w, h int, fills bool) geom.Vec {
	at := geom.NewVec(float32(ev.X), float32(ev.Y))
	if fills && w > 0 && h > 0 {
		at = geom.NewVec(min(max(at.X, 0), float32(w-1)), min(max(at.Y, 0), float32(h-1)))
	}
	switch ev.Type {
	case gpucontext.PointerLeave:
		p.in = false
	case gpucontext.PointerEnter, gpucontext.PointerMove, gpucontext.PointerDown,
		gpucontext.PointerUp:
		p.in = true
	}
	p.summed = geom.NewVec(p.summed.X+float32(ev.DeltaX), p.summed.Y+float32(ev.DeltaY))
	p.last, p.moved = at, true
	return at
}

// apply writes into e whether the window has the cursor and the pass's movement: free, the
// cursor's new place and the way to it; caught, the deltas, the place staying put
func (p *pointer) apply(e *control.InputEvents) {
	e.CursorInWindow = p.in
	e.CursorDelta = geom.Vec{}
	if !p.moved {
		return
	}
	if !p.captured {
		e.CursorDelta = p.last.Sub(e.MousePos)
		e.MousePos = p.last
		return
	}
	switch {
	case p.summed != (geom.Vec{}):
		p.locks = true
		e.CursorDelta = p.summed
	case !p.locks && p.known:
		e.CursorDelta = p.last.Sub(p.abs)
		if !p.told {
			p.told = true
			log.Print("gram: the window gives no mouse deltas while the cursor is caught: the " +
				"look follows the cursor's place, as far as the screen lets it")
		}
	}
	p.abs, p.known = p.last, true
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
