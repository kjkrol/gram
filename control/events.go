package control

import (
	"github.com/kjkrol/aabbworld/geom"
)

type KeyAction int

const (
	ActionPress KeyAction = iota
	ActionRelease
)

type KeyEvent struct {
	Key    Key
	Action KeyAction
}

type ClickEvent struct {
	Pos    geom.Vec
	Button MouseButton
	Action KeyAction
}

type InputEvents struct {
	MousePos    geom.Vec
	CursorDelta geom.Vec
	Modifiers   struct {
		Shift, Ctrl, Alt bool
	}
	MiddleDown bool
	// CursorInWindow reports whether the window has the cursor: it came in and has not left
	// since. MousePos is where it was last seen, zero before any.
	CursorInWindow bool

	ClickQueue  []ClickEvent
	KeyEvents   []KeyEvent
	ScrollDelta float32
}

func (e *InputEvents) ResetTransient() {
	e.ClickQueue = e.ClickQueue[:0]
	e.KeyEvents = e.KeyEvents[:0]
	e.ScrollDelta = 0
	e.CursorDelta = geom.Vec{}
}

func (e *InputEvents) AddKeyEvent(key Key, action KeyAction) {
	e.KeyEvents = append(e.KeyEvents, KeyEvent{Key: key, Action: action})
}

func (e *InputEvents) AddClickEvent(x, y int, button MouseButton, action KeyAction) {
	e.ClickQueue = append(e.ClickQueue, ClickEvent{
		Pos:    geom.NewVec(float32(x), float32(y)),
		Button: button,
		Action: action,
	})
}

var cursorCapture func(on bool)

// SetCursorCapture is the engine's: how the window catches its cursor and lets it go.
func SetCursorCapture(fn func(on bool)) { cursorCapture = fn }

// CaptureCursor catches the window's cursor — hidden, the mouse moving without end — or shows it
// again; nothing without a window.
func CaptureCursor(on bool) {
	if cursorCapture != nil {
		cursorCapture(on)
	}
}

var fullscreenKey = [2]string{"F11", "Full screen (the engine's)"}

// SetFullscreenKey is the engine's: the key that switches full screen on this platform and what
// the controls screen says of it, where the system keeps F11 for itself.
func SetFullscreenKey(key, label string) { fullscreenKey = [2]string{key, label} }

// FullscreenKey is the key that switches full screen and its label for the controls screen:
// F11, unless the engine said otherwise.
func FullscreenKey() (key, label string) { return fullscreenKey[0], fullscreenKey[1] }
